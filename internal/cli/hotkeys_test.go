package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
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
	// A second press on the running block adds its locks again, each a
	// second lock of its kind, after the first ones (§4.6).
	mustRun(t, "hotkey", "run", fmt.Sprint(keys[0].ID))
	e, _ = d.GetLockExtras("b")
	if got := fmt.Sprint(queuedKinds(e)); got != "[text password restart delay timer]" || !db.CheckPassword(e.Queue[1].PasswordHash, "partner") {
		t.Errorf("after the second press: queue %s", got)
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
		{"--for", "1h", "--for", "0m"},
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

// §3.9 Hotkey action, §4.13 (owner 2026-11-17): a hotkey runs its actions in
// order: here it starts a block, runs a mode (locked) and opens things. Its
// locks are confirmed when it is added and at the press; one that switches
// something off needs the settings password when it is added.
func TestHotkeyActions(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	asked := fakeDesktopHotkeys(t)
	blockWithSite(t)
	mustRun(t, "block", "create", "w")
	mustRun(t, "block", "add", "w", "w.com")
	mustRun(t, "mode", "create", "Work")
	mustRun(t, "mode", "add", "Work", "w")
	add := []string{"hotkey", "add", "ctrl+alt+h", "--do", "start b", "--do", "mode-run Work 1h",
		"--do", `custom "open https://example.org/plan" "run echo hi"`}
	if err := runCLI(t, add...); err == nil {
		t.Error("a hotkey that locks was added without confirmation")
	}
	mustRun(t, append(add, "--yes")...)
	keys, _ := d.Hotkeys()
	if len(keys) != 1 || len(keys[0].Actions) != 3 || keys[0].Actions[1].Target != "Work" || keys[0].Actions[1].Minutes != 60 {
		t.Fatalf("hotkeys = %+v", keys)
	}
	if got := describeHotkey(keys[0]); got != "starts 'b', then runs the mode 'Work', locked, for 1 hour, then open https://example.org/plan; run echo hi" {
		t.Errorf("described: %q", got)
	}
	mustRun(t, "hotkey", "run", fmt.Sprint(keys[0].ID))
	if *asked != 1 {
		t.Errorf("the press asked %d times, want 1", *asked)
	}
	trusted := mustTrusted(t, d)
	if on, _ := d.IsBlockActive("b"); !on {
		t.Error("b wasn't started")
	}
	if on, _ := d.IsBlockActive("w"); !on || enforce.ModeRunUntil(d, "w", trusted) < trusted+3500 {
		t.Error("Work doesn't run, locked")
	}
	// Switching off: the settings password when it is added.
	if err := runWith(t, 0, []string{"secret1"}, "password", "set"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, nil, "hotkey", "add", "ctrl+alt+x", "--do", "stop b"); err == nil {
		t.Error("a hotkey that switches a block off was added without the settings password")
	}
	if err := runWith(t, 0, []string{"secret1"}, "hotkey", "add", "ctrl+alt+x", "--do", "stop b"); err != nil {
		t.Errorf("with the settings password: %v", err)
	}
	for _, bad := range []string{"start", "mode-run Work", "monk", "freeze 2h sleep", "teleport", `custom "jump"`} {
		if err := runCLI(t, "hotkey", "add", "ctrl+alt+y", "--do", bad, "--yes"); err == nil {
			t.Errorf("--do %q was taken", bad)
		}
	}
}

// §3.2 Preset (owner 2026-11-17): a hotkey saved as a preset is added again
// on other keys, through the same checks and confirmation.
func TestHotkeyPreset(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	fakeDesktopHotkeys(t)
	blockWithSite(t)
	mustRun(t, "hotkey", "add", "ctrl+alt+h", "--do", "start b", "--do", `custom "open https://example.org"`)
	keys, _ := d.Hotkeys()
	mustRun(t, "preset", "save", "hotkey", fmt.Sprint(keys[0].ID), "Study")
	mustRun(t, "hotkey", "remove", fmt.Sprint(keys[0].ID))
	mustRun(t, "preset", "use", "Study", "--keys", "ctrl+alt+j")
	keys, _ = d.Hotkeys()
	if len(keys) != 1 || keys[0].Keys != "<Primary><Alt>j" || len(keys[0].Actions) != 2 {
		t.Errorf("from the preset: %+v", keys)
	}
}

func queuedKinds(e db.LockExtras) []string {
	var out []string
	for _, q := range e.Queue {
		out = append(out, q.Kind)
	}
	return out
}
