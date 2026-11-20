package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
)

// R-LOCK-1, §3.5: a locked block can't be weakened, by any command. Every
// command that could weaken a block is tried on a locked block that has
// something of everything; each must be refused and change nothing. The
// strengthening ones next to them must still work (R-WEAK-1).
func TestLockedBlockRefusesEveryWeakening(t *testing.T) {
	notTerminal(t)
	d := testDB(t)
	dir := t.TempDir()
	exceptions := filepath.Join(dir, "exceptions.txt")
	if err := os.WriteFile(exceptions, []byte("example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"block", "create", "b"},
		{"block", "add", "b", "reddit.com", "x.com"},
		{"block", "add-app", "b", "steam"},
		{"block", "add-title", "b", "YouTube"},
		{"block", "add-keyword", "b", "casino", "poker"},
		{"block", "threshold", "b", "block", "2"},
		{"list", "hits", "b", "Rendered terms on page", "block", "3"},
		{"list", "action", "b", "Blocked", "greyscale", "on"},
		{"list", "scope", "b", "Blocked", "title", "on"},
		{"phrase-set", "create", "b", "P", "--text"},
		{"phrase-set", "add", "b", "P", "[eat] [grandpa]"},
		{"phrase-set", "threshold", "b", "P", "block", "2"},
		{"social", "b", "youtube:no-shorts", "on"},
		{"bypass", "set", "--block", "b", "--count", "3", "--within", "10m", "--then", "warning"},
		{"autostart", "b", "add", "at", "08:00"},
		{"block", "trigger", "b", "apps", "logout", "on", "--for", "1h"},
		{"cluster", "create", "c"},
		{"cluster", "add", "c", "example.org"},
		{"schedule", "create", "Weekly"},
		{"schedule", "add", "Weekly", "--days", "mon", "--from", "09:00", "--to", "10:00"},
		{"mode", "create", "M"},
		{"schedule", "add", "M", "--days", "mon", "--from", "09:00", "--to", "10:00"},
	} {
		mustRun(t, args...)
	}
	if err := d.ActivateBlock("b", true, 0, 0, false); err != nil { // text-locked
		t.Fatal(err)
	}
	before, err := d.GetBlock("b")
	if err != nil {
		t.Fatal(err)
	}
	beforeSets, _ := d.PhraseSets("b")

	for _, args := range [][]string{
		{"stop", "b"},
		{"toggle", "b"},
		{"block", "remove", "b"},
		{"block", "remove-site", "b", "reddit.com"},
		{"block", "remove-app", "b", "steam"},
		{"block", "remove-title", "b", "YouTube"},
		{"block", "trigger", "b", "apps", "close", "off"},
		{"block", "trigger", "b", "titles", "close", "off"},
		{"block", "trigger", "b", "apps", "logout", "off"},
		{"block", "trigger", "b", "apps", "logout", "on", "--for", "30m"},
		{"block", "remove-keyword", "b", "casino"},
		{"block", "add-exception", "b", "example.com"},
		{"block", "import-list", "b", "exceptions", exceptions},
		{"block", "action", "b", "block", "off"},
		{"block", "threshold", "b", "block", "5"},
		{"list", "remove", "b", "Blocked", "x.com"},
		{"list", "delete", "b", "Blocked"},
		{"list", "delete", "b", "Rendered terms on page"},
		{"list", "scope", "b", "Blocked", "url", "off"},
		{"list", "action", "b", "Blocked", "greyscale", "off"},
		{"list", "threshold", "b", "Rendered terms on page", "block", "9"},
		{"list", "hits", "b", "Rendered terms on page", "block", "9"},
		{"list", "rate", "b", "Rendered terms on page", "block", "5"},
		{"list", "test", "b", "Rendered terms on page", "on"},
		{"list", "add", "b", "Exceptions", "example.com"},
		{"phrase-set", "remove", "b", "P", "[eat] [grandpa]"},
		{"phrase-set", "delete", "b", "P"},
		{"phrase-set", "scope", "b", "P", "text", "off"},
		{"phrase-set", "action", "b", "P", "block", "off"},
		{"phrase-set", "threshold", "b", "P", "block", "5"},
		{"phrase-set", "test", "b", "P", "on"},
		{"schedule", "add", "b", "--days", "daily", "--from", "09:00", "--to", "10:00"},
		{"allowance", "set", "b", "30m"},
		{"pomodoro", "set", "b", "25m", "5m"},
		{"break", "reward", "b", "10m", "--per", "1h"},
		{"break", "text", "b", "5m"},
		{"break", "delay", "b", "5m", "10m"},
		{"social", "b", "youtube:no-shorts", "off"},
		{"bypass", "off", "--block", "b"},
		{"bypass", "set", "--block", "b", "--count", "30", "--within", "10m", "--then", "warning"},
		{"autostart", "b", "remove", "1"},
		{"cluster", "copy", "c", "b", "--exceptions"},
		{"list-preset", "copy", "c", "b", "--exceptions"}, // §4.12: the same command, by its name
		// §3.6 Schedule, §3.2 Mode (2026-11-17): a first window lifts a
		// continuous block outside it.
		{"schedule", "apply", "b", "Weekly"},
		{"mode", "add", "M", "b"},
	} {
		if err := runCLI(t, args...); err == nil {
			t.Errorf("%v: a locked block was weakened", args)
		}
	}

	after, err := d.GetBlock("b")
	if err != nil {
		t.Fatal(err)
	}
	if !after.Active || !after.Locked {
		t.Fatalf("the block is no longer on and locked: %+v", after)
	}
	if !reflect.DeepEqual(before.Lists, after.Lists) || !reflect.DeepEqual(before.Apps, after.Apps) {
		t.Errorf("lists or apps changed:\nbefore %+v\nafter  %+v", before.Lists, after.Lists)
	}
	afterSets, _ := d.PhraseSets("b")
	if !reflect.DeepEqual(beforeSets, afterSets) {
		t.Errorf("phrase sets changed:\nbefore %+v\nafter  %+v", beforeSets, afterSets)
	}
	if k, _ := d.GetBreaks("b"); k != (db.Breaks{}) {
		t.Errorf("breaks were set: %+v", k)
	}
	if q, _ := d.GetQuota("b"); q != nil {
		t.Errorf("an allowance was set: %+v", q)
	}
	if s, _ := d.ListSchedules("b"); len(s) != 0 {
		t.Errorf("a window was added: %+v", s)
	}
	if m, in, _ := d.ModeOf("b"); in {
		t.Errorf("put into mode %s", m.Name)
	}
	// A phrase change that doesn't only add terms is refused whole on a
	// locked block (R-WEAK-5); one that only adds terms is free.
	if err := runCLI(t, "phrase-set", "change", "b", "P", "[eat] [grandpa]", "[eat] [grandma]"); err == nil {
		t.Error("a weakening phrase change on a locked block was taken")
	}
	if s, _ := d.FindPhraseSet("b", "P"); len(s.Phrases) != 1 || s.Phrases[0].String() != "[eat] [grandpa]" {
		t.Errorf("after a refused change on a locked block: %v", s.Phrases)
	}
	if err := runCLI(t, "phrase-set", "change", "b", "P", "[eat] [grandpa]", "[eat] [grandpa, grandma]"); err != nil {
		t.Errorf("a change that only adds terms was refused on a locked block: %v", err)
	}

	// Strengthening still works while locked (R-WEAK-1).
	for _, args := range [][]string{
		{"block", "add", "b", "tiktok.com"},
		{"block", "add-app", "b", "discord"},
		{"list", "action", "b", "Blocked", "no-images", "on"},
		{"block", "threshold", "b", "block", "1"},
		{"phrase-set", "add", "b", "P", "[eat] [grandmother]"},
		{"list", "create", "b", "More"},
		{"social", "b", "youtube:no-comments", "on"},
	} {
		if err := runCLI(t, args...); err != nil {
			t.Errorf("%v: strengthening refused while locked: %v", args, err)
		}
	}
	out, _ := d.GetBlock("b")
	found := false
	for _, l := range out.Lists {
		for _, e := range l.Entries {
			found = found || strings.Contains(e, "tiktok.com")
		}
	}
	if !found {
		t.Error("tiktok.com wasn't added")
	}
}

// The two weakenings the audit found (2026-11-15) refuse only what weakens:
// another window on a scheduled block adds enabled time, and a smaller
// allowance is stricter; both stay allowed while locked.
func TestLockedScheduleAndAllowanceStrengthening(t *testing.T) {
	notTerminal(t)
	d := testDB(t)
	mustRun(t, "block", "create", "s")
	mustRun(t, "block", "add", "s", "reddit.com")
	mustRun(t, "schedule", "add", "s", "--days", "daily", "--from", "09:00", "--to", "10:00")
	mustRun(t, "allowance", "set", "s", "30m") // switches it on, too
	mustRun(t, "start", "s", "--lock", "--lock-chars", "5", "--yes")
	if b, _ := d.GetBlock("s"); !b.Locked {
		t.Fatal("not locked")
	}
	mustRun(t, "schedule", "add", "s", "--days", "daily", "--from", "11:00", "--to", "12:00")
	if s, _ := d.ListSchedules("s"); len(s) != 2 {
		t.Errorf("a second window on a locked scheduled block: %v", s)
	}
	mustRun(t, "allowance", "set", "s", "10m")
	if q, _ := d.GetQuota("s"); q == nil || q.Limit != 600 {
		t.Errorf("a smaller allowance on a locked block: %+v", q)
	}
	for _, args := range [][]string{
		{"allowance", "set", "s", "20m"},
		{"schedule", "add", "s", "--days", "daily", "--from", "13:00", "--to", "14:00", "--allow"},
	} {
		if err := runCLI(t, args...); err == nil {
			t.Errorf("%v: weakened a locked block", args)
		}
	}
	// Unlocked, all of it is free.
	mustRun(t, "block", "create", "u")
	mustRun(t, "block", "add", "u", "reddit.com")
	mustRun(t, "schedule", "add", "u", "--days", "daily", "--from", "09:00", "--to", "10:00")
	mustRun(t, "allowance", "set", "u", "30m")
	mustRun(t, "allowance", "set", "u", "45m")
}
