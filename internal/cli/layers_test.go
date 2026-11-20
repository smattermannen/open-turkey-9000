package cli

import (
	"fmt"
	"strings"
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
	// Another timer lock is a second one, after the first (§4.6); the
	// pending one is not changed.
	mustRun(t, "start", "b", "--for", "30m", "--yes")
	if extras, _ = d.GetLockExtras("b"); extras.TimerAfter != 3600 || len(extras.Queue) != 1 || extras.Queue[0].For != 1800 {
		t.Errorf("second timer lock: %+v", extras)
	}
	if err := runCLI(t, "stop", "b"); err == nil {
		t.Error("stopped while locked")
	}
	if err := runWith(t, 0, []string{"pw"}, "unlock", "b"); err != nil {
		t.Fatal(err)
	}
	detail, _ = d.GetBlock("b")
	trusted = mustTrusted(t, d)
	if detail.LockedUntil < trusted+3590 || detail.LockedUntil > trusted+3600 {
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
	if err := runWith(t, 0, []string{"pw"}, "browser", "safesearch-google", "on", "--password-stdin", "--for", "1h", "--yes"); err != nil {
		t.Fatal(err)
	}
	l, _ := d.GetSettingLock("safesearch-google")
	if l.LockedUntil != 0 || l.TimerAfter != 3600 || l.Order != "password,timer" {
		t.Fatalf("lock = %+v", l)
	}
	if err := runCLI(t, "browser", "safesearch-google", "off"); err == nil {
		t.Error("turned off while locked")
	}
	// Another timer lock is a second one, behind the first (§3.4).
	mustRun(t, "browser", "safesearch-google", "on", "--for", "30m", "--yes")
	if l, _ := d.GetSettingLock("safesearch-google"); l.TimerAfter != 3600 || len(l.Queue) != 1 || l.Queue[0].For != 1800 {
		t.Errorf("another timer lock: %+v", l)
	}
	if err := runWith(t, 0, []string{"pw"}, "browser", "unlock", "safesearch-google"); err != nil {
		t.Fatal(err)
	}
	trusted := mustTrusted(t, d)
	l, _ = d.GetSettingLock("safesearch-google")
	if l.PasswordHash != "" || l.TimerAfter != 0 || l.LockedUntil < trusted+3590 {
		t.Fatalf("after unlocking the password: %+v (now %d)", l, trusted)
	}
	if v, _ := d.GetSetting("browser.safesearch-google"); v != "on" {
		t.Error("turned off while its timer lock runs")
	}
	if err := runCLI(t, "browser", "unlock", "safesearch-google"); err == nil {
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
	mustRun(t, "browser", "safesearch-google", "on", "--range", spec, "--yes")
	if l, _ := d.GetSettingLock("safesearch-google"); l.RangeDays == 0 {
		t.Fatal("no range lock")
	}
	if err := runCLI(t, "browser", "safesearch-google", "off"); err == nil {
		t.Error("turned off during its range lock")
	}
	if err := runCLI(t, "browser", "unlock", "safesearch-google"); err == nil {
		t.Error("unlocked during its range lock")
	}
	mustRun(t, "browser", "web-only", "on", "--range", spec, "--range-invert", "--yes")
	mustRun(t, "browser", "web-only", "off")
}

// R-LOCK-7: locking a block that is already on adds any kind of lock,
// last in the unlock order; locks already set only get stronger.
func TestLockActiveBlock(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "start", "b")
	boot, _ := db.BootID()

	// The owner's report: a password lock on an enabled block.
	if err := runWith(t, 0, []string{"pw"}, "start", "b", "--password-stdin", "--yes"); err != nil {
		t.Fatalf("password lock on an active block: %v", err)
	}
	extras, _ := d.GetLockExtras("b")
	if extras.PasswordHash == "" || extras.Order != "password" {
		t.Fatalf("after the password lock: %+v", extras)
	}
	// A second password lock is another one (§3.4, owner 2026-11-20): see
	// TestSeveralLocksOfOneKind. Here more kinds go last, in the order given; the restart and the timer
	// lock wait behind the password.
	mustRun(t, "start", "b", "--delay", "10m", "--restart", "--for", "1h", "--lock-words", "--yes")
	detail, _ := d.GetBlock("b")
	extras, _ = d.GetLockExtras("b")
	if extras.Order != "password,delay,restart,timer,text" || extras.Delay != 600 || extras.RestartBoot != db.RestartPending ||
		extras.TimerAfter != 3600 || !detail.Locked || extras.Style == "" {
		t.Fatalf("after adding locks: locked %v, %+v", detail.Locked, extras)
	}
	trusted := mustTrusted(t, d)
	if on := db.LayersOn(detail, extras, trusted, boot); len(on) != 5 || on[0] != db.LayerPassword {
		t.Errorf("locks on = %v", on)
	}
	// A delay or timer lock with locks after it: another one, last
	// (§3.4); shorter is fine, it adds a lock.
	mustRun(t, "start", "b", "--delay", "5m", "--yes")
	// Once a lock waits, every new one waits after it, in order.
	mustRun(t, "start", "b", "--range", "daily 00:00-23:55", "--yes")
	mustRun(t, "start", "b", "--range", "mon 09:00-10:00", "--yes")
	mustRun(t, "start", "b", "--relock", "1h", "--yes")
	extras, _ = d.GetLockExtras("b")
	if extras.Delay != 600 || len(extras.Queue) != 3 || extras.Queue[0].Delay != 300 || extras.Queue[2].RangeFrom != 9*60 ||
		extras.Relock != 3600 || extras.Order != "password,delay,restart,timer,text" {
		t.Errorf("after strengthening: %+v", extras)
	}
	if err := runCLI(t, "stop", "b"); err == nil {
		t.Error("stopped while locked")
	}
}

// §3.4 (owner 2026-11-20): a lock of a kind the block already has is a
// second one, last in the unlock order; it takes its kind's place once
// every lock before it is gone.
func TestSeveralLocksOfOneKind(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	for _, b := range []string{"b", "c", "e", "f"} {
		mustRun(t, "block", "create", b)
		mustRun(t, "block", "add", b, "reddit.com")
	}

	// Two password locks: the first password first, then the second.
	if err := runWith(t, 0, []string{"one"}, "start", "b", "--password-stdin", "--yes"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"two"}, "start", "b", "--password-stdin", "--yes"); err != nil {
		t.Fatalf("second password lock refused: %v", err)
	}
	detail, _ := d.GetBlock("b")
	extras, _ := d.GetLockExtras("b")
	if got := fmt.Sprint(db.Layers(detail, extras)); got != "[password password]" || len(extras.Queue) != 1 {
		t.Fatalf("layers %s, queue %+v", got, extras.Queue)
	}
	if err := runWith(t, 0, []string{"two"}, "unlock", "b"); err == nil {
		t.Error("the second password unlocked first")
	}
	if err := runWith(t, 0, []string{"one"}, "unlock", "b"); err != nil {
		t.Fatal(err)
	}
	detail, _ = d.GetBlock("b")
	extras, _ = d.GetLockExtras("b")
	if len(extras.Queue) != 0 || !db.CheckPassword(extras.PasswordHash, "two") {
		t.Fatalf("the second password lock didn't take the first's place: %+v", extras)
	}
	if err := runCLI(t, "stop", "b"); err == nil {
		t.Error("stopped with the second password lock on")
	}
	if err := runWith(t, 0, []string{"two"}, "unlock", "b"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "stop", "b")

	// Password, timer, another password: the second waits for the timer.
	if err := runWith(t, 0, []string{"one"}, "start", "c", "--password-stdin", "--for", "1h", "--yes"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"two"}, "start", "c", "--password-stdin", "--yes"); err != nil {
		t.Fatal(err)
	}
	// Another timer lock now waits behind the second password too.
	mustRun(t, "start", "c", "--for", "30m", "--yes")
	detail, _ = d.GetBlock("c")
	extras, _ = d.GetLockExtras("c")
	if got := fmt.Sprint(db.Layers(detail, extras)); got != "[password timer password timer]" {
		t.Fatalf("layers %s", got)
	}
	if err := runWith(t, 0, []string{"one"}, "unlock", "c"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"two"}, "unlock", "c"); err == nil {
		t.Error("the second password unlocked during the timer lock")
	}
	// The timer runs out.
	detail, _ = d.GetBlock("c")
	extras, _ = d.GetLockExtras("c")
	detail.LockedUntil = mustTrusted(t, d) - 1
	if err := d.SaveLayers(detail, extras); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"two"}, "unlock", "c"); err != nil {
		t.Fatal(err)
	}
	detail, _ = d.GetBlock("c")
	extras, _ = d.GetLockExtras("c")
	trusted := mustTrusted(t, d)
	if len(extras.Queue) != 0 || detail.LockedUntil < trusted+1790 || detail.LockedUntil > trusted+1800 {
		t.Errorf("the second timer lock didn't start after the second password: until %d, now %d, queue %+v", detail.LockedUntil, trusted, extras.Queue)
	}
	if err := runCLI(t, "stop", "c"); err == nil {
		t.Error("stopped during the second timer lock")
	}

	// A second text lock is another one, not a longer first one.
	mustRun(t, "start", "e", "--lock", "--lock-chars", "20", "--yes")
	mustRun(t, "start", "e", "--lock", "--lock-chars", "30", "--yes")
	detail, _ = d.GetBlock("e")
	extras, _ = d.GetLockExtras("e")
	if detail.LockChars != 20 || len(extras.Queue) != 1 || extras.Queue[0].Kind != db.LayerText || extras.Queue[0].Chars != 30 {
		t.Errorf("second text lock: chars %d, queue %+v", detail.LockChars, extras.Queue)
	}

	// The last lock a timer lock: another timer lock, even a shorter one,
	// counting from the first's end (§4.6, R-LOCK-6); the first is kept.
	mustRun(t, "block", "create", "g")
	mustRun(t, "block", "add", "g", "reddit.com")
	mustRun(t, "start", "g", "--for", "1h", "--yes")
	detail, _ = d.GetBlock("g")
	first := detail.LockedUntil
	mustRun(t, "start", "g", "--for", "30m", "--yes")
	detail, _ = d.GetBlock("g")
	extras, _ = d.GetLockExtras("g")
	if detail.LockedUntil != first || len(extras.Queue) != 1 || extras.Queue[0].For != 1800 {
		t.Fatalf("second timer lock: until %d (was %d), queue %+v", detail.LockedUntil, first, extras.Queue)
	}
	detail.LockedUntil = mustTrusted(t, d) - 1
	if err := d.SaveLayers(detail, extras); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "stop", "g"); err == nil {
		t.Error("stopped before the second timer lock")
	}
	detail, _ = d.GetBlock("g")
	if trusted := mustTrusted(t, d); detail.LockedUntil < trusted+1790 {
		t.Errorf("the second timer lock didn't start: until %d, now %d", detail.LockedUntil, trusted)
	}

	// A second delay lock: another one, even a shorter one.
	mustRun(t, "block", "create", "h")
	mustRun(t, "block", "add", "h", "reddit.com")
	mustRun(t, "start", "h", "--delay", "1h", "--yes")
	mustRun(t, "start", "h", "--delay", "10m", "--yes")
	extras, _ = d.GetLockExtras("h")
	if extras.Delay != 3600 || len(extras.Queue) != 1 || extras.Queue[0].Delay != 600 {
		t.Errorf("second delay lock: %+v", extras)
	}

	// A second range lock: another one, after the first.
	mustRun(t, "block", "create", "r")
	mustRun(t, "block", "add", "r", "reddit.com")
	mustRun(t, "start", "r", "--range", "mon-sun 00:00-23:59", "--yes")
	mustRun(t, "start", "r", "--range", "mon-fri 09:00-17:00", "--yes")
	detail, _ = d.GetBlock("r")
	extras, _ = d.GetLockExtras("r")
	if got := fmt.Sprint(db.Layers(detail, extras)); got != "[range range]" || extras.Queue[0].RangeFrom != 9*60 || extras.RangeFrom != 0 {
		t.Errorf("second range lock: layers %s, %+v", got, extras)
	}

	// Re-lock with two password locks and a timer lock: each is unlocked
	// in turn; once none is on, the re-lock time runs, then the whole
	// configuration comes back in its order (§3.4).
	if err := runWith(t, 0, []string{"one"}, "start", "f", "--password-stdin", "--relock", "30m", "--yes"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"two"}, "start", "f", "--password-stdin", "--yes"); err != nil {
		t.Fatalf("a second password lock beside re-lock refused: %v", err)
	}
	mustRun(t, "start", "f", "--for", "10m", "--yes")
	if err := runWith(t, 0, []string{"one"}, "unlock", "f"); err != nil {
		t.Fatal(err)
	}
	extras, _ = d.GetLockExtras("f")
	if extras.LiftedUntil != 0 || !db.CheckPassword(extras.PasswordHash, "two") || len(extras.Config) != 3 {
		t.Fatalf("after the first password: %+v", extras)
	}
	if err := runCLI(t, "stop", "f"); err == nil {
		t.Error("stopped with the second password lock on")
	}
	if err := runWith(t, 0, []string{"two"}, "unlock", "f"); err != nil {
		t.Fatal(err)
	}
	// The timer lock runs, then is over.
	detail, _ = d.GetBlock("f")
	extras, _ = d.GetLockExtras("f")
	trusted = mustTrusted(t, d)
	if detail.LockedUntil < trusted+590 || extras.LiftedUntil != 0 {
		t.Fatalf("the timer lock didn't start after the passwords: until %d, %+v", detail.LockedUntil, extras)
	}
	detail.LockedUntil = trusted - 1
	if err := d.SaveLayers(detail, extras); err != nil {
		t.Fatal(err)
	}
	status := capture(t, "status")
	extras, _ = d.GetLockExtras("f")
	if extras.LiftedUntil < trusted+1790 || !strings.Contains(status, "locks on again in") {
		t.Fatalf("re-lock time didn't start once nothing was on: %+v\n%s", extras, status)
	}
	// The re-lock time is over: the first password is first again, then
	// the second, then the timer lock.
	extras.LiftedUntil = trusted - 1
	detail, _ = d.GetBlock("f")
	if err := d.SaveLayers(detail, extras); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "stop", "f"); err == nil {
		t.Error("stopped after re-lock brought the locks back")
	}
	detail, _ = d.GetBlock("f")
	extras, _ = d.GetLockExtras("f")
	if got := fmt.Sprint(db.Layers(detail, extras)); got != "[password password timer]" {
		t.Errorf("after re-lock: %s", got)
	}
	if err := runWith(t, 0, []string{"two"}, "unlock", "f"); err == nil {
		t.Error("the second password unlocked first after re-lock")
	}
	if err := runWith(t, 0, []string{"one"}, "unlock", "f"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"two"}, "unlock", "f"); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "stop", "f"); err == nil {
		t.Error("stopped during the timer lock re-lock brought back")
	}

	// Re-lock goes with any lock, not only text or password locks.
	mustRun(t, "block", "create", "k")
	mustRun(t, "block", "add", "k", "reddit.com")
	mustRun(t, "start", "k", "--delay", "5m", "--relock", "1h", "--yes")
	if extras, _ = d.GetLockExtras("k"); extras.Relock != 3600 || len(extras.Config) != 1 {
		t.Errorf("re-lock with a delay lock: %+v", extras)
	}
	if err := runCLI(t, "start", "k2", "--relock", "1h", "--yes"); err == nil {
		t.Error("re-lock without a lock")
	}
}

