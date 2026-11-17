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
	"strconv"
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
func resolveTarget(database *db.DB, name string, kind string) (autoTarget, error) {
	if kind == "" || kind == db.KindBlock {
		if _, err := database.ListSchedules(name); err == nil {
			return autoTarget{Name: name, Block: name}, nil
		}
	}
	if kind == "" || kind == db.KindMode {
		if m, err := database.GetMode(name); err == nil {
			return autoTarget{Name: m.Name, Mode: &m}, nil
		}
		if strings.EqualFold(name, "monk") {
			if m, err := database.MonkMode(); err == nil {
				return autoTarget{Name: m.Name, Mode: &m}, nil
			}
		}
	}
	if kind == "" || kind == db.KindSchedule {
		if s, err := database.GetSavedSchedule(name); err == nil {
			return autoTarget{Name: s.Name, Saved: &s}, nil
		}
	}
	if kind != "" {
		return autoTarget{}, fmt.Errorf("no %s named '%s'", kind, name)
	}
	return autoTarget{}, fmt.Errorf("no block, mode or schedule named '%s'", name)
}

// targetKind is what a name means in a command that takes a block, a mode
// or a schedule: a block first, a mode, then a schedule, unless --mode or
// --schedule says (§4.2 Names: a block and a mode may share a name).
func targetKind(cmd *cobra.Command) string {
	for _, kind := range []string{db.KindMode, db.KindSchedule} {
		if f := cmd.Flags().Lookup(kind); f != nil && f.Value.String() == "true" {
			return kind
		}
	}
	return ""
}

// addTargetFlags gives commands that take a block, mode or schedule the
// flags that say which is meant.
func addTargetFlags(cmds ...*cobra.Command) {
	for _, c := range cmds {
		c.Flags().Bool(db.KindMode, false, "The name is a mode's (a block and a mode may share a name)")
		c.Flags().Bool(db.KindSchedule, false, "The name is a schedule's")
		c.MarkFlagsMutuallyExclusive(db.KindMode, db.KindSchedule)
	}
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
	// R-MONK-3 (owner 2026-11-17, later): switched on as a mode it can be
	// switched off, so its automation is free to change; more time to run
	// asks first, as switching it on does.
	if schedule.AutomationWeakens(monkComparable(monkAfter), monkComparable(monkBefore)) {
		return confirmLocks(cmd, monkLock(database, monkAfter))
	}
	return nil
}

// monkLock describes monk mode switched on, for the confirmation: it cuts
// the network while it runs.
func monkLock(database *db.DB, a schedule.Automation) lockToSet {
	if a.Continuous() {
		return lockToSet{"monk mode", "switched on (no websites, no internet; switch it off to end it)", "until switched off"}
	}
	return lockToSet{"monk mode", "switched on, run by its automation (no websites, no internet while it runs; switch it off to end it)",
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
			if m.Monk {
				fmt.Println(db.FrozenChickenName)
				fmt.Println("  " + describeFreeze(database))
			}
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
	Short: "Delete a mode; its blocks are taken out (each gets its own automation back)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		m, err := database.GetMode(args[0])
		if err != nil {
			return err
		}
		if m.Monk {
			return fmt.Errorf("monk mode can't be deleted")
		}
		if len(m.Blocks) > 0 {
			if err := takeOut(cmd, database, m.Blocks); err != nil {
				return err
			}
		}
		if err := database.DeleteMode(m.ID); err != nil {
			return err
		}
		fmt.Printf("Mode '%s' deleted.\n", m.Name)
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
			// R-MODE-1 (owner 2026-11-17): its own automation is kept, for
			// when it is taken out (unless it is in a mode already).
			own, err := database.BlockAutomationSpec(b)
			if err != nil {
				return err
			}
			if err := database.SetBlockMode(b, m.ID); err != nil {
				return err
			}
			if err := database.KeepOwnAutomation(b, own); err != nil {
				return err
			}
			if err := database.SetBlockAutomation(b, m.Spec); err != nil {
				return err
			}
			fmt.Printf("'%s' is in the mode '%s' and has its automation; its own is kept for when it is taken out.\n", b, m.Name)
		}
		return enforce.ApplyFromDB(database)
	}),
}

var modeTakeOutCmd = &cobra.Command{
	Use:     "take-out [block...]",
	Aliases: []string{"remove"},
	Short:   "Take blocks out of their mode; each gets its own automation back",
	Args:    cobra.MinimumNArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		for _, b := range args {
			if _, in, err := database.ModeOf(b); err != nil {
				return err
			} else if !in {
				return fmt.Errorf("'%s' is in no mode", b)
			}
		}
		return takeOut(cmd, database, args)
	}),
}

