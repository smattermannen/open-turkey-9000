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
	if err := runCLI(t, "place", "rule", "b", "home", "on"); err == nil {
		t.Error("an empty block got a place rule")
	}
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "place", "rule", "b", "home", "on")
	mustRun(t, "schedule", "lock", "b", "on") // places protect a block only through it (§3.4)
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