// §3.4, §4.6: extending a timer lock is its own action; it never adds a
// lock and never shortens one.
func TestExtendTimerLock(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	if err := runCLI(t, "extend", "b", "--for", "1h", "--yes"); err == nil {
		t.Error("extended a block that is off")
	}
	mustRun(t, "start", "b")
	if err := runCLI(t, "extend", "b", "--for", "1h", "--yes"); err == nil {
		t.Error("extended a block with no timer lock")
	}
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	detail, _ := d.GetBlock("b")
	first := detail.LockedUntil
	if err := runCLI(t, "extend", "b", "--for", "30m"); err == nil {
		t.Error("extended without confirmation")
	}
	mustRun(t, "extend", "b", "--for", "30m", "--yes")
	detail, _ = d.GetBlock("b")
	extras, _ := d.GetLockExtras("b")
	if detail.LockedUntil != first+1800 || len(extras.Queue) != 0 {
		t.Errorf("extend --for: until %d (was %d), queue %+v", detail.LockedUntil, first, extras.Queue)
	}
	if err := runCLI(t, "extend", "b", "--until", "2020-01-01 10:00", "--yes"); err == nil {
		t.Error("a timer lock shortened by extend --until")
	}
	mustRun(t, "extend", "b", "--until", "2099-12-24 18:00", "--yes")
	detail, _ = d.GetBlock("b")
	if time.Unix(detail.LockedUntil, 0).Year() != 2099 {
		t.Errorf("extend --until: until %d", detail.LockedUntil)
	}

	// Waiting behind a password lock: only --for, added to its duration.
	mustRun(t, "block", "create", "c")
	mustRun(t, "block", "add", "c", "reddit.com")
	if err := runWith(t, 0, []string{"pw"}, "start", "c", "--password-stdin", "--for", "1h", "--yes"); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "extend", "c", "--until", "2099-12-24 18:00", "--yes"); err == nil {
		t.Error("a waiting timer lock given a date")
	}
	mustRun(t, "extend", "c", "--for", "1h", "--yes")
	if extras, _ = d.GetLockExtras("c"); extras.TimerAfter != 7200 {
		t.Errorf("waiting timer lock: %+v", extras)
	}
}
