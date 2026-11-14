package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

// lockCase is one way to set a lock: the command line, with --yes appended
// for the confirmed run, the lines it reads from standard input (lock
// passwords), and a check that the lock is set.
type lockCase struct {
	name  string
	setup func(t *testing.T)
	args  []string
	lines []string
	isSet func(t *testing.T, d *db.DB) bool
}

// freeWindow is a window of today that isn't open now, so no freeze or
// block window starts during the test.
func freeWindow() (from, to string) {
	h := (time.Now().Hour() + 12) % 24
	return fmt.Sprintf("%02d:00", h), fmt.Sprintf("%02d:30", h)
}

func blockWithSite(t *testing.T) {
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
}

func blockActive(t *testing.T, d *db.DB) bool {
	active, err := d.IsBlockActive("b")
	if err != nil {
		t.Fatal(err)
	}
	return active
}

func settingLocked(name string) func(t *testing.T, d *db.DB) bool {
	return func(t *testing.T, d *db.DB) bool {
		l, err := d.GetSettingLock(name)
		if err != nil {
			t.Fatal(err)
		}
		return l.LockedUntil != 0 || l.LockChars > 0 || l.PasswordHash != "" || l.RestartBoot != ""
	}
}

func lockCases() []lockCase {
	from, to := freeWindow()
	return []lockCase{
		{name: "block timer lock", setup: blockWithSite, args: []string{"start", "b", "--for", "1h"}, isSet: blockActive},
		{name: "block timer lock until", setup: blockWithSite, args: []string{"start", "b", "--until", "23:59"}, isSet: blockActive},
		{name: "block text lock", setup: blockWithSite, args: []string{"start", "b", "--lock"}, isSet: blockActive},
		{name: "block text lock of words", setup: blockWithSite, args: []string{"start", "b", "--lock-words"}, isSet: blockActive},
		{name: "block password lock", setup: blockWithSite, args: []string{"start", "b", "--password-stdin"}, lines: []string{"partner"}, isSet: blockActive},
		{name: "block restart lock", setup: blockWithSite, args: []string{"start", "b", "--restart"}, isSet: blockActive},
		{name: "block delay lock", setup: blockWithSite, args: []string{"start", "b", "--delay", "10m"}, isSet: blockActive},
		{name: "block timer lock extended", setup: func(t *testing.T) {
			blockWithSite(t)
			mustRun(t, "start", "b", "--for", "1h", "--yes")
		}, args: []string{"start", "b", "--for", "2h"}, isSet: func(t *testing.T, d *db.DB) bool {
			b, _ := d.GetBlock("b")
			return b.LockedUntil > time.Now().Add(90*time.Minute).Unix()
		}},
		{name: "text lock added to an active block", setup: func(t *testing.T) {
			blockWithSite(t)
			mustRun(t, "start", "b")
		}, args: []string{"start", "b", "--lock"}, isSet: func(t *testing.T, d *db.DB) bool {
			b, _ := d.GetBlock("b")
			return b.Locked
		}},
		{name: "schedule lock", setup: func(t *testing.T) {
			blockWithSite(t)
			mustRun(t, "schedule", "add", "b", "--from", from, "--to", to)
		}, args: []string{"schedule", "lock", "b", "on", "--lead", "1h"}, isSet: func(t *testing.T, d *db.DB) bool {
			on, _, _ := d.ScheduleLock("b")
			return on
		}},
		{name: "strict mode lock", args: []string{"strict", "on", "--for", "1h"}, isSet: settingLocked(enforce.StrictLockName)},
		{name: "browser setting timer lock", args: []string{"browser", "safesearch", "on", "--for", "1h"}, isSet: settingLocked("safesearch")},
		{name: "browser setting text lock", args: []string{"browser", "safesearch", "on", "--lock"}, isSet: settingLocked("safesearch")},
		{name: "browser setting password lock", args: []string{"browser", "safesearch", "on", "--password-stdin"}, lines: []string{"partner"}, isSet: settingLocked("safesearch")},
		{name: "browser setting restart lock", args: []string{"browser", "safesearch", "on", "--restart"}, isSet: settingLocked("safesearch")},
		{name: "bypass prevention lock", args: []string{"bypass", "set", "--count", "3", "--within", "10m", "--then", "close-tab", "--for", "1h"},
			isSet: settingLocked(db.NepsisLockName)},
		{name: "freeze schedule with a lock", args: []string{"freeze", "schedule", "add", "--from", from, "--to", to, "--lock-lead", "1h"},
			isSet: func(t *testing.T, d *db.DB) bool {
				s, _ := d.ListFreezeSchedules()
				return len(s) == 1
			}},
		{name: "freeze schedule lock", setup: func(t *testing.T) {
			mustRun(t, "freeze", "schedule", "add", "--from", from, "--to", to, "--yes")
		}, args: []string{"freeze", "schedule", "lock", "1", "on", "--lead", "1h"}, isSet: func(t *testing.T, d *db.DB) bool {
			s, _ := d.ListFreezeSchedules()
			return len(s) == 1 && s[0].LockLead == 60
		}},
	}
}

