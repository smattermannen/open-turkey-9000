// loosen.go — gating edits that loosen a block, and the queue of pending edits.
//
// A block with allow windows (schedule add --allow) is meant to be on at all
// times except during those windows. If it could be loosened freely while an
// allow window is open, that window would be the moment to dismantle it, so a
// loosening edit to such a block never applies on its own:
//
//   - by default it is queued, and the daemon applies it LoosenDelay later
//     (kc9k pending lists the queue; pending cancel drops an entry);
//   - with --now it applies immediately, after the same typing challenge as
//     unlock.
//
// Either way it is refused while the block is char-locked or time-locked, and
// a queued edit waits for such a lock to lift. Edits that only make a block
// stricter are never gated. Blocks without allow windows keep the plain rule:
// loosening edits are refused while the block is held (see ensureEditable).
//
//	kc9k pending
//	kc9k pending cancel <number>
package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/lock"
	"github.com/spf13/cobra"
)

// LoosenDelay is how long a queued loosening edit to a block with allow
// windows waits before the daemon applies it.
const LoosenDelay = db.DefaultLoosenDelay * time.Second

// gateEdit decides whether an edit to a block may go ahead right now, and
// reports true if so. Edits that strengthen the block always may. A loosening
// edit to a block with allow windows is queued (gateEdit then reports false
// and the caller stops) unless --now is given and the typing challenge is
// passed. Other blocks refuse loosening edits while held.
func gateEdit(cmd *cobra.Command, database *db.DB, name string, strengthens bool, e db.Edit) (bool, error) {
	if err := refuseMonkEdit(database, name); err != nil {
		return false, err
	}
	if strengthens {
		return true, nil
	}
	if err := requireSettingsPassword(database); err != nil {
		return false, err
	}
	allowBlock, err := database.HasAllowWindows(name)
	if err != nil {
		return false, err
	}
	if !allowBlock {
		return true, ensureEditable(database, name, false)
	}

	hold, err := blockHold(database, name, time.Now())
	if err != nil {
		return false, err
	}
	if hold.active && hold.timeLocked() {
		return false, refused(database, name, fmt.Errorf("block '%s' is time-locked until %s and can't be loosened before then", name, hold.until()))
	}
	if hold.active && hold.charLocked {
		return false, refused(database, name, fmt.Errorf("block '%s' is locked. Use 'kc9k unlock %s' first", name, name))
	}

	if now, _ := cmd.Flags().GetBool("now"); now {
		fmt.Printf("Block '%s' has allow windows, so loosening it right away needs the typing challenge.\n", name)
		ok, err := lock.RunChallenge(lock.DefaultLockChars)
		if err != nil {
			return false, fmt.Errorf("failed to run the typing challenge: %w", err)
		}
		if !ok {
			return false, fmt.Errorf("challenge failed; block '%s' is unchanged", name)
		}
		return true, nil
	}

	delay, err := database.LoosenDelay(name)
	if err != nil {
		return false, err
	}
	applyAt := time.Now().Add(time.Duration(delay) * time.Second)
	if err := database.QueueEdit(name, e, applyAt.Unix()); err != nil {
		return false, err
	}
	fmt.Printf("Block '%s' has allow windows, so this change is queued: %s\n", name, e)
	fmt.Printf("It takes effect at %s. See 'kc9k pending'; to apply it now instead, cancel it and rerun with --now (typing challenge).\n",
		applyAt.Format("2006-01-02 15:04"))
	return false, nil
}

// refused logs a refused attempt to loosen or stop a held block as an event
// and returns err. It is in the log only: asking is ordinary use, and bypass
// detection doesn't count it.
func refused(database *db.DB, block string, err error) error {
	if logErr := database.RecordEvent(db.EventRefused, block, err.Error()); logErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", logErr)
	}
	return err
}

// addNowFlag adds --now to commands whose edits gateEdit may queue.
func addNowFlag(cmds ...*cobra.Command) {
	for _, c := range cmds {
		c.Flags().Bool("now", false, "For a block with allow windows: apply a loosening change immediately after the typing challenge, instead of queueing it")
	}
}

// requireAll errors if any of want is missing from have, so an edit that
// would fail isn't queued.
func requireAll(kind, block string, have, want []string) error {
	set := make(map[string]bool, len(have))
	for _, h := range have {
		set[h] = true
	}
	for _, w := range want {
		if !set[w] {
			return fmt.Errorf("%s '%s' not found in block '%s'", kind, w, block)
		}
	}
	return nil
}

var pendingCmd = &cobra.Command{
	Use:   "pending",
	Short: "List queued changes to blocks with allow windows",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		pending, err := database.ListPendingEdits()
		if err != nil {
			return err
		}
		if len(pending) == 0 {
			fmt.Println("No pending changes.")
			return nil
		}
		fmt.Println("Pending changes:")
		for _, p := range pending {
			fmt.Printf("  [%d] %s: %s (at %s)\n", p.Num, p.BlockName, p.Edit,
				time.Unix(p.ApplyAt, 0).Format("2006-01-02 15:04"))
		}
		return nil
	},
}

var pendingCancelCmd = &cobra.Command{
	Use:   "cancel [number]",
	Short: "Cancel a queued change by its number",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var num int
		if _, err := fmt.Sscan(args[0], &num); err != nil {
			return fmt.Errorf("invalid number %q", args[0])
		}
		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		pending, err := database.ListPendingEdits()
		if err != nil {
			return err
		}
		if num < 1 || num > len(pending) {
			return fmt.Errorf("pending change %d not found", num)
		}
		p := pending[num-1]
		// Cancelling keeps the block as strict as it is, so it's always allowed.
		if err := database.DeletePendingEdit(p.ID); err != nil {
			return err
		}
		fmt.Printf("Cancelled pending change to '%s': %s\n", p.BlockName, p.Edit)
		return nil
	},
}

var pendingDelayCmd = &cobra.Command{
	Use:   "delay [block] [time]",
	Short: "How long a block's queued loosening changes wait (default 24h)",
	Long: `A block with allow windows is never loosened directly: the change is queued
and applied later. This sets how much later for one block, from 1h to 720h.
A longer delay applies at once. A shorter one loosens the block, so it is
itself queued for the delay the block has now.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		name := args[0]
		current, err := database.LoosenDelay(name)
		if err != nil {
			return err
		}
		if len(args) == 1 {
			fmt.Printf("Loosening changes to '%s' wait %s.\n", name, time.Duration(current)*time.Second)
			return nil
		}
		d, err := time.ParseDuration(args[1])
		if err != nil {
			return fmt.Errorf("invalid time %q (e.g. 48h)", args[1])
		}
		want := int64(d.Seconds())
		if want < db.MinLoosenDelay || want > db.MaxLoosenDelay {
			return fmt.Errorf("a loosening delay is from 1h to 720h")
		}
		proceed, err := gateEdit(cmd, database, name, want >= current,
			db.Edit{Action: db.EditSetDelay, Values: []string{fmt.Sprint(want)}})
		if err != nil || !proceed {
			return err
		}
		if err := database.SetLoosenDelay(name, want); err != nil {
			return err
		}
		fmt.Printf("Loosening changes to '%s' now wait %s.\n", name, d)
		return nil
	}),
}

func init() {
	addNowFlag(pendingDelayCmd)
	pendingCmd.AddCommand(pendingDelayCmd)
	pendingCmd.AddCommand(pendingCancelCmd)
	rootCmd.AddCommand(pendingCmd)
}
