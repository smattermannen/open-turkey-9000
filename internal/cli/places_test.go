package cli

import (
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

func TestPlaces(t *testing.T) {
	d := testDB(t)
	home := []db.Network{{BSSID: "AA:AA:AA:AA:AA:01", SSID: "mine"}, {BSSID: "AA:AA:AA:AA:AA:02", SSID: "neighbour"},
		{BSSID: "AA:AA:AA:AA:AA:03", SSID: "other neighbour"}}
	cafe := []db.Network{{BSSID: "BB:BB:BB:BB:BB:01", SSID: "cafe"}, {BSSID: "BB:BB:BB:BB:BB:02", SSID: "shop"}}
	seen := home
	oldScan := enforce.ScanWifi
	enforce.ScanWifi = func(bool) ([]db.Network, error) { return seen, nil }
	t.Cleanup(func() { enforce.ScanWifi = oldScan })

	mustRun(t, "place", "add", "home")
	mustRun(t, "block", "create", "b")
	// A rule only says where the block is enabled: a block gets one before
	// it has anything to enforce (owner, 2026-11-16: it was refused).
	mustRun(t, "place", "rule", "b", "home", "on")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "schedule", "lock", "b", "on", "--yes") // places protect a block only through it (§3.4)
	p, err := d.GetPlace("home")
	if err != nil || len(p.Networks) != 3 || len(p.Blocks) != 1 {
		t.Fatalf("place = %+v, %v", p, err)
	}

	// At home: two of three networks are enough; one is not; none seen counts as there.
	if !enforce.AtPlace(p, home[:2]) || enforce.AtPlace(p, home[:1]) || enforce.AtPlace(p, cafe) || !enforce.AtPlace(p, nil) {
		t.Error("AtPlace disagrees")
	}
	on, err := enforce.PlaceBlocks(d, time.Now())
	if err != nil || !on["b"] {
		t.Fatalf("at home the rule doesn't turn the block on: %v %v", on, err)
	}
	// The block's schedule lock is on there: the rule can't be taken away.
	if held, _ := enforce.ScheduleActiveNow(d, "b", time.Now()); !held {
		t.Error("the block isn't held at the place")
	}
	if err := d.ActivateBlock("b", false, 0, 0, true); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "place", "rule", "b", "home", "off"); err == nil {
		t.Error("the rule was removed while at the place")
	}
	if err := runCLI(t, "place", "remove", "home"); err == nil {
		t.Error("the place was removed while it holds a block")
	}

	// Saving the place again elsewhere only adds networks.
	seen = cafe
	mustRun(t, "place", "add", "cafe")
	if p, _ = d.GetPlace("home"); len(p.Networks) != 3 {
		t.Errorf("home changed: %+v", p.Networks)
	}
}

// §3.6 (owner, 2026-11-16): the user picks how many of a place's networks
// must be in range. Fewer is always allowed; more weakens the blocks on
// there, refused while one is locked; more than the place knows is refused.
func TestPlaceNeed(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	home := []db.Network{{BSSID: "AA:AA:AA:AA:AA:01"}, {BSSID: "AA:AA:AA:AA:AA:02"}, {BSSID: "AA:AA:AA:AA:AA:03"},
		{BSSID: "AA:AA:AA:AA:AA:04"}}
	oldScan := enforce.ScanWifi
	enforce.ScanWifi = func(bool) ([]db.Network, error) { return home, nil }
	t.Cleanup(func() { enforce.ScanWifi = oldScan })

	mustRun(t, "place", "add", "home", "--need", "3")
	p, _ := d.GetPlace("home")
	if p.Needed() != 3 || !enforce.AtPlace(p, home[:3]) || enforce.AtPlace(p, home[:2]) {
		t.Fatalf("need 3: %+v", p)
	}
	if err := runCLI(t, "place", "need", "home", "5"); err == nil {
		t.Error("more networks needed than the place knows")
	}
	if err := runCLI(t, "place", "need", "home", "0"); err == nil {
		t.Error("zero networks needed")
	}
	mustRun(t, "place", "need", "home", "1")
	if p, _ = d.GetPlace("home"); !enforce.AtPlace(p, home[3:]) {
		t.Error("one network is enough after 'need 1'")
	}
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "place", "rule", "b", "home", "on")
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runCLI(t, "place", "need", "home", "4"); err == nil {
		t.Error("needing more networks weakened a locked block")
	}
	mustRun(t, "place", "need", "home", "1") // the same: free
	// An old place (no number chosen) needs two, or one if it knows one.
	if (db.Place{Networks: home}).Needed() != 2 || (db.Place{Networks: home[:1]}).Needed() != 1 {
		t.Error("default need")
	}
}
