package cli

import (
	"fmt"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

// R-LOCK-6/7: layered sequential locks on a block.
func TestLayeredBlockLocks(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	boot, _ := db.BootID()

	// Password first, then an hour counted from unlocking it.
	if err := runWith(t, 0, []string{"pw"}, "start", "b", "--password-stdin", "--for", "1h", "--yes"); err != nil {
		t.Fatal(err)
	}
	detail, _ := d.GetBlock("b")
	extras, _ := d.GetLockExtras("b")
	if detail.LockedUntil != 0 || extras.TimerAfter != 3600 || extras.Order != "password,timer" {
		t.Fatalf("start: until %d, %+v", detail.LockedUntil, extras)
	}
	trusted := mustTrusted(t, d)
	if !enforce.BlockLocked(detail, extras, trusted, boot) {
		t.Error("not locked")
	}
	// Only longer: a pending timer can't be shortened or dated.
	if err := runCLI(t, "start", "b", "--for", "30m", "--yes"); err == nil {
		t.Error("pending timer shortened")
	}
	if err := runCLI(t, "start", "b", "--until", "23:59", "--yes"); err == nil {
		t.Error("pending timer given a date")
	}
	mustRun(t, "start", "b", "--for", "2h", "--yes")
	if extras, _ = d.GetLockExtras("b"); extras.TimerAfter != 7200 {
		t.Errorf("extended pending timer = %d", extras.TimerAfter)
	}
	if err := runCLI(t, "stop", "b"); err == nil {
		t.Error("stopped while locked")
	}
	if err := runWith(t, 0, []string{"pw"}, "unlock", "b"); err != nil {
		t.Fatal(err)
	}
	detail, _ = d.GetBlock("b")
	trusted = mustTrusted(t, d)
	if detail.LockedUntil < trusted+7190 || detail.LockedUntil > trusted+7200 {
		t.Errorf("timer after unlocking the password: until %d, now %d", detail.LockedUntil, trusted)
	}
	if err := runCLI(t, "unlock", "b"); err == nil {
		t.Error("unlocked a running timer lock")
	}
	if err := runCLI(t, "stop", "b"); err == nil {
		t.Error("stopped during the timer lock")
	}
	// A dated timer lock simply holds until its date.
	if err := d.DeactivateBlock("b"); err != nil {
		t.Fatal(err)
	}

	// Timer first: unlock refused while it runs, though a password is next.
	if err := runWith(t, 0, []string{"pw"}, "start", "b", "--for", "1h", "--password-stdin", "--yes"); err != nil {
		t.Fatal(err)
	}
	detail, _ = d.GetBlock("b")
	if detail.LockedUntil == 0 {
		t.Fatal("first timer lock not running")
	}
	if err := runWith(t, 0, []string{"pw"}, "unlock", "b"); err == nil {
		t.Error("password unlocked before the timer before it ran out")
	}
	// R-LOCK-7: a lock added later goes last.
	if err := runCLI(t, "start", "b", "--lock", "--lock-chars", "20", "--yes"); err != nil {
		t.Fatal(err)
	}
	if extras, _ = d.GetLockExtras("b"); extras.Order != "timer,password,text" {
		t.Errorf("order after adding a text lock = %q", extras.Order)
	}
	_ = d.DeactivateBlock("b")

	// Restart behind a password: pinned to the boot after the password goes.
	if err := runWith(t, 0, []string{"pw"}, "start", "b", "--password-stdin", "--restart", "--yes"); err != nil {
		t.Fatal(err)
	}
	if extras, _ = d.GetLockExtras("b"); extras.RestartBoot != db.RestartPending {
		t.Fatalf("restart not pending: %q", extras.RestartBoot)
	}
	if err := runWith(t, 0, []string{"pw"}, "unlock", "b"); err != nil {
		t.Fatal(err)
	}
	if extras, _ = d.GetLockExtras("b"); extras.RestartBoot != boot {
		t.Errorf("restart after unlock = %q; want this boot", extras.RestartBoot)
	}
	if err := runCLI(t, "stop", "b"); err == nil {
		t.Error("stopped before a restart")
	}
	_ = d.DeactivateBlock("b")

	// Delay first, then password: the first unlock starts the countdown.
	if err := runWith(t, 0, []string{"pw"}, "start", "b", "--delay", "1m", "--password-stdin", "--yes"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "unlock", "b")
	extras, _ = d.GetLockExtras("b")
	if extras.DelayUntil == 0 || extras.PasswordHash == "" {
		t.Fatalf("after the first unlock: %+v", extras)
	}
	extras.Delay, extras.DelayUntil = 0, 0 // the countdown is over
	_ = d.SetLockExtras("b", extras)
	if err := runWith(t, 0, []string{"wrong"}, "unlock", "b"); err == nil {
		t.Error("wrong password accepted")
	}
	if err := runWith(t, 0, []string{"pw"}, "unlock", "b"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "stop", "b")
}

// R-LOCK-6/7 on setting locks: unlocked one at a time in the flags' order;
// a duration timer behind a password counts from its unlocking.
func TestLayeredSettingLocks(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	if err := runWith(t, 0, []string{"pw"}, "browser", "safesearch", "on", "--password-stdin", "--for", "1h", "--yes"); err != nil {
		t.Fatal(err)
	}
	l, _ := d.GetSettingLock("safesearch")
	if l.LockedUntil != 0 || l.TimerAfter != 3600 || l.Order != "password,timer" {
		t.Fatalf("lock = %+v", l)
	}
	if err := runCLI(t, "browser", "safesearch", "off"); err == nil {
		t.Error("turned off while locked")
	}
	if err := runCLI(t, "browser", "safesearch", "on", "--for", "30m", "--yes"); err == nil {
		t.Error("waiting timer shortened")
	}
	if err := runWith(t, 0, []string{"pw"}, "browser", "unlock", "safesearch"); err != nil {
		t.Fatal(err)
	}
	trusted := mustTrusted(t, d)
	l, _ = d.GetSettingLock("safesearch")
	if l.PasswordHash != "" || l.TimerAfter != 0 || l.LockedUntil < trusted+3590 {
		t.Fatalf("after unlocking the password: %+v (now %d)", l, trusted)
	}
	if v, _ := d.GetSetting("browser.safesearch"); v != "on" {
		t.Error("turned off while its timer lock runs")
	}
	if err := runCLI(t, "browser", "unlock", "safesearch"); err == nil {
		t.Error("unlocked during the timer lock")
	}

	// Timer first: the password behind it can't be unlocked yet.
	if err := runWith(t, 0, []string{"pw"}, "strict", "on", "--for", "1h", "--password-stdin", "--yes"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"pw"}, "strict", "unlock"); err == nil {
		t.Error("strict mode's password unlocked before its timer ran out")
	}
	l, _ = d.GetSettingLock(enforce.StrictLockName)
	l.LockedUntil = trusted - 1 // the timer ran out
	_ = d.SetSettingLock(l)
	if err := runWith(t, 0, []string{"pw"}, "strict", "unlock"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "strict", "off")
}

