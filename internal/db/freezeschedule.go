// freezeschedule.go — recurring Frozen Chicken windows, such as every night
// from 23:00 to 06:00.
package db

import (
	"fmt"

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
}

// AddFreezeSchedule stores a recurring freeze window.
func (d *DB) AddFreezeSchedule(w schedule.Window, mode string) error {
	_, err := d.conn.Exec("INSERT INTO freeze_schedules (days, start, end, mode) VALUES (?, ?, ?, ?)",
		w.Days, w.Start, w.End, mode)
	if err != nil {
		return fmt.Errorf("failed to add the freeze schedule: %w", err)
	}
	return nil
}

// ListFreezeSchedules returns every freeze window in the order it was added.
func (d *DB) ListFreezeSchedules() ([]FreezeSchedule, error) {
	rows, err := d.conn.Query("SELECT id, days, start, end, mode FROM freeze_schedules ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("failed to list freeze schedules: %w", err)
	}
	defer rows.Close()
	var out []FreezeSchedule
	for rows.Next() {
		f := FreezeSchedule{Num: len(out) + 1}
		if err := rows.Scan(&f.ID, &f.Window.Days, &f.Window.Start, &f.Window.End, &f.Mode); err != nil {
			return nil, fmt.Errorf("failed to read a freeze schedule: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// RemoveFreezeSchedule removes a freeze window by its row ID.
func (d *DB) RemoveFreezeSchedule(id int) error {
	if _, err := d.conn.Exec("DELETE FROM freeze_schedules WHERE id = ?", id); err != nil {
		return fmt.Errorf("failed to remove the freeze schedule: %w", err)
	}
	return nil
}
