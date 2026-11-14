package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/smattermannen/kc9k/internal/lock"
)

// R-STRICT-1: strict mode is turned off freely unless its own setting lock
// or R-WEAK-4 refuses it; another block's lock alone doesn't.
func TestStrictModeLock(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	if err := d.ActivateBlock("b", true, 0, 0, false); err != nil { // a text lock
		t.Fatal(err)
	}
	mustRun(t, "strict", "on")
	mustRun(t, "strict", "off")
	if v, _ := d.GetSetting(enforce.SettingStrict); v != "off" {
		t.Fatalf("strict = %q after off during another block's lock", v)
	}

	// Its own timer lock refuses turning it off.
	mustRun(t, "strict", "on", "--for", "1h", "--yes")
	if err := runCLI(t, "strict", "off"); err == nil || !strings.Contains(err.Error(), "locked until") {
		t.Errorf("strict off under its own timer lock: %v", err)
	}
	if err := runCLI(t, "strict", "on", "--for", "10m", "--yes"); err == nil {
		t.Error("strict mode's timer lock was shortened")
	}
	if locked, _ := enforce.AnySettingLocked(d, mustTrusted(t, d)); !locked {
		t.Error("strict mode's lock doesn't count as a setting lock")
	}
	if err := d.ClearSettingLock(enforce.StrictLockName); err != nil {
		t.Fatal(err)
	}

	// A password lock is lifted by unlock; strict mode stays on.
	if err := runWith(t, 0, []string{"partner"}, "strict", "on", "--password-stdin", "--yes"); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "strict", "off"); err == nil || !strings.Contains(err.Error(), "strict unlock") {
		t.Errorf("strict off under a password lock: %v", err)
	}
	if err := runWith(t, 0, []string{"wrong"}, "strict", "unlock"); err == nil {
		t.Error("a wrong password unlocked strict mode")
	}
	if err := runWith(t, 0, []string{"partner"}, "strict", "unlock"); err != nil {
		t.Fatal(err)
	}
	if v, _ := d.GetSetting(enforce.SettingStrict); v != "on" {
		t.Errorf("unlock switched strict mode %q; a lock is only lifted", v)
	}

	// R-WEAK-4: with no weakening on, the block's lock keeps it on.
	mustRun(t, "no-weakening", "on")
	if err := runCLI(t, "strict", "off"); err == nil || !strings.Contains(err.Error(), "no weakening") {
		t.Errorf("strict off with no-weakening during a lock: %v", err)
	}
	if err := runCLI(t, "no-weakening", "off"); err == nil {
		t.Error("no-weakening turned off during a lock")
	}
	if err := runCLI(t, "browser", "safesearch", "on"); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "browser", "safesearch", "off"); err == nil {
		t.Error("a browser setting was weakened under no-weakening during a lock")
	}
	if err := d.DeactivateBlock("b"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "strict", "off")
	mustRun(t, "no-weakening", "off")
}

// R-WEAK-1: setting a lock needs the settings password only if the user
// chose that.
func TestPasswordForLocks(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	if err := runWith(t, 0, []string{"secret1"}, "password", "set"); err != nil {
		t.Fatal(err)
	}
	// No password given: a lock is still set.
	if err := runWith(t, 0, nil, "browser", "safesearch", "on", "--for", "1m", "--yes"); err != nil {
		t.Fatalf("a lock without the settings password: %v", err)
	}
	if err := runWith(t, 0, nil, "password", "locks", "on"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, nil, "browser", "safesearch", "on", "--for", "2m", "--yes"); err == nil {
		t.Error("a lock was set without the settings password after 'password locks on'")
	}
	if err := runWith(t, 0, []string{"secret1"}, "browser", "safesearch", "on", "--for", "3m", "--yes"); err != nil {
		t.Errorf("a lock with the settings password: %v", err)
	}
	if err := runWith(t, 0, nil, "password", "locks", "off"); err == nil {
		t.Error("'password locks off' without the password")
	}
	if err := runWith(t, 0, []string{"secret1"}, "password", "locks", "off"); err != nil {
		t.Fatal(err)
	}
	if v, _ := d.GetSetting(SettingPasswordForLocks); v != "off" {
		t.Errorf("password for locks = %q", v)
	}
}