// §3.4 range lock from the CLI: while on, stop and unlock are refused.
func TestRangeLockCLI(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "r")
	mustRun(t, "block", "add", "r", "reddit.com")
	if err := runCLI(t, "start", "r", "--range", "daily 25:00-26:00", "--yes"); err == nil {
		t.Error("a bad range was accepted")
	}
	// Daily, all but one minute from now: on now.
	now := time.Now()
	from := now.Add(-time.Minute)
	to := now.Add(2 * time.Minute)
	spec := fmt.Sprintf("daily %02d:%02d-%02d:%02d", from.Hour(), from.Minute(), to.Hour(), to.Minute())
	mustRun(t, "start", "r", "--range", spec, "--yes")
	if e, _ := d.GetLockExtras("r"); e.RangeDays == 0 {
		t.Fatalf("no range lock stored: %+v", e)
	}
	if err := runCLI(t, "stop", "r"); err == nil {
		t.Error("stopped during its range lock")
	}
	if err := runCLI(t, "unlock", "r"); err == nil {
		t.Error("unlocked during its range lock")
	}
	// Inverted: off now, so it stops.
	_ = d.DeactivateBlock("r")
	mustRun(t, "start", "r", "--range", spec, "--range-invert", "--yes")
	mustRun(t, "stop", "r")
}

// §3.4 range lock on a setting: while on, it can't be turned off.
func TestSettingRangeLock(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	now := time.Now()
	from, to := now.Add(-time.Minute), now.Add(2*time.Minute)
	spec := fmt.Sprintf("daily %02d:%02d-%02d:%02d", from.Hour(), from.Minute(), to.Hour(), to.Minute())
	mustRun(t, "browser", "safesearch", "on", "--range", spec, "--yes")
	if l, _ := d.GetSettingLock("safesearch"); l.RangeDays == 0 {
		t.Fatal("no range lock")
	}
	if err := runCLI(t, "browser", "safesearch", "off"); err == nil {
		t.Error("turned off during its range lock")
	}
	if err := runCLI(t, "browser", "unlock", "safesearch"); err == nil {
		t.Error("unlocked during its range lock")
	}
	mustRun(t, "browser", "web-only", "on", "--range", spec, "--range-invert", "--yes")
	mustRun(t, "browser", "web-only", "off")
}
