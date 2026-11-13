// freezeschedule.go — recurring Frozen Chicken windows.
//
//	kc9k freeze schedule add --days daily --from 23:00 --to 24:00 [--mode lock] [--lock-lead 2h] --yes
//	kc9k freeze schedule list
//	kc9k freeze schedule lock <number> on --lead 2h | off
//	kc9k freeze schedule remove <number>
//
// While a window is open, the daemon keeps Frozen Chicken running until the
// window ends (it only ever extends a freeze). A schedule with a lock can't
// be removed from its lead time before a window opens until it closes
// (R-FREEZE-2); one without can always be removed.
package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/smattermannen/kc9k/internal/freeze"
	"github.com/smattermannen/kc9k/internal/schedule"
	"github.com/spf13/cobra"
)

var freezeScheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Freeze the computer at set times, e.g. every night",
}

var freezeScheduleAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Freeze the computer at set times",
	Long: `Freeze the computer at set times, for example every night:

  kc9k freeze schedule add --days daily --from 23:00 --to 24:00 --yes
  kc9k freeze schedule add --days daily --from 00:00 --to 06:00 --yes

Windows are within one day; for an overnight freeze add two, as above, and
they run on from one into the other. While a window is open the computer is
frozen until it ends, in the chosen mode (lock, logout or shutdown); a
running freeze can't be ended. With --lock-lead the schedule is locked: it
can't be removed from that long before its window opens until it closes.
Without it, it can be removed whenever no freeze is running from it.`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		daysSpec, _ := cmd.Flags().GetString("days")
		fromSpec, _ := cmd.Flags().GetString("from")
		toSpec, _ := cmd.Flags().GetString("to")
		mode, _ := cmd.Flags().GetString("mode")
		yes, _ := cmd.Flags().GetBool("yes")
		if fromSpec == "" || toSpec == "" {
			return fmt.Errorf("--from and --to are required (HH:MM)")
		}
		if !freeze.ValidMode(mode) {
			return fmt.Errorf("unknown mode %q (use lock, logout or shutdown)", mode)
		}
		days, err := schedule.ParseDays(daysSpec)
		if err != nil {
			return err
		}
		start, err := schedule.ParseTimeOfDay(fromSpec)
		if err != nil {
			return err
		}
		end, err := schedule.ParseEndTime(toSpec)
		if err != nil {
			return err
		}
		if start >= end {
			return fmt.Errorf("--from (%s) must be before --to (%s); for an overnight freeze add two schedules", fromSpec, toSpec)
		}
		w := schedule.Window{Days: days, Start: start, End: end}
		lead, err := leadMinutes(cmd.Flag("lock-lead").Value.String(), cmd.Flags().Changed("lock-lead"))
		if err != nil {
			return err
		}
		if !yes {
			return fmt.Errorf("this freezes the computer %s in %s mode; add --yes to confirm", w, mode)
		}
		if err := requireLockPassword(database); err != nil {
			return err
		}
		if err := database.AddFreezeSchedule(w, mode, lead); err != nil {
			return err
		}
		fmt.Printf("Frozen Chicken scheduled: %s (%s)\n", w, mode)
		if w.ActiveAt(time.Now()) {
			fmt.Println("The window is open now: the computer freezes within a few seconds.")
		}
		return nil
	}),
}

var freezeScheduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the freeze schedules",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		schedules, err := database.ListFreezeSchedules()
		if err != nil {
			return err
		}
		if len(schedules) == 0 {
			fmt.Println("No freeze schedules.")
			return nil
		}
		now := time.Now()
		for _, s := range schedules {
			note := ""
			if s.Window.ActiveAt(now) {
				note = "  (frozen now)"
			} else if enforce.FreezeScheduleHeld(s, now) {
				note = "  (locked: can't be removed until it ends)"
			}
			if s.LockLead >= 0 {
				note += fmt.Sprintf("  [lock from %s before]", time.Duration(s.LockLead)*time.Minute)
			}
			fmt.Printf("%d. %s (%s)%s\n", s.Num, s.Window, s.Mode, note)
		}
		return nil
	}),
}