func mustTrusted(t *testing.T, d *db.DB) int64 {
	t.Helper()
	trusted, err := d.TrustedNow(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return trusted
}

// R-MONK-1: monk mode is a mode, not a block: no block of that name, and
// nothing to edit. (Through KC9K_DB it never touches the real network.)
func TestR_MONK_1_NoBlock(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "monk", "--for", "30m")
	if until, err := d.MonkUntil(); err != nil || until == 0 {
		t.Fatalf("monk mode not on: %d, %v", until, err)
	}
	if blocks, _ := d.ListBlocks(); len(blocks) != 0 {
		t.Errorf("blocks = %+v; want none", blocks)
	}
	if err := runCLI(t, "block", "add-exception", "monk-mode", "docs.python.org"); err == nil {
		t.Error("an exception was added to monk mode")
	}
	if err := runCLI(t, "start", "monk-mode"); err == nil {
		t.Error("monk-mode started as a block")
	}
}

// §3.4: a text lock of random characters, random words or custom text.
func TestTextLockStyles(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "start", "b", "--lock-text", "I  choose to focus", "--yes")
	extras, _ := d.GetLockExtras("b")
	detail, _ := d.GetBlock("b")
	if extras.Style != lock.StyleCustom+"I choose to focus" || !detail.Locked || detail.LockChars != len("I choose to focus") {
		t.Errorf("custom text lock: style %q, locked %v, %d chars", extras.Style, detail.Locked, detail.LockChars)
	}
	if err := runCLI(t, "start", "b", "--lock-words", "--yes"); err == nil {
		t.Error("a text lock's kind changed on an active block")
	}
	mustRun(t, "browser", "safesearch", "on", "--lock-words", "--lock-chars", "40", "--yes")
	s, _ := d.GetSettingLock("safesearch")
	if s.Style != lock.StyleWords || s.LockChars != 40 {
		t.Errorf("setting lock: %+v", s)
	}
	if err := runCLI(t, "browser", "safesearch", "on", "--lock", "--yes"); err == nil {
		t.Error("a word lock became a character lock")
	}
	if err := runCLI(t, "start", "b", "--lock-text", "café", "--yes"); err == nil {
		t.Error("custom text with an accent accepted")
	}
}

// §3.4, R-LOCK-2: unlocking lifts a lock and leaves the block on; a delay
// lock counts down from asking to unlock.
func TestUnlockOnlyLifts(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	if err := runWith(t, 0, []string{"partner"}, "start", "b", "--password-stdin", "--delay", "1m", "--yes"); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "stop", "b"); err == nil {
		t.Fatal("stop worked on a locked block")
	}
	if err := runWith(t, 0, []string{"partner"}, "unlock", "b"); err != nil {
		t.Fatal(err)
	}
	detail, _ := d.GetBlock("b")
	extras, _ := d.GetLockExtras("b")
	trusted := mustTrusted(t, d)
	if !detail.Active || extras.PasswordHash != "" || extras.DelayUntil < trusted+50 || extras.Delay != 60 {
		t.Fatalf("after unlock: active %v, password %q, delay %d until %d (now %d)", detail.Active, extras.PasswordHash, extras.Delay, extras.DelayUntil, trusted)
	}
	if err := runCLI(t, "stop", "b"); err == nil {
		t.Error("stop worked while the delay lock counted down")
	}
}

// §4.10, R-LOCK-9: tamper protection can't be turned off while locked.
func TestTamperClockSetting(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "tamper", "clock", "on")
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	if err := d.ActivateBlock("b", true, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "tamper", "clock", "off"); err == nil {
		t.Error("tamper protection turned off during a lock")
	}
	if err := d.DeactivateBlock("b"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "tamper", "clock", "off")
	if v, _ := d.GetSetting(db.SettingTamperClock); v != "off" {
		t.Errorf("tamper.clock = %q", v)
	}
}

// §3.4 re-lock: unlocking lifts the locks for a while, then they are on
// again by themselves.
func TestRelockOption(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	if err := runCLI(t, "start", "b", "--relock", "30m", "--yes"); err == nil {
		t.Error("--relock without a text or password lock was accepted")
	}
	if err := runWith(t, 0, []string{"partner"}, "start", "b", "--password-stdin", "--relock", "30m", "--yes"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"partner"}, "unlock", "b"); err != nil {
		t.Fatal(err)
	}
	extras, _ := d.GetLockExtras("b")
	trusted := mustTrusted(t, d)
	if extras.PasswordHash == "" || extras.LiftedUntil < trusted+1700 || !extras.Lifted(trusted) {
		t.Fatalf("after unlock: %+v", extras)
	}
	if err := runCLI(t, "block", "remove-keyword", "b", "x"); err != nil && strings.Contains(err.Error(), "locked") {
		t.Errorf("the block is still locked while lifted: %v", err)
	}
	// Past the re-lock time the password lock is on again.
	extras.LiftedUntil = trusted - 1
	if err := d.SetLockExtras("b", extras); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "stop", "b"); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Errorf("stop after the re-lock time: %v; want refused", err)
	}
}
