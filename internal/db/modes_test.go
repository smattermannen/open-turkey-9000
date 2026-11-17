package db

import (
	"slices"
	"testing"

	"github.com/smattermannen/kc9k/internal/schedule"
)

// Evening and Weekend overlap on Saturday and Sunday evenings: stored, the
// weekend's windows leave out what the evening covers, so a block's
// windows never overlap (R-SCHED-2) and cover the same time.
func TestResolveWindowsClips(t *testing.T) {
	evening := SavedSchedule{ID: 1, Windows: []schedule.Window{{Days: schedule.AllDays, Start: 18 * 60, End: 23 * 60}}}
	weekend := SavedSchedule{ID: 2, Windows: []schedule.Window{{Days: schedule.Weekend, Start: 0, End: 1440}}}
	own := []schedule.Window{{Days: schedule.Monday, Start: 9 * 60, End: 10 * 60}}
	got := ResolveWindows(own, []SavedSchedule{evening, weekend})
	want := []SourcedWindow{
		{Window: own[0]},
		{Window: evening.Windows[0], Source: 1},
		{Window: schedule.Window{Days: schedule.Weekend, Start: 0, End: 18 * 60}, Source: 2},
		{Window: schedule.Window{Days: schedule.Weekend, Start: 23 * 60, End: 1440}, Source: 2},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v", got)
	}
	for i, a := range got {
		for _, b := range got[i+1:] {
			if a.Overlaps(b.Window) {
				t.Errorf("%v overlaps %v", a, b)
			}
		}
	}
	// The same time either way round.
	other := ResolveWindows(own, []SavedSchedule{weekend, evening})
	for day := 0; day < 7; day++ {
		for m := 0; m < 1440; m += 5 {
			in := func(ws []SourcedWindow) bool {
				for _, w := range ws {
					if w.Days&(1<<uint(day)) != 0 && m >= w.Start && m < w.End {
						return true
					}
				}
				return false
			}
			if in(got) != in(other) {
				t.Fatalf("day %d minute %d differs", day, m)
			}
		}
	}
}

