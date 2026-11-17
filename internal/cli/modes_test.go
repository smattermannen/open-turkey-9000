package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

// today is the day name of now, for windows that cover it.
func today() string {
	return strings.ToLower(time.Now().Weekday().String()[:3])
}

// §3.6 Schedule (owner 2026-11-17): a schedule kept outside blocks,
// applied to a block, kept up to date there; changing it is refused as a
// whole when it weakens a locked block using it (R-MODE-2); deleting it
// leaves its windows to the block.
func TestSavedSchedules(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "schedule", "create", "Evening")
	// §4.2 Names (owner 2026-11-17): a block may share a schedule's name;
	// --schedule says which is meant (a block is read first).
	mustRun(t, "block", "create", "Evening")
	mustRun(t, "schedule", "add", "Evening", "--schedule", "--days", "weekdays", "--from", "18:00", "--to", "22:00")
	if rows, _ := d.ListSchedules("Evening"); len(rows) != 0 {
		t.Errorf("--schedule changed the block: %+v", rows)
	}
	mustRun(t, "block", "remove", "Evening")
	if err := runCLI(t, "schedule", "add", "Evening", "--days", "mon", "--from", "21:00", "--to", "23:00"); err == nil {
		t.Error("a schedule's windows overlap")
	}
	mustRun(t, "schedule", "apply", "b", "Evening")
	rows, _ := d.ListSchedules("b")
	if len(rows) != 1 || rows[0].Source == 0 || rows[0].StartMin != 18*60 {
		t.Fatalf("applied: %+v", rows)
	}
	if out := capture(t, "schedule", "list", "b"); !strings.Contains(out, "from the schedule 'Evening'") {
		t.Errorf("list: %s", out)
	}
	if err := runCLI(t, "schedule", "remove", "b", "1"); err == nil || !strings.Contains(err.Error(), "comes from the schedule") {
		t.Errorf("removed a schedule's window from the block: %v", err)
	}
	// Its own window beside the schedule's; the schedule grows: the block too.
	mustRun(t, "schedule", "add", "b", "--days", "sat", "--from", "10:00", "--to", "12:00")
	mustRun(t, "schedule", "add", "Evening", "--days", "sun", "--from", "18:00", "--to", "22:00")
	if rows, _ := d.ListSchedules("b"); len(rows) != 3 {
		t.Fatalf("after the schedule grew: %+v", rows)
	}
	if out := capture(t, "schedule", "list"); !strings.Contains(out, "used by 'b'") {
		t.Errorf("schedules: %s", out)
	}

	// Locked: the schedule can grow, not shrink; it can't come off.
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runCLI(t, "schedule", "remove", "Evening", "1"); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Errorf("shrank a schedule a locked block uses: %v", err)
	}
	if err := runCLI(t, "schedule", "take-off", "b", "Evening"); err == nil {
		t.Error("took a schedule off a locked block")
	}
	mustRun(t, "schedule", "add", "Evening", "--days", "sat", "--from", "18:00", "--to", "22:00")
	if rows, _ := d.ListSchedules("b"); len(rows) != 4 {
		t.Errorf("a growing schedule didn't reach the locked block: %+v", rows)
	}
	// Deleted: the block keeps its windows.
	mustRun(t, "schedule", "delete", "Evening")
	rows, _ = d.ListSchedules("b")
	if len(rows) != 4 {
		t.Fatalf("after delete: %+v", rows)
	}
	for _, r := range rows {
		if r.Source != 0 {
			t.Errorf("still from a schedule: %+v", r)
		}
	}

	// Overlapping schedules count once, stored without overlap.
	mustRun(t, "block", "create", "c")
	mustRun(t, "schedule", "create", "Late")
	mustRun(t, "schedule", "add", "Late", "--days", "daily", "--from", "20:00", "--to", "24:00")
	mustRun(t, "schedule", "create", "Weekend")
	mustRun(t, "schedule", "add", "Weekend", "--days", "weekends", "--from", "00:00", "--to", "24:00")
	mustRun(t, "schedule", "apply", "c", "Late")
	mustRun(t, "schedule", "apply", "c", "Weekend")
	rows, _ = d.ListSchedules("c")
	for i, a := range rows {
		for _, b := range rows[i+1:] {
			if a.Window().Overlaps(b.Window()) {
				t.Errorf("%v overlaps %v", a.Window(), b.Window())
			}
		}
	}
	if len(rows) != 2 {
		t.Errorf("rows %+v", rows)
	}
}

