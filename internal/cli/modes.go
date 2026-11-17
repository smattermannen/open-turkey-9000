// modes.go — kc9k mode: groups of blocks with one automation for all of
// them, and monk mode run by its automation (§3.2 Mode, §3.8 Monk mode,
// R-MODE-1/2, R-MONK-1–3; owner 2026-11-17). Also the gate every automation
// change goes through, for a block, a mode (all its blocks) or a schedule
// kept outside blocks (everything using it).
//
//	kc9k mode                          # the modes, and what is in them
//	kc9k mode create Evening
//	kc9k mode add Evening social games # puts blocks in (they get its automation)
//	kc9k mode take-out games           # out again, keeping that automation
//	kc9k mode on Evening               # switches its blocks on (off: off)
//	kc9k schedule apply Evening Weekdays
//	kc9k place rule "Monk mode" library on
//	kc9k mode on "Monk mode" --yes     # its automation runs monk mode
package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/smattermannen/kc9k/internal/schedule"
	"github.com/spf13/cobra"
)

// autoTarget is what an automation command names: a block, a mode, or a
// schedule kept outside blocks. Blocks, modes and schedules share one set
// of names.
type autoTarget struct {
	Name  string
	Block string
	Mode  *db.Mode
	Saved *db.SavedSchedule
}

func (t autoTarget) what() string {
	switch {
	case t.Mode != nil && t.Mode.Monk:
		return "monk mode"
	case t.Mode != nil:
		return "mode '" + t.Mode.Name + "'"
	case t.Saved != nil:
		return "schedule '" + t.Saved.Name + "'"
	}
	return "block '" + t.Block + "'"
}

// resolveTarget finds what name names; "monk" names monk mode too.
func resolveTarget(database *db.DB, name string) (autoTarget, error) {
	if _, err := database.ListSchedules(name); err == nil {
		return autoTarget{Name: name, Block: name}, nil
	}
	if m, err := database.GetMode(name); err == nil {
		return autoTarget{Name: m.Name, Mode: &m}, nil
	}
	if strings.EqualFold(name, "monk") {
		if m, err := database.MonkMode(); err == nil {
			return autoTarget{Name: m.Name, Mode: &m}, nil
		}
	}
	if s, err := database.GetSavedSchedule(name); err == nil {
		return autoTarget{Name: s.Name, Saved: &s}, nil
	}
	return autoTarget{}, fmt.Errorf("no block, mode or schedule named '%s'", name)
}

// targetSpec returns the automation of a block or mode.
func targetSpec(database *db.DB, t autoTarget) (db.AutomationSpec, error) {
	if t.Mode != nil {
		return t.Mode.Spec.Clone(), nil
	}
	if t.Block == "" {
		return db.AutomationSpec{}, fmt.Errorf("%s is a schedule: it has windows, and blocks and modes apply it", t.what())
	}
	return database.BlockAutomationSpec(t.Block)
}

// refuseInMode refuses changing the automation of a block in a mode: it
// has the mode's (R-MODE-1).
func refuseInMode(database *db.DB, block string) error {
	m, in, err := database.ModeOf(block)
	if err != nil || !in {
		return err
	}
	return fmt.Errorf("'%s' is in the mode '%s' and has its automation: change the mode's, or take the block out "+
		"of it first ('kc9k mode take-out %s')", block, m.Name, block)
}

// neverOn stands for an automation that never has monk mode on: monk mode
// without windows or places doesn't run (a block's would be continuous).
var neverOn = schedule.Automation{Windows: []schedule.Window{{Days: 0, Start: 0, End: 5}}}

func monkComparable(a schedule.Automation) schedule.Automation {
	if a.Continuous() {
		return neverOn
	}
	a.DefaultOn = false
	return a
}

