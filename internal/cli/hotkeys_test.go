package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
)

func fakeDesktopHotkeys(t *testing.T) *int {
	t.Helper()
	old, oldAsk := syncDesktopHotkeys, confirmOnDesktop
	syncDesktopHotkeys = func([]blocker.DesktopHotkey) error { return nil }
	asked := 0
	confirmOnDesktop = func(title, text string) bool { asked++; return true }
	t.Cleanup(func() { syncDesktopHotkeys, confirmOnDesktop = old, oldAsk })
	return &asked
}

// §3.4: a timer lock has no upper limit, a hotkey's too (it was capped at
// 24 hours).
func TestHotkeyTimerLockHasNoCap(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	fakeDesktopHotkeys(t)
	blockWithSite(t)
	mustRun(t, "hotkey", "add", "ctrl+alt+s", "b", "--for", "90d", "--yes", "--no-confirm")
	keys, _ := d.Hotkeys()
	if len(keys) != 1 {
		t.Fatalf("hotkeys = %+v", keys)
	}
	mustRun(t, "hotkey", "run", fmt.Sprint(keys[0].ID))
	b, _ := d.GetBlock("b")
	if left := b.LockedUntil - mustTrusted(t, d); !b.Active || left < 90*86400-60 || left > 90*86400 {
		t.Errorf("after the press: active %v, locked for %d s more", b.Active, left)
	}
}

// §3.13: a hotkey takes every lock kind start takes, in the same unlock
// order (R-LOCK-6); a password lock's password is given when it is added.
func TestHotkeyEveryLockKind(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	asked := fakeDesktopHotkeys(t)
	blockWithSite(t)
	if err := runWith(t, 0, []string{"partner"}, "hotkey", "add", "ctrl+alt+w", "b",
		"--lock", "--lock-chars", "20", "--password-stdin", "--restart", "--delay", "10m", "--for", "2h", "--yes"); err != nil {
		t.Fatal(err)
	}
	keys, _ := d.Hotkeys()
	if len(keys) != 1 || keys[0].PasswordHash == "" || !db.CheckPassword(keys[0].PasswordHash, "partner") {
		t.Fatalf("hotkeys = %+v", keys)
	}
	if got := strings.Join(keys[0].Args(), " "); got != "--lock --lock-chars 20 --password-stdin --restart --delay 10m --for 2h" &&
		got != "--lock --password-stdin --restart --delay 10m --for 2h --lock-chars 20" {
		t.Errorf("stored lock flags = %q", got)
	}
	// The password hash never reaches the GUI.
	if raw, _ := json.Marshal(keys[0]); strings.Contains(string(raw), keys[0].PasswordHash) {
		t.Errorf("hotkey JSON shows the password hash: %s", raw)
	}
	mustRun(t, "hotkey", "run", fmt.Sprint(keys[0].ID))
	if *asked != 1 {
		t.Errorf("the press asked %d times, want 1", *asked)
	}
	b, _ := d.GetBlock("b")
	e, _ := d.GetLockExtras("b")
	if !b.Active || !b.Locked || b.LockChars != 20 || e.Delay != 600 || e.TimerAfter != 7200 || e.RestartBoot == "" ||
		!db.CheckPassword(e.PasswordHash, "partner") || e.Order != "text,password,restart,delay,timer" {
		t.Errorf("after the press: block %+v, extras %+v", b, e)
	}
	// A later start reads its own password again: the hotkey's isn't kept.
	if presetPasswordHash != "" {
		t.Error("the hotkey's password hash outlived its press")
	}
	// A second press on the running block only adds or extends: here
	// everything is there already, so it is refused and nothing changes.
	if err := runCLI(t, "hotkey", "run", fmt.Sprint(keys[0].ID)); err == nil {
		t.Error("a second press set a second password lock")
	}
}

// Old hotkeys kept only a timer lock's duration; they work as before.
func TestHotkeyOldTimerLock(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	fakeDesktopHotkeys(t)
	blockWithSite(t)
	if _, err := d.AddHotkey(db.Hotkey{Keys: "<Primary><Alt>o", Block: "b", LockFor: "1h", NoConfirm: true}); err != nil {
		t.Fatal(err)
	}
	keys, _ := d.Hotkeys()
	mustRun(t, "hotkey", "run", fmt.Sprint(keys[0].ID))
	b, _ := d.GetBlock("b")
	if left := b.LockedUntil - mustTrusted(t, d); !b.Active || left < 3540 || left > 3600 {
		t.Errorf("old hotkey: active %v, locked for %d s more", b.Active, left)
	}
}

// Wrong lock flags are refused when the hotkey is added, not at a press.
func TestHotkeyLockFlagsChecked(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	fakeDesktopHotkeys(t)
	blockWithSite(t)
	for _, args := range [][]string{
		{"--for", "1h", "--until", "18:00"},
		{"--delay", "50d"},
		{"--relock", "10m"},
		{"--lock-chars", "6000", "--lock"},
		{"--range-invert"},
		{"--until", "2001-01-01 10:00"},
	} {
		if err := runCLI(t, append([]string{"hotkey", "add", "ctrl+alt+x", "b", "--yes"}, args...)...); err == nil {
			t.Errorf("hotkey add %v was accepted", args)
		}
	}
	if keys, _ := d.Hotkeys(); len(keys) != 0 {
		t.Errorf("hotkeys = %+v", keys)
	}
	if err := runCLI(t, "hotkey", "add", "ctrl+alt+x", "nosuchblock"); err == nil {
		t.Error("a hotkey for a block that doesn't exist was accepted")
	}
}