// §3.2 Mode, R-MODE-1/2 (owner 2026-11-17): a mode's blocks share its
// automation; a block in it isn't changed on its own; a change weakening
// a locked block is refused for all; taking a block out keeps what it has;
// the switch switches the blocks.
func TestModeBlocksShareAutomation(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	for _, b := range []string{"a", "c"} {
		mustRun(t, "block", "create", b)
		mustRun(t, "block", "add", b, b+".com")
	}
	mustRun(t, "schedule", "add", "a", "--days", "mon", "--from", "08:00", "--to", "09:00")
	mustRun(t, "mode", "create", "Night")
	// §4.2 Names (owner 2026-11-17): a mode may share a block's name; --mode
	// says which is meant (a block is read first).
	mustRun(t, "mode", "create", "a")
	mustRun(t, "schedule", "add", "a", "--mode", "--days", "tue", "--from", "08:00", "--to", "09:00")
	if m, _ := d.GetMode("a"); len(m.Spec.Own) != 1 {
		t.Errorf("--mode: the mode's windows %+v", m.Spec.Own)
	}
	if rows, _ := d.ListSchedules("a"); len(rows) != 1 {
		t.Errorf("--mode changed the block: %+v", rows)
	}
	mustRun(t, "schedule", "add", "a", "--days", "wed", "--from", "08:00", "--to", "09:00")
	if rows, _ := d.ListSchedules("a"); len(rows) != 2 {
		t.Errorf("without --mode the block's windows: %+v", rows)
	}
	mustRun(t, "schedule", "remove", "a", "2")
	mustRun(t, "mode", "delete", "a")
	if err := runCLI(t, "mode", "create", "night"); err == nil {
		t.Error("two modes share a name")
	}
	mustRun(t, "mode", "add", "Night", "a", "c")
	if rows, _ := d.ListSchedules("a"); len(rows) != 0 {
		t.Errorf("a kept its own windows in the mode: %+v", rows)
	}
	mustRun(t, "schedule", "add", "Night", "--days", "daily", "--from", "22:00", "--to", "24:00")
	for _, b := range []string{"a", "c"} {
		if rows, _ := d.ListSchedules(b); len(rows) != 1 || rows[0].StartMin != 22*60 {
			t.Errorf("%s: %+v", b, rows)
		}
	}
	if err := runCLI(t, "schedule", "add", "a", "--days", "tue", "--from", "08:00", "--to", "09:00"); err == nil ||
		!strings.Contains(err.Error(), "in the mode 'Night'") {
		t.Errorf("changed a block in a mode on its own: %v", err)
	}
	if err := runCLI(t, "automation", "a", "default", "enabled"); err == nil {
		t.Error("changed a block in a mode on its own")
	}
	// The switch: both on.
	mustRun(t, "mode", "on", "Night")
	for _, b := range []string{"a", "c"} {
		if on, _ := d.IsBlockActive(b); !on {
			t.Errorf("%s off after mode on", b)
		}
	}
	// c locked: the mode's windows can grow, not shrink.
	mustRun(t, "start", "c", "--for", "1h", "--yes")
	mustRun(t, "schedule", "add", "Night", "--days", "daily", "--from", "06:00", "--to", "07:00")
	err := runCLI(t, "schedule", "remove", "Night", "1")
	if err == nil || !strings.Contains(err.Error(), "'c'") {
		t.Errorf("shrank a mode with a locked block: %v", err)
	}
	if rows, _ := d.ListSchedules("a"); len(rows) != 2 {
		t.Errorf("refused, yet a changed: %+v", rows)
	}
	// Off: a goes off, c is locked and stays on.
	if err := runCLI(t, "mode", "off", "Night"); err == nil || !strings.Contains(err.Error(), "'c'") {
		t.Errorf("mode off: %v", err)
	}
	if on, _ := d.IsBlockActive("a"); on {
		t.Error("a still on")
	}
	if on, _ := d.IsBlockActive("c"); !on {
		t.Error("a locked block switched off")
	}
	// Taking a out keeps its automation; it is its own again.
	mustRun(t, "mode", "take-out", "a")
	if rows, _ := d.ListSchedules("a"); len(rows) != 2 {
		t.Errorf("a lost its windows leaving the mode: %+v", rows)
	}
	mustRun(t, "schedule", "remove", "a", "1")
	// A block put into a mode whose automation weakens it, while locked:
	// refused (R-MODE-1).
	mustRun(t, "mode", "create", "Day")
	mustRun(t, "schedule", "add", "Day", "--days", "daily", "--from", "09:00", "--to", "10:00")
	mustRun(t, "mode", "take-out", "c")
	if err := runCLI(t, "mode", "add", "Day", "c"); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Errorf("a locked block took a weaker mode's automation: %v", err)
	}
	// Deleting a mode leaves its blocks.
	mustRun(t, "mode", "delete", "Night")
	if _, err := d.GetBlock("a"); err != nil {
		t.Error(err)
	}
}