// gateAutomation checks that each block may get its new automation, and
// that monk mode may (with mode set and its automations): refused as a
// whole, naming what refuses, if it weakens a locked block (R-MODE-2) or a
// running monk mode (R-MONK-3). Adding time to monk mode while it is
// switched on asks for confirmation (R-LOCK-12).
func gateAutomation(cmd *cobra.Command, database *db.DB, changes map[string]schedule.Automation, monk *db.Mode, monkBefore, monkAfter schedule.Automation) error {
	names := make([]string, 0, len(changes))
	for name := range changes {
		names = append(names, name)
	}
	slices.Sort(names)
	strengthens := map[string]bool{}
	weakening := false
	for _, name := range names {
		before, err := enforce.AutomationOf(database, name)
		if err != nil {
			return err
		}
		strengthens[name] = automationStrengthens(database, name, before, changes[name])
		weakening = weakening || !strengthens[name]
	}
	monkLess := monk != nil && schedule.AutomationWeakens(monkComparable(monkBefore), monkComparable(monkAfter))
	if weakening || monkLess {
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
	}
	var locked []string
	var only error
	for _, name := range names {
		if err := ensureEditable(database, name, strengthens[name]); err != nil {
			locked, only = append(locked, name), err
		}
	}
	switch {
	case len(locked) == 1 && len(names) == 1:
		return only
	case len(locked) > 0:
		return fmt.Errorf("that weakens %s, which %s locked; nothing was changed", quoted(locked), map[bool]string{true: "is", false: "are"}[len(locked) == 1])
	}
	if monk == nil || !monk.Active {
		return nil
	}
	if monkLess && enforce.MonkAutomationOn(database, time.Now()) {
		return refused(database, db.MonkModeName, fmt.Errorf("monk mode is running by its automation: until it ends, its automation can only be added to"))
	}
	if schedule.AutomationWeakens(monkComparable(monkAfter), monkComparable(monkBefore)) {
		return confirmLocks(cmd, monkLock(database, monkAfter))
	}
	return nil
}

// monkLock describes monk mode run by an automation, for the confirmation.
func monkLock(database *db.DB, a schedule.Automation) lockToSet {
	return lockToSet{"monk mode", "run by its automation (the network is cut; it can't be switched off while it runs)",
		automationWords(database, a)}
}

// automationWords says in a few words when an automation switches.
func automationWords(database *db.DB, a schedule.Automation) string {
	var parts []string
	for _, w := range a.Windows {
		parts = append(parts, w.String())
	}
	if len(a.Places) > 0 {
		places, _ := database.Places()
		var names []string
		for _, p := range places {
			if slices.Contains(a.Places, p.ID) {
				names = append(names, p.Name)
			}
		}
		parts = append(parts, "at "+strings.Join(names, " or "))
	}
	join := ", or "
	if a.NeedBoth && len(a.Windows) > 0 && len(a.Places) > 0 {
		join = ", and "
	}
	return strings.Join(parts, join)
}

func quoted(names []string) string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = "'" + n + "'"
	}
	return strings.Join(out, ", ")
}

// changeAutomation gives a block, or a mode and all its blocks, the
// automation after, through the gate.
func changeAutomation(cmd *cobra.Command, database *db.DB, t autoTarget, after db.AutomationSpec) error {
	all, err := database.SavedSchedules()
	if err != nil {
		return err
	}
	newAuto, _ := after.Automation(all)
	if t.Block != "" {
		if err := refuseInMode(database, t.Block); err != nil {
			return err
		}
		if err := gateAutomation(cmd, database, map[string]schedule.Automation{t.Block: newAuto}, nil, schedule.Automation{}, schedule.Automation{}); err != nil {
			return err
		}
		if err := database.SetBlockAutomation(t.Block, after); err != nil {
			return err
		}
		return reapplyIfActive(database, t.Block)
	}
	m := t.Mode
	if m == nil {
		return fmt.Errorf("%s has no automation of its own", t.what())
	}
	if m.Monk && after.DefaultOn {
		return fmt.Errorf("monk mode is always disabled by default: its windows and places turn it on")
	}
	changes := map[string]schedule.Automation{}
	for _, b := range m.Blocks {
		changes[b] = newAuto
	}
	var monk *db.Mode
	var monkBefore schedule.Automation
	if m.Monk {
		monk = m
		monkBefore, _ = m.Spec.Automation(all)
	}
	if err := gateAutomation(cmd, database, changes, monk, monkBefore, newAuto); err != nil {
		return err
	}
	if err := database.SetModeAutomation(m.ID, after); err != nil {
		return err
	}
	for _, b := range m.Blocks {
		if err := database.SetBlockAutomation(b, after); err != nil {
			return err
		}
	}
	return enforce.ApplyFromDB(database)
}

