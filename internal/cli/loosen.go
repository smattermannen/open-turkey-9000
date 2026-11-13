// loosen.go — gating edits that weaken a block, and what is left of the
// queue of pending edits.
//
// Only locks protect a block (R-LOCK-1, R-LOCK-10): an edit that weakens it
// is refused while it is locked (see ensureEditable), and goes ahead at once
// otherwise. Blocks with allow windows used to queue such edits for a day;
// that built-in delay is gone (a schedule lock protects them now, see
// db/schedulelock.go), and the daemon only applies what was queued before.
//
//	kc9k pending
//	kc9k pending cancel <number>
package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

// LoosenDelay is how long a queued loosening edit to a block with allow
// windows waits before the daemon applies it.
const LoosenDelay = db.DefaultLoosenDelay * time.Second

// gateEdit decides whether an edit to a block may go ahead right now, and
// reports true if so. Edits that strengthen the block always may (except on
// monk mode while it is on); edits that weaken it may unless it is locked.
// e is what the edit would have been queued as, kept for the callers.
func gateEdit(cmd *cobra.Command, database *db.DB, name string, strengthens bool, e db.Edit) (bool, error) {
	if err := refuseMonkEdit(database, name); err != nil {
		return false, err
	}
	if strengthens {
		return true, nil
	}
	return true, ensureEditable(database, name, false)
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
		c.Flags().Bool("now", false, "No longer needed: changes are never queued (kept for old scripts)")
		_ = c.Flags().MarkHidden("now")
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
	Short: "List changes queued by earlier versions, which the daemon still applies",
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
	Hidden: true, // changes are no longer queued
	Use:    "delay [block] [time]",
	Short:  "How long a block's queued weakening changes wait (default 24h)",
	Long: `A block with allow windows is never weakened directly: the change is queued
and applied later. This sets how much later for one block, from 1h to 720h.
A longer delay applies at once. A shorter one weakens the block, so it is
itself queued for the delay the block has now.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		name := args[0]
		current, err := database.LoosenDelay(name)
		if err != nil {
			return err
		}
		if len(args) == 1 {
			fmt.Printf("Weakening changes to '%s' wait %s.\n", name, time.Duration(current)*time.Second)
			return nil
		}
		d, err := time.ParseDuration(args[1])
		if err != nil {
			return fmt.Errorf("invalid time %q (e.g. 48h)", args[1])
		}
		want := int64(d.Seconds())
		if want < db.MinLoosenDelay || want > db.MaxLoosenDelay {
			return fmt.Errorf("a weakening delay is from 1h to 720h")
		}
		proceed, err := gateEdit(cmd, database, name, want >= current,
			db.Edit{Action: db.EditSetDelay, Values: []string{fmt.Sprint(want)}})
		if err != nil || !proceed {
			return err
		}
		if err := database.SetLoosenDelay(name, want); err != nil {
			return err
		}
		fmt.Printf("Weakening changes to '%s' now wait %s.\n", name, d)
		return nil
	}),
}

func init() {
	addNowFlag(pendingDelayCmd)
	pendingCmd.AddCommand(pendingDelayCmd)
	pendingCmd.AddCommand(pendingCancelCmd)
	rootCmd.AddCommand(pendingCmd)
}