// takeOut takes blocks out of their modes (R-MODE-1, owner 2026-11-17): each
// gets back the automation it had of its own, through the gate as a whole
// (refused if that weakens a locked one); one put in before its own was
// kept keeps the mode's.
func takeOut(cmd *cobra.Command, database *db.DB, blocks []string) error {
	all, err := database.SavedSchedules()
	if err != nil {
		return err
	}
	owns := map[string]db.AutomationSpec{}
	changes := map[string]schedule.Automation{}
	for _, b := range blocks {
		own, has, err := database.OwnAutomation(b)
		if err != nil {
			return err
		}
		if has {
			owns[b] = own
			changes[b], _ = own.Automation(all)
		}
	}
	if err := gateAutomation(cmd, database, changes, nil, schedule.Automation{}, schedule.Automation{}); err != nil {
		return err
	}
	for _, b := range blocks {
		m, _, _ := database.ModeOf(b)
		if err := database.SetBlockMode(b, 0); err != nil {
			return err
		}
		if own, has := owns[b]; has {
			if err := database.SetBlockAutomation(b, own); err != nil {
				return err
			}
			fmt.Printf("'%s' is out of the mode '%s' and has its own automation again.\n", b, m.Name)
		} else {
			fmt.Printf("'%s' is out of the mode '%s'; it keeps the automation it had.\n", b, m.Name)
		}
	}
	return enforce.ApplyFromDB(database)
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

// isFrozenChicken reports whether a mode's name means Frozen Chicken (§3.8).
func isFrozenChicken(name string) bool {
	for _, n := range []string{db.FrozenChickenName, "freeze", "frozen"} {
		if strings.EqualFold(name, n) {
			return true
		}
	}
	return false
}

// describeFreeze says whether Frozen Chicken runs, and its scheduled freezes.
func describeFreeze(database *db.DB) string {
	text := "off"
	if until, err := database.FreezeUntil(); err == nil && until > time.Now().Unix() {
		mode, _ := database.FreezeMode()
		text = fmt.Sprintf("on (%s) until %s; it can't end early", mode, time.Unix(until, 0).Format("2006-01-02 15:04"))
	}
	if schedules, err := database.ListFreezeSchedules(); err == nil && len(schedules) > 0 {
		var parts []string
		for _, s := range schedules {
			parts = append(parts, s.Window.String()+" ("+s.Mode+")")
		}
		text += "; scheduled: " + strings.Join(parts, ", ")
	}
	return "Frozen Chicken is " + text + "."
}

// switchFreeze: Frozen Chicken is switched on only with an end time
// (R-FREEZE-3: 'kc9k freeze'), and a running freeze can't end (R-FREEZE-2).
func switchFreeze(on bool) error {
	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()
	if on {
		return fmt.Errorf("Frozen Chicken starts with an end time, at most a week ahead: 'kc9k freeze --for 2h' (or --until; --mode lock, logout, shutdown or apps)")
	}
	if until, err := database.FreezeUntil(); err == nil && until > time.Now().Unix() {
		return refused(database, db.FrozenChickenName, fmt.Errorf("Frozen Chicken is on until %s and can't end early",
			time.Unix(until, 0).Format("2006-01-02 15:04")))
	}
	fmt.Println(describeFreeze(database))
	return nil
}

// switchMode switches a mode's blocks on or off, as 'kc9k start' (without
// locks) and 'kc9k stop' do for each; or monk mode's automation.
func switchMode(cmd *cobra.Command, name string, on bool) error {
	if isFrozenChicken(name) {
		return switchFreeze(on)
	}
	database, err := openDB()
	if err != nil {
		return err
	}
	t, err := resolveTarget(database, name, db.KindMode)
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
	if !on {
		if trusted, err := database.TrustedNow(time.Now()); err == nil {
			if end := enforce.MonkEnd(database, trusted); end > 0 {
				return refused(database, db.MonkModeName, fmt.Errorf("monk mode is locked for %s more (it was started for a time) and ends only then",
					humanDuration(time.Duration(end-trusted)*time.Second)))
			}
		}
	}
	if on == m.Active {
		fmt.Println(describeTarget(database, autoTarget{Name: m.Name, Mode: &m}))
		return nil
	}
	// R-MONK-3 (owner 2026-11-17, later): switched on and off like any
	// mode; only a run for a set time is locked.
	if on {
		if err := confirmLocks(cmd, monkLock(database, a)); err != nil {
			return err
		}
	} else {
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
	}
	if err := database.SetModeActive(m.ID, on); err != nil {
		return err
	}
	m.Active = on
	fmt.Println(describeTarget(database, autoTarget{Name: m.Name, Mode: &m}))
	return enforce.ApplyFromDB(database)
}

var modeLockCmd = &cobra.Command{
	Use:   "lock [mode]",
	Short: "Lock a mode: each of its blocks gets the locks (as 'kc9k start' takes them)",
	Long: `Locks a mode of blocks (R-MODE-3): each of its blocks is started with the
locks given, as 'kc9k start BLOCK <locks>' would (it only strengthens: on a
block that is on, new locks go last). A locked mode can't be switched off or
weakened while its blocks are locked, and they are unlocked as blocks are
('kc9k unlock BLOCK').

  kc9k mode lock Evening --for 2h --yes
  kc9k mode lock Work --lock --lock-chars 500 --yes

Monk mode is locked by running it for a time ('kc9k monk --for 2h'), Frozen
Chicken by freezing ('kc9k freeze --for 2h').`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if isFrozenChicken(args[0]) {
			return fmt.Errorf("Frozen Chicken is locked by freezing: 'kc9k freeze --for 2h --yes'")
		}
		database, err := openDB()
		if err != nil {
			return err
		}
		t, err := resolveTarget(database, args[0], db.KindMode)
		database.Close()
		if err != nil {
			return err
		}
		if t.Mode == nil {
			return fmt.Errorf("'%s' is no mode", args[0])
		}
		if t.Mode.Monk {
			return fmt.Errorf("monk mode is locked by running it for a time: 'kc9k monk --for 2h'")
		}
		locks := lockArgs(cmd)
		if len(locks) == 0 {
			return fmt.Errorf("give the locks, as 'kc9k start' takes them (--for 2h, --lock, --password-stdin, …)")
		}
		return lockMode(cmd, *t.Mode, locks, "")
	},
}

