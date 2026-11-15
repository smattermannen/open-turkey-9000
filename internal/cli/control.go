// control.go — Block control commands: start, stop, unlock, status.
//
// Lifecycle:
//
//	kc9k start <block>   → Activate a block (start blocking)
//	kc9k stop <block>    → Deactivate a block (stop blocking)
//	kc9k unlock <block>  → Unlock a char-locked block via the challenge
//	kc9k status          → Show the status of every block
//
// WAYS A BLOCK CAN BE "HELD" (not freely removable)
// -------------------------------------------------
//   - Character lock (--lock): removable only by completing the typing
//     challenge via "unlock".
//   - Time lock (--for / --until): cannot be removed AT ALL until the time
//     passes — not even with the challenge. The strongest lock.
//   - Schedule lock (kc9k schedule lock): on while a window is open or a
//     place rule has the block on; for edits (not stop/unlock) also for the
//     lead time the user chose before each window. Without it a schedule
//     protects nothing (R-LOCK-10).
//
// Locks can be strengthened while held ("start" on an active block adds or
// lengthens a char lock and adds or extends a time lock), never weakened. A
// lock added to a block its schedule activated outlives the schedule: the
// daemon won't deactivate a locked block (see enforce.reconcileSchedules).
//
// All of these are enforced centrally by blockHold so start/stop/unlock/edit and
// uninstall agree on when a block may be removed.
package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/smattermannen/kc9k/internal/lock"
	"github.com/spf13/cobra"
)

// ============================================================================
// startCmd — Activate a block
// ============================================================================

