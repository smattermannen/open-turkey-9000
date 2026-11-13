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
	mustRun(t, "strict", "on", "--for", "1h")
	if err := runCLI(t, "strict", "off"); err == nil || !strings.Contains(err.Error(), "locked until") {
		t.Errorf("strict off under its own timer lock: %v", err)
	}
	if err := runCLI(t, "strict", "on", "--for", "10m"); err == nil {
		t.Error("strict mode's timer lock was shortened")
	}
	if locked, _ := enforce.AnySettingLocked(d, mustTrusted(t, d)); !locked {
		t.Error("strict mode's lock doesn't count as a setting lock")
	}
	if err := d.ClearSettingLock(enforce.StrictLockName); err != nil {
		t.Fatal(err)
	}

	// A password lock is lifted by unlock; strict mode stays on.
	if err := runWith(t, 0, []string{"partner"}, "strict", "on", "--password-stdin"); err != nil {
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
	if err := runWith(t, 0, nil, "browser", "safesearch", "on", "--for", "1m"); err != nil {
		t.Fatalf("a lock without the settings password: %v", err)
	}
	if err := runWith(t, 0, nil, "password", "locks", "on"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, nil, "browser", "safesearch", "on", "--for", "2m"); err == nil {
		t.Error("a lock was set without the settings password after 'password locks on'")
	}
	if err := runWith(t, 0, []string{"secret1"}, "browser", "safesearch", "on", "--for", "3m"); err != nil {
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

// R-MONK-1, §3.8: the monk-mode block is system-owned; exceptions and apps
// change only while it is off; it never gets the monk action.
func TestMonkBlockIsSystemOwned(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "monk", "--for", "30m")
	for _, args := range [][]string{
		{"block", "add-exception", enforce.MonkBlock, "docs.python.org"},
		{"block", "add-title", enforce.MonkBlock, "chess"},
		{"block", "rename", enforce.MonkBlock, "calm"},
		{"start", enforce.MonkBlock},
	} {
		if err := runCLI(t, args...); err == nil {
			t.Errorf("%s while monk mode is on: accepted", strings.Join(args, " "))
		}
	}
	if err := d.DeactivateBlock(enforce.MonkBlock); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "block", "add-exception", enforce.MonkBlock, "docs.python.org")
	mustRun(t, "block", "add-title", enforce.MonkBlock, "chess")
	if err := runCLI(t, "list", "action", enforce.MonkBlock, "Blocked", "monk", "on", "--for", "1h"); err == nil {
		t.Error("the monk action inside monk mode's block was accepted")
	}
	if err := runCLI(t, "block", "rename", enforce.MonkBlock, "calm"); err == nil {
		t.Error("monk mode's block was renamed")
	}
}

// §3.4: a text lock of random characters, random words or custom text.
func TestTextLockStyles(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "start", "b", "--lock-text", "I  choose to focus")
	extras, _ := d.GetLockExtras("b")
	detail, _ := d.GetBlock("b")
	if extras.Style != lock.StyleCustom+"I choose to focus" || !detail.Locked || detail.LockChars != len("I choose to focus") {
		t.Errorf("custom text lock: style %q, locked %v, %d chars", extras.Style, detail.Locked, detail.LockChars)
	}
	if err := runCLI(t, "start", "b", "--lock-words"); err == nil {
		t.Error("a text lock's kind changed on an active block")
	}
	mustRun(t, "browser", "safesearch", "on", "--lock-words", "--lock-chars", "40")
	s, _ := d.GetSettingLock("safesearch")
	if s.Style != lock.StyleWords || s.LockChars != 40 {
		t.Errorf("setting lock: %+v", s)
	}
	if err := runCLI(t, "browser", "safesearch", "on", "--lock"); err == nil {
		t.Error("a word lock became a character lock")
	}
	if err := runCLI(t, "start", "b", "--lock-text", "café"); err == nil {
		t.Error("custom text with an accent accepted")
	}
}