var freezeScheduleRemoveCmd = &cobra.Command{
	Use:   "remove [number]",
	Short: "Remove a freeze schedule",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		num, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("give the schedule's number from 'kc9k freeze schedule list'")
		}
		schedules, err := database.ListFreezeSchedules()
		if err != nil {
			return err
		}
		if num < 1 || num > len(schedules) {
			return fmt.Errorf("there is no freeze schedule %d", num)
		}
		s := schedules[num-1]
		if enforce.FreezeScheduleHeld(s, time.Now()) {
			return refused(database, "", fmt.Errorf("freeze schedule %d (%s) is locked: it is running or starts within its lock's lead time; it can be removed once it ends", num, s.Window))
		}
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
		if err := database.RemoveFreezeSchedule(s.ID); err != nil {
			return err
		}
		fmt.Printf("Freeze schedule %d removed: %s\n", num, s.Window)
		return nil
	}),
}

// leadMinutes reads a lock's lead time; NoScheduleLock when not given.
func leadMinutes(spec string, given bool) (int, error) {
	if !given {
		return db.NoScheduleLock, nil
	}
	d, err := time.ParseDuration(spec)
	if err != nil || d < 0 || d%time.Minute != 0 || d > time.Duration(db.MaxScheduleLockLead)*time.Minute {
		return 0, fmt.Errorf("invalid lead time %q (whole minutes, up to 7 days, e.g. 2h)", spec)
	}
	return int(d / time.Minute), nil
}

var freezeScheduleLockCmd = &cobra.Command{
	Use:   "lock [number] [on|off] [--lead D]",
	Short: "Lock a freeze schedule: it can't be removed from the lead time before its window until it ends",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		num, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("give the schedule's number from 'kc9k freeze schedule list'")
		}
		schedules, err := database.ListFreezeSchedules()
		if err != nil {
			return err
		}
		if num < 1 || num > len(schedules) {
			return fmt.Errorf("there is no freeze schedule %d", num)
		}
		s := schedules[num-1]
		want := db.NoScheduleLock
		switch args[1] {
		case "on":
			if want, err = leadMinutes(cmd.Flag("lead").Value.String(), true); err != nil {
				return err
			}
		case "off":
		default:
			return fmt.Errorf("use 'on' or 'off'")
		}
		if want >= 0 && want >= s.LockLead {
			if err := requireLockPassword(database); err != nil {
				return err
			}
		} else {
			if err := requireSettingsPassword(database); err != nil {
				return err
			}
			if enforce.FreezeScheduleHeld(s, time.Now()) {
				return refused(database, "", fmt.Errorf("freeze schedule %d is locked now; its lock can be shortened or taken off once it ends", num))
			}
		}
		if err := database.SetFreezeScheduleLock(s.ID, want); err != nil {
			return err
		}
		if want < 0 {
			fmt.Printf("Freeze schedule %d has no lock now.\n", num)
		} else {
			fmt.Printf("Freeze schedule %d is locked from %s before each window.\n", num, time.Duration(want)*time.Minute)
		}
		return nil
	}),
}

func init() {
	freezeScheduleAddCmd.Flags().String("days", "daily", "Days: daily, weekdays, weekends, mon-fri, or mon,wed,fri")
	freezeScheduleAddCmd.Flags().String("from", "", "Start time HH:MM (required)")
	freezeScheduleAddCmd.Flags().String("to", "", "End time HH:MM, or 24:00 for midnight (required)")
	freezeScheduleAddCmd.Flags().String("mode", freeze.ModeLock, "lock, logout or shutdown")
	freezeScheduleAddCmd.Flags().Bool("yes", false, "Confirm")
	freezeScheduleAddCmd.Flags().String("lock-lead", "", "Lock the schedule from this long before each window (e.g. 2h; 0m: only while it runs)")
	freezeScheduleLockCmd.Flags().String("lead", "0m", "How long before each window the lock comes on (e.g. 2h)")
	freezeScheduleCmd.AddCommand(freezeScheduleAddCmd, freezeScheduleListCmd, freezeScheduleRemoveCmd, freezeScheduleLockCmd)
	freezeCmd.AddCommand(freezeScheduleCmd)
}
