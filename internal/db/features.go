// features.go — database support for the newer features: block modes,
// browser patterns, time locks and recurring schedules.
//
// The schema in db.go's `migrations` already declares the newer columns and
// tables for fresh databases. upgradeSchema here brings OLDER databases (made
// before those columns existed) up to date with idempotent ALTERs, so an
// existing install keeps working after an upgrade.
package db

import (
	"database/sql"
	"fmt"

	"github.com/smattermannen/kc9k/internal/schedule"
)

// upgradeSchema adds any columns that older databases are missing. New tables
// (patterns, schedules) are handled by CREATE TABLE IF NOT EXISTS already.
func upgradeSchema(conn *sql.DB) error {
	adds := []struct{ table, col, def string }{
		{"blocks", "mode", "TEXT NOT NULL DEFAULT 'blocklist'"},
		{"active_blocks", "locked_until", "INTEGER NOT NULL DEFAULT 0"},
		{"active_blocks", "scheduled", "BOOLEAN NOT NULL DEFAULT 0"},
		{"schedules", "allow", "BOOLEAN NOT NULL DEFAULT 0"},
		{"freeze_state", "mode", "TEXT NOT NULL DEFAULT 'lock'"},
		{"active_blocks", "lock_password", "TEXT NOT NULL DEFAULT ''"},
		{"active_blocks", "restart_boot", "TEXT NOT NULL DEFAULT ''"},
		{"active_blocks", "lock_delay", "INTEGER NOT NULL DEFAULT 0"},
		{"active_blocks", "stop_at", "INTEGER NOT NULL DEFAULT 0"},
		{"blocks", "act_block", "BOOLEAN NOT NULL DEFAULT 1"},
		{"blocks", "act_greyscale", "BOOLEAN NOT NULL DEFAULT 0"},
		{"blocks", "act_no_images", "BOOLEAN NOT NULL DEFAULT 0"},
		{"block_nepsis", "config", "TEXT NOT NULL DEFAULT ''"},
		{"events", "extra", "INTEGER NOT NULL DEFAULT 0"},
		{"pomodoro", "long_seconds", "INTEGER NOT NULL DEFAULT 0"},
		{"pomodoro", "every_n", "INTEGER NOT NULL DEFAULT 0"},
		{"blocks", "loosen_delay", "INTEGER NOT NULL DEFAULT 0"},
		{"active_blocks", "lock_style", "TEXT NOT NULL DEFAULT ''"},
		{"setting_locks", "lock_style", "TEXT NOT NULL DEFAULT ''"},
		{"active_blocks", "delay_until", "INTEGER NOT NULL DEFAULT 0"},
		{"active_blocks", "relock", "INTEGER NOT NULL DEFAULT 0"},
		{"active_blocks", "lifted_until", "INTEGER NOT NULL DEFAULT 0"},
		{"clusters", "phrases", "BOOLEAN NOT NULL DEFAULT 0"},
		{"setting_locks", "relock", "INTEGER NOT NULL DEFAULT 0"},
		{"setting_locks", "lifted_until", "INTEGER NOT NULL DEFAULT 0"},
		{"active_blocks", "lock_order", "TEXT NOT NULL DEFAULT ''"},
		{"active_blocks", "timer_after", "INTEGER NOT NULL DEFAULT 0"},
		{"active_blocks", "lifted_kinds", "TEXT NOT NULL DEFAULT ''"},
		{"setting_locks", "lock_order", "TEXT NOT NULL DEFAULT ''"},
		{"setting_locks", "timer_after", "INTEGER NOT NULL DEFAULT 0"},
		{"setting_locks", "lifted_kinds", "TEXT NOT NULL DEFAULT ''"},
		{"web_list_actions", "rate_min", "INTEGER NOT NULL DEFAULT 0"},
		{"active_blocks", "range_days", "INTEGER NOT NULL DEFAULT 0"},
		{"active_blocks", "range_from", "INTEGER NOT NULL DEFAULT 0"},
		{"active_blocks", "range_to", "INTEGER NOT NULL DEFAULT 0"},
		{"active_blocks", "range_invert", "BOOLEAN NOT NULL DEFAULT 0"},
		{"blocks", "view_password", "TEXT NOT NULL DEFAULT ''"},
		{"blocks", "users", "TEXT NOT NULL DEFAULT ''"},       // §3.2 affected users
		{"web_lists", "in_ads", "BOOLEAN NOT NULL DEFAULT 0"}, // §3.9 ad trigger
		{"clusters", "in_ads", "BOOLEAN NOT NULL DEFAULT 0"},
		{"block_breaks", "delay_length", "INTEGER NOT NULL DEFAULT 0"},
		{"block_breaks", "delay_wait", "INTEGER NOT NULL DEFAULT 0"},
		{"block_breaks", "delay_at", "INTEGER NOT NULL DEFAULT 0"},
		{"block_breaks", "keep", "TEXT NOT NULL DEFAULT ''"},
		{"block_breaks", "period", "TEXT NOT NULL DEFAULT ''"},
		{"block_breaks", "chunks", "TEXT NOT NULL DEFAULT ''"},
		{"places", "need", "INTEGER NOT NULL DEFAULT 2"},
		{"block_breaks", "on_break", "TEXT NOT NULL DEFAULT ''"},
		{"quotas", "period", "TEXT NOT NULL DEFAULT ''"},
		{"quotas", "refill", "INTEGER NOT NULL DEFAULT 0"},
		{"quotas", "max_sessions", "INTEGER NOT NULL DEFAULT 0"},
		{"quotas", "sessions", "INTEGER NOT NULL DEFAULT 0"},
		{"quotas", "last_use", "INTEGER NOT NULL DEFAULT 0"},
		{"quotas", "rolling", "TEXT NOT NULL DEFAULT ''"},
		{"quotas", "roll_hour", "INTEGER NOT NULL DEFAULT 0"},
		{"setting_locks", "range_days", "INTEGER NOT NULL DEFAULT 0"},
		{"setting_locks", "range_from", "INTEGER NOT NULL DEFAULT 0"},
		{"setting_locks", "range_to", "INTEGER NOT NULL DEFAULT 0"},
		{"setting_locks", "range_invert", "BOOLEAN NOT NULL DEFAULT 0"},
		{"schedules", "break_seconds", "INTEGER NOT NULL DEFAULT 0"},
		{"schedules", "break_taken", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, a := range adds {
		if err := addColumnIfMissing(conn, a.table, a.col, a.def); err != nil {
			return err
		}
	}
	if err := migrateScheduleLocks(conn); err != nil {
		return err
	}
	if err := migrateFreezeScheduleLocks(conn); err != nil {
		return err
	}
	if err := migrateHotkeys(conn); err != nil {
		return err
	}
	if err := migrateTamperClock(conn); err != nil {
		return err
	}
	if err := migrateForceLibreWolf(conn); err != nil {
		return err
	}
	if err := migrateWebLists(conn); err != nil {
		return err
	}
	// Lists have no types (§3.2): an older database may still have the
	// column web_lists.type (and clusters.type); nothing reads it.
	if err := migrateWordsListName(conn); err != nil {
		return err
	}
	if err := migrateJoinLists(conn); err != nil {
		return err
	}
	if err := migrateCombinations(conn); err != nil {
		return err
	}
	if err := migrateListCaseNames(conn); err != nil {
		return err
	}
	if err := migrateRemovedActions(conn); err != nil {
		return err
	}
	if err := migrateScheduledSwitchOn(conn); err != nil {
		return err
	}
	if err := migratePlaceTriggers(conn); err != nil {
		return err
	}
	if err := migrateAutomation(conn); err != nil {
		return err
	}
	return migrateMonkMode(conn)
}

// addColumnIfMissing runs "ALTER TABLE ... ADD COLUMN" only when the column
// isn't already present, so it's safe to call on every open.
func addColumnIfMissing(conn *sql.DB, table, column, definition string) error {
	rows, err := conn.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("failed to inspect table %s: %w", table, err)
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		var (
			cid, notnull, pk int
			name, ctype      string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("failed to read columns of %s: %w", table, err)
		}
		if name == column {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if found {
		return nil
	}

	if _, err := conn.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)); err != nil {
		return fmt.Errorf("failed to add column %s.%s: %w", table, column, err)
	}
	return nil
}

// --------------------------------------------------------------------------
// Schedules
// --------------------------------------------------------------------------

// Schedule is one recurring window attached to a block.
//
// Num is the number shown to the user: the schedule's 1-based position among
// its block's schedules (in creation order). It is derived, not stored, so the
// numbers are always 1..n with no gaps: a new schedule takes n+1, the lowest
// free number, and removing one shifts every later one down by one. ID is the
// internal row ID and is never shown.
type Schedule struct {
	ID        int
	Num       int
	BlockName string
	DaysMask  int
	StartMin  int
	EndMin    int
	Allow     bool // allow window rather than block window (see schedule.Enforced)
	// Break per window (§3.6): seconds of one break each time the window
	// is open (0: none); BreakTaken names the occurrence it was taken in.
	Break      int64
	BreakTaken string
}

// Window returns the schedule's time window.
func (s Schedule) Window() schedule.Window {
	return schedule.Window{Days: s.DaysMask, Start: s.StartMin, End: s.EndMin, Allow: s.Allow}
}

// Windows returns the time windows of a list of schedules.
func Windows(schedules []Schedule) []schedule.Window {
	out := make([]schedule.Window, len(schedules))
	for i, s := range schedules {
		out[i] = s.Window()
	}
	return out
}

// AddSchedule attaches a recurring window to a block. A block's schedules
// must be all block windows or all allow windows, so adding the other kind is
// refused.
func (d *DB) AddSchedule(blockName string, w schedule.Window) error {
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return err
	}
	existing, err := d.ListSchedules(blockName)
	if err != nil {
		return err
	}
	for _, s := range existing {
		if s.Allow != w.Allow {
			kind := "block"
			if s.Allow {
				kind = "allow"
			}
			return fmt.Errorf("block '%s' has %s windows; a block's schedules must be all block windows or all allow windows", blockName, kind)
		}
	}
	_, err = d.conn.Exec(
		"INSERT INTO schedules (block_id, days_mask, start_min, end_min, allow) VALUES (?, ?, ?, ?, ?)",
		blockID, w.Days, w.Start, w.End, w.Allow,
	)
	if err != nil {
		return fmt.Errorf("failed to add schedule to block '%s': %w", blockName, err)
	}
	return nil
}

