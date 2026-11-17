package enforce

import (
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// R-MONK-3 (owner 2026-11-17, later): monk mode is switched on and off like
// any mode: switched on it runs while its window is open (and not outside
// it), with no windows or places whenever it is on; switched off it doesn't
// run. None of that is locked: only a run for a set time is (MonkEnd).
func TestMonkAutomationWindow(t *testing.T) {
	d := testDB(t)
	newFake()
	m, err := d.MonkMode()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 17, 10, 0, 0, 0, time.Local)
	day := 1 << uint(now.Weekday())
	spec := db.AutomationSpec{Own: []schedule.Window{{Days: day, Start: 9 * 60, End: 12 * 60}}}
	if err := d.SetModeAutomation(m.ID, spec); err != nil {
		t.Fatal(err)
	}
	trusted := now.Unix()
	if MonkAutomationOn(d, now) {
		t.Fatal("ran while switched off")
	}
	if err := d.SetModeActive(m.ID, true); err != nil {
		t.Fatal(err)
	}
	if !MonkAutomationOn(d, now) || MonkAutomationOn(d, now.Add(3*time.Hour)) {
		t.Error("on outside its window, or off in it")
	}
	if until, _ := d.MonkUntil(); until != 0 || MonkEnd(d, trusted) != 0 {
		t.Errorf("a switched-on run is locked: until %d", until)
	}
	// Without windows or places it runs whenever it is switched on.
	if err := d.SetModeAutomation(m.ID, db.AutomationSpec{}); err != nil {
		t.Fatal(err)
	}
	if !MonkAutomationOn(d, now) {
		t.Error("switched on with no automation, it doesn't run")
	}
	if err := d.SetModeActive(m.ID, false); err != nil {
		t.Fatal(err)
	}
	if MonkAutomationOn(d, now) {
		t.Error("switched off, it runs")
	}
}

// R-MONK-2/3: while its places decide, monk mode runs at its place and not
// away from it; the Wi-Fi radio stays on to see where it is, Ethernet and
// the firewall are cut.
func TestMonkAutomationPlace(t *testing.T) {
	d := testDB(t)
	n := newFake()
	library := []db.Network{{BSSID: "AA:AA:AA:AA:AA:01"}, {BSSID: "AA:AA:AA:AA:AA:02"}}
	seen := library
	oldScan := ScanWifi
	ScanWifi = func(bool) ([]db.Network, error) { return seen, nil }
	t.Cleanup(func() { ScanWifi = oldScan })
	ForgetSeenNetworks()
	if _, err := d.SavePlace("library", library); err != nil {
		t.Fatal(err)
	}
	p, _ := d.GetPlace("library")
	m, _ := d.MonkMode()
	if err := d.SetModeAutomation(m.ID, db.AutomationSpec{Places: []int{p.ID}}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetModeActive(m.ID, true); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	trusted := trustedNow(t, d)
	if !MonkAutomationOn(d, now) {
		t.Fatal("not on at its place")
	}
	if !enforceMonk(d, trusted) {
		t.Fatal("not on")
	}
	if !n.wifi || n.eth["eth0"] || !n.fwCut {
		t.Errorf("cuts: wifi %v, eth0 %v, firewall %v", n.wifi, n.eth["eth0"], n.fwCut)
	}
	// Away from the library: not kept on.
	seen = []db.Network{{BSSID: "EE:EE:EE:EE:EE:01"}}
	ForgetSeenNetworks()
	if MonkAutomationOn(d, now) {
		t.Error("on away from its place")
	}
}