// R-LOCK-12: every lock kind, on every target, is refused without
// confirmation, leaves nothing locked, and is set once confirmed.
func TestRLOCK12_EveryLockAsksConfirmation(t *testing.T) {
	for _, c := range lockCases() {
		t.Run(c.name, func(t *testing.T) {
			d := testDB(t)
			notTerminal(t)
			if c.setup != nil {
				c.setup(t)
			}
			err := runWith(t, 0, c.lines, c.args...)
			if err == nil || !strings.Contains(err.Error(), "--yes") {
				t.Fatalf("%s without confirmation: err = %v; want refused", strings.Join(c.args, " "), err)
			}
			if c.isSet(t, d) {
				t.Fatalf("%s: locked without confirmation", strings.Join(c.args, " "))
			}
			if err := runWith(t, 0, c.lines, append(c.args, "--yes")...); err != nil {
				t.Fatalf("%s --yes: %v", strings.Join(c.args, " "), err)
			}
			if !c.isSet(t, d) {
				t.Fatalf("%s --yes: the lock isn't set", strings.Join(c.args, " "))
			}
		})
	}
}

// R-LOCK-12: on a terminal the user is asked: "yes" sets the lock, any other
// answer sets nothing. The question shows the target, kind and length.
func TestRLOCK12_TerminalConfirmation(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	blockWithSite(t)
	isTerminalInput = func() bool { return true }

	answer := func(line string) (string, error) {
		stdinLines = bufio.NewReader(strings.NewReader(line + "\n"))
		resetFlags(rootCmd)
		return captureStdout(func() error { return runCLI(t, "start", "b", "--for", "2h", "--lock") })
	}
	out, err := answer("no")
	if err == nil || blockActive(t, d) {
		t.Fatalf("answered no: err = %v, active = %v", err, blockActive(t, d))
	}
	for _, want := range []string{"block 'b'", "timer lock", "2h0m0s, until", "text lock (random characters (300 chars))"} {
		if !strings.Contains(out, want) {
			t.Errorf("the question %q doesn't show %q", out, want)
		}
	}
	if _, err := answer("yes"); err != nil || !blockActive(t, d) {
		t.Fatalf("answered yes: err = %v, active = %v", err, blockActive(t, d))
	}
	b, _ := d.GetBlock("b")
	if !b.Locked || b.LockedUntil == 0 {
		t.Errorf("answered yes: locks = %+v", b)
	}
}

// R-LOCK-12: starting a block without a lock, or a setting without a lock,
// asks nothing.
func TestRLOCK12_NoLockNoQuestion(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	blockWithSite(t)
	mustRun(t, "start", "b")
	mustRun(t, "browser", "safesearch", "on")
	mustRun(t, "strict", "on")
	if !blockActive(t, d) {
		t.Error("the block didn't start")
	}
}

// R-LOCK-12: locks in an imported project (schedule locks of blocks, locks of
// scheduled freezes) are confirmed before anything is imported.
func TestRLOCK12_ProjectImportConfirmation(t *testing.T) {
	testDB(t)
	notTerminal(t)
	from, to := freeWindow()
	blockWithSite(t)
	mustRun(t, "schedule", "add", "b", "--from", from, "--to", to)
	mustRun(t, "schedule", "lock", "b", "on", "--lead", "30m", "--yes")
	mustRun(t, "freeze", "schedule", "add", "--from", from, "--to", to, "--lock-lead", "1h", "--yes")
	file := filepath.Join(t.TempDir(), "p.json")
	if err := os.WriteFile(file, []byte(capture(t, "project", "export")), 0o644); err != nil {
		t.Fatal(err)
	}

	d := testDB(t) // a fresh computer
	err := runWith(t, 0, nil, "project", "import", file)
	if err == nil || !strings.Contains(err.Error(), "schedule lock") {
		t.Fatalf("import without --yes: %v", err)
	}
	if _, err := d.GetBlock("b"); err == nil {
		t.Fatal("the block was imported without confirmation")
	}
	// A dry run sets nothing, so asks nothing, and lists the locks.
	if out := capture(t, "project", "import", file, "--dry-run"); !strings.Contains(out, "Would: set a lock: imported block 'b': schedule lock") {
		t.Errorf("the dry run doesn't list the lock: %s", out)
	}
	mustRun(t, "project", "import", file, "--yes")
	if on, lead, _ := d.ScheduleLock("b"); !on || lead != 30 {
		t.Errorf("imported schedule lock = %v %d", on, lead)
	}
	if s, _ := d.ListFreezeSchedules(); len(s) != 1 || s[0].LockLead != 60 {
		t.Errorf("imported freeze schedules = %+v", s)
	}
}

