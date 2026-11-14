package enforce

import (
	"os"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
)

// net is a fake network: nothing here touches the machine running the tests.
type fakeNet struct {
	wifi, wifiKnown, fwCut bool
	eth                    map[string]bool
}

func (n *fakeNet) install() {
	wifiRadio = func() (bool, bool) { return n.wifi, n.wifiKnown }
	setWifiRadio = func(on bool) error { n.wifi = on; return nil }
	ethernetLinks = func() map[string]bool {
		out := map[string]bool{}
		for k, v := range n.eth {
			out[k] = v
		}
		return out
	}
	setLink = func(name string, up bool) error { n.eth[name] = up; return nil }
	firewallIsCut = func() bool { return n.fwCut }
	cutFirewall = func() error { n.fwCut = true; return nil }
	monkDeviceless = func() bool { return false }
}

// Every test runs with a fake network, so none can cut the real one.
func TestMain(m *testing.M) {
	(&fakeNet{eth: map[string]bool{}}).install()
	os.Exit(m.Run())
}

func newFake() *fakeNet {
	n := &fakeNet{wifi: true, wifiKnown: true, eth: map[string]bool{"eth0": true, "eth1": false}}
	n.install()
	return n
}

func trustedNow(t *testing.T, d *db.DB) int64 {
	t.Helper()
	tr, err := d.TrustedNow(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

// R-MONK-1, R-MONK-2: it is no block, blocks every website and cuts the
// connection three ways, saving how it was.
func TestR_MONK_2_CutsThreeWays(t *testing.T) {
	d := testDB(t)
	n := newFake()
	if err := StartMonk(d, time.Hour); err != nil {
		t.Fatal(err)
	}
	if blocks, _ := d.ListBlocks(); len(blocks) != 0 {
		t.Errorf("monk mode made blocks: %+v", blocks)
	}
	if n.wifi || n.eth["eth0"] || !n.fwCut {
		t.Errorf("not cut: wifi %v, eth %v, firewall %v", n.wifi, n.eth, n.fwCut)
	}
	saved, _ := d.MonkNet()
	if saved == nil || !saved.Wifi || !saved.Ethernet["eth0"] || saved.Ethernet["eth1"] {
		t.Errorf("saved state = %+v", saved)
	}
	rules, err := BuildRules(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules.Block) != 1 || rules.Block[0] != "*.*" || len(rules.Exceptions) != 0 && rules.Exceptions[0] != db.FileException {
		t.Errorf("rules block %v, exceptions %v", rules.Block, rules.Exceptions)
	}
	// A longer time extends, a shorter changes nothing.
	before, _ := d.MonkUntil()
	StartMonk(d, time.Minute)
	if after, _ := d.MonkUntil(); after != before {
		t.Errorf("a shorter monk mode changed the end: %d -> %d", before, after)
	}
}

// R-MONK-2: whatever the user switches back on is put down again, and the
// saved state stays the one from before the first cut.
func TestR_MONK_2_HeldEveryCycle(t *testing.T) {
	d := testDB(t)
	n := newFake()
	StartMonk(d, time.Hour)
	n.wifi, n.eth["eth0"], n.fwCut = true, true, false
	n.eth["eth1"] = true // plugged in or switched on meanwhile
	if !enforceMonk(d, trustedNow(t, d)) {
		t.Fatal("monk mode not on")
	}
	if n.wifi || n.eth["eth0"] || n.eth["eth1"] || !n.fwCut {
		t.Errorf("not held: wifi %v, eth %v, firewall %v", n.wifi, n.eth, n.fwCut)
	}
	saved, _ := d.MonkNet()
	if saved.Ethernet["eth1"] {
		t.Errorf("the cut replaced the saved state: %+v", saved)
	}
}

// R-MONK-2: at its end, after a restart or a crash, the saved state returns.
func TestR_MONK_2_Restored(t *testing.T) {
	d := testDB(t)
	n := newFake()
	StartMonk(d, time.Hour)
	// Time is over (as seen from a later trusted time, e.g. after a reboot).
	over := trustedNow(t, d) + 2*3600
	if enforceMonk(d, over) {
		t.Fatal("monk mode still on after its time")
	}
	if !n.wifi || !n.eth["eth0"] || n.eth["eth1"] {
		t.Errorf("not restored: wifi %v, eth %v", n.wifi, n.eth)
	}
	if saved, _ := d.MonkNet(); saved != nil {
		t.Errorf("saved state kept: %+v", saved)
	}
	if until, _ := d.MonkUntil(); until != 0 {
		t.Errorf("end time kept: %d", until)
	}
	// A crash after the end time was cleared but before restoring: the
	// saved state alone restores.
	n.wifi, n.eth["eth0"] = false, false
	d.SaveMonkNet(&db.MonkNet{Wifi: true, Ethernet: map[string]bool{"eth0": true}})
	enforceMonk(d, over)
	if !n.wifi || !n.eth["eth0"] {
		t.Errorf("crash not restored: wifi %v, eth %v", n.wifi, n.eth)
	}
	// Wi-Fi that was off stays off.
	n2 := newFake()
	n2.wifi = false
	d2 := testDB(t)
	StartMonk(d2, time.Hour)
	enforceMonk(d2, trustedNow(t, d2)+2*3600)
	if n2.wifi {
		t.Error("Wi-Fi that was off was switched on")
	}
}

// R-MONK-2: no state is saved or cut on a database used through KC9K_DB.
func TestR_MONK_2_DevicelessNeverCuts(t *testing.T) {
	d := testDB(t)
	n := newFake()
	monkDeviceless = func() bool { return true }
	StartMonk(d, time.Hour)
	if !n.wifi || !n.eth["eth0"] || n.fwCut {
		t.Errorf("cut anyway: wifi %v, eth %v, firewall %v", n.wifi, n.eth, n.fwCut)
	}
}

// R-MONK-1: a list's monk action and a ladder step start monk mode.
func TestR_MONK_1_StartedByConsequence(t *testing.T) {
	d := testDB(t)
	fakeDevice(t)
	n := newFake()
	if err := Run(d, Consequence{Kind: db.ConsequenceMonk, Minutes: 5}, "test"); err != nil {
		t.Fatal(err)
	}
	if until, _ := d.MonkUntil(); until == 0 || n.wifi {
		t.Errorf("monk not started: until %d, wifi %v", until, n.wifi)
	}
}
