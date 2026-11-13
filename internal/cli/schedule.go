// schedule.go — recurring block schedules.
//
//	kc9k schedule add <block> --days weekdays --from 09:00 --to 17:00
//	kc9k schedule add <block> --allow --days fri,sat --from 18:00 --to 24:00
//	kc9k schedule list <block>
//	kc9k schedule remove <block> <number>
//
// A block's schedules are either block windows (the block is on while one is
// open) or allow windows (the block is on at all times except while one is
// open); a block can't mix the two. The daemon switches the block on and off
// by them. A schedule protects nothing by itself: the block's schedule lock
// (kc9k schedule lock, §3.4) does, during its windows and for its lead time
// before them (see blockHold).
//
// Schedule changes that weaken the block (removing a block window, adding an
// allow window, removing the last allow window) are refused while the block
// is locked. Windows are same-day (the end may be 24:00); express an
// overnight span as two windows.
//
// Schedules are numbered 1..n per block; removing one renumbers the later
// ones down, and a new schedule takes the next (lowest free) number.
package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
	"github.com/spf13/cobra"
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage recurring block schedules",
}

var scheduleAddCmd = &cobra.Command{
	Use:   "add [block]",
	Short: "Add a recurring block window (or, with --allow, an allow window) to a block",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		daysSpec, _ := cmd.Flags().GetString("days")
		fromSpec, _ := cmd.Flags().GetString("from")
		toSpec, _ := cmd.Flags().GetString("to")
		allow, _ := cmd.Flags().GetBool("allow")

		if fromSpec == "" || toSpec == "" {
			return fmt.Errorf("--from and --to are required (HH:MM)")
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
			return fmt.Errorf("--from (%s) must be before --to (%s); for an overnight window add two schedules", fromSpec, toSpec)
		}
		w := schedule.Window{Days: days, Start: start, End: end, Allow: allow}
		if err := w.CheckStep(); err != nil {
			return err
		}

		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()
		if err := requireSettingsPassword(database); err != nil {
			return err
		}

		// ListSchedules also confirms the block exists.
		schedules, err := database.ListSchedules(name)
		if err != nil {
			return err
		}
		for i, s := range schedules {
			if s.Allow == w.Allow && s.Window().Overlaps(w) {
				return fmt.Errorf("that window overlaps schedule %d (%s); a block's windows never overlap", i+1, s.Window())
			}
		}
		// An allow window opens a gap in the block: a weakening, refused
		// while the block is locked (R-LOCK-1). A block window only adds.
		if allow {
			if err := ensureEditable(database, name, false); err != nil {
				return err
			}
		}

		if err := database.AddSchedule(name, w); err != nil {
			return err
		}
		fmt.Printf("Schedule %d added to '%s': %s\n", len(schedules)+1, name, w)
		if allow && len(schedules) == 0 {
			fmt.Printf("'%s' will now be on at all times except during its allow windows.\n", name)
		}
		return nil
	},
}

var scheduleListCmd = &cobra.Command{
	Use:   "list [block]",
	Short: "List a block's schedules",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		schedules, err := database.ListSchedules(name)
		if err != nil {
			return err
		}
		if len(schedules) == 0 {
			fmt.Printf("Block '%s' has no schedules.\n", name)
			return nil
		}
		fmt.Printf("Schedules for '%s':\n", name)
		for _, s := range schedules {
			fmt.Printf("  [%d] %s\n", s.Num, s.Window())
		}
		return nil
	},
}

