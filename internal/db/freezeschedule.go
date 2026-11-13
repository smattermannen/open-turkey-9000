// freezeschedule.go — recurring Frozen Chicken windows, such as every night
// from 23:00 to 06:00.
package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/smattermannen/kc9k/internal/schedule"
)

const freezeScheduleSchema = `CREATE TABLE IF NOT EXISTS freeze_schedules (
    id    INTEGER PRIMARY KEY AUTOINCREMENT,
    days  INTEGER NOT NULL,
    start INTEGER NOT NULL,
    end   INTEGER NOT NULL,
    mode  TEXT NOT NULL DEFAULT 'lock'
);`

// FreezeSchedule is one recurring freeze window. Num is its 1-based position
// in ListFreezeSchedules, as shown to users.
type FreezeSchedule struct {
	ID     int
	Num    int
	Window schedule.Window
	Mode   string
	// LockLead is the lead time in minutes of its lock (R-FREEZE-2): from
	// then before its window until it ends it can't be removed.
	// NoScheduleLock: it has no lock.
	LockLead int
}

// Locked reports whether the schedule's lock is on at now: its window is
// open or opens within the lock's lead time.
func (f FreezeSchedule) Locked(now time.Time) bool {
	return f.LockLead >= 0 && f.Window.ActiveWithin(now, time.Duration(f.LockLead)*time.Minute)
}

// AddFreezeSchedule stores a recurring freeze window with a lock of lead
// minutes, or none (NoScheduleLock).
func (d *DB) AddFreezeSchedule(w schedule.Window, mode string, lead int) error {
	if lead < 0 {
		lead = NoScheduleLock
	}
	_, err := d.conn.Exec("INSERT INTO freeze_schedules (days, start, end, mode, lock_lead) VALUES (?, ?, ?, ?, ?)",
		w.Days, w.Start, w.End, mode, lead)
	if err != nil {
		return fmt.Errorf("failed to add the freeze schedule: %w", err)
	}
	return nil
}

// ListFreezeSchedules returns every freeze window in the order it was added.
func (d *DB) ListFreezeSchedules() ([]FreezeSchedule, error) {
	rows, err := d.conn.Query("SELECT id, days, start, end, mode, lock_lead FROM freeze_schedules ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("failed to list freeze schedules: %w", err)
	}
	defer rows.Close()
	var out []FreezeSchedule
	for rows.Next() {
		f := FreezeSchedule{Num: len(out) + 1}
		if err := rows.Scan(&f.ID, &f.Window.Days, &f.Window.Start, &f.Window.End, &f.Mode, &f.LockLead); err != nil {
			return nil, fmt.Errorf("failed to read a freeze schedule: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// SetFreezeScheduleLock sets a freeze schedule's lock lead time in minutes,
// or takes the lock away (NoScheduleLock).
func (d *DB) SetFreezeScheduleLock(id, lead int) error {
	if lead < 0 {
		lead = NoScheduleLock
	}
	if _, err := d.conn.Exec("UPDATE freeze_schedules SET lock_lead = ? WHERE id = ?", lead, id); err != nil {
		return fmt.Errorf("failed to save the freeze schedule's lock: %w", err)
	}
	return nil
}

// migrateFreezeScheduleLocks adds the lock column; schedules from before it
// were protected from 2 hours before their window, and keep that.
func migrateFreezeScheduleLocks(conn *sql.DB) error {
	var n int
	if err := conn.QueryRow("SELECT COUNT(*) FROM pragma_table_info('freeze_schedules') WHERE name = 'lock_lead'").Scan(&n); err != nil {
		return fmt.Errorf("failed to inspect freeze_schedules: %w", err)
	}
	if n > 0 {
		return nil
	}
	if err := addColumnIfMissing(conn, "freeze_schedules", "lock_lead", "INTEGER NOT NULL DEFAULT -1"); err != nil {
		return err
	}
	_, err := conn.Exec("UPDATE freeze_schedules SET lock_lead = 120")
	return err
}

// RemoveFreezeSchedule removes a freeze window by its row ID.
func (d *DB) RemoveFreezeSchedule(id int) error {
	if _, err := d.conn.Exec("DELETE FROM freeze_schedules WHERE id = ?", id); err != nil {
		return fmt.Errorf("failed to remove the freeze schedule: %w", err)
	}
	return nil
}