// Stored as a whole, a block's windows that stay keep their rows (and so
// their breaks); a schedule's change reaches its copies; deleting a
// schedule leaves its windows to the block as its own.
func TestSetBlockAutomation(t *testing.T) {
	d, _ := openTest(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	evening, err := d.CreateSavedSchedule("Evening")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetSavedScheduleWindows(evening, []schedule.Window{{Days: schedule.Weekdays, Start: 18 * 60, End: 22 * 60}}); err != nil {
		t.Fatal(err)
	}
	own := schedule.Window{Days: schedule.Saturday, Start: 10 * 60, End: 12 * 60}
	spec := AutomationSpec{Own: []schedule.Window{own}, Schedules: []int{evening}}
	if err := d.SetBlockAutomation("b", spec); err != nil {
		t.Fatal(err)
	}
	rows, _ := d.ListSchedules("b")
	if len(rows) != 2 || rows[0].Source != 0 || rows[1].Source != evening {
		t.Fatalf("rows %+v", rows)
	}
	if err := d.SetScheduleBreak(rows[0].ID, 600); err != nil {
		t.Fatal(err)
	}
	got, err := d.BlockAutomationSpec("b")
	if err != nil || !got.Equal(spec) {
		t.Fatalf("spec %+v, %v", got, err)
	}
	// The schedule grows: its copy changes, the own window keeps its row.
	if err := d.SetSavedScheduleWindows(evening, []schedule.Window{{Days: schedule.Weekdays, Start: 17 * 60, End: 22 * 60}}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetBlockAutomation("b", spec); err != nil {
		t.Fatal(err)
	}
	after, _ := d.ListSchedules("b")
	if len(after) != 2 || after[0].ID != rows[0].ID || after[0].Break != 600 || after[1].StartMin != 17*60 {
		t.Fatalf("after %+v", after)
	}
	// Enabled by default: every window is an allow window.
	spec.DefaultOn = true
	if err := d.SetBlockAutomation("b", spec); err != nil {
		t.Fatal(err)
	}
	if allow, _ := d.HasAllowWindows("b"); !allow {
		t.Error("not allow windows")
	}
	// Deleted: the block keeps the windows as its own.
	if err := d.DeleteSavedSchedule(evening); err != nil {
		t.Fatal(err)
	}
	left, _ := d.ListSchedules("b")
	if len(left) != 2 || left[1].Source != 0 || left[1].StartMin != 17*60 {
		t.Fatalf("left %+v", left)
	}
	if s, _ := d.BlockAutomationSpec("b"); len(s.Own) != 2 || len(s.Schedules) != 0 {
		t.Errorf("spec after delete %+v", s)
	}
}

// Monk mode is a mode from the start; modes and schedules take names no
// block has, and blocks none they have; a block is in one mode at most.
func TestModes(t *testing.T) {
	d, _ := openTest(t)
	monk, err := d.MonkMode()
	if err != nil || monk.Name != MonkModeName || !monk.Monk {
		t.Fatalf("monk mode %+v, %v", monk, err)
	}
	if err := d.DeleteMode(monk.ID); err == nil {
		t.Error("monk mode deleted")
	}
	if err := d.CreateBlock("work"); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateMode("WORK"); err == nil {
		t.Error("a mode took a block's name")
	}
	if err := d.CreateMode("Evening"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateSavedSchedule("evening"); err == nil {
		t.Error("a schedule took a mode's name")
	}
	if err := d.CreateBlock("Evening"); err == nil {
		t.Error("a block took a mode's name")
	}
	if err := d.RenameBlock("work", "evening"); err == nil {
		t.Error("a block renamed to a mode's name")
	}
	if err := d.CreateMode("Night"); err != nil {
		t.Fatal(err)
	}
	evening, _ := d.GetMode("evening")
	night, _ := d.GetMode("Night")
	for _, id := range []int{evening.ID, night.ID} {
		if err := d.SetBlockMode("work", id); err != nil {
			t.Fatal(err)
		}
	}
	if m, ok, _ := d.ModeOf("work"); !ok || m.Name != "Night" {
		t.Errorf("in %v %v", m.Name, ok)
	}
	if e, _ := d.GetMode("Evening"); len(e.Blocks) != 0 {
		t.Errorf("in two modes: %v", e.Blocks)
	}
	spec := AutomationSpec{Own: []schedule.Window{{Days: schedule.AllDays, Start: 60, End: 120}}}
	if err := d.SetModeAutomation(night.ID, spec); err != nil {
		t.Fatal(err)
	}
	if n, _ := d.GetMode("night"); !n.Spec.Equal(spec) || !slices.Equal(n.Blocks, []string{"work"}) {
		t.Errorf("night %+v", n)
	}
	if err := d.DeleteMode(night.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := d.ModeOf("work"); ok {
		t.Error("still in a deleted mode")
	}
}

// A block's own window added or removed the old way (queued edits from
// older versions) re-clips its schedules' copies: no overlap, and a
// removed own window's time is the schedule's again.
func TestOwnWindowReclips(t *testing.T) {
	d, _ := openTest(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	id, _ := d.CreateSavedSchedule("Evening")
	_ = d.SetSavedScheduleWindows(id, []schedule.Window{{Days: schedule.Monday, Start: 18 * 60, End: 22 * 60}})
	if err := d.SetBlockAutomation("b", AutomationSpec{Schedules: []int{id}}); err != nil {
		t.Fatal(err)
	}
	if err := d.AddSchedule("b", schedule.Window{Days: schedule.Monday, Start: 20 * 60, End: 23 * 60}); err != nil {
		t.Fatal(err)
	}
	rows, _ := d.ListSchedules("b")
	for i, a := range rows {
		for _, b := range rows[i+1:] {
			if a.Window().Overlaps(b.Window()) {
				t.Errorf("%v overlaps %v", a.Window(), b.Window())
			}
		}
	}
	own := 0
	for _, r := range rows {
		if r.Source == 0 {
			own = r.Num
		}
	}
	if err := d.RemoveSchedule("b", own); err != nil {
		t.Fatal(err)
	}
	rows, _ = d.ListSchedules("b")
	if len(rows) != 1 || rows[0].StartMin != 18*60 || rows[0].EndMin != 22*60 || rows[0].Source != id {
		t.Errorf("after removing the own window: %+v", rows)
	}
}