// R-MONK-3 (owner 2026-11-17, later): monk mode is switched on and off like
// any mode (switching on asks first): with no windows or places it runs
// until switched off, with them while they have it on; neither is locked,
// so it can be switched off and its automation changed. Only a run for a set
// time is locked. It takes no blocks, no allow windows, and is never enabled
// by default.
func TestMonkModeAutomation(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	if err := runCLI(t, "mode", "on", "monk"); err == nil {
		t.Error("switched on without confirmation")
	}
	mustRun(t, "mode", "on", "monk", "--yes")
	if !enforce.MonkAutomationOn(d, time.Now()) {
		t.Fatal("switched on with no windows or places, it doesn't run")
	}
	mustRun(t, "mode", "off", "monk")
	if enforce.MonkAutomationOn(d, time.Now()) {
		t.Fatal("switched off, it runs")
	}
	mustRun(t, "block", "create", "b")
	if err := runCLI(t, "mode", "add", "monk", "b"); err == nil {
		t.Error("monk mode took a block")
	}
	if err := runCLI(t, "schedule", "add", "monk", "--allow", "--days", "mon", "--from", "08:00", "--to", "09:00"); err == nil {
		t.Error("monk mode took an allow window")
	}
	mustRun(t, "schedule", "add", "Monk mode", "--mode", "--days", today(), "--from", "00:00", "--to", "24:00")
	if err := runCLI(t, "automation", "monk", "default", "enabled"); err == nil {
		t.Error("monk mode enabled by default")
	}
	mustRun(t, "mode", "on", "monk", "--yes")
	if !enforce.MonkAutomationOn(d, time.Now()) {
		t.Fatal("its window doesn't run it")
	}
	// Not locked: its window can go, and it can be switched off.
	mustRun(t, "schedule", "remove", "monk", "1")
	if err := runCLI(t, "schedule", "add", "monk", "--days", today(), "--from", "00:00", "--to", "24:00"); err == nil {
		t.Error("added monk time without confirmation")
	}
	mustRun(t, "schedule", "add", "monk", "--days", today(), "--from", "00:00", "--to", "24:00", "--yes")
	tomorrow := strings.ToLower(time.Now().AddDate(0, 0, 1).Weekday().String()[:3])
	mustRun(t, "schedule", "add", "monk", "--days", tomorrow, "--from", "00:00", "--to", "01:00", "--yes")
	mustRun(t, "mode", "off", "monk")
	mustRun(t, "mode", "on", "monk", "--yes")
	// A run for a set time is locked: it can't be switched off.
	mustRun(t, "monk", "--for", "1h", "--yes")
	if err := runCLI(t, "mode", "off", "monk"); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Errorf("switched off while locked: %v", err)
	}
	m, _ := d.MonkMode()
	if len(m.Spec.Own) != 2 || !m.Active || m.Name != db.MonkModeName {
		t.Errorf("monk mode %+v", m)
	}
	if out := capture(t, "mode"); !strings.Contains(out, "Monk mode runs") {
		t.Errorf("mode: %s", out)
	}
	// §4.1: the Overview sees its windows today, and that it runs.
	if state := capture(t, "gui-state"); !strings.Contains(state, `"today_windows":["00:00-24:00"]`) ||
		!strings.Contains(state, `"running":true`) {
		t.Errorf("gui-state: %.300s", state[strings.Index(state, `"modes"`):])
	}
}

