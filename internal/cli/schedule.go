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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
	"github.com/spf13/cobra"
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Windows of blocks and modes, and schedules kept outside blocks",
}

var scheduleAddCmd = &cobra.Command{
	Use:   "add [block|mode|schedule]",
	Short: "Add a window to a block, mode or schedule (on a block, --allow: an allow window)",
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
		t, err := resolveTarget(database, name, targetKind(cmd))
		if err != nil {
			return err
		}
		if t.Saved != nil {
			// A schedule kept outside blocks (§3.6): what its windows do is
			// up to each block's default.
			if allow {
				return fmt.Errorf("--allow is for a block: a schedule's windows turn each block using it on or off as that block's default says")
			}
			for i, o := range t.Saved.Windows {
				if o.Overlaps(w) {
					return fmt.Errorf("that window overlaps window %d (%s); a schedule's windows never overlap", i+1, o)
				}
			}
			if err := changeSchedule(cmd, database, *t.Saved, append(slices.Clone(t.Saved.Windows), w)); err != nil {
				return err
			}
			fmt.Printf("Window %d added to the schedule '%s': %s\n", len(t.Saved.Windows)+1, t.Name, w)
			return nil
		}
		if t.Block != "" {
			if err := refuseInMode(database, t.Block); err != nil {
				return err
			}
		}
		spec, err := targetSpec(database, t)
		if err != nil {
			return err
		}
		all, err := database.SavedSchedules()
		if err != nil {
			return err
		}
		// §3.6: what a window does follows the default; on a block without
		// windows or places, --allow sets it (enabled by default).
		before, _ := spec.Automation(all)
		monk := t.Mode != nil && t.Mode.Monk
		switch {
		case monk && allow:
			return fmt.Errorf("monk mode's windows turn it on: it is never enabled by default, so --allow doesn't fit")
		case before.Continuous():
			spec.DefaultOn = allow
		case allow != spec.DefaultOn && spec.DefaultOn:
			return fmt.Errorf("'%s' is enabled by default: its windows turn it off, so add them with --allow "+
				"('kc9k automation %s default disabled' makes its windows turn it on)", t.Name, t.Name)
		case allow != spec.DefaultOn:
			return fmt.Errorf("'%s' is disabled by default: its windows turn it on, so --allow doesn't fit "+
				"('kc9k automation %s default enabled' makes its windows turn it off)", t.Name, t.Name)
		}
		w.Allow = false
		for i, o := range spec.Own {
			if o.Overlaps(w) {
				return fmt.Errorf("that window overlaps its window %d (%s); its windows never overlap", i+1, o)
			}
		}
		spec.Own = append(spec.Own, w)
		// Adding enabled time is free; anything that lifts the block
		// somewhere it is enabled now weakens it (R-LOCK-1): a window that
		// turns it off, or the first one of a continuous block.
		if err := changeAutomation(cmd, database, t, spec); err != nil {
			return err
		}
		w.Allow = spec.DefaultOn
		if t.Block == "" {
			fmt.Printf("Window added to %s: %s\n", t.what(), w)
			return nil
		}
		num := 0
		if rows, err := database.ListSchedules(name); err == nil {
			for _, r := range rows {
				if r.Source == 0 && r.DaysMask == w.Days && r.StartMin == w.Start && r.EndMin == w.End {
					num = r.Num
				}
			}
		}
		fmt.Printf("Schedule %d added to '%s': %s\n", num, name, w)
		if allow && before.Continuous() {
			fmt.Printf("'%s' is now enabled at all times except during its allow windows, while it is on.\n", name)
		}
		if active, _ := database.IsBlockActive(name); !active {
			// §3.3: a schedule never switches a block on (R-SCHED-1).
			fmt.Printf("'%s' is off: switch it on with 'kc9k start %s'; it then enforces only as its schedule says.\n", name, name)
		}
		return nil
	},
}