var scheduleRemoveCmd = &cobra.Command{
	Use:   "remove [block] [number]",
	Short: "Remove a schedule from a block by its number (later ones are renumbered)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		num, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid schedule number %q", args[1])
		}
		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		// Removing a block window, or the last allow window, weakens the
		// block: refused while it is locked (a schedule lock included, from
		// its lead time before a window). Removing one of several allow
		// windows only makes it stricter.
		schedules, err := database.ListSchedules(name)
		if err != nil {
			return err
		}
		if num < 1 || num > len(schedules) {
			return fmt.Errorf("schedule %d not found in block '%s'", num, name)
		}
		if !schedules[num-1].Allow || len(schedules) == 1 {
			if err := ensureEditable(database, name, false); err != nil {
				return err
			}
		}

		if err := database.RemoveSchedule(name, num); err != nil {
			return err
		}
		fmt.Printf("Schedule %d removed from '%s'.\n", num, name)
		return nil
	},
}

var scheduleLockCmd = &cobra.Command{
	Use:   "lock [block] [on|off] [--lead D]",
	Short: "Schedule lock: the block is locked during its windows and at its places, and for a lead time before each window",
	Long: `A schedule lock (§3.4) keeps a block from being weakened, switched off or
deleted during its block windows, while a place rule has it on, and for the
lead time you choose before each window. For a block with allow windows it is
on outside them. Without it, a schedule only switches the block on and off.

  kc9k schedule lock work on --lead 2h
  kc9k schedule lock work            # shows it
  kc9k schedule lock work off        # a weakening: refused while it is locked

A longer lead time is always allowed; a shorter one, or off, weakens the block.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		name := args[0]
		on, lead, err := database.ScheduleLock(name)
		if err != nil {
			return err
		}
		if len(args) == 1 {
			if !on {
				fmt.Printf("Block '%s' has no schedule lock.\n", name)
			} else {
				fmt.Printf("Block '%s' has a schedule lock, on from %s before each window.\n", name, time.Duration(lead)*time.Minute)
			}
			return nil
		}
		want := db.NoScheduleLock
		switch args[1] {
		case "on":
			d, err := time.ParseDuration(cmd.Flag("lead").Value.String())
			if err != nil || d < 0 {
				return fmt.Errorf("invalid --lead %q (e.g. 2h)", cmd.Flag("lead").Value.String())
			}
			if d%time.Minute != 0 {
				return fmt.Errorf("--lead is in whole minutes")
			}
			want = int(d / time.Minute)
			if want > db.MaxScheduleLockLead {
				return fmt.Errorf("the lead time is at most 7 days")
			}
		case "off":
			if !on {
				return fmt.Errorf("block '%s' has no schedule lock", name)
			}
		default:
			return fmt.Errorf("use 'on' or 'off'")
		}
		strengthens := want >= 0 && (!on || want >= lead)
		if strengthens {
			if err := requireLockPassword(database); err != nil {
				return err
			}
		}
		if err := ensureEditable(database, name, strengthens); err != nil {
			return err
		}
		if err := database.SetScheduleLock(name, want); err != nil {
			return err
		}
		if want < 0 {
			fmt.Printf("Block '%s' has no schedule lock any more.\n", name)
		} else {
			fmt.Printf("Block '%s' has a schedule lock, on from %s before each window.\n", name, time.Duration(want)*time.Minute)
		}
		return nil
	}),
}

func init() {
	scheduleAddCmd.Flags().String("days", "daily", "Days: daily, weekdays, weekends, mon-fri, or mon,wed,fri")
	scheduleAddCmd.Flags().String("from", "", "Start time HH:MM (required)")
	scheduleAddCmd.Flags().String("to", "", "End time HH:MM, or 24:00 for midnight (required)")
	scheduleAddCmd.Flags().Bool("allow", false, "Allow window: keep the block on at all times except during its allow windows")

	addNowFlag(scheduleAddCmd, scheduleRemoveCmd)
	scheduleLockCmd.Flags().String("lead", "0m", "How long before each window the lock comes on (e.g. 2h; at most 7 days)")
	scheduleCmd.AddCommand(scheduleLockCmd)

	scheduleCmd.AddCommand(scheduleAddCmd)
	scheduleCmd.AddCommand(scheduleListCmd)
	scheduleCmd.AddCommand(scheduleRemoveCmd)
	rootCmd.AddCommand(scheduleCmd)
}