// §3.13 Project (owner 2026-11-17): schedules and modes travel in project
// files; imported, blocks use the schedule again (not copies of its
// windows), a mode gets its blocks back, monk mode its automation (never
// switched on); a share leaves the mode's places out.
func TestProjectModes(t *testing.T) {
	notTerminal(t)
	testDB(t)
	mustRun(t, "schedule", "create", "Weekend")
	mustRun(t, "schedule", "add", "Weekend", "--days", "weekends", "--from", "00:00", "--to", "24:00")
	for _, b := range []string{"social", "games", "news"} {
		mustRun(t, "block", "create", b)
		mustRun(t, "block", "add", b, b+".com")
	}
	mustRun(t, "schedule", "apply", "news", "Weekend")
	mustRun(t, "schedule", "add", "news", "--days", "mon", "--from", "08:00", "--to", "09:00")
	mustRun(t, "mode", "create", "Evening")
	mustRun(t, "mode", "add", "Evening", "social", "games")
	mustRun(t, "schedule", "add", "Evening", "--days", "daily", "--from", "19:00", "--to", "23:00")
	mustRun(t, "schedule", "apply", "Evening", "Weekend")
	mustRun(t, "schedule", "add", "monk", "--days", "sun", "--from", "08:00", "--to", "10:00")
	file := filepath.Join(t.TempDir(), "p.json")
	if err := os.WriteFile(file, []byte(capture(t, "project", "export", "--share")), 0o644); err != nil {
		t.Fatal(err)
	}

	d := testDB(t) // a fresh computer
	mustRun(t, "project", "import", file, "--yes")
	news, err := d.BlockAutomationSpec("news")
	if err != nil {
		t.Fatal(err)
	}
	if len(news.Own) != 1 || len(news.Schedules) != 1 {
		t.Errorf("news: %+v", news)
	}
	if rows, _ := d.ListSchedules("news"); len(rows) != 2 {
		t.Errorf("news windows: %+v", rows)
	}
	m, err := d.GetMode("Evening")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Blocks, []string{"games", "social"}) || len(m.Spec.Own) != 1 || len(m.Spec.Schedules) != 1 {
		t.Errorf("mode %+v", m)
	}
	if rows, _ := d.ListSchedules("social"); len(rows) != 3 {
		t.Errorf("social follows the mode: %+v", rows)
	}
	monk, _ := d.MonkMode()
	if len(monk.Spec.Own) != 1 || monk.Active {
		t.Errorf("monk mode %+v", monk)
	}
	// Imported again: schedules and modes here are left as they are; the
	// blocks come in renamed.
	mustRun(t, "project", "import", file, "--yes")
	if all, _ := d.SavedSchedules(); len(all) != 1 {
		t.Errorf("schedules after a second import: %+v", all)
	}
	if m, _ := d.GetMode("Evening"); len(m.Blocks) != 4 {
		t.Errorf("the second import's blocks join the mode: %v", m.Blocks)
	}
}