// R-LOCK-12: a hotkey's lock is confirmed when the hotkey is added. With
// "Run without confirmation" (--no-confirm) that is the only time; without
// it, pressing the hotkey asks again in a desktop dialog and a "no" sets
// nothing.
func TestRLOCK12_HotkeyConfirmation(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	old, oldAsk := syncDesktopHotkeys, confirmOnDesktop
	syncDesktopHotkeys = func([]blocker.DesktopHotkey) error { return nil }
	asked := 0
	answer := true
	confirmOnDesktop = func(title, text string) bool { asked++; return answer }
	t.Cleanup(func() { syncDesktopHotkeys, confirmOnDesktop = old, oldAsk })
	blockWithSite(t)

	err := runCLI(t, "hotkey", "add", "ctrl+alt+s", "b", "--for", "1h")
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("hotkey add with a lock, unconfirmed: %v", err)
	}
	if keys, _ := d.Hotkeys(); len(keys) != 0 {
		t.Fatalf("the hotkey was added without confirmation: %+v", keys)
	}
	if err := runCLI(t, "hotkey", "add", "ctrl+alt+k", "b", "--no-confirm"); err == nil {
		t.Fatal("--no-confirm without a lock was accepted")
	}
	mustRun(t, "hotkey", "add", "ctrl+alt+k", "b")                                         // no lock, no question
	mustRun(t, "hotkey", "add", "ctrl+alt+s", "b", "--for", "1h", "--yes")                 // asks at each press
	mustRun(t, "hotkey", "add", "ctrl+alt+n", "b", "--for", "1h", "--yes", "--no-confirm") // never asks again
	keys, _ := d.Hotkeys()
	if len(keys) != 3 || keys[1].NoConfirm || !keys[2].NoConfirm {
		t.Fatalf("hotkeys = %+v", keys)
	}
	press := func(id int) { mustRun(t, "hotkey", "run", fmt.Sprint(id)) }

	answer = false
	if err := runCLI(t, "hotkey", "run", fmt.Sprint(keys[1].ID)); err == nil || asked != 1 {
		t.Fatalf("a pressed hotkey, answered no: err = %v, asked = %d", err, asked)
	}
	if b, _ := d.GetBlock("b"); b.Active {
		t.Fatal("the block started although the press wasn't confirmed")
	}
	press(keys[2].ID) // without confirmation: no dialog
	if asked != 1 {
		t.Errorf("a hotkey that runs without confirmation asked (%d dialogs)", asked)
	}
	if b, _ := d.GetBlock("b"); !b.Active || b.LockedUntil == 0 {
		t.Errorf("hotkey run without confirmation: active = %v, locked until %d", b.Active, b.LockedUntil)
	}
}

// R-LOCK-12: monk mode started by itself (a bypass prevention step, a list's
// or a phrase set's monk action) asks nothing, so setting it up needs the
// acknowledgement "I know this will not ask for confirmation".
func TestRLOCK12_AutomaticMonkNeedsAcknowledgement(t *testing.T) {
	testDB(t)
	notTerminal(t)
	blockWithSite(t)
	mustRun(t, "phrase-set", "create", "b", "Words")
	mustRun(t, "phrase-set", "add", "b", "Words", "[eat, nice] [hello, grandpa]")
	refused := func(args ...string) {
		t.Helper()
		err := runCLI(t, args...)
		if err == nil || !strings.Contains(err.Error(), "--no-confirm-ok") {
			t.Errorf("%s without the acknowledgement: %v", strings.Join(args, " "), err)
		}
	}
	refused("list", "action", "b", "Blocked", "monk", "on", "--for", "10m")
	refused("phrase-set", "action", "b", "Words", "monk", "on", "--for", "10m")
	refused("bypass", "set", "--count", "3", "--within", "5m", "--then", "monk", "--then-for", "1h")
	refused("bypass", "set", "--block", "b", "--step", "3 in 5m: close-browser, monk 5m")
	mustRun(t, "list", "action", "b", "Blocked", "monk", "on", "--for", "10m", "--no-confirm-ok")
	mustRun(t, "phrase-set", "action", "b", "Words", "monk", "on", "--for", "10m", "--no-confirm-ok")
	mustRun(t, "bypass", "set", "--count", "3", "--within", "5m", "--then", "monk", "--then-for", "1h", "--no-confirm-ok")
	// Other consequences need no acknowledgement.
	mustRun(t, "bypass", "set", "--count", "3", "--within", "5m", "--then", "close-tab")
}
