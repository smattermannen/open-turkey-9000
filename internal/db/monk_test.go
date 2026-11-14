package db

import (
	"reflect"
	"testing"
	"time"
)

// R-MONK-1: the monk-mode block of older versions is removed when the
// database is opened, and a running one goes on as the mode, to the same end.
func TestR_MONK_1_MigratesRunningBlock(t *testing.T) {
	d, path := openTest(t)
	until := time.Now().Add(time.Hour).Unix()
	for _, name := range []string{legacyMonkBlock, "work"} {
		if err := d.CreateBlock(name); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.ActivateBlock(legacyMonkBlock, false, 0, until, false); err != nil {
		t.Fatal(err)
	}
	if err := d.SetLockExtras(legacyMonkBlock, LockExtras{StopAt: until}); err != nil {
		t.Fatal(err)
	}
	// As a database from before the change: not looked at yet.
	if _, err := d.conn.Exec("DELETE FROM settings WHERE key = ?", settingMonkMigrated); err != nil {
		t.Fatal(err)
	}
	d.Close()

	d, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if got, err := d.MonkUntil(); err != nil || got != until {
		t.Errorf("MonkUntil = %d, %v; want %d", got, err, until)
	}
	blocks, err := d.ListBlocks()
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 || blocks[0].Name != "work" {
		t.Errorf("blocks after the upgrade = %+v; want only work", blocks)
	}
	if active, err := d.GetActiveBlocks(); err != nil || len(active) != 0 {
		t.Errorf("active blocks = %+v, %v; want none", active, err)
	}
	var lists int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM web_lists").Scan(&lists); err != nil || lists != 2 {
		t.Errorf("website lists left = %d, %v; want work's two", lists, err)
	}

	// Once only: a block the user names monk-mode afterwards is theirs.
	if err := d.CreateBlock(legacyMonkBlock); err != nil {
		t.Fatal(err)
	}
	d.Close()
	if d, err = OpenDB(path); err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.GetBlock(legacyMonkBlock); err != nil {
		t.Errorf("a later block named monk-mode was removed: %v", err)
	}
}

// R-MONK-1: a monk-mode block that isn't running just goes.
func TestR_MONK_1_MigratesIdleBlock(t *testing.T) {
	d, path := openTest(t)
	if err := d.CreateBlock(legacyMonkBlock); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("DELETE FROM settings WHERE key = ?", settingMonkMigrated); err != nil {
		t.Fatal(err)
	}
	d.Close()
	d, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if blocks, _ := d.ListBlocks(); len(blocks) != 0 {
		t.Errorf("blocks = %+v; want none", blocks)
	}
	if until, err := d.MonkUntil(); err != nil || until != 0 {
		t.Errorf("MonkUntil = %d, %v; want 0", until, err)
	}
}

// R-MONK-1: monk mode's time only grows.
func TestR_MONK_1_ExtendOnly(t *testing.T) {
	d, _ := openTest(t)
	for _, step := range []struct{ set, want int64 }{{1000, 1000}, {500, 1000}, {2000, 2000}} {
		if err := d.ExtendMonk(step.set); err != nil {
			t.Fatal(err)
		}
		if got, _ := d.MonkUntil(); got != step.want {
			t.Errorf("after ExtendMonk(%d): %d, want %d", step.set, got, step.want)
		}
	}
	if err := d.SetMonkUntil(0); err != nil {
		t.Fatal(err)
	}
	if err := d.ExtendMonk(700); err != nil {
		t.Fatal(err)
	}
	if got, _ := d.MonkUntil(); got != 700 {
		t.Errorf("after it ended, ExtendMonk(700) gave %d", got)
	}
}

// R-MONK-2: the network state saved first is the one kept.
func TestR_MONK_2_NetSavedOnce(t *testing.T) {
	d, _ := openTest(t)
	before := MonkNet{Wifi: true, Ethernet: map[string]bool{"eth0": true, "eth1": false}}
	if saved, err := d.SaveMonkNetIfNone(before); err != nil || !saved {
		t.Fatalf("first save: %v, %v", saved, err)
	}
	if saved, err := d.SaveMonkNetIfNone(MonkNet{Ethernet: map[string]bool{"eth0": false}}); err != nil || saved {
		t.Fatalf("second save replaced the first: %v, %v", saved, err)
	}
	got, err := d.MonkNet()
	if err != nil || got == nil || !reflect.DeepEqual(*got, before) {
		t.Errorf("MonkNet = %+v, %v; want %+v", got, err, before)
	}
	if err := d.SaveMonkNet(nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := d.MonkNet(); got != nil {
		t.Errorf("MonkNet after forgetting = %+v", got)
	}
	if saved, err := d.SaveMonkNetIfNone(before); err != nil || !saved {
		t.Errorf("save after forgetting: %v, %v", saved, err)
	}
}

// R-MONK-1: trusted time doesn't follow the clock forward while monk mode
// runs, or setting the clock ahead would end it.
func TestR_MONK_1_HoldsTrustedTime(t *testing.T) {
	d, _ := openTest(t)
	if err := d.SetSetting(SettingTamperClock, "on"); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	trusted, err := d.TrustedNow(now)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.ExtendMonk(trusted + 3600); err != nil {
		t.Fatal(err)
	}
	later, err := d.TrustedNow(now.Add(48 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if later >= trusted+3600 {
		t.Errorf("a clock set two days ahead ended monk mode: trusted %d -> %d", trusted, later)
	}
}
