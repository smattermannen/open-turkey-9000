package db

import (
	"path/filepath"
	"testing"

	"github.com/smattermannen/kc9k/internal/schedule"
)

// Before R-SCHED-1 was built the daemon switched scheduled blocks on; the
// upgrade switches them on once so their schedules keep working.
func TestMigrateScheduledSwitchOn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	d, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"sched", "plain", "quota"} {
		if err := d.CreateBlock(n); err != nil {
			t.Fatal(err)
		}
	}
	w := schedule.Window{Days: schedule.AllDays, Start: 60, End: 120}
	_ = d.AddSchedule("sched", w)
	_ = d.AddSchedule("quota", w)
	if _, err := d.conn.Exec(`INSERT INTO quotas (block_id, limit_seconds, used_seconds, day, open, session_end, last_tick, pending_limit)
		SELECT id, 60, 0, '', 0, 0, 0, 0 FROM blocks WHERE name = 'quota'`); err != nil {
		t.Fatal(err)
	}
	_ = d.SetSetting("schedule.stopped.sched", "1")
	_ = d.SetSetting(scheduledSwitchOnDone, "")
	if _, err := d.conn.Exec("DELETE FROM settings WHERE key = ?", scheduledSwitchOnDone); err != nil {
		t.Fatal(err)
	}
	d.Close()
	if d, err = OpenDB(path); err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for n, want := range map[string]bool{"sched": true, "plain": false, "quota": false} {
		if active, _ := d.IsBlockActive(n); active != want {
			t.Errorf("%s active = %v; want %v", n, active, want)
		}
	}
	if v, _ := d.GetSetting("schedule.stopped.sched"); v != "" {
		t.Error("old stopped-in-window mark kept")
	}
	// Once only: switching it off stays off.
	if err := d.DeactivateBlock("sched"); err != nil {
		t.Fatal(err)
	}
	d.Close()
	d, _ = OpenDB(path)
	defer d.Close()
	if active, _ := d.IsBlockActive("sched"); active {
		t.Error("migration ran twice")
	}
}