// ListSchedules returns the schedules for one block.
func (d *DB) ListSchedules(blockName string) ([]Schedule, error) {
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return nil, err
	}
	rows, err := d.conn.Query(
		"SELECT id, days_mask, start_min, end_min, allow, break_seconds, break_taken FROM schedules WHERE block_id = ? ORDER BY id",
		blockID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list schedules for '%s': %w", blockName, err)
	}
	defer rows.Close()

	var out []Schedule
	for rows.Next() {
		s := Schedule{BlockName: blockName, Num: len(out) + 1}
		if err := rows.Scan(&s.ID, &s.DaysMask, &s.StartMin, &s.EndMin, &s.Allow, &s.Break, &s.BreakTaken); err != nil {
			return nil, fmt.Errorf("failed to read schedule: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SetScheduleBreak sets a window's break (seconds; 0 none).
func (d *DB) SetScheduleBreak(id int, seconds int64) error {
	_, err := d.conn.Exec("UPDATE schedules SET break_seconds = ? WHERE id = ?", seconds, id)
	return err
}

// TakeScheduleBreak records that a window's break was taken in occurrence.
func (d *DB) TakeScheduleBreak(id int, occurrence string) error {
	_, err := d.conn.Exec("UPDATE schedules SET break_taken = ? WHERE id = ?", occurrence, id)
	return err
}

// AllSchedules returns every schedule across all blocks. The daemon uses it to
// decide which blocks to auto-activate or auto-deactivate.
func (d *DB) AllSchedules() ([]Schedule, error) {
	rows, err := d.conn.Query(`
		SELECT s.id, b.name, s.days_mask, s.start_min, s.end_min, s.allow
		FROM schedules s
		JOIN blocks b ON b.id = s.block_id
		ORDER BY b.name, s.id
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all schedules: %w", err)
	}
	defer rows.Close()

	var out []Schedule
	for rows.Next() {
		var s Schedule
		if err := rows.Scan(&s.ID, &s.BlockName, &s.DaysMask, &s.StartMin, &s.EndMin, &s.Allow); err != nil {
			return nil, fmt.Errorf("failed to read schedule: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// HasAllowWindows reports whether a block's schedules are allow windows.
// Such a block can't be loosened by a plain edit (see the cli package).
func (d *DB) HasAllowWindows(blockName string) (bool, error) {
	schedules, err := d.ListSchedules(blockName)
	if err != nil {
		return false, err
	}
	return len(schedules) > 0 && schedules[0].Allow, nil
}

// removeScheduleWindow deletes the first of a block's schedules equal to w.
func (d *DB) removeScheduleWindow(blockName string, w schedule.Window) error {
	schedules, err := d.ListSchedules(blockName)
	if err != nil {
		return err
	}
	for _, s := range schedules {
		if s.Window() == w {
			return d.RemoveSchedule(blockName, s.Num)
		}
	}
	return fmt.Errorf("block '%s' has no schedule %s", blockName, w)
}

// GetSchedule returns a block's schedule by its number (Num, as shown in
// "schedule list").
func (d *DB) GetSchedule(blockName string, num int) (Schedule, error) {
	schedules, err := d.ListSchedules(blockName)
	if err != nil {
		return Schedule{}, err
	}
	if num < 1 || num > len(schedules) {
		return Schedule{}, fmt.Errorf("schedule %d not found in block '%s'", num, blockName)
	}
	return schedules[num-1], nil
}

// RemoveSchedule deletes one schedule from a block by its number (Num, as
// shown in "schedule list"). Later schedules move down one number.
func (d *DB) RemoveSchedule(blockName string, num int) error {
	s, err := d.GetSchedule(blockName, num)
	if err != nil {
		return err
	}
	if _, err := d.conn.Exec("DELETE FROM schedules WHERE id = ?", s.ID); err != nil {
		return fmt.Errorf("failed to remove schedule %d from block '%s': %w", num, blockName, err)
	}
	return nil
}
