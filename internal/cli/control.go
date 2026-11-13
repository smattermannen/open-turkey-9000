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
		if blockName == enforce.MonkBlock {
			return fmt.Errorf("monk mode starts for a set time: kc9k monk --for 45m")
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
			d, err := time.ParseDuration(delaySpec)
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
			d, err := time.ParseDuration(spec)
			if err != nil || d < time.Minute || d > 30*24*time.Hour {
				return fmt.Errorf("invalid --relock %q (1m to 720h)", spec)
			}
			if !useLock && !usePassword {
				return fmt.Errorf("--relock goes with a text or password lock")
			}
			extras.Relock = int64(d / time.Second)
		}
		hasExtras := usePassword || useRestart || delaySpec != "" || style != "" || extras.Relock > 0

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
		var lockedUntil int64
		now := time.Now()
		switch {
		case forDur != "":
			d, err := time.ParseDuration(forDur)
			if err != nil {
				return fmt.Errorf("invalid --for duration %q: %w", forDur, err)
			}
			if d <= 0 {
				return fmt.Errorf("--for duration must be positive")
			}
			lockedUntil = now.Add(d).Unix()
		case untilClock != "":
			t, err := untilToday(now, untilClock)
			if err != nil {
				return err
			}
			lockedUntil = t.Unix()
		}

		// §3.4: a timer lock over 24 hours is confirmed first.
		if lockedUntil != 0 && time.Unix(lockedUntil, 0).Sub(now) > 24*time.Hour {
			if yes, _ := cmd.Flags().GetBool("yes"); !yes {
				left := time.Unix(lockedUntil, 0).Sub(now).Round(time.Minute)
				if !isTerminalInput() {
					return fmt.Errorf("a timer lock of %s is over 24 hours; add --yes to confirm", left)
				}
				fmt.Printf("Lock '%s' for %s, until %s? Type yes to confirm: ", blockName, left, time.Unix(lockedUntil, 0).Format("2006-01-02 15:04"))
				answer, _ := stdinLines.ReadString('\n')
				if strings.TrimSpace(strings.ToLower(answer)) != "yes" {
					return fmt.Errorf("not locked")
				}
			}
		}

		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		if lockedUntil != 0 {
			trusted, err := database.TrustedNow(now)
			if err != nil {
				return err
			}
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
			return fmt.Errorf("block '%s' is already active; password, restart and delay locks, and a text lock of words or your own text, are set when it starts", blockName)
		}
		if detail.Active {
			return strengthenLocks(database, detail, useLock, lockChars, cmd.Flags().Changed("lock-chars"), lockedUntil)
		}

		chars := 0
		if useLock {
			chars = lockChars
		}
		if err := database.ActivateBlock(blockName, useLock, chars, lockedUntil, false); err != nil {
			return err
		}
		_ = database.SetSetting(enforce.StoppedInWindowKey(blockName), "")
		if hasExtras {
			if err := database.SetLockExtras(blockName, extras); err != nil {
				return err
			}
		}

		if err := enforce.ApplyFromDB(database); err != nil {
			return fmt.Errorf("failed to apply blocking layers: %w", err)
		}

		msg := fmt.Sprintf("Block '%s' activated successfully", blockName)
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

// startLockRequested reports whether a start asks for any kind of lock.
func startLockRequested(cmd *cobra.Command) bool {
	for _, f := range []string{"lock", "lock-chars", "for", "until", "password-stdin", "restart", "delay", "text-lock", "lock-words", "lock-text", "relock"} {
		if cmd.Flags().Changed(f) {
			return true
		}
	}
	return false
}

