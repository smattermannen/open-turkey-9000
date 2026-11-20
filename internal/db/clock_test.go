package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAdvanceClock(t *testing.T) {
	prev := &clockState{BootID: "a", Boottime: 100, Trusted: 1000}
	cases := []struct {
		name     string
		wall     int64
		bootID   string
		boottime int64
		lock     bool
		want     int64
	}{
		{"wall jumps forward during lock", 999999, "a", 110, true, 1010},
		{"wall jumps backward during lock", 5, "a", 110, true, 1010},
		{"reboot during lock counts no downtime", 999999, "b", 3, true, 1000},
		{"no lock follows wall", 5000, "a", 110, false, 5000},
	}
	for _, c := range cases {
		if got := advanceClock(prev, c.wall, c.bootID, c.boottime, c.lock).Trusted; got != c.want {
			t.Errorf("%s: trusted = %d; want %d", c.name, got, c.want)
		}
	}
	if got := advanceClock(nil, 42, "a", 1, true).Trusted; got != 42 {
		t.Errorf("first use: trusted = %d; want 42", got)
	}
}

// §4.10 clock check: powered-off time counts once the internet's time
// proves it, never more than the hardware clock says passed; without an
// earlier anchor nothing is given back; a time before the last anchor is
// refused.
func TestCheckClock(t *testing.T) {
	anchored := clockState{BootID: "a", Boottime: 500, Trusted: 1100,
		Anchor: clockAnchor{Boot: "a", Boottime: 400, Real: 1000, Trusted: 1000, RTC: 5000}}
	// Rebooted after an hour off: the new boot counted 10 s so far.
	rebooted := anchored
	rebooted.BootID, rebooted.Boottime, rebooted.Trusted = "b", 10, 1110
	cases := []struct {
		name         string
		prev         clockState
		real, rtc    int64
		rtcOK        bool
		wantTrusted  int64
		wantCredited int64
	}{
		{"an hour off, both clocks agree", rebooted, 1000 + 3610, 5000 + 3610, true, 4610, 3500},
		{"the hardware clock says less: it wins", rebooted, 1000 + 3610, 5000 + 1000, true, 2000, 890},
		{"the hardware clock moved forward in firmware: internet time wins", rebooted, 1000 + 3610, 5000 + 99999, true, 4610, 3500},
		{"no hardware clock: internet time", rebooted, 1000 + 3610, 0, false, 4610, 3500},
		{"same boot, nothing off: nothing given back", anchored, 1100, 5100, true, 1100, 0},
		{"no anchor yet: nothing given back", clockState{BootID: "b", Boottime: 10, Trusted: 1110}, 99999, 5000, true, 1110, 0},
	}
	for _, c := range cases {
		next, credited, ok := checkClock(c.prev, c.prev.BootID, c.prev.Boottime, c.real, c.rtc, c.rtcOK)
		if !ok || next.Trusted != c.wantTrusted || credited != c.wantCredited {
			t.Errorf("%s: trusted %d credited %d ok %v; want %d, %d", c.name, next.Trusted, credited, ok, c.wantTrusted, c.wantCredited)
		}
		if next.Anchor.Real != c.real || next.Anchor.Trusted != next.Trusted || next.Anchor.Boot != c.prev.BootID {
			t.Errorf("%s: anchor %+v not taken at the check", c.name, next.Anchor)
		}
	}
	if _, _, ok := checkClock(rebooted, "b", 10, 999, 5000, true); ok {
		t.Error("an internet time before the last anchor was taken")
	}
}

// The anchor is a floor for the boot it was taken in: trusted time never
// falls below anchor trusted + boot clock since, whatever was written.
func TestAdvanceClockKeepsAnchorFloor(t *testing.T) {
	prev := &clockState{BootID: "b", Boottime: 100, Trusted: 1000,
		Anchor: clockAnchor{Boot: "b", Boottime: 90, Real: 7000, Trusted: 5000}}
	if got := advanceClock(prev, 1, "b", 110, true).Trusted; got != 5020 {
		t.Errorf("trusted %d; want the anchor's 5000 + 20 s", got)
	}
	if got := advanceClock(prev, 1, "c", 5, true).Trusted; got != 1000 {
		t.Errorf("after a reboot the old boot's anchor gave %d; want 1000 until the next check", got)
	}
}