var startCmd = &cobra.Command{
	Use:   "start [block]",
	Short: "Activate a block",
	Long: `Activate a block, applying every blocking layer.

Locks (optional, and combinable):
  --lock            Require the typing challenge ('unlock') to deactivate.
  --lock-chars N    Challenge length in characters (default 300, 10 to 5000).
  --for DURATION    Timer lock: cannot be removed for this long (e.g. 2h, 90m, 1h30m).
  --until HH:MM     Timer lock until a clock time today (or tomorrow if already past).

A timer lock cannot be lifted early by anyone short of root — that is the point.

On a block that is already active, start only strengthens its locks: --lock
adds a text lock (or lengthens it with a larger --lock-chars), and --for or
--until adds a timer lock or extends it to a later time. Locks can never be
shortened or removed this way.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		blockName := args[0]
		if database, err := openDB(); err == nil {
			of := draftOf(database, blockName)
			database.Close()
			if of != "" {
				return fmt.Errorf("'%s' is a draft of '%s': save it with 'kc9k block draft %s save'", blockName, of, of)
			}
		}

		// A lock may need the settings password (the user's choice), asked
		// before any lock password.
		if startLockRequested(cmd) {
			database, err := openDB()
			if err != nil {
				return err
			}
			err = requireLockPassword(database)
			database.Close()
			if err != nil {
				return err
			}
		}

		usePassword, _ := cmd.Flags().GetBool("password-stdin")
		useRestart, _ := cmd.Flags().GetBool("restart")
		delaySpec, _ := cmd.Flags().GetString("delay")
		var extras db.LockExtras
		if delaySpec != "" {
			d, err := parseUserDuration(delaySpec)
			if err != nil {
				return fmt.Errorf("invalid --delay duration %q: %w", delaySpec, err)
			}
			if d < time.Minute || d > 40*24*time.Hour {
				return fmt.Errorf("--delay must be between 1 minute and 40 days")
			}
			extras.Delay = int64(d.Seconds())
		}
		if useRestart {
			boot, err := db.BootID()
			if err != nil {
				return err
			}
			extras.RestartBoot = boot
		}
		if usePassword {
			password, err := readPasswordStdin("Block password")
			if err != nil {
				return err
			}
			if extras.PasswordHash, err = db.HashPassword(password); err != nil {
				return err
			}
		}
		useLock, style, lockChars, err := textLockRequested(cmd)
		if err != nil {
			return err
		}
		extras.Style = style
		if spec, _ := cmd.Flags().GetString("relock"); spec != "" {
			d, err := parseUserDuration(spec)
			if err != nil || d < time.Minute || d > 30*24*time.Hour {
				return fmt.Errorf("invalid --relock %q (1m to 720h)", spec)
			}
			if !useLock && !usePassword {
				return fmt.Errorf("--relock goes with a text or password lock")
			}
			extras.Relock = int64(d / time.Second)
		}
		if spec, _ := cmd.Flags().GetString("range"); spec != "" {
			if extras.RangeDays, extras.RangeFrom, extras.RangeTo, err = parseRange(spec); err != nil {
				return err
			}
			extras.RangeInvert, _ = cmd.Flags().GetBool("range-invert")
		} else if cmd.Flags().Changed("range-invert") {
			return fmt.Errorf("--range-invert goes with --range")
		}
		hasExtras := usePassword || useRestart || delaySpec != "" || style != "" || extras.Relock > 0 || extras.RangeDays != 0

		forDur, err := cmd.Flags().GetString("for")
		if err != nil {
			return err
		}
		untilClock, err := cmd.Flags().GetString("until")
		if err != nil {
			return err
		}

		if forDur != "" && untilClock != "" {
			return fmt.Errorf("use only one of --for or --until")
		}

		// Compute the time-lock expiry (0 = none).
		var lockedUntil, forSeconds int64
		now := time.Now()
		switch {
		case forDur != "":
			d, err := parseUserDuration(forDur)
			if err != nil {
				return fmt.Errorf("invalid --for duration %q: %w", forDur, err)
			}
			if d <= 0 {
				return fmt.Errorf("--for duration must be positive")
			}
			lockedUntil = now.Add(d).Unix()
			forSeconds = int64(d / time.Second)
		case untilClock != "":
			t, err := untilToday(now, untilClock)
			if err != nil {
				return err
			}
			lockedUntil = t.Unix()
		}

		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		trusted, err := database.TrustedNow(now)
		if err != nil {
			return err
		}
		if lockedUntil != 0 {
			lockedUntil += trusted - now.Unix()
		}

		detail, err := database.GetBlock(blockName)
		if err != nil {
			return err
		}
		if blockEmpty(detail) {
			return fmt.Errorf("block '%s' has nothing to enforce. Add websites to a list, apps or window titles first", blockName)
		}
		if detail.Active && hasExtras {
			return fmt.Errorf("block '%s' is already active; password, restart, delay and range locks, and a text lock of words or your own text, are set when it starts", blockName)
		}
		if detail.Active {
			return strengthenLocks(cmd, database, detail, useLock, lockChars, cmd.Flags().Changed("lock-chars"), lockedUntil, forSeconds, trusted)
		}

		chars := 0
		if useLock {
			chars = lockChars
		}
		// Layered locks (R-LOCK-6): unlocked in the order given. A timer
		// lock for a duration, or a restart lock, behind another lock
		// waits for it.
		kinds := lockKindsInOrder(cmd)
		if len(kinds) > 1 {
			extras.Order = strings.Join(kinds, ",")
			if kinds[0] != db.LayerTimer && forSeconds > 0 {
				extras.TimerAfter, lockedUntil = forSeconds, 0
			}
			if kinds[0] != db.LayerRestart && useRestart {
				extras.RestartBoot = db.RestartPending
			}
			hasExtras = true
		}
		// R-LOCK-12: nothing is locked until the user confirms.
		if err := confirmLocks(cmd, startLocks(blockName, kinds, style, chars, lockedUntil, trusted, extras)...); err != nil {
			return err
		}
		if err := database.ActivateBlock(blockName, useLock, chars, lockedUntil, false); err != nil {
			return err
		}
		if hasExtras {
			if err := database.SetLockExtras(blockName, extras); err != nil {
				return err
			}
		}

		if err := enforce.ApplyFromDB(database); err != nil {
			return fmt.Errorf("failed to apply blocking layers: %w", err)
		}

		msg := fmt.Sprintf("Block '%s' activated successfully", blockName)
		if enforce.HasScheduleOrPlace(database, blockName) {
			// §3.3: active, enabled only as its windows and places say.
			if enforce.LiftedBySchedule(database, blockName, now) {
				msg += " [lifted now: enabled only in its windows or at its places]"
			} else {
				msg += " [enabled now by its schedule]"
			}
		}
		if useLock {
			msg += " [text lock: " + lock.DescribeStyle(style, chars) + "]"
		}
		if lockedUntil > 0 {
			msg += fmt.Sprintf(" [timer lock until %s]", time.Unix(lockedUntil, 0).Format("2006-01-02 15:04"))
		}
		fmt.Println(msg)
		notify("Killer Chicken 9000", fmt.Sprintf("Block '%s' is now active", blockName))
		return nil
	},
}

// startLocks lists the locks a start of an inactive block sets, in unlock
// order (kinds), for confirmLocks (R-LOCK-12).
func startLocks(name string, kinds []string, style string, chars int, until, trusted int64, extras db.LockExtras) []lockToSet {
	target := "block '" + name + "'"
	relock := ""
	if extras.Relock > 0 {
		relock = fmt.Sprintf(", re-lock %s after unlocking", humanDuration(time.Duration(extras.Relock)*time.Second))
	}
	var locks []lockToSet
	for i, k := range kinds {
		order := ""
		if len(kinds) > 1 {
			order = fmt.Sprintf("%d of %d: ", i+1, len(kinds))
		}
		after := ""
		if i > 0 {
			after = "after the " + db.LayerName(kinds[i-1]) + " is unlocked"
		}
		switch k {
		case db.LayerTimer:
			length := timerLength(trusted, until)
			if extras.TimerAfter > 0 {
				length = fmt.Sprintf("%s, counting from when the %s is unlocked", humanDuration(time.Duration(extras.TimerAfter)*time.Second), db.LayerName(kinds[i-1]))
			}
			locks = append(locks, lockToSet{target, order + "timer lock", length})
		case db.LayerText:
			locks = append(locks, lockToSet{target, order + textLockKind(style, chars) + relock, after})
		case db.LayerPassword:
			locks = append(locks, lockToSet{target, order + "password lock" + relock, after})
		case db.LayerRestart:
			length := "until the computer restarts"
			if extras.RestartBoot == db.RestartPending {
				length = "until a restart " + after
			}
			locks = append(locks, lockToSet{target, order + "restart lock", length})
		case db.LayerRange:
			locks = append(locks, lockToSet{target, order + "range lock", describeRange(extras) + " (can't be unlocked then)"})
		case db.LayerDelay:
			length := fmt.Sprintf("unlocking takes %s", humanDuration(time.Duration(extras.Delay)*time.Second))
			if after != "" {
				length += ", " + after
			}
			locks = append(locks, lockToSet{target, order + "delay lock", length})
		}
	}
	return locks
}

// startLockRequested reports whether a start asks for any kind of lock.
func startLockRequested(cmd *cobra.Command) bool {
	for _, f := range []string{"lock", "lock-chars", "for", "until", "password-stdin", "restart", "delay", "text-lock", "lock-words", "lock-text", "relock", "range", "range-invert"} {
		if cmd.Flags().Changed(f) {
			return true
		}
	}
	return false
}

// strengthenLocks handles "start" on a block that is already active: it can add
// or lengthen a char lock and add or extend a time lock, but refuses anything
// that would weaken the current locks. This is safe on a held block, since it
// can only make the hold stronger. New locks go last (R-LOCK-7): a timer
// lock for a duration behind locks still on counts from their unlocking.
func strengthenLocks(cmd *cobra.Command, database *db.DB, detail *db.BlockDetail, useLock bool, lockChars int, charsGiven bool, lockedUntil, forSeconds, trusted int64) error {
	name := detail.Name
	if !useLock && lockedUntil == 0 {
		return fmt.Errorf("block '%s' is already active (use --lock, --for or --until to strengthen its locks)", name)
	}
	bootID, err := db.BootID()
	if err != nil {
		return err
	}
	e, err := database.GetLockExtras(name)
	if err != nil {
		return err
	}
	db.AdvanceLayers(detail, &e, trusted, bootID)
	// Locks that are gone leave the order; a new one goes last.
	if detail.LockedUntil != 0 && detail.LockedUntil <= trusted && e.TimerAfter == 0 {
		detail.LockedUntil = 0
	}
	if e.RestartBoot != "" && !db.LayerOn(db.LayerRestart, detail, e, trusted, bootID) {
		e.RestartBoot = ""
	}
	order := db.Layers(detail, e)
	othersOn := false
	for _, k := range db.LayersOn(detail, e, trusted, bootID) {
		othersOn = othersOn || k != db.LayerTimer
	}

	var changes []string
	var confirm []lockToSet
	target := "block '" + name + "'"
	if useLock {
		switch {
		case !detail.Locked:
			detail.Locked, detail.LockChars = true, lockChars
			order = db.AppendLayers(order, db.LayerText)
			changes = append(changes, fmt.Sprintf("text lock of %d chars", lockChars))
			confirm = append(confirm, lockToSet{target, textLockKind("", lockChars), ""})
		case lockChars > detail.LockChars && charsGiven:
			detail.LockChars = lockChars
			changes = append(changes, fmt.Sprintf("challenge lengthened to %d chars", lockChars))
			confirm = append(confirm, lockToSet{target, textLockKind("", lockChars), "lengthened"})
		case lockChars < detail.LockChars && charsGiven:
			return fmt.Errorf("block '%s' is already locked with a %d-character challenge; it can't be shortened", name, detail.LockChars)
		}
	}
	if lockedUntil != 0 {
		switch {
		case e.TimerAfter > 0:
			// Waiting behind other locks: only a longer duration.
			if forSeconds == 0 {
				return fmt.Errorf("block '%s' has a timer lock that counts from unlocking the locks before it; extend it with --for", name)
			}
			if forSeconds <= e.TimerAfter {
				return fmt.Errorf("block '%s' already has a timer lock of %s after the locks before it; a timer lock can only be extended", name, time.Duration(e.TimerAfter)*time.Second)
			}
			e.TimerAfter = forSeconds
			changes = append(changes, fmt.Sprintf("timer lock of %s after the locks before it", time.Duration(forSeconds)*time.Second))
			confirm = append(confirm, lockToSet{target, "timer lock", fmt.Sprintf("%s, counting from when the locks before it are unlocked", humanDuration(time.Duration(forSeconds)*time.Second))})
		case detail.LockedUntil != 0:
			if lockedUntil <= detail.LockedUntil {
				return fmt.Errorf("block '%s' is already timer-locked until %s; a timer lock can only be extended", name, time.Unix(detail.LockedUntil, 0).Format("2006-01-02 15:04"))
			}
			detail.LockedUntil = lockedUntil
			changes = append(changes, fmt.Sprintf("timer lock until %s", time.Unix(lockedUntil, 0).Format("2006-01-02 15:04")))
			confirm = append(confirm, lockToSet{target, "timer lock", timerLength(trusted, lockedUntil)})
		case othersOn && forSeconds > 0:
			e.TimerAfter = forSeconds
			order = db.AppendLayers(order, db.LayerTimer)
			changes = append(changes, fmt.Sprintf("timer lock of %s after the locks before it", time.Duration(forSeconds)*time.Second))
			confirm = append(confirm, lockToSet{target, "timer lock", fmt.Sprintf("%s, counting from when the locks before it are unlocked", humanDuration(time.Duration(forSeconds)*time.Second))})
		default:
			detail.LockedUntil = lockedUntil
			order = db.AppendLayers(order, db.LayerTimer)
			changes = append(changes, fmt.Sprintf("timer lock until %s", time.Unix(lockedUntil, 0).Format("2006-01-02 15:04")))
			confirm = append(confirm, lockToSet{target, "timer lock", timerLength(trusted, lockedUntil)})
		}
	}
	if len(changes) == 0 {
		return fmt.Errorf("block '%s' is already active with those locks", name)
	}
	if err := confirmLocks(cmd, confirm...); err != nil {
		return err
	}
	e.Order = strings.Join(order, ",")
	if err := database.SaveLayers(detail, e); err != nil {
		return err
	}
	fmt.Printf("Block '%s' strengthened: %s\n", name, strings.Join(changes, ", "))
	if layers := db.Layers(detail, e); len(layers) > 1 {
		fmt.Printf("Unlock order: %s\n", describeLayers(layers, detail, e))
	}
	return nil
}

// ============================================================================
// stopCmd — Deactivate a block
// ============================================================================

var stopCmd = &cobra.Command{
	Use:   "stop [block]",
	Short: "Deactivate a block",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		blockName := args[0]

		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()
		if err := requireSettingsPassword(database); err != nil {
			return err
		}

		hold, err := blockHold(database, blockName, time.Now())
		if err != nil {
			return err
		}
		if !hold.active {
			return fmt.Errorf("block '%s' is not active", blockName)
		}
		if hold.quota {
			return fmt.Errorf("block '%s' has a daily allowance and stays on. Use 'kc9k allowance start %s'", blockName, blockName)
		}
		if hold.scheduledNow {
			return refused(database, blockName, fmt.Errorf("block '%s' is in a scheduled window and can't be stopped until it closes", blockName))
		}
		if hold.timeLocked() {
			return refused(database, blockName, fmt.Errorf("block '%s' is timer-locked until %s and can't be stopped before then", blockName, hold.until()))
		}
		if hold.restartLocked {
			return refused(database, blockName, fmt.Errorf("block '%s' stays on until the computer restarts", blockName))
		}
		if hold.charLocked || hold.passwordHash != "" || hold.extras.Delay > 0 {
			return refused(database, blockName, fmt.Errorf("block '%s' is locked. Use 'kc9k unlock %s' first", blockName, blockName))
		}
		if len(hold.layers) > 0 { // a range lock, or any other still on
			return refused(database, blockName, fmt.Errorf("block '%s' is locked (%s)", blockName, db.LayerName(hold.layers[0])))
		}

		if err := database.DeactivateBlock(blockName); err != nil {
			return err
		}
		// Inactive: never enforces, whatever its schedule (R-SCHED-1).
		if err := enforce.ApplyFromDB(database); err != nil {
			return err
		}
		fmt.Printf("Block '%s' deactivated successfully\n", blockName)
		notify("Killer Chicken 9000", fmt.Sprintf("Block '%s' deactivated", blockName))
		return nil
	},
}

// ============================================================================
// unlockCmd — Unlock a char-locked block via the typing challenge
// ============================================================================

var unlockCmd = &cobra.Command{
	Use:   "unlock [block]",
	Short: "Unlock a block: its text or password lock is lifted, its delay lock starts counting down; the block stays on",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		blockName := args[0]

		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()
		if err := requireSettingsPassword(database); err != nil {
			return err
		}

		hold, err := blockHold(database, blockName, time.Now())
		if err != nil {
			return err
		}
		if !hold.active {
			return fmt.Errorf("block '%s' is not active", blockName)
		}
		if hold.scheduledNow {
			return refused(database, blockName, fmt.Errorf("block '%s' is in a scheduled window; it will lift automatically when the window closes", blockName))
		}
		if len(hold.layers) == 0 {
			return fmt.Errorf("block '%s' is not locked. Use 'kc9k stop' to switch it off", blockName)
		}
		// R-LOCK-6: only the first lock still on is unlocked.
		switch hold.layers[0] {
		case db.LayerTimer:
			return refused(database, blockName, fmt.Errorf("block '%s' is timer-locked until %s; it cannot be unlocked before then", blockName, hold.until()))
		case db.LayerRestart:
			return refused(database, blockName, fmt.Errorf("block '%s' stays on until the computer restarts", blockName))
		case db.LayerRange:
			return refused(database, blockName, fmt.Errorf("block '%s' has a range lock (%s) that is on now; it can't be unlocked, only waited out", blockName, describeRange(hold.extras)))
		case db.LayerPassword:
			password, err := readPasswordStdin("Block password")
			if err != nil {
				return err
			}
			if !db.CheckPassword(hold.passwordHash, password) {
				return refused(database, blockName, fmt.Errorf("wrong password; block '%s' remains locked", blockName))
			}
		case db.LayerText:
			success, err := lock.RunStyled(hold.extras.Style, hold.lockChars)
			if err != nil {
				return fmt.Errorf("failed to run unlock challenge: %w", err)
			}
			if !success {
				_ = refused(database, blockName, fmt.Errorf("failed the unlock challenge"))
				fmt.Println("Unlock failed. The block remains active and locked.")
				os.Exit(1)
			}
		case db.LayerDelay:
			extras, err := database.GetLockExtras(blockName)
			if err != nil {
				return err
			}
			if extras.DelayUntil == 0 {
				extras.DelayUntil = hold.trusted + extras.Delay
				if err := database.SetLockExtras(blockName, extras); err != nil {
					return err
				}
			}
			fmt.Printf("Block '%s': its delay lock is unlocked in %s. The block stays on.\n",
				blockName, time.Duration(extras.DelayUntil-hold.trusted)*time.Second)
			printNextLayer(blockName, hold.layers[1:])
			return nil
		}

		// With break text set, unlocking is a break: the block is lifted
		// for a while and comes back with its locks. Unlocking again during
		// that break lifts the lock.
		if breaks, err := database.GetBreaks(blockName); err != nil {
			return err
		} else if breaks.Relock > 0 {
			if until, _ := database.PausedUntil(blockName); until <= hold.trusted {
				if err := database.SetPause(blockName, hold.trusted+breaks.Relock); err != nil {
					return err
				}
				if err := enforce.ApplyFromDB(database); err != nil {
					return err
				}
				fmt.Printf("Block '%s' is lifted for %s, then it is back with its locks. Unlock it again before then to end it.\n",
					blockName, seconds(breaks.Relock))
				return nil
			}
		}

		// §3.4: the lock is lifted, the block stays on. With re-lock it is
		// only lifted for a while, then on again.
		detail, err := database.GetBlock(blockName)
		if err != nil {
			return err
		}
		extras, err := database.GetLockExtras(blockName)
		if err != nil {
			return err
		}
		bootID, err := db.BootID()
		if err != nil {
			return err
		}
		if err := liftTextLayer(database, detail, &extras, hold.layers[0], hold.trusted, bootID); err != nil {
			return err
		}
		rest := db.LayersOn(detail, extras, hold.trusted, bootID)
		if extras.Relock > 0 && extras.LiftedUntil > hold.trusted {
			fmt.Printf("Block '%s': its %s is unlocked; its text and password locks are on again in %s (re-lock). 'kc9k stop %s' switches it off meanwhile.\n",
				blockName, db.LayerName(hold.layers[0]), time.Duration(extras.Relock)*time.Second, blockName)
		} else {
			fmt.Printf("Block '%s': its %s is unlocked.\n", blockName, db.LayerName(hold.layers[0]))
		}
		if len(rest) == 0 {
			fmt.Printf("Block '%s' is unlocked and stays on. 'kc9k stop %s' switches it off.\n", blockName, blockName)
		} else {
			printNextLayerAt(blockName, rest, detail, hold.trusted)
		}
		return nil
	},
}

// printNextLayer says which lock comes next after one being unlocked.
func printNextLayer(name string, rest []string) {
	if len(rest) > 0 {
		fmt.Printf("Next: its %s ('kc9k unlock %s' once the delay is over).\n", db.LayerName(rest[0]), name)
	}
}

// printNextLayerAt says which lock holds the block now.
func printNextLayerAt(name string, rest []string, detail *db.BlockDetail, trusted int64) {
	switch rest[0] {
	case db.LayerTimer:
		fmt.Printf("Next: its timer lock, until %s.\n", time.Unix(detail.LockedUntil, 0).Format("2006-01-02 15:04"))
	case db.LayerRestart:
		fmt.Println("Next: its restart lock, until the computer restarts.")
	default:
		fmt.Printf("Next: its %s ('kc9k unlock %s').\n", db.LayerName(rest[0]), name)
	}
}

// ============================================================================
// statusCmd — Show the status of every block
// ============================================================================

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of all blocks",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		// Monk mode is no block (R-MONK-1), so it has its own line.
		if line := monkStatus(database); line != "Monk mode is off." {
			fmt.Println(line)
		}
		blocks, err := database.ListBlocks()
		if err != nil {
			return err
		}
		if len(blocks) == 0 {
			fmt.Println("No blocks registered. Create one with 'kc9k block create'.")
			return nil
		}

		trusted, err := database.TrustedNow(time.Now())
		if err != nil {
			return err
		}
		enabled := enforce.EnabledNames(database, trusted)
		var active, inactive []string
		for _, block := range blocks {
			detail, err := database.GetBlock(block.Name)
			if err != nil {
				return err
			}
			tags := ""
			hold, err := blockHold(database, block.Name, time.Now())
			if err != nil {
				return err
			}
			if hold.scheduleLock {
				state := "off now"
				if hold.scheduleFrozen || hold.scheduledNow {
					state = "on now"
				}
				tags += fmt.Sprintf(" [schedule lock, %s lead, %s]", time.Duration(hold.scheduleLead)*time.Minute, state)
			}
			if detail.Active {
				if enabled[detail.Name] {
					tags += " [enabled]" // §3.3
				} else {
					tags += " [lifted]"
				}
				if enforce.LiftedBySchedule(database, detail.Name, time.Now()) {
					tags += " [lifted by its schedule]"
				}
				if schedules, err := database.ListSchedules(detail.Name); err == nil {
					if left := windowsLeftToday(db.Windows(schedules), time.Now()); len(left) > 0 {
						tags += " [today: " + strings.Join(left, ", ") + "]" // §4.1
					}
				}
				if q, _ := database.GetQuota(detail.Name); q != nil && q.Open {
					tags += " [lifted: allowance session open]"
				}
				if hold.charLocked {
					tags += " [text lock: " + lock.DescribeStyle(hold.extras.Style, detail.LockChars) + "]"
				}
				if hold.extras.LiftedUntil > trusted {
					tags += fmt.Sprintf(" [unlocked: locks on again in %s]", time.Duration(hold.extras.LiftedUntil-trusted)*time.Second)
				} else if hold.extras.Relock > 0 {
					tags += fmt.Sprintf(" [re-lock after %s]", time.Duration(hold.extras.Relock)*time.Second)
				}
				if detail.LockedUntil > trusted {
					tags += fmt.Sprintf(" [timer lock until %s]", time.Unix(detail.LockedUntil, 0).Format("2006-01-02 15:04"))
				}
				if hold.passwordHash != "" {
					tags += " [password lock]"
				}
				if hold.restartLocked {
					tags += " [restart lock]"
				}
				switch {
				case hold.extras.DelayUntil > trusted:
					tags += fmt.Sprintf(" [delay lock: unlocked in %s]", time.Duration(hold.extras.DelayUntil-trusted)*time.Second)
				case hold.extras.Delay > 0:
					tags += fmt.Sprintf(" [delay lock of %s]", time.Duration(hold.extras.Delay)*time.Second)
				}
				if hold.extras.RangeDays != 0 {
					state := "off now"
					if hold.extras.InRange(time.Now()) {
						state = "on now"
					}
					tags += " [range lock: " + describeRange(hold.extras) + ", " + state + "]"
				}
				if hold.extras.TimerAfter > 0 {
					tags += fmt.Sprintf(" [timer lock of %s after the locks before it]", time.Duration(hold.extras.TimerAfter)*time.Second)
				}
				if len(hold.layers) > 1 {
					// R-LOCK-6: unlocked one at a time, in this order.
					tags += " [unlock order: " + describeLayers(hold.layers, detail, hold.extras) + "]"
				}
				active = append(active, fmt.Sprintf("  %s%s", detail.Name, tags))
			} else {
				inactive = append(inactive, fmt.Sprintf("  %s%s", detail.Name, tags))
			}
		}

		fmt.Println("=== Killer Chicken 9000 Status ===")
		fmt.Println()
		fmt.Println("Active blocks:")
		printLinesOrNone(active)
		fmt.Println()
		fmt.Println("Inactive blocks:")
		printLinesOrNone(inactive)
		fmt.Println()
		fmt.Printf("Total: %d blocks (%d active, %d inactive)\n", len(active)+len(inactive), len(active), len(inactive))
		if pending, err := database.ListPendingEdits(); err == nil && len(pending) > 0 {
			fmt.Printf("Pending changes: %d (see 'kc9k pending')\n", len(pending))
		}
		fmt.Println()
		fmt.Println("Browser extension:", extensionStatus(database))
		if warning := blocker.DesktopWarning(); warning != "" {
			fmt.Println("Desktop:", warning) // R-DESKTOP-1
		}
		if set, err := settingsPasswordSet(database); err == nil && set {
			msg := "Settings password: on (weakening asks for it"
			if v, _ := database.GetSetting(SettingPasswordForLocks); v == "on" {
				msg += ", and setting a lock"
			}
			fmt.Println(msg + ")")
		}
		return nil
	},
}

// extensionStatus describes whether the browser extension is working, for
// 'kc9k status' and the interface.
func extensionStatus(database *db.DB) string {
	if !blocker.ExtensionBrowserInstalled() {
		return "no supported browser (LibreWolf, Google Chrome, Chromium) is installed, so wildcards, keywords, greyscale, images and web-only aren't enforced in the browser, and no browser is closed. Install one: 'kc9k browsers install librewolf'."
	}
	required, err := enforce.ExtensionRequiredNow(database)
	if err != nil {
		return "unknown: " + err.Error()
	}
	status := "not needed right now"
	if required {
		status = "needed now: browsers without it are closed after 60 seconds"
	}
	if enforce.UnsupportedBrowsersOn(database) {
		status = "always needed (block unsupported browsers is on): other browsers are closed at once, supported ones without it after 60 seconds"
	}
	if last, ok := enforce.LastExtensionCheckIn(database); ok {
		return fmt.Sprintf("%s; last checked in %s ago", status, fmtAgo(time.Since(last)))
	}
	return status + "; no check-in yet (open a supported browser)"
}

// extensionMissing reports whether the interface should offer to install
// the extension (§4.10): no supported browser is installed, or the
// extension is needed and hasn't checked in for 5 minutes.
func extensionMissing(database *db.DB) bool {
	if !blocker.ExtensionBrowserInstalled() {
		return true
	}
	required, err := enforce.ExtensionRequiredNow(database)
	if err != nil || !required {
		return false
	}
	last, ok := enforce.LastExtensionCheckIn(database)
	return !ok || time.Since(last) > 5*time.Minute
}

// fmtAgo formats an elapsed time roughly: "12 s", "5 min", "3 h", "2 days".
func fmtAgo(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h", int(d.Hours()))
	}
	return fmt.Sprintf("%d days", int(d.Hours()/24))
}

func printLinesOrNone(lines []string) {
	if len(lines) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, l := range lines {
		fmt.Println(l)
	}
}

// ============================================================================
// Hold state — the single source of truth for "can this block be removed now?"
// ============================================================================

type holdState struct {
	active      bool
	charLocked  bool
	lockChars   int
	lockedUntil int64
	// scheduledNow: the block has a schedule lock (§3.4) and a window or
	// place has it on now.
	scheduledNow bool
	// scheduleFrozen: the block has a schedule lock, and a window is open
	// now or opens within its lead time, whether or not the block is active.
	scheduleFrozen bool
	scheduleLock   bool
	scheduleLead   int  // minutes
	quota          bool // a daily quota keeps the block on
	extras         db.LockExtras
	passwordHash   string
	restartLocked  bool
	// layers: the locks on, in unlock order (R-LOCK-6); the first is the
	// one 'unlock' acts on.
	layers  []string
	now     time.Time
	trusted int64 // see db.TrustedNow
}

func (h holdState) timeLocked() bool { return h.lockedUntil > h.trusted }
func (h holdState) until() string {
	return time.Unix(h.lockedUntil, 0).Format("2006-01-02 15:04")
}

// held reports whether the block currently can't be freely removed/edited.
func (h holdState) held() bool {
	return h.scheduleFrozen || h.quota || h.active && (h.charLocked || h.timeLocked() || h.scheduledNow ||
		h.passwordHash != "" || h.restartLocked || h.extras.Delay > 0 || len(h.layers) > 0)
}

// blockHold gathers everything that determines whether a block may be removed
// or edited right now.
func blockHold(database *db.DB, name string, now time.Time) (holdState, error) {
	trusted, err := database.TrustedNow(now)
	if err != nil {
		return holdState{}, err
	}
	detail, err := database.GetBlock(name)
	if err != nil {
		return holdState{}, err
	}
	h := holdState{
		active:      detail.Active,
		charLocked:  detail.Locked,
		lockChars:   detail.LockChars,
		lockedUntil: detail.LockedUntil,
		now:         now,
		trusted:     trusted,
	}
	if h.scheduleLock, h.scheduleLead, err = database.ScheduleLock(name); err != nil {
		return holdState{}, err
	}
	if detail.Active && h.scheduleLock {
		sched, err := enforce.ScheduleActiveNow(database, name, now)
		if err != nil {
			return holdState{}, err
		}
		h.scheduledNow = sched
	}
	if h.extras, err = database.GetLockExtras(name); err != nil {
		return holdState{}, err
	}
	if h.extras.DelayUntil != 0 && trusted >= h.extras.DelayUntil {
		h.extras.Delay = 0 // the countdown is over; the daemon lifts it next cycle
	}
	if detail.Active {
		boot, err := db.BootID()
		if err != nil {
			return holdState{}, err
		}
		// Layered locks (R-LOCK-6): start the lock after any that are gone.
		if db.AdvanceLayers(detail, &h.extras, trusted, boot) {
			if err := database.SaveLayers(detail, h.extras); err != nil {
				return holdState{}, err
			}
			h.lockedUntil = detail.LockedUntil
		}
		h.layers = db.LayersOn(detail, h.extras, trusted, boot)
		// Unlocked with re-lock: off until LiftedUntil, then on again.
		h.charLocked = db.LayerOn(db.LayerText, detail, h.extras, trusted, boot)
		if db.LayerOn(db.LayerPassword, detail, h.extras, trusted, boot) {
			h.passwordHash = h.extras.PasswordHash
		}
		h.restartLocked = db.LayerOn(db.LayerRestart, detail, h.extras, trusted, boot)
	}
	q, err := database.GetQuota(name)
	if err != nil {
		return holdState{}, err
	}
	h.quota = q != nil
	if h.scheduleLock {
		frozen, err := enforce.ScheduleFrozen(database, name, now, time.Duration(h.scheduleLead)*time.Minute)
		if err != nil {
			return holdState{}, err
		}
		h.scheduleFrozen = frozen
	}
	return h, nil
}

func init() {
	startCmd.Flags().Bool("lock", false, "Lock the block (requires a challenge to deactivate)")
	startCmd.Flags().Int("lock-chars", lock.DefaultLockChars, "Number of characters in the unlock challenge")
	addTextLockFlags(startCmd)
	startCmd.Flags().Bool("password-stdin", false, "Password lock: read the password from standard input")
	startCmd.Flags().Bool("restart", false, "Restart lock: keep the block on until the computer restarts")
	startCmd.Flags().String("delay", "", "Delay lock: unlocking takes this long (1m to 960h)")
	addYesFlag(startCmd)
	startCmd.Flags().String("relock", "", "Re-lock: after unlocking, the text and password locks come on again after this long (e.g. 30m)")
	startCmd.Flags().String("for", "", "Timer lock: keep the block un-removable for this duration (e.g. 90m, 2h, 3d, 1w)")
	startCmd.Flags().String("until", "", "Timer lock: keep the block un-removable until this time (HH:MM, or a date: YYYY-MM-DD HH:MM)")

	startCmd.Flags().String("range", "", `Range lock: locked at these times, e.g. "mon-fri 09:00-17:00" (22:00-06:00 runs past midnight)`)
	startCmd.Flags().Bool("range-invert", false, "Range lock: locked outside those times instead")
	trackLockFlagOrder(startCmd) // after every lock flag is defined
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(unlockCmd)
	rootCmd.AddCommand(statusCmd)
}
