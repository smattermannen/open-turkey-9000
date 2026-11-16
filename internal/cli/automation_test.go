package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

// §3.6 Automation (owner, 2026-11-16): a block enabled or disabled by
// default, switched by its windows and any number of places, needing both
// or either; a change that lifts a locked block anywhere it is enabled now
// is refused (R-LOCK-1), so is one that leaves a schedule-locked block
// continuous; no network seen keeps a block enabled.
func TestAutomationCommand(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	home := []db.Network{{BSSID: "AA:AA:AA:AA:AA:01"}, {BSSID: "AA:AA:AA:AA:AA:02"}}
	office := []db.Network{{BSSID: "CC:CC:CC:CC:CC:01"}, {BSSID: "CC:CC:CC:CC:CC:02"}}
	seen := home
	oldScan := enforce.ScanWifi
	enforce.ScanWifi = func(bool) ([]db.Network, error) { return seen, nil }
	t.Cleanup(func() { enforce.ScanWifi = oldScan })
	rescan := func(networks []db.Network) {
		seen = networks
		enforce.ForgetSeenNetworks()
	}
	mustRun(t, "place", "add", "home")
	rescan(office)
	mustRun(t, "place", "add", "office")
	rescan(home)
	lifted := func() bool { return enforce.LiftedBySchedule(d, "b", time.Now()) }

	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	if out := capture(t, "automation", "b"); !strings.Contains(out, "continuous") {
		t.Errorf("a new block: %s", out)
	}
	// Disabled by default: being home turns it on; any number of places.
	mustRun(t, "place", "rule", "b", "home", "on")
	mustRun(t, "place", "rule", "b", "office", "on")
	if lifted() {
		t.Error("lifted at home, which turns it on")
	}
	rescan([]db.Network{{BSSID: "EE:EE:EE:EE:EE:01"}})
	if !lifted() {
		t.Error("enabled away from its places")
	}
	// Enabled by default: arriving home turns it off.
	mustRun(t, "automation", "b", "default", "enabled")
	if lifted() {
		t.Error("lifted away from its places though enabled by default")
	}
	rescan(home)
	if !lifted() {
		t.Error("arriving home didn't turn it off")
	}
	// No network seen: whatever keeps it enabled (switching Wi-Fi off lifts nothing).
	rescan(nil)
	if lifted() {
		t.Error("no network seen lifted a block enabled by default")
	}
	mustRun(t, "automation", "b", "default", "disabled")
	if lifted() {
		t.Error("no network seen lifted a block disabled by default")
	}
	rescan(home)

	// Windows follow the default; --allow must fit it.
	if err := runCLI(t, "schedule", "add", "b", "--days", "daily", "--from", "09:00", "--to", "10:00", "--allow"); err == nil ||
		!strings.Contains(err.Error(), "disabled by default") {
		t.Errorf("--allow on a block disabled by default: %v", err)
	}
	mustRun(t, "schedule", "add", "b", "--days", "daily", "--from", "00:00", "--to", "24:00")
	mustRun(t, "automation", "b", "needs", "both")
	if out := capture(t, "automation", "b"); !strings.Contains(out, "both") || !strings.Contains(out, "home, office") {
		t.Errorf("shown: %s", out)
	}
	mustRun(t, "automation", "b", "default", "enabled")
	if s, _ := d.ListSchedules("b"); len(s) != 1 || !s[0].Allow {
		t.Errorf("the window didn't follow the default: %+v", s)
	}
	if !lifted() {
		t.Error("enabled by default, both: at home in its window it should be lifted")
	}
	mustRun(t, "automation", "b", "default", "disabled")
	mustRun(t, "automation", "b", "needs", "either")
	// A window that covers the whole day leaves the places nothing to add:
	// one hour, so that they count.
	mustRun(t, "schedule", "remove", "b", "1")
	mustRun(t, "schedule", "add", "b", "--days", "daily", "--from", "09:00", "--to", "10:00")

	// Locked: more enabled time is free, less is refused.
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	mustRun(t, "automation", "b", "needs", "either") // nothing changes
	for _, args := range [][]string{
		{"automation", "b", "needs", "both"},      // less: needs a window and a place
		{"automation", "b", "default", "enabled"}, // flips its times
		{"place", "rule", "b", "office", "off"},   // one place fewer turning it on
		{"schedule", "remove", "b", "1"},          // its window turning it on
	} {
		if err := runCLI(t, args...); err == nil {
			t.Errorf("%v: weakened a locked block", args)
		}
	}
	// The lock bypass this closes: a first place on a locked continuous
	// block made it enabled only there.
	mustRun(t, "block", "create", "c")
	mustRun(t, "block", "add", "c", "reddit.com")
	mustRun(t, "start", "c", "--for", "1h", "--yes")
	if err := runCLI(t, "place", "rule", "c", "office", "on"); err == nil {
		t.Error("a first place on a locked continuous block was allowed")
	}
	// A schedule lock holds only while the automation has the block on:
	// leaving it continuous is refused while it holds.
	mustRun(t, "block", "create", "s")
	mustRun(t, "block", "add", "s", "reddit.com")
	mustRun(t, "place", "rule", "s", "home", "on")
	mustRun(t, "schedule", "lock", "s", "on", "--yes")
	if err := d.ActivateBlock("s", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "place", "rule", "s", "home", "off"); err == nil {
		t.Error("the last place of a schedule-locked block was removed while it held")
	}
}