// targetRows returns the windows of a block or mode as numbered for 'kc9k
// schedule list' and 'remove', with where each comes from.
func targetRows(database *db.DB, t autoTarget) ([]db.SourcedWindow, error) {
	if t.Block != "" {
		rows, err := database.ListSchedules(t.Block)
		if err != nil {
			return nil, err
		}
		out := make([]db.SourcedWindow, len(rows))
		for i, r := range rows {
			out[i] = db.SourcedWindow{Window: r.Window(), Source: r.Source}
		}
		return out, nil
	}
	if t.Saved != nil {
		out := make([]db.SourcedWindow, len(t.Saved.Windows))
		for i, w := range t.Saved.Windows {
			out[i] = db.SourcedWindow{Window: w}
		}
		return out, nil
	}
	all, err := database.SavedSchedules()
	if err != nil {
		return nil, err
	}
	_, rows := t.Mode.Spec.Automation(all)
	return rows, nil
}

// scheduleName returns the name of the schedule with id.
func scheduleName(database *db.DB, id int) string {
	all, _ := database.SavedSchedules()
	for _, s := range all {
		if s.ID == id {
			return s.Name
		}
	}
	return fmt.Sprint(id)
}

var scheduleListCmd = &cobra.Command{
	Use:   "list [block|mode|schedule]",
	Short: "List the schedules kept outside blocks, or the windows of a block, mode or schedule",
	Args:  cobra.MaximumNArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 0 {
			all, err := database.SavedSchedules()
			if err != nil {
				return err
			}
			if len(all) == 0 {
				fmt.Println("No schedules yet. Make one with 'kc9k schedule create Evening', then 'kc9k schedule add Evening …'.")
				return nil
			}
			for _, sc := range all {
				fmt.Printf("%s (%s)\n", sc.Name, plural(len(sc.Windows), "1 window", fmt.Sprintf("%d windows", len(sc.Windows))))
				for i, w := range sc.Windows {
					fmt.Printf("  [%d] %s\n", i+1, w)
				}
				if users := scheduleUsersWords(database, sc.ID); users != "" {
					fmt.Println("  used by " + users)
				}
			}
			return nil
		}
		t, err := resolveTarget(database, args[0], targetKind(cmd))
		if err != nil {
			return err
		}
		rows, err := targetRows(database, t)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			if t.Block != "" {
				fmt.Printf("Block '%s' has no schedules.\n", t.Name)
			} else {
				fmt.Printf("%s has no windows.\n", strings.ToUpper(t.what()[:1])+t.what()[1:])
			}
			return nil
		}
		if t.Block != "" {
			fmt.Printf("Schedules for '%s':\n", t.Name)
		} else {
			fmt.Printf("Windows of %s:\n", t.what())
		}
		for i, r := range rows {
			from := ""
			if r.Source != 0 {
				from = " (from the schedule '" + scheduleName(database, r.Source) + "')"
			}
			fmt.Printf("  [%d] %s%s\n", i+1, r.Window, from)
		}
		return nil
	}),
}

// scheduleUsersWords names what uses a schedule.
func scheduleUsersWords(database *db.DB, id int) string {
	blocks, modes, err := database.ScheduleUsers(id)
	if err != nil {
		return ""
	}
	all, _ := database.Modes()
	var parts []string
	for _, m := range all {
		if slices.Contains(modes, m.ID) {
			parts = append(parts, "mode '"+m.Name+"'")
		}
	}
	for _, b := range blocks {
		if _, in, _ := database.ModeOf(b); !in {
			parts = append(parts, "'"+b+"'")
		}
	}
	return strings.Join(parts, ", ")
}