// §3.4, §4.10: a 2-hour timer lock, the computer off for 3 hours. Without
// the check the off time isn't counted; with internet time it is, and the
// lock ends. Moving the system clock plays no part.
func TestClockCheckEndsLockAfterPoweredOffTime(t *testing.T) {
	d, _ := openTest(t)
	dir := t.TempDir()
	savedPath, savedClock := bootIDPath, bootClock
	t.Cleanup(func() { bootIDPath, bootClock = savedPath, savedClock })
	bootIDPath = filepath.Join(dir, "boot_id")
	boot := func(id string, at int64) {
		if err := os.WriteFile(bootIDPath, []byte(id+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		bootClock = func() (int64, error) { return at, nil }
	}
	if err := d.SetSetting(SettingTamperClock, "on"); err != nil {
		t.Fatal(err)
	}
	start := time.Unix(1_800_000_000, 0)
	boot("one", 1000)
	trusted, err := d.TrustedNow(start)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetSettingLock(SettingLock{Name: "web-only", LockedUntil: trusted + 7200}); err != nil {
		t.Fatal(err)
	}
	if due, err := d.ClockCheckDue(start); err != nil || !due {
		t.Fatalf("check due %v (%v); want due in a boot without an anchor", due, err)
	}
	if _, err := d.CheckClock(start, start, 50_000, true); err != nil {
		t.Fatal(err)
	}
	if due, _ := d.ClockCheckDue(start); due {
		t.Error("check still due in the boot it was taken in")
	}

	// 10 minutes on, then off for 3 hours; a new boot, the system clock set
	// back a year (it plays no part).
	boot("one", 1600)
	d.TrustedNow(start.Add(10 * time.Minute))
	boot("two", 20)
	wall := start.AddDate(-1, 0, 0)
	if got, _ := d.TrustedNow(wall); got >= trusted+7200 {
		t.Fatalf("trusted %d: the lock ended before any check", got)
	}
	if due, _ := d.ClockCheckDue(wall); !due {
		t.Fatal("check not due after a reboot")
	}
	real := start.Add(10*time.Minute + 3*time.Hour + 20*time.Second)
	credited, err := d.CheckClock(wall, real, 50_000+600+3*3600+20, true)
	if err != nil {
		t.Fatal(err)
	}
	// The 3 hours off, and the new boot's first 20 s (not counted before).
	if credited != 3*3600+20 {
		t.Errorf("gave back %d s; want the 3 hours off and 20 s", credited)
	}
	// (Once no lock runs, trusted time follows the system clock again.)
	if got, _ := d.TrustedNow(real); got < trusted+7200 {
		t.Errorf("trusted %d: the 2-hour lock (until %d) still runs after 3 h 10 min", got, trusted+7200)
	}
}

// R-WEAK-1 / §3.4: an anchor from an earlier lock must not shorten the next
// one. Lock, check, lock over; the clock set back a day with no lock
// running; a new 24-hour lock: neither the floor nor the next check may end
// it early.
func TestClockAnchorDoesNotShortenNextLock(t *testing.T) {
	d, _ := openTest(t)
	dir := t.TempDir()
	savedPath, savedClock := bootIDPath, bootClock
	t.Cleanup(func() { bootIDPath, bootClock = savedPath, savedClock })
	bootIDPath = filepath.Join(dir, "boot_id")
	boot := func(id string, at int64) {
		if err := os.WriteFile(bootIDPath, []byte(id+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		bootClock = func() (int64, error) { return at, nil }
	}
	if err := d.SetSetting(SettingTamperClock, "on"); err != nil {
		t.Fatal(err)
	}
	real := time.Unix(1_800_000_000, 0)
	boot("one", 1000)
	trusted, _ := d.TrustedNow(real)
	if err := d.SetSettingLock(SettingLock{Name: "web-only", LockedUntil: trusted + 60}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CheckClock(real, real, 50_000, true); err != nil {
		t.Fatal(err)
	}
	// The first lock is over; the system clock is set back a day.
	boot("one", 1100)
	real = real.Add(100 * time.Second)
	d.TrustedNow(real) // the lock is seen to be over
	back := real.Add(-24 * time.Hour)
	trusted, _ = d.TrustedNow(back)
	if trusted != back.Unix() {
		t.Fatalf("with no lock running trusted time is %d, not the clock %d", trusted, back.Unix())
	}
	if err := d.SetSettingLock(SettingLock{Name: "no-private", LockedUntil: trusted + 24*3600}); err != nil {
		t.Fatal(err)
	}
	boot("one", 1101)
	if got, _ := d.TrustedNow(back.Add(time.Second)); got >= trusted+24*3600 {
		t.Fatalf("the old anchor's floor ended a fresh 24-hour lock after 1 s (trusted %d, until %d)", got, trusted+24*3600)
	}
	// After a reboot the next check may not use the old anchor either.
	boot("two", 5)
	real = real.Add(10 * time.Second)
	if _, err := d.CheckClock(back, real, 50_000+110, true); err != nil {
		t.Fatal(err)
	}
	if got, _ := d.TrustedNow(back.Add(10 * time.Second)); got >= trusted+24*3600 {
		t.Errorf("the check after a reboot ended a fresh 24-hour lock (trusted %d, until %d)", got, trusted+24*3600)
	}
}
