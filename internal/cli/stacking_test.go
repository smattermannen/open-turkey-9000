package cli

import (
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

// §3.4, §4.6: a lock flag given twice in one start is two locks of its kind
// (it used to be one: the last won). Each kind's first lock takes its place;
// from the first second one on, the locks wait in the queue, in the order
// given. Every lock is confirmed (R-LOCK-12) and kept for re-lock.
func TestSeveralOfOneKindInOneStart(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	for _, b := range []string{"b", "c", "p"} {
		mustRun(t, "block", "create", b)
		mustRun(t, "block", "add", b, "reddit.com")
	}
	err := runCLI(t, "start", "b", "--for", "1h", "--lock", "--for", "2h")
	if err == nil || !strings.Contains(err.Error(), "3 of 3: another timer lock") {
		t.Fatalf("unconfirmed: %v", err)
	}
	mustRun(t, "start", "b", "--for", "1h", "--lock", "--for", "2h", "--yes")
	detail, _ := d.GetBlock("b")
	e, _ := d.GetLockExtras("b")
	trusted := mustTrusted(t, d)
	if !detail.Locked || detail.LockedUntil < trusted+3590 || detail.LockedUntil > trusted+3600 {
		t.Errorf("placed: %+v", detail)
	}
	if len(e.Queue) != 1 || e.Queue[0].Kind != db.LayerTimer || e.Queue[0].For != 7200 || len(e.Config) != 3 {
		t.Errorf("queue %+v, config %+v", e.Queue, e.Config)
	}
	if got := db.Layers(detail, e); strings.Join(got, ",") != "timer,text,timer" {
		t.Errorf("unlock order = %v", got)
	}

	// Two of a kind first: everything after the second waits, in order.
	mustRun(t, "start", "c", "--for", "1h", "--for", "2h", "--lock", "--yes")
	detail, _ = d.GetBlock("c")
	e, _ = d.GetLockExtras("c")
	if detail.Locked || len(e.Queue) != 2 || e.Queue[0].Kind != db.LayerTimer || e.Queue[1].Kind != db.LayerText {
		t.Errorf("c: locked %v, queue %+v", detail.Locked, e.Queue)
	}

	// Two password locks in one start: one password read for each.
	if err := runWith(t, 0, []string{"one", "two"}, "start", "p", "--password-stdin", "--password-stdin", "--yes"); err != nil {
		t.Fatal(err)
	}
	e, _ = d.GetLockExtras("p")
	if !db.CheckPassword(e.PasswordHash, "one") || len(e.Queue) != 1 || !db.CheckPassword(e.Queue[0].PasswordHash, "two") {
		t.Fatalf("passwords: %+v", e)
	}
	if err := runWith(t, 0, []string{"two"}, "unlock", "p"); err == nil {
		t.Error("the second password unlocked the first lock")
	}
	if err := runWith(t, 0, []string{"one"}, "unlock", "p"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"two"}, "unlock", "p"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "stop", "p")

	// The same flags on an active block add each as a lock.
	mustRun(t, "start", "p", "--range", "mon 03:00-03:05", "--range", "tue 03:00-03:05", "--yes")
	e, _ = d.GetLockExtras("p")
	if e.RangeDays == 0 || len(e.Queue) != 1 || e.Queue[0].Kind != db.LayerRange {
		t.Errorf("two range locks: %+v", e)
	}
}

