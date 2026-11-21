package db

import (
	"path/filepath"
	"reflect"
	"testing"
)

// Decision 2026-11-14: force-librewolf became block unsupported browsers.
// An upgrade carries over its on/off state and its lock exactly, once.
func TestMigrateForceLibreWolf(t *testing.T) {
	cases := []struct {
		name  string
		value string // old setting; "" none
		lock  *SettingLock
	}{
		{name: "on with every lock", value: "on", lock: &SettingLock{LockedUntil: 4102444800, LockChars: 420,
			Style: "words", PasswordHash: "$2a$hash", RestartBoot: "boot-1"}},
		{name: "on, timer lock only", value: "on", lock: &SettingLock{LockedUntil: 4102444800}},
		{name: "on, unlocked", value: "on"},
		{name: "off", value: "off"},
		{name: "never set"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "t.db")
			d, err := OpenDB(path)
			if err != nil {
				t.Fatal(err)
			}
			// What an install from before the change holds.
			if c.value != "" {
				if err := d.SetSetting(oldForceLibreWolfSetting, c.value); err != nil {
					t.Fatal(err)
				}
			}
			if c.lock != nil {
				old := *c.lock
				old.Name = oldForceLibreWolf
				if err := d.SetSettingLock(old); err != nil {
					t.Fatal(err)
				}
			}
			d.Close()

			for i := 0; i < 2; i++ { // the upgrade, then a later open
				d, err = OpenDB(path)
				if err != nil {
					t.Fatal(err)
				}
				got, _ := d.GetSetting(SettingUnsupportedBrowsers)
				if got != c.value {
					t.Errorf("open %d: %s = %q; want %q", i, SettingUnsupportedBrowsers, got, c.value)
				}
				if v, _ := d.GetSetting(oldForceLibreWolfSetting); v != "" {
					t.Errorf("open %d: the old setting is still there: %q", i, v)
				}
				lock, err := d.GetSettingLock(UnsupportedBrowsers)
				if err != nil {
					t.Fatal(err)
				}
				want := SettingLock{Name: UnsupportedBrowsers}
				if c.lock != nil {
					want = *c.lock
					want.Name = UnsupportedBrowsers
				}
				if !reflect.DeepEqual(lock, want) {
					t.Errorf("open %d: lock = %+v; want %+v", i, lock, want)
				}
				if old, _ := d.GetSettingLock(oldForceLibreWolf); !reflect.DeepEqual(old, SettingLock{Name: oldForceLibreWolf}) {
					t.Errorf("open %d: the old lock is still there: %+v", i, old)
				}
				d.Close()
			}
		})
	}
}
