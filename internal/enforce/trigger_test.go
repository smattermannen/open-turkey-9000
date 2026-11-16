package enforce

import (
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
)

func fastConsequences(t *testing.T) *[]string {
	t.Helper()
	did := fakeDevice(t)
	oldDevice, oldWarn, oldRunning := deviceDelay, warnDelay, runningApps
	deviceDelay, warnDelay = 0, 0
	triggerFired.Lock()
	triggerFired.at = map[string]time.Time{}
	triggerFired.Unlock()
	t.Cleanup(func() { deviceDelay, warnDelay, runningApps = oldDevice, oldWarn, oldRunning })
	return did
}

func waitFor(t *testing.T, did *[]string, want string) {
	t.Helper()
	for i := 0; i < 100 && !strings.Contains(joinDid(did), want); i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(joinDid(did), want) {
		t.Fatalf("did %q; want %q in it", joinDid(did), want)
	}
}

// Owner, 2026-11-16: a block's apps and window titles take actions the
// user picks, one set per group. Without one, they close, as before.
func TestTriggerActionsStored(t *testing.T) {
	d := testDB(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	for group, want := range map[string]string{db.TriggerApps: db.TriggerClose, db.TriggerTitles: db.TriggerClose} {
		if got, _ := d.TriggerActions("b", group); len(got) != 1 || got[0].Name != want {
			t.Errorf("%s default = %+v", group, got)
		}
	}
	set := []db.TriggerAction{{Name: db.ConsequenceLogout, Minutes: 30}}
	if err := d.SetTriggerActions("b", db.TriggerApps, set); err != nil {
		t.Fatal(err)
	}
	if got, _ := d.TriggerActions("b", db.TriggerApps); len(got) != 1 || got[0] != set[0] {
		t.Errorf("stored = %+v", got)
	}
	if err := d.SetTriggerActions("b", db.TriggerTitles, []db.TriggerAction{}); err != nil {
		t.Fatal(err)
	}
	if got, _ := d.TriggerActions("b", db.TriggerTitles); len(got) != 0 {
		t.Errorf("no actions = %+v", got)
	}
	for _, bad := range []struct {
		group string
		a     db.TriggerAction
	}{{db.TriggerApps, db.TriggerAction{Name: "enable"}},
		{db.TriggerApps, db.TriggerAction{Name: db.ConsequenceMonk}}, {db.TriggerApps, db.TriggerAction{Name: "explode"}},
		{db.TriggerApps, db.TriggerAction{Name: db.TriggerClose, Minutes: 5}}} {
		if db.CheckTriggerAction(bad.group, bad.a) == nil {
			t.Errorf("%s %+v accepted", bad.group, bad.a)
		}
	}
}

// An app left open (no close action) fires the other actions, at most once
// a minute; a closing one fires them when it is closed.
func TestAppsTrigger(t *testing.T) {
	d := testDB(t)
	did := fastConsequences(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetTriggerActions("b", db.TriggerApps, []db.TriggerAction{{Name: db.ConsequenceCloseBrowser}}); err != nil {
		t.Fatal(err)
	}
	running := []string{"game"}
	runningApps = func(names []string) []string { return running }
	b := db.ActiveBlockDetail{BlockName: "b", Apps: []string{"game"}}
	now := time.Now()
	appsTrigger(d, b, now)
	waitFor(t, did, "close-browsers")
	appsTrigger(d, b, now.Add(10*time.Second)) // within a minute: not again
	time.Sleep(50 * time.Millisecond)
	if n := strings.Count(joinDid(did), "close-browsers"); n != 1 {
		t.Errorf("fired %d times within a minute", n)
	}
	appsTrigger(d, b, now.Add(2*time.Minute))
	waitFor(t, did, "close-browsers notify close-browsers")
	running = nil
	before := joinDid(did)
	appsTrigger(d, b, now.Add(5*time.Minute)) // not running: nothing
	time.Sleep(50 * time.Millisecond)
	if joinDid(did) != before {
		t.Errorf("fired without the app running: %q", joinDid(did))
	}
}

// §3.6: a place rule has no actions; at its place it enables its block.
func TestPlaceRuleEnables(t *testing.T) {
	d := testDB(t)
	home := []db.Network{{BSSID: "AA:AA:AA:AA:AA:01"}, {BSSID: "AA:AA:AA:AA:AA:02"}}
	oldScan := ScanWifi
	ScanWifi = func(bool) ([]db.Network, error) { return home, nil }
	t.Cleanup(func() { ScanWifi = oldScan })
	placeMu.Lock()
	placeAt = time.Time{}
	placeMu.Unlock()
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SavePlace("home", home); err != nil {
		t.Fatal(err)
	}
	p, _ := d.GetPlace("home")
	if err := d.SetPlaceRule("b", p.ID, true); err != nil {
		t.Fatal(err)
	}
	if on, _ := PlaceBlocks(d, time.Now()); !on["b"] {
		t.Error("a place rule didn't enable its block there")
	}
	if ruled, _ := PlaceRuled(d); !ruled["b"] {
		t.Error("a place-ruled block isn't automated")
	}
}
