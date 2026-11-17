package enforce

import (
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// R-MODE-4 (owner 2026-11-17): a Mode action runs a mode of blocks now for a
// time, whatever its automation says: its blocks are switched on and
// enforce, held for the time, and those that were off go off again after it.
func TestModeRun(t *testing.T) {
	d := testDB(t)
	newFake()
	for _, b := range []string{"a", "c"} {
		if err := d.CreateBlock(b); err != nil {
			t.Fatal(err)
		}
		detail, _ := d.GetBlock(b)
		l, _ := detail.Lists.Find(db.DefaultBlockedList)
		if err := d.AddListEntries(l.ID, []string{b + ".com"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.CreateMode("Work"); err != nil {
		t.Fatal(err)
	}
	m, _ := d.GetMode("Work")
	// Its window is never now: the automation alone would lift it.
	now := time.Now()
	other := 1 << uint((now.Weekday()+3)%7)
	spec := db.AutomationSpec{Own: []schedule.Window{{Days: other, Start: 0, End: 60}}}
	if err := d.SetModeAutomation(m.ID, spec); err != nil {
		t.Fatal(err)
	}
	for _, b := range []string{"a", "c"} {
		if err := d.SetBlockMode(b, m.ID); err != nil {
			t.Fatal(err)
		}
		if err := d.SetBlockAutomation(b, spec); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.ActivateBlock("c", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := StartModeRun(d, "Work", 2*time.Hour); err != nil {
		t.Fatal(err)
	}
	trusted := trustedNow(t, d)
	active, _ := d.GetActiveBlocks()
	enabled := map[string]bool{}
	for _, b := range enabledBlocks(d, active, trusted) {
		enabled[b.BlockName] = true
	}
	if !enabled["a"] || !enabled["c"] {
		t.Errorf("not enforcing while it runs: %v", enabled)
	}
	if until := ModeRunUntil(d, "a", trusted); until < trusted+7000 {
		t.Errorf("a held until %d (now %d)", until, trusted)
	}
	if locked, _ := BlocksLocked(d, now, trusted); !locked {
		t.Error("a running mode doesn't count as locked")
	}
	// A shorter run never shortens it.
	if err := StartModeRun(d, "Work", time.Minute); err != nil {
		t.Fatal(err)
	}
	if until := ModeRunUntil(d, "a", trusted); until < trusted+7000 {
		t.Errorf("shortened to %d", until)
	}
	// Over: a (off before) goes off again, c stays on.
	runs := d.ModeRuns()
	runs[0].Until = trusted - 1
	if err := d.SaveModeRuns(runs); err != nil {
		t.Fatal(err)
	}
	endModeRuns(d, trusted)
	if on, _ := d.IsBlockActive("a"); on {
		t.Error("a stayed on after the run")
	}
	if on, _ := d.IsBlockActive("c"); !on {
		t.Error("c, on before, went off")
	}
	if len(d.ModeRuns()) != 0 {
		t.Errorf("the run is still there: %+v", d.ModeRuns())
	}
	if err := StartModeRun(d, "Nothing", time.Hour); err == nil {
		t.Error("a mode that doesn't exist ran")
	}
}

// §3.9 Custom action (R-CUSTOM-1/2): run through Run, at most once a minute
// per block and trigger from a page or a trigger.
func TestCustomAction(t *testing.T) {
	d := testDB(t)
	var ran []string
	old := runCustom
	runCustom = func(param string) error { ran = append(ran, param); return nil }
	t.Cleanup(func() { runCustom = old })
	param := db.CustomParam([]db.CustomItem{{Open: "https://example.org"}, {Run: "notify-send hi"}})
	if err := Run(d, Consequence{Kind: db.ConsequenceCustom, Param: param}, "test"); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 1 || ran[0] != param {
		t.Errorf("ran %v", ran)
	}
	if err := Run(d, Consequence{Kind: db.ConsequenceCustom, Param: "[]"}, "test"); err == nil {
		t.Error("an empty custom action ran")
	}
	now := time.Now()
	if !customDue("b\x00apps", now) || customDue("b\x00apps", now.Add(30*time.Second)) || !customDue("b\x00apps", now.Add(61*time.Second)) {
		t.Error("custom actions aren't held to once a minute")
	}
}