// R-MODE-3 (owner 2026-11-17, later): a mode of blocks is locked by locking
// its blocks (any lock 'kc9k start' takes), after one confirmation for all;
// locked, it can't be switched off. Monk mode is locked by a run for a time.
func TestModeLock(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	for _, b := range []string{"a", "c"} {
		mustRun(t, "block", "create", b)
		mustRun(t, "block", "add", b, b+".com")
	}
	mustRun(t, "mode", "create", "M")
	mustRun(t, "mode", "add", "M", "a", "c")
	if err := runCLI(t, "mode", "lock", "M", "--for", "1h"); err == nil {
		t.Error("locked without confirmation")
	}
	if on, _ := d.IsBlockActive("a"); on {
		t.Fatal("a block was started before the confirmation")
	}
	mustRun(t, "mode", "lock", "M", "--for", "1h", "--lock", "--lock-chars", "20", "--yes")
	for _, b := range []string{"a", "c"} {
		hold, err := blockHold(d, b, time.Now())
		if on, _ := d.IsBlockActive(b); err != nil || !on || !hold.held() {
			t.Errorf("%s: on %v, held %v (%v)", b, on, hold.held(), err)
		}
	}
	if err := runCLI(t, "mode", "off", "M"); err == nil {
		t.Error("a locked mode was switched off")
	}
	if on, _ := d.IsBlockActive("a"); !on {
		t.Error("a locked block of the mode went off")
	}
	if err := runCLI(t, "mode", "lock", "monk", "--for", "1h", "--yes"); err == nil {
		t.Error("monk mode took locks of blocks")
	}
	mustRun(t, "mode", "create", "Empty")
	if err := runCLI(t, "mode", "lock", "Empty", "--for", "1h", "--yes"); err == nil {
		t.Error("an empty mode was locked")
	}
}

// §3.8, §4.8, R-FREEZE-2/3 (owner 2026-11-17): Frozen Chicken is a mode:
// listed with them, switched on only with an end time at most a week ahead
// (kc9k freeze --for or --until), never switched off while it runs; no mode
// takes its name.
func TestFrozenChickenMode(t *testing.T) {
	testDB(t)
	notTerminal(t)
	if out := capture(t, "mode"); !strings.Contains(out, "Frozen Chicken\n  Frozen Chicken is off.") {
		t.Errorf("mode: %s", out)
	}
	if err := runCLI(t, "mode", "on", "Frozen Chicken"); err == nil || !strings.Contains(err.Error(), "end time") {
		t.Errorf("switched on without an end time: %v", err)
	}
	if err := runCLI(t, "mode", "create", "frozen chicken"); err == nil {
		t.Error("a mode took Frozen Chicken's name")
	}
	if err := runCLI(t, "freeze", "--for", "8d", "--yes"); err == nil {
		t.Error("frozen for more than a week")
	}
	if err := runCLI(t, "freeze", "--until", time.Now().Add(8*24*time.Hour).Format("2006-01-02 15:04"), "--yes"); err == nil {
		t.Error("frozen until more than a week ahead")
	}
	mustRun(t, "freeze", "--for", "6d", "--mode", "logout", "--yes")
	if err := runCLI(t, "mode", "off", "freeze"); err == nil {
		t.Error("a running freeze was switched off")
	}
	state := capture(t, "gui-state")
	i := strings.Index(state, `"name":"Frozen Chicken"`)
	if i < 0 || !strings.Contains(state[i:i+400], `"freeze":true`) || !strings.Contains(state[i:i+400], `"running":true`) ||
		!strings.Contains(state[i:i+400], `"freeze_mode":"logout"`) {
		t.Errorf("gui-state: %.400s", state[max(i, 0):])
	}
}
