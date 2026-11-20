package enforce

import (
	"slices"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/freeze"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// R-FREEZE-1, §3.8: a running freeze acts by its freeze mode; "allowed apps
// only" closes everything but the freeze's app list; nothing once it ended.
func TestEnforceFreezeByMode(t *testing.T) {
	d := testDB(t)
	var did []string
	var apps []string
	oldL, oldP, oldS, oldA := freezeLogout, freezePowerOff, freezeLockScreen, freezeCloseApps
	t.Cleanup(func() { freezeLogout, freezePowerOff, freezeLockScreen, freezeCloseApps = oldL, oldP, oldS, oldA })
	freezeLogout = func() error { did = append(did, "logout"); return nil }
	freezePowerOff = func() error { did = append(did, "shutdown"); return nil }
	freezeLockScreen = func() error { did = append(did, "lock"); return nil }
	freezeCloseApps = func(a []string) int { did = append(did, "apps"); apps = a; return 0 }
	if err := d.SetSetting(SettingFreezeApps, `["libreoffice","org.gnome.Calculator"]`); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for _, mode := range []string{freeze.ModeLock, freeze.ModeLogout, freeze.ModeShutdown, freeze.ModeApps} {
		did = nil
		if err := d.SetFreeze(now.Add(time.Hour).Unix(), mode); err != nil {
			t.Fatal(err)
		}
		enforceFreeze(d, now)
		if len(did) != 1 || did[0] != mode {
			t.Errorf("mode %s did %v", mode, did)
		}
	}
	if !slices.Equal(apps, []string{"libreoffice", "org.gnome.Calculator"}) {
		t.Errorf("allowed apps passed = %v", apps)
	}
	did = nil
	enforceFreeze(d, now.Add(2*time.Hour)) // over
	if len(did) != 0 {
		t.Errorf("after the end: %v", did)
	}
}

// A scheduled "allowed apps only" freeze that extends a running lock freeze
// keeps the lock (R-FREEZE-1).
func TestScheduledAppsFreezeKeepsRunningLock(t *testing.T) {
	d := testDB(t)
	now := time.Date(2026, 11, 23, 10, 0, 0, 0, time.Local)
	if err := d.AddFreezeSchedule(schedule.Window{Days: schedule.AllDays, Start: 9 * 60, End: 12 * 60}, freeze.ModeApps, db.NoScheduleLock); err != nil {
		t.Fatal(err)
	}
	if err := d.SetFreeze(now.Add(30*time.Minute).Unix(), freeze.ModeLock); err != nil {
		t.Fatal(err)
	}
	if err := reconcileFreezeSchedules(d, now); err != nil {
		t.Fatal(err)
	}
	if until, _ := d.FreezeUntil(); until <= now.Add(30*time.Minute).Unix() {
		t.Fatalf("not extended: %d", until)
	}
	if m, _ := d.FreezeMode(); m != freeze.ModeLock {
		t.Errorf("mode = %q; want lock", m)
	}
}
