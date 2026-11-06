package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSettingLocks(t *testing.T) {
	d, err := OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	l, err := d.GetSettingLock("safesearch")
	if err != nil || l.InForce(0, "boot") {
		t.Fatalf("unset lock = %+v, %v; want unlocked", l, err)
	}
	want := SettingLock{Name: "safesearch", LockedUntil: 2000, LockChars: 80, PasswordHash: "h", RestartBoot: "boot"}
	if err := d.SetSettingLock(want); err != nil {
		t.Fatal(err)
	}
	if got, err := d.GetSettingLock("safesearch"); err != nil || got != want {
		t.Fatalf("stored lock = %+v, %v; want %+v", got, err, want)
	}

	cases := []struct {
		lock    SettingLock
		trusted int64
		boot    string
		want    bool
	}{
		{SettingLock{LockedUntil: 2000}, 1999, "b", true},
		{SettingLock{LockedUntil: 2000}, 2000, "b", false},
		{SettingLock{RestartBoot: "b"}, 0, "b", true},
		{SettingLock{RestartBoot: "a"}, 0, "b", false},
		{SettingLock{LockChars: 30}, 0, "b", true},
		{SettingLock{PasswordHash: "h"}, 0, "b", true},
	}
	for _, c := range cases {
		if got := c.lock.InForce(c.trusted, c.boot); got != c.want {
			t.Errorf("%+v at %d on %s: in force = %v; want %v", c.lock, c.trusted, c.boot, got, c.want)
		}
	}

	if err := d.ClearSettingLock("safesearch"); err != nil {
		t.Fatal(err)
	}
	if locks, err := d.SettingLocks(); err != nil || len(locks) != 0 {
		t.Fatalf("after clear = %+v, %v", locks, err)
	}
}

// A setting's time lock freezes trusted time against the wall clock, like a
// block's: moving the clock forward must not end it.
func TestSettingTimeLockHoldsTrustedTime(t *testing.T) {
	d, err := OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	now := time.Now()
	trusted, err := d.TrustedNow(now)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetSettingLock(SettingLock{Name: "web-only", LockedUntil: trusted + 3600}); err != nil {
		t.Fatal(err)
	}
	later, err := d.TrustedNow(now.Add(48 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if later >= trusted+3600 {
		t.Errorf("trusted time jumped with the wall clock to %d; the lock (until %d) would have ended", later, trusted+3600)
	}
}