// strengthenLocks handles "start" on a block that is already active: it can add
// or lengthen a char lock and add or extend a time lock, but refuses anything
// that would weaken the current locks. This is safe on a held block, since it
// can only make the hold stronger.
func strengthenLocks(database *db.DB, detail *db.BlockDetail, useLock bool, lockChars int, charsGiven bool, lockedUntil int64) error {
	name := detail.Name
	if !useLock && lockedUntil == 0 {
		return fmt.Errorf("block '%s' is already active (use --lock, --for or --until to strengthen its locks)", name)
	}

	locked, chars, until := detail.Locked, detail.LockChars, detail.LockedUntil
	var changes []string
	if useLock {
		switch {
		case !locked:
			locked, chars = true, lockChars
			changes = append(changes, fmt.Sprintf("text lock of %d chars", chars))
		case lockChars > chars && charsGiven:
			chars = lockChars
			changes = append(changes, fmt.Sprintf("challenge lengthened to %d chars", chars))
		case lockChars < chars && charsGiven:
			return fmt.Errorf("block '%s' is already locked with a %d-character challenge; it can't be shortened", name, chars)
		}
	}
	if lockedUntil != 0 {
		if lockedUntil <= until {
			return fmt.Errorf("block '%s' is already timer-locked until %s; a timer lock can only be extended", name, time.Unix(until, 0).Format("2006-01-02 15:04"))
		}
		until = lockedUntil
		changes = append(changes, fmt.Sprintf("timer lock until %s", time.Unix(until, 0).Format("2006-01-02 15:04")))
	}
	if len(changes) == 0 {
		return fmt.Errorf("block '%s' is already active with those locks", name)
	}

	if err := database.StrengthenLocks(name, locked, chars, until); err != nil {
		return err
	}
	fmt.Printf("Block '%s' strengthened: %s\n", name, strings.Join(changes, ", "))
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

		if err := database.DeactivateBlock(blockName); err != nil {
			return err
		}
		// Switched off inside its window: the daemon leaves it off until
		// the window ends (§3.3: the user's switch, R-LOCK-1).
		if within, err := enforce.ScheduleActiveNow(database, blockName, time.Now()); err == nil && within {
			if err := database.SetSetting(enforce.StoppedInWindowKey(blockName), "1"); err != nil {
				return err
			}
			fmt.Printf("Its schedule leaves it off until the current window ends.\n")
		}
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
		if hold.timeLocked() {
			return refused(database, blockName, fmt.Errorf("block '%s' is timer-locked until %s; it cannot be unlocked before then", blockName, hold.until()))
		}
		if hold.restartLocked {
			return refused(database, blockName, fmt.Errorf("block '%s' stays on until the computer restarts", blockName))
		}
		if !hold.charLocked && hold.passwordHash == "" && hold.extras.Delay == 0 {
			return fmt.Errorf("block '%s' is not locked. Use 'kc9k stop' to switch it off", blockName)
		}

		if hold.passwordHash != "" {
			password, err := readPasswordStdin("Block password")
			if err != nil {
				return err
			}
			if !db.CheckPassword(hold.passwordHash, password) {
				return refused(database, blockName, fmt.Errorf("wrong password; block '%s' remains locked", blockName))
			}
		}
		if hold.charLocked {
			success, err := lock.RunStyled(hold.extras.Style, hold.lockChars)
			if err != nil {
				return fmt.Errorf("failed to run unlock challenge: %w", err)
			}
			if !success {
				_ = refused(database, blockName, fmt.Errorf("failed the unlock challenge"))
				fmt.Println("Unlock failed. The block remains active and locked.")
				os.Exit(1)
			}
		}

		// With relock set, unlocking is a break: the block is lifted for a
		// while and comes back with its locks. Unlocking again during that
		// break lifts the locks.
		if breaks, err := database.GetBreaks(blockName); err != nil {
			return err
		} else if breaks.Relock > 0 {
			trusted, err := database.TrustedNow(time.Now())
			if err != nil {
				return err
			}
			if until, _ := database.PausedUntil(blockName); until <= trusted {
				if err := database.SetPause(blockName, trusted+breaks.Relock); err != nil {
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

		// §3.4: the locks are lifted, the block stays on. With re-lock they
		// are only lifted for that long, then on again.
		if (hold.charLocked || hold.passwordHash != "") && hold.extras.Relock > 0 {
			extras, err := database.GetLockExtras(blockName)
			if err != nil {
				return err
			}
			extras.LiftedUntil = hold.trusted + extras.Relock
			if err := database.SetLockExtras(blockName, extras); err != nil {
				return err
			}
			fmt.Printf("Block '%s' is unlocked for %s; then its locks are on again (re-lock). 'kc9k stop %s' switches it off meanwhile.\n",
				blockName, time.Duration(extras.Relock)*time.Second, blockName)
			if hold.extras.Delay == 0 {
				return nil
			}
		} else if hold.charLocked || hold.passwordHash != "" {
			if err := database.LiftTextAndPasswordLocks(blockName); err != nil {
				return err
			}
		}
		if hold.extras.Delay > 0 {
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
			fmt.Printf("Block '%s': its delay lock is unlocked in %s. The block stays on; then 'kc9k stop %s' switches it off.\n",
				blockName, time.Duration(extras.DelayUntil-hold.trusted)*time.Second, blockName)
			return nil
		}
		fmt.Printf("Block '%s' is unlocked and stays on. 'kc9k stop %s' switches it off.\n", blockName, blockName)
		return nil
	},
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
				if detail.Scheduled {
					tags += " [switched on by its schedule]"
				}
				if hold.charLocked {
					tags += " [text lock: " + lock.DescribeStyle(hold.extras.Style, detail.LockChars) + "]"
				}
				if hold.extras.Lifted(trusted) {
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
		return "neither LibreWolf nor Google Chrome is installed, so wildcards, keywords, greyscale, images and web-only aren't enforced in the browser. Run 'kc9k browser install-librewolf'."
	}
	required, err := enforce.ExtensionRequiredNow(database)
	if err != nil {
		return "unknown: " + err.Error()
	}
	status := "not needed right now"
	if required {
		status = "needed now: browsers without it are closed after 60 seconds"
	}
	if last, ok := enforce.LastExtensionCheckIn(database); ok {
		return fmt.Sprintf("%s; last checked in %s ago", status, fmtAgo(time.Since(last)))
	}
	return status + "; no check-in yet (open LibreWolf)"
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
	now            time.Time
	trusted        int64 // see db.TrustedNow
}

func (h holdState) timeLocked() bool { return h.lockedUntil > h.trusted }
func (h holdState) until() string {
	return time.Unix(h.lockedUntil, 0).Format("2006-01-02 15:04")
}

// held reports whether the block currently can't be freely removed/edited.
func (h holdState) held() bool {
	return h.scheduleFrozen || h.quota || h.active && (h.charLocked || h.timeLocked() || h.scheduledNow ||
		h.passwordHash != "" || h.restartLocked || h.extras.Delay > 0)
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
	h.passwordHash = h.extras.PasswordHash
	if h.extras.Lifted(trusted) {
		// Unlocked with re-lock: off until LiftedUntil, then on again.
		h.charLocked, h.passwordHash = false, ""
	}
	if h.extras.RestartBoot != "" {
		boot, err := db.BootID()
		if err != nil {
			return holdState{}, err
		}
		h.restartLocked = h.extras.RestartBoot == boot
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

// untilToday parses "HH:MM" into the next time it occurs from now (today, or
// tomorrow if the time has already passed today). Tomorrow is the next
// calendar day, not 24 hours on: the two differ when the clocks change.
func untilToday(now time.Time, clock string) (time.Time, error) {
	var h, m int
	if _, err := fmt.Sscanf(clock, "%d:%d", &h, &m); err != nil {
		return time.Time{}, fmt.Errorf("invalid --until time %q (want HH:MM)", clock)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return time.Time{}, fmt.Errorf("invalid --until time %q", clock)
	}
	t := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	if !t.After(now) {
		t = time.Date(now.Year(), now.Month(), now.Day()+1, h, m, 0, 0, now.Location())
	}
	return t, nil
}

func init() {
	startCmd.Flags().Bool("lock", false, "Lock the block (requires a challenge to deactivate)")
	startCmd.Flags().Int("lock-chars", lock.DefaultLockChars, "Number of characters in the unlock challenge")
	addTextLockFlags(startCmd)
	startCmd.Flags().Bool("password-stdin", false, "Password lock: read the password from standard input")
	startCmd.Flags().Bool("restart", false, "Restart lock: keep the block on until the computer restarts")
	startCmd.Flags().String("delay", "", "Delay lock: unlocking takes this long (1m to 960h)")
	startCmd.Flags().Bool("yes", false, "Confirm a timer lock over 24 hours")
	startCmd.Flags().String("relock", "", "Re-lock: after unlocking, the text and password locks come on again after this long (e.g. 30m)")
	startCmd.Flags().String("for", "", "Timer lock: keep the block un-removable for this duration (e.g. 2h, 90m)")
	startCmd.Flags().String("until", "", "Timer lock: keep the block un-removable until this clock time (HH:MM)")

	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(unlockCmd)
	rootCmd.AddCommand(statusCmd)
}
