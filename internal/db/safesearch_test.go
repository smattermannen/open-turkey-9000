package db

import (
	"path/filepath"
	"testing"
)

// §6 2026-11-18: SafeSearch became a browser setting per engine. An upgrade
// carries the one setting's on/off state and its lock over to every engine,
// once; a running lock holds each engine as it held SafeSearch.
func TestMigrateSafeSearch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	d, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetSetting("browser."+OldSafeSearch, "on"); err != nil {
		t.Fatal(err)
	}
	lock := SettingLock{Name: OldSafeSearch, LockedUntil: 4102444800, LockChars: 420, Style: "words",
		PasswordHash: "$2a$hash", Order: "timer,text,password", RangeDays: 3, RangeFrom: 60, RangeTo: 120}
	if err := d.SetSettingLock(lock); err != nil {
		t.Fatal(err)
	}
	d.Close()
	for i := 0; i < 2; i++ { // the upgrade, then a later open
		d, err = OpenDB(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range SafeSearchSettings {
			if v, _ := d.GetSetting("browser." + name); v != "on" {
				t.Errorf("open %d: %s = %q", i, name, v)
			}
			want := lock
			want.Name = name
			if got, _ := d.GetSettingLock(name); got != want {
				t.Errorf("open %d: %s lock = %+v; want %+v", i, name, got, want)
			}
		}
		if v, _ := d.GetSetting("browser." + OldSafeSearch); v != "" {
			t.Errorf("open %d: the old setting is still there", i)
		}
		if old, _ := d.GetSettingLock(OldSafeSearch); old != (SettingLock{Name: OldSafeSearch}) {
			t.Errorf("open %d: the old lock is still there: %+v", i, old)
		}
		d.Close()
	}
}
