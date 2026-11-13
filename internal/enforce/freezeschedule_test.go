package enforce

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
)

func TestReconcileFreezeSchedules(t *testing.T) {
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if err := d.AddFreezeSchedule(schedule.Window{Days: schedule.AllDays, Start: 22 * 60, End: 23 * 60}, "logout", db.NoScheduleLock); err != nil {
		t.Fatal(err)
	}
	evening := time.Date(2026, 11, 9, 22, 15, 0, 0, time.Local)
	end := time.Date(2026, 11, 9, 23, 0, 0, 0, time.Local)

	// Outside the window nothing happens.
	if err := reconcileFreezeSchedules(d, evening.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if until, _ := d.FreezeUntil(); until != 0 {
		t.Fatalf("frozen outside the window until %d", until)
	}

	// Inside it, the freeze runs to the window's end, in its mode.
	if err := reconcileFreezeSchedules(d, evening); err != nil {
		t.Fatal(err)
	}
	until, _ := d.FreezeUntil()
	mode, _ := d.FreezeMode()
	if until != end.Unix() || mode != "logout" {
		t.Fatalf("freeze until %v (%s), want %v (logout)", time.Unix(until, 0), mode, end)
	}

	// A longer manual freeze is never shortened.
	longer := end.Add(2 * time.Hour).Unix()
	if err := d.SetFreeze(longer, "lock"); err != nil {
		t.Fatal(err)
	}
	if err := reconcileFreezeSchedules(d, evening.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if until, _ := d.FreezeUntil(); until != longer {
		t.Fatalf("a longer freeze was shortened to %v", time.Unix(until, 0))
	}
}

// R-FREEZE-2: a freeze schedule is held only by its lock, from the lock's
// lead time before its window until it ends.
func TestFreezeScheduleHeld(t *testing.T) {
	w := schedule.Window{Days: schedule.AllDays, Start: 22 * 60, End: 23 * 60}
	day := func(h, m int) time.Time { return time.Date(2026, 11, 9, h, m, 0, 0, time.Local) }
	locked := db.FreezeSchedule{Window: w, LockLead: 120}
	if FreezeScheduleHeld(locked, day(19, 30)) {
		t.Error("held 2.5 hours before the window")
	}
	if !FreezeScheduleHeld(locked, day(20, 30)) || !FreezeScheduleHeld(locked, day(22, 30)) {
		t.Error("not held 1.5 hours before, or during, the window")
	}
	free := db.FreezeSchedule{Window: w, LockLead: db.NoScheduleLock}
	if FreezeScheduleHeld(free, day(21, 59)) || FreezeScheduleHeld(free, day(22, 30)) {
		t.Error("a schedule without a lock was held")
	}
}
