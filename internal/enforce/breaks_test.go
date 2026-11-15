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

// R-SCHED-1, §3.3: the schedule never switches a block on or off; it only
// lifts an active block outside its windows. An inactive block never
// enforces, and a block switched off in its window stays off.
func TestScheduleOnlyLifts(t *testing.T) {
	d := testDB(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, d, "b", db.DefaultBlockedList, "reddit.com")
	monday := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	if err := d.AddSchedule("b", schedule.Window{Days: 1 << uint(time.Monday), Start: 9 * 60, End: 11 * 60}); err != nil {
		t.Fatal(err)
	}
	if err := reconcileSchedulesAt(d, monday, monday.Unix()); err != nil {
		t.Fatal(err)
	}
	if active, _ := d.IsBlockActive("b"); active {
		t.Fatal("the schedule switched a block on")
	}
	if scheduleLifted(d, monday)["b"] {
		t.Error("lifted inside its window")
	}
	if !scheduleLifted(d, monday.Add(2*time.Hour))["b"] {
		t.Error("enabled outside its window")
	}
	if err := d.ActivateBlock("b", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	after := monday.Add(2 * time.Hour)
	if err := reconcileSchedulesAt(d, after, after.Unix()); err != nil {
		t.Fatal(err)
	}
	if active, _ := d.IsBlockActive("b"); !active {
		t.Error("the schedule switched an active block off")
	}
	// Allow windows lift a continuous block only inside them.
	if err := d.CreateBlock("c"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddSchedule("c", schedule.Window{Days: 1 << uint(time.Monday), Start: 12 * 60, End: 13 * 60, Allow: true}); err != nil {
		t.Fatal(err)
	}
	if scheduleLifted(d, monday)["c"] || !scheduleLifted(d, monday.Add(150*time.Minute))["c"] {
		t.Error("allow window lifting wrong")
	}
	if scheduleLifted(d, monday)["none"] {
		t.Error("a block without windows lifted")
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

// R-LOCK-6 in the daemon: once the password before it is unlocked, the
// timer lock behind it starts counting (finishDelayedStops moves layers).
func TestDaemonMovesLayers(t *testing.T) {
	d := testDB(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, d, "b", db.DefaultBlockedList, "reddit.com")
	if err := d.ActivateBlock("b", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := d.SetLockExtras("b", db.LockExtras{Order: "password,timer", TimerAfter: 600}); err != nil {
		t.Fatal(err)
	}
	finishDelayedStops(d, 5000)
	if b, _ := d.GetBlock("b"); b.LockedUntil != 5600 {
		t.Fatalf("timer after the password: until %d", b.LockedUntil)
	}
	boot, _ := db.BootID()
	detail, _ := d.GetBlock("b")
	e, _ := d.GetLockExtras("b")
	if !BlockLocked(detail, e, 5000, boot) || BlockLocked(detail, e, 5601, boot) {
		t.Error("timer lock not holding exactly its time")
	}
}

// §3.6 delay break: asked for, it begins after its countdown and lifts the
// block for its length; the locks stay.
func TestDelayBreak(t *testing.T) {
	d := testDB(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, d, "b", db.DefaultBlockedList, "reddit.com")
	if err := d.ActivateBlock("b", true, 50, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := d.SetBreaks("b", db.Breaks{DelayLength: 600, DelayWait: 1800, DelayAt: 5000}); err != nil {
		t.Fatal(err)
	}
	finishDelayedStops(d, 4999)
	if until, _ := d.PausedUntil("b"); until != 0 {
		t.Fatal("began before its countdown ended")
	}
	finishDelayedStops(d, 5000)
	if until, _ := d.PausedUntil("b"); until != 5600 {
		t.Fatalf("paused until %d; want 5600", until)
	}
	if k, _ := d.GetBreaks("b"); k.DelayAt != 0 || k.DelayLength != 600 {
		t.Errorf("after: %+v", k)
	}
	if detail, _ := d.GetBlock("b"); !detail.Locked {
		t.Error("the lock went")
	}
}

// R-SCHED-3: the Pomodoro rhythm starts afresh, work first, each time the
// block is enabled; not on the daemon's first look.
func TestPomodoroRestarts(t *testing.T) {
	d := testDB(t)
	pomodoroWatch = nil
	t.Cleanup(func() { pomodoroWatch = nil })
	if err := d.CreateBlock("p"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetPomodoro("p", db.Pomodoro{On: 1500, Off: 300, Anchor: 100}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	restartPomodoros(d, now, 1000) // first look, block off
	if err := d.ActivateBlock("p", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	restartPomodoros(d, now, 2000)
	if p, _ := d.GetPomodoro("p"); p.Anchor != 2000 {
		t.Fatalf("anchor after switching on = %d; want 2000", p.Anchor)
	}
	restartPomodoros(d, now, 2005)
	if p, _ := d.GetPomodoro("p"); p.Anchor != 2000 {
		t.Error("re-anchored while it stayed enabled")
	}
	_ = d.SetPause("p", 3000) // a break lifts it
	restartPomodoros(d, now, 2500)
	restartPomodoros(d, now, 3001) // the break ended
	if p, _ := d.GetPomodoro("p"); p.Anchor != 3001 {
		t.Errorf("anchor after a break = %d; want 3001", p.Anchor)
	}
}