var scheduleRemoveCmd = &cobra.Command{
	Use:   "remove [block|mode|schedule] [number]",
	Short: "Remove a window by its number (later ones are renumbered)",
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

		t, err := resolveTarget(database, name, targetKind(cmd))
		if err != nil {
			return err
		}
		rows, err := targetRows(database, t)
		if err != nil {
			return err
		}
		if num < 1 || num > len(rows) {
			if t.Block != "" {
				return fmt.Errorf("schedule %d not found in block '%s'", num, name)
			}
			return fmt.Errorf("%s has no window %d", t.what(), num)
		}
		row := rows[num-1]
		if t.Saved != nil {
			ws := slices.Delete(slices.Clone(t.Saved.Windows), num-1, num)
			if err := changeSchedule(cmd, database, *t.Saved, ws); err != nil {
				return err
			}
			fmt.Printf("Window %d removed from the schedule '%s'.\n", num, t.Name)
			return nil
		}
		if row.Source != 0 {
			s := scheduleName(database, row.Source)
			return fmt.Errorf("window %d comes from the schedule '%s': change the schedule ('kc9k schedule list %s'), "+
				"or take it off ('kc9k schedule take-off %s %s')", num, s, s, t.Name, s)
		}
		if t.Block != "" {
			if err := refuseInMode(database, t.Block); err != nil {
				return err
			}
		}
		spec, err := targetSpec(database, t)
		if err != nil {
			return err
		}
		// Removing a window that turns the block on weakens it, so does one
		// that turns it off when that leaves places alone to decide less
		// often; refused while it is locked (a schedule lock included, from
		// its lead time before a window). The rest only adds enabled time.
		spec.Own = slices.DeleteFunc(spec.Own, func(w schedule.Window) bool {
			return w.Days == row.Days && w.Start == row.Start && w.End == row.End
		})
		if err := changeAutomation(cmd, database, t, spec); err != nil {
			return err
		}
		if t.Block != "" {
			fmt.Printf("Schedule %d removed from '%s'.\n", num, name)
		} else {
			fmt.Printf("Window %d removed from %s.\n", num, t.what())
		}
		return nil
	},
}

var scheduleCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Make a schedule kept outside blocks (Evening, Weekend), for blocks and modes to apply",
	Long: `A schedule is named weekly windows kept outside blocks (§3.6). Give it
windows with 'kc9k schedule add NAME --days … --from … --to …', then apply
it to blocks and modes ('kc9k schedule apply BLOCK NAME'). Changing it later
changes everything using it, refused as a whole if that weakens a locked
block.`,
	Args: cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if _, err := database.CreateSavedSchedule(args[0]); err != nil {
			return err
		}
		fmt.Printf("Schedule '%s' made. Give it windows with 'kc9k schedule add %s --days … --from … --to …'.\n", args[0], args[0])
		return nil
	}),
}

var scheduleRenameCmd = &cobra.Command{
	Use:   "rename [schedule] [new name]",
	Short: "Rename a schedule kept outside blocks",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		s, err := database.GetSavedSchedule(args[0])
		if err != nil {
			return err
		}
		if err := database.RenameSavedSchedule(s.ID, args[1]); err != nil {
			return err
		}
		fmt.Printf("Schedule '%s' is now '%s'.\n", s.Name, args[1])
		return nil
	}),
}

var scheduleDeleteCmd = &cobra.Command{
	Use:   "delete [schedule]",
	Short: "Delete a schedule; blocks and modes using it keep its windows as their own",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		s, err := database.GetSavedSchedule(args[0])
		if err != nil {
			return err
		}
		if err := database.DeleteSavedSchedule(s.ID); err != nil {
			return err
		}
		fmt.Printf("Schedule '%s' deleted; what used it keeps its windows as its own.\n", s.Name)
		return nil
	}),
}

var scheduleApplyCmd = &cobra.Command{
	Use:   "apply [block|mode] [schedule]",
	Short: "Give a block or mode a schedule's windows (Evening, Weekend), kept up to date",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		return applySchedule(cmd, database, args[0], args[1], true)
	}),
}

var scheduleTakeOffCmd = &cobra.Command{
	Use:     "take-off [block|mode] [schedule]",
	Aliases: []string{"unapply"},
	Short:   "Take a schedule's windows off a block or mode",
	Args:    cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		return applySchedule(cmd, database, args[0], args[1], false)
	}),
}

