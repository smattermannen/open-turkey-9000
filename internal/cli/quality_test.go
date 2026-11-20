package cli

import (
	"strings"
	"testing"
)

// §3.6 Schedule (2026-11-17): a schedule kept outside blocks is renamed in
// place (the blocks using it keep it); a name another schedule has is
// refused, a change of case is not.
func TestScheduleRename(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "schedule", "create", "Evening")
	mustRun(t, "schedule", "create", "Weekend")
	mustRun(t, "schedule", "add", "Evening", "--days", "daily", "--from", "18:00", "--to", "22:00")
	mustRun(t, "block", "create", "b")
	mustRun(t, "schedule", "apply", "b", "Evening")
	if err := runCLI(t, "schedule", "rename", "Evening", "Weekend"); err == nil {
		t.Error("renamed onto another schedule's name")
	}
	if err := runCLI(t, "schedule", "rename", "Nothing", "X"); err == nil {
		t.Error("renamed a schedule that isn't there")
	}
	mustRun(t, "schedule", "rename", "Evening", "evening")
	mustRun(t, "schedule", "rename", "evening", "Nights")
	if _, err := d.GetSavedSchedule("Nights"); err != nil {
		t.Fatalf("not renamed: %v", err)
	}
	if w, _ := d.ListSchedules("b"); len(w) != 1 {
		t.Errorf("the block lost the renamed schedule's window: %+v", w)
	}
}

// §4.4: list presets are listed and shown with where they look and how much
// they hold.
func TestListPresetListAndShow(t *testing.T) {
	testDB(t)
	notTerminal(t)
	if out := capture(t, "list-preset", "list"); !strings.Contains(out, "No list presets") {
		t.Errorf("none yet: %s", out)
	}
	mustRun(t, "list-preset", "create", "Games", "--title")
	mustRun(t, "list-preset", "add", "Games", "steam", "lutris")
	out := capture(t, "list-preset", "list")
	if !strings.Contains(out, "Games looks at: title; 2 entries") {
		t.Errorf("list: %s", out)
	}
	out = capture(t, "list-preset", "show", "Games")
	if !strings.Contains(out, "Games looks at: title; 2 entries") || !strings.Contains(out, "  steam") || !strings.Contains(out, "  lutris") {
		t.Errorf("show: %s", out)
	}
	if err := runCLI(t, "list-preset", "show", "Nothing"); err == nil {
		t.Error("showed a preset that isn't there")
	}
}
