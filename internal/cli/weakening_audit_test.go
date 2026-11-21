package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
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

	// §4.3: the same settings through the block's draft are kept there, and
	// refused at Save edits.
	mustRun(t, "block", "draft", "b")
	for _, args := range [][]string{
		{"allowance", "set", "b (draft)", "30m"},
		{"pomodoro", "set", "b (draft)", "25m", "5m"},
		{"break", "reward", "b (draft)", "10m", "--per", "1h"},
		{"break", "text", "b (draft)", "5m"},
		{"break", "delay", "b (draft)", "5m", "10m"},
		{"bypass", "off", "--block", "b (draft)"},
	} {
		mustRun(t, args...)
	}
	if err := runCLI(t, "block", "draft", "b", "save"); err == nil {
		t.Error("a draft's weakening settings were saved on a locked block")
	}
	if n, _ := d.BlockNepsis("b"); len(n.Steps) == 0 {
		t.Error("bypass prevention went off through a draft")
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

// §3.4, §3.8, §4.10: while anything is locked, device and tamper
// protection can't be taken away (factory reset protection, no-escape,
// every tamper protection setting, an allowed app for "allowed apps
// only"); each refusal is logged and never counted (R-WEAK-2, principle 5).
func TestLockedDeviceProtectionRefusesWeakening(t *testing.T) {
	notTerminal(t)
	d := testDB(t)
	fakeFirmwareRoot(t)
	mustRun(t, "reset-guard", "on")
	for _, name := range tamperOrder {
		mustRun(t, "tamper", name, "on")
	}
	mustRun(t, "tamper", "grace", "30")
	if err := runWith(t, 0, []string{"secret"}, "firmware-lock", "on", "--yes"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetSetting("no_escape", "on"); err != nil { // the boot lock itself is never run here
		t.Fatal(err)
	}
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	weakenings := [][]string{
		{"reset-guard", "off"},
		{"firmware-lock", "off"},
		{"no-escape", "off"},
		{"freeze", "apps", "add", "steam"},
		{"tamper", "grace", "60"},
	}
	for _, name := range tamperOrder {
		weakenings = append(weakenings, []string{"tamper", name, "off"})
	}
	for _, args := range weakenings {
		if err := runWith(t, 0, []string{"secret"}, args...); err == nil {
			t.Errorf("%v: weakened while locked", args)
		}
	}
	for _, name := range tamperOrder {
		if !enforce.TamperOn(d, simpleTamper[name]) {
			t.Errorf("tamper %s went off", name)
		}
	}
	events, _ := d.RecentEvents(0, 50)
	refusals := 0
	for _, e := range events {
		if e.Kind == db.EventRefused {
			refusals++
		}
	}
	if refusals != len(weakenings) {
		t.Errorf("%d refusals logged, want %d", refusals, len(weakenings))
	}
	if n, _ := d.CountEvents("", 0); n != 0 {
		t.Errorf("refusals counted by bypass prevention: %d", n)
	}
	if v, _ := d.GetSetting("no_escape"); v != "on" {
		t.Error("no-escape turned off")
	}
}

// R-LOCK-1, §3.6 places: on a locked block that two places turn on
// (disabled by default), unticking one, needing more of its networks, or
// forgetting it all lift the block somewhere: refused. Needing fewer
// networks is free. (Unticking the last place makes the block continuous,
// which is stronger.)
func TestLockedBlockRefusesPlaceWeakening(t *testing.T) {
	notTerminal(t)
	d := testDB(t)
	home := []db.Network{{BSSID: "AA:AA:AA:AA:AA:01"}, {BSSID: "AA:AA:AA:AA:AA:02"}, {BSSID: "AA:AA:AA:AA:AA:03"}}
	office := []db.Network{{BSSID: "CC:CC:CC:CC:CC:01"}, {BSSID: "CC:CC:CC:CC:CC:02"}}
	seen := home
	oldScan := enforce.ScanWifi
	enforce.ScanWifi = func(bool) ([]db.Network, error) { return seen, nil }
	t.Cleanup(func() { enforce.ScanWifi = oldScan })
	mustRun(t, "place", "add", "home")
	seen = office
	enforce.ForgetSeenNetworks()
	mustRun(t, "place", "add", "office")
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "place", "rule", "b", "home", "on")
	mustRun(t, "place", "rule", "b", "office", "on")
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	for _, args := range [][]string{
		{"place", "rule", "b", "home", "off"},
		{"place", "need", "home", "3"},
		{"place", "remove", "home"},
	} {
		if err := runCLI(t, args...); err == nil {
			t.Errorf("%v: a locked block was weakened", args)
		}
	}
	p, err := d.GetPlace("home")
	if err != nil || !slices.Contains(p.Blocks, "b") || p.Needed() != 2 {
		t.Errorf("the place changed: %+v %v", p, err)
	}
	if err := runCLI(t, "place", "need", "home", "1"); err != nil {
		t.Errorf("needing fewer networks refused: %v", err)
	}
}