// lockMode locks a mode of blocks (R-MODE-3): each of its blocks is started
// with locks (start's flags, in unlock order), as 'kc9k start' would, after
// one confirmation for all (R-LOCK-12) unless cmd has --yes. A password lock
// takes passwordHash, or a password read once from standard input.
func lockMode(cmd *cobra.Command, m db.Mode, locks []string, passwordHash string) error {
	if len(m.Blocks) == 0 {
		return fmt.Errorf("mode '%s' has no blocks to lock", m.Name)
	}
	req, err := parseLockArgs(locks, time.Now())
	if err != nil {
		return err
	}
	database, err := openDB()
	if err != nil {
		return err
	}
	err = requireLockPassword(database)
	database.Close()
	if err != nil {
		return err
	}
	var items []lockToSet
	for _, b := range m.Blocks {
		items = append(items, pressLocks(b, req)...)
	}
	if err := confirmLocks(cmd, items...); err != nil {
		return err
	}
	if req.password && passwordHash == "" {
		password, err := readPasswordStdin("Lock password")
		if err != nil {
			return err
		}
		if passwordHash, err = db.HashPassword(password); err != nil {
			return err
		}
	}
	presetPasswordHash = passwordHash
	defer func() { presetPasswordHash = "" }()
	var failed []string
	for _, b := range m.Blocks {
		if err := startCmd.ParseFlags(locks); err != nil {
			return err
		}
		if err := startCmd.Flags().Set("yes", "true"); err != nil {
			return err
		}
		if err := startCmd.RunE(startCmd, []string{b}); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", b, err)
			failed = append(failed, b)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("%s couldn't be locked", quoted(failed))
	}
	fmt.Printf("Mode '%s' is locked: %s.\n", m.Name, describeLockList(pressLocks(m.Name, req)))
	return nil
}