// changeSchedule gives a schedule kept outside blocks the windows ws, and
// everything using it the change, through the gate (R-MODE-2).
func changeSchedule(cmd *cobra.Command, database *db.DB, s db.SavedSchedule, ws []schedule.Window) error {
	all, err := database.SavedSchedules()
	if err != nil {
		return err
	}
	changed := slices.Clone(all)
	for i := range changed {
		if changed[i].ID == s.ID {
			changed[i].Windows = ws
		}
	}
	blocks, modes, err := database.ScheduleUsers(s.ID)
	if err != nil {
		return err
	}
	changes := map[string]schedule.Automation{}
	specs := map[string]db.AutomationSpec{}
	for _, b := range blocks {
		spec, err := database.BlockAutomationSpec(b)
		if err != nil {
			return err
		}
		specs[b] = spec
		changes[b], _ = spec.Automation(changed)
	}
	var monk *db.Mode
	var monkBefore, monkAfter schedule.Automation
	for _, id := range modes {
		if m, err := database.MonkMode(); err == nil && m.ID == id {
			monk = &m
			monkBefore, _ = m.Spec.Automation(all)
			monkAfter, _ = m.Spec.Automation(changed)
		}
	}
	if err := gateAutomation(cmd, database, changes, monk, monkBefore, monkAfter); err != nil {
		return err
	}
	if err := database.SetSavedScheduleWindows(s.ID, ws); err != nil {
		return err
	}
	for _, b := range blocks {
		if err := database.SetBlockAutomation(b, specs[b]); err != nil {
			return err
		}
	}
	if len(blocks) > 0 || monk != nil {
		return enforce.ApplyFromDB(database)
	}
	return nil
}

// describeTarget says what switches a block or mode, in words.
func describeTarget(database *db.DB, t autoTarget) string {
	if t.Block != "" {
		a, err := enforce.AutomationOf(database, t.Block)
		if err != nil {
			return err.Error()
		}
		return describeAutomation(database, t.Block, a)
	}
	all, _ := database.SavedSchedules()
	m, err := database.GetMode(t.Mode.Name)
	if err != nil {
		return err.Error()
	}
	a, _ := m.Spec.Automation(all)
	if m.Monk {
		if a.Continuous() {
			return "Monk mode has no automation: give it windows or places, or start it for a time ('kc9k monk --for 2h')."
		}
		state := "off: its automation doesn't run it ('kc9k mode on monk')"
		if m.Active {
			state = "on: its automation runs it"
		}
		return fmt.Sprintf("Monk mode runs %s; it is %s.", automationWords(database, a), state)
	}
	return describeAutomation(database, "mode "+m.Name, a)
}

// --------------------------------------------------------------------------
// kc9k mode
// --------------------------------------------------------------------------

var modeCmd = &cobra.Command{
	Use:   "mode",
	Short: "Modes: groups of blocks with one automation for all, and monk mode",
	Long: `A mode is a group of blocks with one automation for all of them: its
windows ('kc9k schedule add MODE', 'kc9k schedule apply MODE SCHEDULE') and
places ('kc9k place rule MODE PLACE on') switch all its blocks, which keep
copies of it. A block is in one mode at most. Putting a block in changes its
automation to the mode's (refused if that weakens it while it is locked);
taking it out leaves it as it is. A change of the mode's automation that
weakens one of its blocks while that one is locked is refused as a whole.

Monk mode is a mode too: with windows or places and switched on ('kc9k mode
on monk'), it runs while they have it on. Running so, it can't be switched
off and its automation can only be added to.`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		modes, err := database.Modes()
		if err != nil {
			return err
		}
		for _, m := range modes {
			fmt.Println(m.Name)
			if !m.Monk {
				if len(m.Blocks) == 0 {
					fmt.Println("  no blocks yet ('kc9k mode add " + m.Name + " BLOCK')")
				} else {
					fmt.Println("  blocks: " + strings.Join(m.Blocks, ", "))
				}
			}
			fmt.Println("  " + describeTarget(database, autoTarget{Name: m.Name, Mode: &m}))
		}
		return nil
	}),
}

var modeCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Make a mode: a group of blocks with one automation",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if err := database.CreateMode(args[0]); err != nil {
			return err
		}
		fmt.Printf("Mode '%s' made. Put blocks in with 'kc9k mode add %s BLOCK'.\n", args[0], args[0])
		return nil
	}),
}

var modeRenameCmd = &cobra.Command{
	Use:   "rename [mode] [new name]",
	Short: "Rename a mode",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		m, err := database.GetMode(args[0])
		if err != nil {
			return err
		}
		if err := database.RenameMode(m.ID, args[1]); err != nil {
			return err
		}
		fmt.Printf("Mode '%s' is now '%s'.\n", m.Name, args[1])
		return nil
	}),
}

var modeDeleteCmd = &cobra.Command{
	Use:   "delete [mode]",
	Short: "Delete a mode; its blocks stay, with the automation they have",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		m, err := database.GetMode(args[0])
		if err != nil {
			return err
		}
		if err := database.DeleteMode(m.ID); err != nil {
			return err
		}
		fmt.Printf("Mode '%s' deleted; its blocks stay as they are.\n", m.Name)
		return nil
	}),
}

var modeAddCmd = &cobra.Command{
	Use:   "add [mode] [block...]",
	Short: "Put blocks into a mode: they get its automation",
	Args:  cobra.MinimumNArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		m, err := database.GetMode(args[0])
		if err != nil {
			return err
		}
		if m.Monk {
			return fmt.Errorf("monk mode holds no blocks: it blocks everything by itself")
		}
		all, err := database.SavedSchedules()
		if err != nil {
			return err
		}
		auto, _ := m.Spec.Automation(all)
		changes := map[string]schedule.Automation{}
		for _, b := range args[1:] {
			if _, err := database.ListSchedules(b); err != nil {
				return err
			}
			if of := draftOf(database, b); of != "" {
				return fmt.Errorf("'%s' is a draft of '%s': drafts go in no mode", b, of)
			}
			changes[b] = auto
		}
		// R-MODE-1: its automation becomes the mode's; refused as a whole
		// if that weakens a locked one.
		if err := gateAutomation(cmd, database, changes, nil, schedule.Automation{}, schedule.Automation{}); err != nil {
			return err
		}
		for _, b := range args[1:] {
			if err := database.SetBlockMode(b, m.ID); err != nil {
				return err
			}
			if err := database.SetBlockAutomation(b, m.Spec); err != nil {
				return err
			}
			fmt.Printf("'%s' is in the mode '%s' and has its automation.\n", b, m.Name)
		}
		return enforce.ApplyFromDB(database)
	}),
}

var modeTakeOutCmd = &cobra.Command{
	Use:     "take-out [block...]",
	Aliases: []string{"remove"},
	Short:   "Take blocks out of their mode; they keep the automation they have",
	Args:    cobra.MinimumNArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		for _, b := range args {
			m, in, err := database.ModeOf(b)
			if err != nil {
				return err
			}
			if !in {
				return fmt.Errorf("'%s' is in no mode", b)
			}
			if err := database.SetBlockMode(b, 0); err != nil {
				return err
			}
			fmt.Printf("'%s' is out of the mode '%s'; it keeps the automation it had.\n", b, m.Name)
		}
		return nil
	}),
}

var modeOnCmd = &cobra.Command{
	Use:   "on [mode]",
	Short: "Switch a mode's blocks on; monk mode: let its automation run it",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return switchMode(cmd, args[0], true) },
}