// applySchedule puts a schedule on a block or mode, or takes it off,
// through the gate: on a block disabled by default it adds enabled time
// (free); enabled by default it lifts it (weakens).
func applySchedule(cmd *cobra.Command, database *db.DB, name, schedName string, on bool) error {
	if err := requireSettingsPassword(database); err != nil {
		return err
	}
	t, err := resolveTarget(database, name, targetKind(cmd))
	if err != nil {
		return err
	}
	if t.Saved != nil {
		return fmt.Errorf("'%s' is a schedule: apply schedules to blocks and modes", name)
	}
	s, err := database.GetSavedSchedule(schedName)
	if err != nil {
		return err
	}
	if t.Block != "" {
		if err := refuseInMode(database, t.Block); err != nil {
			return err
		}
	}
	spec, err := targetSpec(database, t)
	if err != nil {
		return err
	}
	has := slices.Contains(spec.Schedules, s.ID)
	switch {
	case on && has:
		return fmt.Errorf("%s already has the schedule '%s'", t.what(), s.Name)
	case !on && !has:
		return fmt.Errorf("%s doesn't have the schedule '%s'", t.what(), s.Name)
	case on:
		spec.Schedules = append(spec.Schedules, s.ID)
	default:
		spec.Schedules = slices.DeleteFunc(spec.Schedules, func(id int) bool { return id == s.ID })
	}
	if err := changeAutomation(cmd, database, t, spec); err != nil {
		return err
	}
	if on {
		fmt.Printf("%s has the schedule '%s'; changing the schedule changes it too.\n", strings.ToUpper(t.what()[:1])+t.what()[1:], s.Name)
	} else {
		fmt.Printf("The schedule '%s' is off %s.\n", s.Name, t.what())
	}
	return nil
}

var scheduleLockCmd = &cobra.Command{
	Use:   "lock [block] [on|off] [--lead D]",
	Short: "Schedule lock: the block is locked during its windows and at its places, and for a lead time before each window",
	Long: `A schedule lock (§3.4) keeps a block from being weakened, switched off or
deleted during its block windows, while a place rule has it on, and for the
lead time you choose before each window. For a block with allow windows it is
on outside them. Without it, a schedule only lifts the block outside its windows.

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
			d, err := parseUserDuration(cmd.Flag("lead").Value.String())
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
		if want >= 0 && (!on || want > lead) {
			if err := confirmLocks(cmd, lockToSet{"block '" + name + "'", "schedule lock", leadLength(want)}); err != nil {
				return err
			}
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

var scheduleBreakCmd = &cobra.Command{
	Use:   "break [block] [number] [time|off]",
	Short: "A break for each time a window is open (§3.6), e.g. 10m",
	Long: `Gives window N of a block one break of this length each time it is open:
'kc9k break start B' takes it then, also while the block is locked; the block
comes back by itself. A new or longer break weakens; 'off' is always allowed.`,
	Args: cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		num, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("give the window's number from 'kc9k schedule list %s'", args[0])
		}
		schedules, err := database.ListSchedules(args[0])
		if err != nil {
			return err
		}
		if num < 1 || num > len(schedules) {
			return fmt.Errorf("block '%s' has no window %d", args[0], num)
		}
		s := schedules[num-1]
		if s.Allow {
			return fmt.Errorf("window %d is an allow window: the block is lifted then anyway", num)
		}
		var secs int64
		if args[2] != "off" {
			if secs, err = parseBreak(args[2], db.MaxBreakSeconds); err != nil {
				return err
			}
		}
		if secs > s.Break {
			if err := ensureEditable(database, args[0], false); err != nil {
				return err
			}
		}
		if err := database.SetScheduleBreak(s.ID, secs); err != nil {
			return err
		}
		if secs == 0 {
			fmt.Printf("Window %d of '%s' has no break.\n", num, args[0])
		} else {
			fmt.Printf("Window %d of '%s' gives one break of %s each time it is open.\n", num, args[0], seconds(secs))
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
	addYesFlag(scheduleAddCmd, scheduleRemoveCmd, scheduleApplyCmd, scheduleTakeOffCmd)
	addTargetFlags(scheduleAddCmd, scheduleListCmd, scheduleRemoveCmd, scheduleApplyCmd, scheduleTakeOffCmd)
	scheduleCmd.AddCommand(scheduleCreateCmd, scheduleRenameCmd, scheduleDeleteCmd, scheduleApplyCmd, scheduleTakeOffCmd)
	scheduleLockCmd.Flags().String("lead", "0m", "How long before each window the lock comes on (e.g. 2h; at most 7 days)")
	addYesFlag(scheduleLockCmd)
	scheduleCmd.AddCommand(scheduleBreakCmd, scheduleLockCmd)

	scheduleCmd.AddCommand(scheduleAddCmd)
	scheduleCmd.AddCommand(scheduleListCmd)
	scheduleCmd.AddCommand(scheduleRemoveCmd)
	rootCmd.AddCommand(scheduleCmd)
}