var modeRunCmd = &cobra.Command{
	Use:   "run [mode]",
	Short: "Run a mode now for a time, locked, whatever its automation says (a Mode action)",
	Long: `The Mode action by hand (§3.9, R-MODE-4): the mode runs now for --for,
whatever its windows and places say, and is locked until then: it can't be
switched off or weakened. Then it is as it was: its blocks that were off
are switched off again. Monk mode runs for the time; Frozen Chicken locks
the screen for it. A longer time extends a run; a shorter one changes
nothing.

  kc9k mode run Work --for 2h --yes`,
	Args: cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		spec, _ := cmd.Flags().GetString("for")
		d, err := parseUserDuration(spec)
		if err != nil || d < time.Minute {
			return fmt.Errorf("give --for, at least 1m (e.g. 45m, 2h)")
		}
		name := args[0]
		switch {
		case isFrozenChicken(name):
			name = db.FrozenChickenName
		case strings.EqualFold(name, "monk") || strings.EqualFold(name, db.MonkModeName):
			name = db.MonkModeName
		default:
			m, err := database.GetMode(name)
			if err != nil {
				return err
			}
			name = m.Name
		}
		if err := requireLockPassword(database); err != nil {
			return err
		}
		if err := confirmLocks(cmd, lockToSet{Target: "mode '" + name + "'", Kind: "runs now, locked, whatever its automation says",
			Length: monkLength(d)}); err != nil {
			return err
		}
		if err := enforce.StartModeRun(database, name, d); err != nil {
			return err
		}
		fmt.Printf("Mode '%s' runs, locked, for %s.\n", name, humanDuration(d))
		return enforce.ApplyFromDB(database)
	}),
}

var modeCustomCmd = &cobra.Command{
	Use:   "custom [mode] [add \"open …\"|\"run …\" | remove N | clear]",
	Short: "What a mode opens or runs when it becomes enabled (custom actions)",
	Long: `A custom action of a mode (§3.9): when the mode becomes enabled (its
automation turns it on, or it is switched on, or a Mode action runs it),
these are opened or run as you, in your desktop session: sites, files,
folders (open), programs and commands (run). A computer started while the
mode is enabled runs them once, when you sign in.

  kc9k mode custom Work                         what it does
  kc9k mode custom Work add "open https://…"    add one
  kc9k mode custom Work add "run libreoffice"
  kc9k mode custom Work remove 2                remove the second
  kc9k mode custom Work clear                   nothing

A custom action can do nothing you couldn't do yourself: a KC9K command in
it meets every lock and password as if typed (R-CUSTOM-1).`,
	Args: cobra.RangeArgs(1, 3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		t, err := resolveTarget(database, args[0], db.KindMode)
		if err != nil {
			return err
		}
		if t.Mode == nil {
			return fmt.Errorf("'%s' is no mode", args[0])
		}
		name := t.Mode.Name
		param, err := database.ModeCustom(name)
		if err != nil {
			return err
		}
		var items []db.CustomItem
		if param != "" {
			if items, err = db.CustomItems(param); err != nil {
				return err
			}
		}
		if len(args) == 1 {
			if len(items) == 0 {
				fmt.Printf("Mode '%s' opens nothing when it becomes enabled.\n", name)
			}
			for i, it := range items {
				fmt.Printf("%d. %s\n", i+1, it)
			}
			return nil
		}
		switch args[1] {
		case "add":
			if len(args) != 3 {
				return fmt.Errorf("give what to add: \"open ADDRESS-FILE-OR-FOLDER\" or \"run COMMAND\"")
			}
			it, err := db.ParseCustomItem(args[2])
			if err != nil {
				return err
			}
			items = append(items, it)
		case "remove":
			n, err := strconv.Atoi(strings.TrimSpace(strings.Join(args[2:], "")))
			if err != nil || n < 1 || n > len(items) {
				return fmt.Errorf("give the number of one to remove (1 to %d)", len(items))
			}
			items = append(items[:n-1], items[n:]...)
		case "clear":
			items = nil
		default:
			return fmt.Errorf("use add, remove or clear")
		}
		param = ""
		if len(items) > 0 {
			param = db.CustomParam(items)
		}
		if err := database.SetModeCustom(name, param); err != nil {
			return err
		}
		fmt.Printf("Mode '%s' opens or runs %d %s when it becomes enabled.\n", name, len(items),
			map[bool]string{true: "thing", false: "things"}[len(items) == 1])
		return nil
	}),
}

var modeShowCmd = &cobra.Command{
	Use:   "show [mode]",
	Short: "What a mode holds and what switches it",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if isFrozenChicken(args[0]) {
			fmt.Println(db.FrozenChickenName)
			fmt.Println("  " + describeFreeze(database))
			return nil
		}
		t, err := resolveTarget(database, args[0], db.KindMode)
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
	addStartLockFlags(modeLockCmd)
	addYesFlag(modeLockCmd, modeRunCmd)
	modeRunCmd.Flags().String("for", "", "How long it runs, locked (e.g. 45m, 2h)")
	modeCmd.AddCommand(modeCreateCmd, modeRenameCmd, modeDeleteCmd, modeAddCmd, modeTakeOutCmd, modeOnCmd, modeOffCmd,
		modeLockCmd, modeRunCmd, modeCustomCmd, modeShowCmd)
	rootCmd.AddCommand(modeCmd)
}