var modeOffCmd = &cobra.Command{
	Use:   "off [mode]",
	Short: "Switch a mode's blocks off (locked ones stay on); monk mode: stop its automation",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return switchMode(cmd, args[0], false) },
}

// switchMode switches a mode's blocks on or off, as 'kc9k start' (without
// locks) and 'kc9k stop' do for each; or monk mode's automation.
func switchMode(cmd *cobra.Command, name string, on bool) error {
	database, err := openDB()
	if err != nil {
		return err
	}
	t, err := resolveTarget(database, name)
	if err == nil && t.Mode == nil {
		err = fmt.Errorf("'%s' is no mode", name)
	}
	if err != nil {
		database.Close()
		return err
	}
	m := *t.Mode
	if m.Monk {
		defer database.Close()
		return switchMonk(cmd, database, m, on)
	}
	database.Close()
	var failed []string
	for _, b := range m.Blocks {
		database, err := openDB()
		if err != nil {
			return err
		}
		active, err := database.IsBlockActive(b)
		database.Close()
		if err != nil {
			return err
		}
		if active == on {
			continue
		}
		// As 'kc9k start' (no locks: its flags are at their defaults here)
		// or 'kc9k stop' would.
		c := stopCmd
		if on {
			c = startCmd
		}
		if err := c.RunE(c, []string{b}); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", b, err)
			failed = append(failed, b)
		}
	}
	switch {
	case len(m.Blocks) == 0:
		return fmt.Errorf("mode '%s' has no blocks yet", m.Name)
	case len(failed) > 0 && on:
		return fmt.Errorf("%s couldn't be switched on", quoted(failed))
	case len(failed) > 0:
		return fmt.Errorf("%s stay on", quoted(failed))
	}
	return nil
}

func switchMonk(cmd *cobra.Command, database *db.DB, m db.Mode, on bool) error {
	all, err := database.SavedSchedules()
	if err != nil {
		return err
	}
	a, _ := m.Spec.Automation(all)
	if on == m.Active {
		fmt.Println(describeTarget(database, autoTarget{Name: m.Name, Mode: &m}))
		return nil
	}
	if on {
		if a.Continuous() {
			return fmt.Errorf("monk mode has no windows or places to run in: give it some first ('kc9k schedule add monk …', " +
				"'kc9k place rule monk PLACE on'), or start it for a time ('kc9k monk --for 2h')")
		}
		if err := requireLockPassword(database); err != nil {
			return err
		}
		if err := confirmLocks(cmd, monkLock(database, a)); err != nil {
			return err
		}
	} else {
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
		if enforce.MonkAutomationOn(database, time.Now()) {
			return refused(database, db.MonkModeName, fmt.Errorf("monk mode is running by its automation and ends only when it does (R-MONK-3)"))
		}
	}
	if err := database.SetModeActive(m.ID, on); err != nil {
		return err
	}
	m.Active = on
	fmt.Println(describeTarget(database, autoTarget{Name: m.Name, Mode: &m}))
	return enforce.ApplyFromDB(database)
}

var modeShowCmd = &cobra.Command{
	Use:   "show [mode]",
	Short: "What a mode holds and what switches it",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		t, err := resolveTarget(database, args[0])
		if err != nil {
			return err
		}
		if t.Mode == nil {
			return fmt.Errorf("'%s' is no mode", args[0])
		}
		fmt.Println(t.Mode.Name)
		if !t.Mode.Monk {
			fmt.Println("  blocks: " + strings.Join(t.Mode.Blocks, ", "))
		}
		fmt.Println("  " + describeTarget(database, t))
		return nil
	}),
}

func init() {
	addYesFlag(modeOnCmd, modeAddCmd)
	modeCmd.AddCommand(modeCreateCmd, modeRenameCmd, modeDeleteCmd, modeAddCmd, modeTakeOutCmd, modeOnCmd, modeOffCmd, modeShowCmd)
	rootCmd.AddCommand(modeCmd)
}
