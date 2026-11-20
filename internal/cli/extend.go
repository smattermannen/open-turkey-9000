// extend.go — extending a block's timer lock (§3.4: timer locks are
// extendable; §4.6: extending is its own action, a new timer lock is a new
// layer).
package cli

import (
	"fmt"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var extendCmd = &cobra.Command{
	Use:   "extend [block]",
	Short: "Extend a block's timer lock: --for adds time, --until moves its end later",
	Long: `Extend the timer lock of a block that is on, without adding a lock:

  kc9k extend study --for 30m           30 minutes more
  kc9k extend study --until 22:00       until 22:00 (only later)

The timer lock extended is the first one still to come in the unlock order:
the one running, or waiting behind other locks, or else the first waiting
after them. A timer lock that counts from unlocking the locks before it
takes only --for. A timer lock is never shortened. To add a second timer
lock instead, use 'kc9k start BLOCK --for …'.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		forSpec, _ := cmd.Flags().GetString("for")
		untilSpec, _ := cmd.Flags().GetString("until")
		if (forSpec == "") == (untilSpec == "") {
			return fmt.Errorf("give --for (time to add) or --until (a later end)")
		}
		now := time.Now()
		var add time.Duration
		var untilWall int64
		if forSpec != "" {
			d, err := parseUserDuration(forSpec)
			if err != nil {
				return fmt.Errorf("invalid --for duration %q: %w", forSpec, err)
			}
			if d <= 0 {
				return fmt.Errorf("--for must be positive")
			}
			add = d
		} else {
			t, err := untilToday(now, untilSpec)
			if err != nil {
				return err
			}
			untilWall = t.Unix()
		}

		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()
		if err := requireLockPassword(database); err != nil {
			return err
		}
		trusted, err := database.TrustedNow(now)
		if err != nil {
			return err
		}
		until := int64(0)
		if untilWall != 0 {
			until = untilWall + trusted - now.Unix()
		}
		detail, err := database.GetBlock(name)
		if err != nil {
			return err
		}
		if !detail.Active {
			return fmt.Errorf("block '%s' is not on; start it with a timer lock: 'kc9k start %s --for …'", name, name)
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

		change, length, err := extendTimer(detail, &e, add, until, trusted)
		if err != nil {
			return err
		}
		if err := confirmLocks(cmd, lockToSet{"block '" + name + "'", "timer lock extended", length}); err != nil {
			return err
		}
		if err := database.SaveLayers(detail, e); err != nil {
			return err
		}
		if err := enforce.ApplyFromDB(database); err != nil {
			return fmt.Errorf("failed to apply blocking layers: %w", err)
		}
		fmt.Printf("Block '%s': %s\n", name, change)
		return nil
	},
}

// extendTimer extends the block's first timer lock still to come: by add,
// or to until (trusted time, only later). It reports what changed and how
// long the lock now holds.
func extendTimer(detail *db.BlockDetail, e *db.LockExtras, add time.Duration, until, trusted int64) (change, length string, err error) {
	at := func(t int64) string { return time.Unix(t, 0).Format("2006-01-02 15:04") }
	later := func(end int64) (int64, error) {
		if until == 0 {
			return end + int64(add/time.Second), nil
		}
		if until <= end {
			return 0, fmt.Errorf("the timer lock already holds until %s; a timer lock is only extended", at(end))
		}
		return until, nil
	}
	durationOnly := fmt.Errorf("this timer lock counts from when the locks before it are unlocked; extend it with --for")
	switch {
	case detail.LockedUntil > trusted:
		end, err := later(detail.LockedUntil)
		if err != nil {
			return "", "", err
		}
		detail.LockedUntil = end
		return "timer lock extended until " + at(end), timerLength(trusted, end), nil
	case e.TimerAfter > 0:
		if until != 0 {
			return "", "", durationOnly
		}
		e.TimerAfter += int64(add / time.Second)
		d := humanDuration(time.Duration(e.TimerAfter) * time.Second)
		return "timer lock extended to " + d + " after the locks before it", d + ", counting from when the locks before it are unlocked", nil
	}
	for i := range e.Queue {
		q := &e.Queue[i]
		if q.Kind != db.LayerTimer {
			continue
		}
		if q.For > 0 {
			if until != 0 {
				return "", "", durationOnly
			}
			q.For += int64(add / time.Second)
			d := humanDuration(time.Duration(q.For) * time.Second)
			return "waiting timer lock extended to " + d, d + ", counting from when it starts", nil
		}
		end, err := later(q.Until)
		if err != nil {
			return "", "", err
		}
		q.Until = end
		return "waiting timer lock extended until " + at(end), "until " + at(end), nil
	}
	return "", "", fmt.Errorf("it has no timer lock to extend; add one with 'start --for …'")
}

func init() {
	extendCmd.Flags().String("for", "", "Time to add to the timer lock (e.g. 30m, 2h, 1w)")
	extendCmd.Flags().String("until", "", "A later end for the timer lock (HH:MM, or YYYY-MM-DD HH:MM)")
	addYesFlag(extendCmd)
	rootCmd.AddCommand(extendCmd)
}
