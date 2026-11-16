package db

import "testing"

// 2026-11-16: place rules had actions for a day; a place rule only enables
// or lifts its block (§3.6). Opening drops the stored sets and the daemon's
// notes about arriving; apps' and window titles' sets stay.
func TestPlaceTriggersDropped(t *testing.T) {
	d, path := openTest(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	blockID, _ := d.getBlockID("b")
	if _, err := d.conn.Exec(`INSERT INTO trigger_actions (block_id, trigger, actions) VALUES
		(?, 'place:1', '[{"name":"logout"}]'), (?, 'apps', '[{"name":"close-browser"}]')`, blockID, blockID); err != nil {
		t.Fatal(err)
	}
	if err := d.SetSetting("place.here.1", "1"); err != nil {
		t.Fatal(err)
	}
	d.Close()
	d, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var n int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM trigger_actions WHERE trigger LIKE 'place:%'").Scan(&n); err != nil || n != 0 {
		t.Errorf("place action sets left: %d, %v", n, err)
	}
	if v, _ := d.GetSetting("place.here.1"); v != "" {
		t.Errorf("arrival note left: %q", v)
	}
	if got, _ := d.TriggerActions("b", TriggerApps); len(got) != 1 || got[0].Name != ConsequenceCloseBrowser {
		t.Errorf("the apps' set went too: %+v", got)
	}
	if ValidTriggerGroup("place:1") || CheckTriggerAction(TriggerApps, TriggerAction{Name: "enable"}) == nil {
		t.Error("a place group or the enable action is still known")
	}
}
