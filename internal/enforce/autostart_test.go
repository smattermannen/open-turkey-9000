package enforce

import (
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// §3.6 autostart: once, at a time of day, at sign-in (a new desktop
// session) and at the next window; never on the daemon's first look.
func TestAutostarts(t *testing.T) {
	d := testDB(t)
	sessions := map[string]bool{"c1": true}
	old := blocker.GraphicalSessions
	blocker.GraphicalSessions = func() map[string]bool { return sessions }
	t.Cleanup(func() { blocker.GraphicalSessions = old; autostartWatch.started = false })
	autostartWatch.started = false
	for _, n := range []string{"once", "at", "signin", "win"} {
		if err := d.CreateBlock(n); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 11, 16, 8, 2, 0, 0, time.Local)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(d.AddAutostart(db.Autostart{Block: "once", Kind: db.AutostartOnce, At: now.Add(-time.Minute).Unix()}))
	must(d.AddAutostart(db.Autostart{Block: "at", Kind: db.AutostartAt, Minute: 8 * 60, Days: schedule.AllDays}))
	must(d.AddAutostart(db.Autostart{Block: "signin", Kind: db.AutostartSignIn}))
	must(d.AddAutostart(db.Autostart{Block: "win", Kind: db.AutostartWindow}))
	must(d.AddSchedule("win", schedule.Window{Days: schedule.AllDays, Start: 9 * 60, End: 10 * 60}))
	active := func(n string) bool { a, _ := d.IsBlockActive(n); return a }

	runAutostarts(d, now) // first look: nothing
	if active("once") || active("at") {
		t.Fatal("started on the first look")
	}
	runAutostarts(d, now)
	if !active("once") || !active("at") || active("signin") || active("win") {
		t.Fatalf("once %v at %v signin %v win %v", active("once"), active("at"), active("signin"), active("win"))
	}
	if rules, _ := d.Autostarts("once"); len(rules) != 0 {
		t.Error("a once rule stayed")
	}
	sessions = map[string]bool{"c1": true, "c2": true}
	runAutostarts(d, now)
	if !active("signin") {
		t.Error("sign-in didn't start it")
	}
	runAutostarts(d, now.Add(time.Hour)) // 9:02: the window opened
	if !active("win") {
		t.Error("the next window didn't start it")
	}
	// The at rule fires once a day: switched off, it stays off that day.
	must(d.DeactivateBlock("at"))
	runAutostarts(d, now.Add(3*time.Minute))
	if active("at") {
		t.Error("an at rule fired twice in a day")
	}
}
