package enforce

import (
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// R-BREAK-2: a reward is earned only while the computer is in use.
func TestRewardOnlyWhileInUse(t *testing.T) {
	d := testDB(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetBreaks("b", db.Breaks{Allowance: 600, Per: 3600}); err != nil {
		t.Fatal(err)
	}
	inUse := false
	oldUse, oldLast := computerInUse, lastEarned
	computerInUse = func() bool { return inUse }
	t.Cleanup(func() { computerInUse, lastEarned = oldUse, oldLast })

	enforced := []db.ActiveBlockDetail{{BlockName: "b"}}
	lastEarned = 0
	earnBreaks(d, enforced, 1000) // the first cycle only starts the clock
	earnBreaks(d, enforced, 1036)
	if k, _ := d.GetBreaks("b"); k.BalanceMs != 0 {
		t.Fatalf("idle computer earned %d ms", k.BalanceMs)
	}
	inUse = true
	earnBreaks(d, enforced, 1072)
	if k, _ := d.GetBreaks("b"); k.BalanceMs != 6000 {
		t.Fatalf("36 s in use earned %d ms; want 6000", k.BalanceMs)
	}
}

// §3.4: when a delay lock's countdown is over, the lock is gone and the
// block stays on.
func TestDelayLockEndsOnlyTheLock(t *testing.T) {
	d := testDB(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	if err := d.ActivateBlock("b", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := d.SetLockExtras("b", db.LockExtras{Delay: 60, DelayUntil: 1000}); err != nil {
		t.Fatal(err)
	}
	finishDelayedStops(d, 999)
	if e, _ := d.GetLockExtras("b"); e.Delay != 60 {
		t.Fatal("the delay lock was lifted early")
	}
	finishDelayedStops(d, 1000)
	e, _ := d.GetLockExtras("b")
	if b, _ := d.GetBlock("b"); !b.Active || e.Delay != 0 || e.DelayUntil != 0 {
		t.Errorf("after the countdown: active %v, delay %d until %d", b.Active, e.Delay, e.DelayUntil)
	}
}

// R-LOCK-1: a block without a schedule lock that is switched off during its
// window stays off until the window ends, then its schedule works again.
func TestStoppedInWindowStaysOff(t *testing.T) {
	d := testDB(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, d, "b", db.DefaultBlockedList, "reddit.com")
	monday := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	if err := d.AddSchedule("b", schedule.Window{Days: 1 << uint(time.Monday), Start: 9 * 60, End: 11 * 60}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetSetting(StoppedInWindowKey("b"), "1"); err != nil {
		t.Fatal(err)
	}
	if err := reconcileSchedulesAt(d, monday, monday.Unix()); err != nil {
		t.Fatal(err)
	}
	if active, _ := d.IsBlockActive("b"); active {
		t.Fatal("the schedule switched a block on that was switched off in its window")
	}
	after := monday.Add(2 * time.Hour)
	if err := reconcileSchedulesAt(d, after, after.Unix()); err != nil {
		t.Fatal(err)
	}
	if v, _ := d.GetSetting(StoppedInWindowKey("b")); v != "" {
		t.Error("the mark stayed after the window")
	}
	next := monday.AddDate(0, 0, 7)
	if err := reconcileSchedulesAt(d, next, next.Unix()); err != nil {
		t.Fatal(err)
	}
	if active, _ := d.IsBlockActive("b"); !active {
		t.Error("the next window didn't switch the block on")
	}
}

// §3.4: a schedule lock counts as a lock for strict mode and the settings
// that hold while anything is locked; a schedule alone doesn't.
func TestScheduleLockIsALock(t *testing.T) {
	d := testDB(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	start := (now.Hour()*60 + now.Minute() + 30) % (24 * 60) / 5 * 5
	w := schedule.Window{Days: schedule.AllDays, Start: start, End: min(start+60, 24*60)}
	if w.End <= w.Start {
		t.Skip("too close to midnight for this window")
	}
	if err := d.AddSchedule("b", w); err != nil {
		t.Fatal(err)
	}
	if locked, _ := BlocksLocked(d, now, now.Unix()); locked {
		t.Fatal("a schedule without a lock counted as a lock")
	}
	if err := d.SetScheduleLock("b", 60); err != nil {
		t.Fatal(err)
	}
	if locked, _ := BlocksLocked(d, now, now.Unix()); !locked {
		t.Error("a schedule lock within its lead time didn't count")
	}
	if err := d.SetScheduleLock("b", 5); err != nil {
		t.Fatal(err)
	}
	if locked, _ := BlocksLocked(d, now, now.Unix()); locked {
		t.Error("a schedule lock outside its lead time counted")
	}
}