// §3.4 on settings (§6 2026-11-21 By Opus): a setting's locks stack like a
// block's, several of one kind in one command included.
func TestSettingLocksStack(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	if err := runWith(t, 0, []string{"a", "b"}, "strict", "on", "--password-stdin", "--password-stdin", "--yes"); err != nil {
		t.Fatal(err)
	}
	l, _ := d.GetSettingLock(enforce.StrictLockName)
	if !db.CheckPassword(l.PasswordHash, "a") || len(l.Queue) != 1 || !db.CheckPassword(l.Queue[0].PasswordHash, "b") || len(l.Config) != 2 {
		t.Fatalf("strict lock = %+v", l)
	}
	if !strings.Contains(capture(t, "gui-state"), `"lock_order":["password","password"]`) {
		t.Error("gui-state doesn't show both password locks")
	}
	if err := runWith(t, 0, []string{"a"}, "strict", "unlock"); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "strict", "off"); err == nil {
		t.Error("strict mode off with its second password lock on")
	}
	if err := runWith(t, 0, []string{"b"}, "strict", "unlock"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "strict", "off")

	// A second timer lock on a browser setting waits behind the first.
	mustRun(t, "browser", "web-only", "on", "--for", "1h", "--yes")
	mustRun(t, "browser", "web-only", "on", "--for", "1h", "--yes")
	l, _ = d.GetSettingLock("web-only")
	if len(l.Queue) != 1 || l.Queue[0].For != 3600 {
		t.Errorf("web-only lock = %+v", l)
	}
	l.LockedUntil = mustTrusted(t, d) - 1 // the first ran out: the second starts
	if err := d.SetSettingLock(l); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "browser", "web-only", "off"); err == nil {
		t.Error("turned off while the second timer lock runs")
	}
	if l, _ = d.GetSettingLock("web-only"); len(l.Queue) != 0 || l.LockedUntil < mustTrusted(t, d)+3590 {
		t.Errorf("the second timer lock didn't start: %+v", l)
	}
}

// §3.4 re-lock on a setting brings back its whole configuration, every lock
// in its order, as on a block.
func TestSettingRelockWholeConfiguration(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	if err := runWith(t, 0, []string{"a", "b"}, "browser", "web-only", "on", "--password-stdin", "--restart", "--password-stdin",
		"--relock", "10m", "--yes"); err != nil {
		t.Fatal(err)
	}
	l, _ := d.GetSettingLock("web-only")
	if len(l.Config) != 3 || l.Relock != 600 {
		t.Fatalf("lock = %+v", l)
	}
	// Unlock everything: the password, the restart lock (a restart), the
	// second password.
	if err := runWith(t, 0, []string{"a"}, "browser", "unlock", "web-only"); err != nil {
		t.Fatal(err)
	}
	l, _ = d.GetSettingLock("web-only")
	l.RestartBoot = "an-earlier-boot" // the computer restarted
	if err := d.SetSettingLock(l); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"b"}, "browser", "unlock", "web-only"); err != nil {
		t.Fatal(err)
	}
	trusted := mustTrusted(t, d)
	l, _ = d.GetSettingLock("web-only")
	if !l.PendingRelock(trusted) || l.InForce(trusted, "boot") || l.RelockAt(trusted) < trusted+590 {
		t.Fatalf("after unlocking all: %+v", l)
	}
	// The re-lock time is over: every lock is back, in its order.
	l.LiftedUntil = trusted - 1
	if err := d.SetSettingLock(l); err != nil {
		t.Fatal(err)
	}
	out := capture(t, "browser")
	if !strings.Contains(out, "web-only on  (locked: password → restart lock → password") {
		t.Errorf("after the re-lock time:\n%s", out)
	}
	if err := runWith(t, 0, []string{"b"}, "browser", "unlock", "web-only"); err == nil {
		t.Error("the second password opened the first lock again")
	}
	if err := runWith(t, 0, []string{"a"}, "browser", "unlock", "web-only"); err != nil {
		t.Fatal(err)
	}
}

// A hotkey keeps every lock flag, a repeated one too; it keeps one password.
func TestHotkeyKeepsRepeatedLockFlags(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	fakeDesktopHotkeys(t)
	blockWithSite(t)
	mustRun(t, "hotkey", "add", "ctrl+alt+x", "b", "--for", "1h", "--lock", "--for", "2h", "--yes")
	keys, _ := d.Hotkeys()
	if len(keys) != 1 || strings.Join(keys[0].Actions[0].LockArgs, " ") != "--for 1h --lock --for 2h" {
		t.Fatalf("hotkeys = %+v", keys)
	}
	if err := runWith(t, 0, []string{"a", "b"}, "hotkey", "add", "ctrl+alt+y", "b", "--password-stdin", "--password-stdin", "--yes"); err == nil {
		t.Error("a hotkey with two password locks")
	}
	mustRun(t, "hotkey", "run", "1")
	e, _ := d.GetLockExtras("b")
	if len(e.Queue) != 1 || e.Queue[0].For != 7200 {
		t.Errorf("after the press: %+v", e)
	}
}
