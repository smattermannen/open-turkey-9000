// schedulelock.go — the schedule lock (§3.4): a lock the user puts on a
// block, on during its block windows and at its places, for a lead time
// the user chooses before each window, and, for a block with allow windows,
// outside them. Nothing about a schedule protects a block without it
// (R-LOCK-10).
package db

import (
	"database/sql"
	"fmt"
)

// NoScheduleLock is the stored lead time of a block without a schedule lock.
const NoScheduleLock = -1

// MaxScheduleLockLead is the longest lead time, in minutes (a week).
const MaxScheduleLockLead = 7 * 24 * 60

// ScheduleLock returns a block's schedule lock: whether it has one, and its
// lead time in minutes.
func (d *DB) ScheduleLock(name string) (on bool, leadMinutes int, err error) {
	lead := NoScheduleLock
	err = d.conn.QueryRow("SELECT schedule_lock_lead FROM blocks WHERE name = ?", name).Scan(&lead)
	if err == sql.ErrNoRows {
		return false, 0, fmt.Errorf("block '%s' doesn't exist", name)
	}
	if err != nil {
		return false, 0, fmt.Errorf("failed to read the schedule lock of '%s': %w", name, err)
	}
	if lead < 0 {
		return false, 0, nil
	}
	return true, lead, nil
}

// SetScheduleLock gives a block a schedule lock with a lead time in minutes,
// or takes it away (NoScheduleLock).
func (d *DB) SetScheduleLock(name string, leadMinutes int) error {
	if leadMinutes > MaxScheduleLockLead {
		return fmt.Errorf("the lead time is at most %d days", MaxScheduleLockLead/(24*60))
	}
	if leadMinutes < 0 {
		leadMinutes = NoScheduleLock
	}
	res, err := d.conn.Exec("UPDATE blocks SET schedule_lock_lead = ? WHERE name = ?", leadMinutes, name)
	if err != nil {
		return fmt.Errorf("failed to save the schedule lock of '%s': %w", name, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("block '%s' doesn't exist", name)
	}
	return nil
}

// migrateScheduleLocks adds the schedule lock column. Before it, every
// block with schedules or place rules was protected by them, from 2 hours
// before a window: those blocks get a schedule lock that keeps that.
func migrateScheduleLocks(conn *sql.DB) error {
	var n int
	if err := conn.QueryRow("SELECT COUNT(*) FROM pragma_table_info('blocks') WHERE name = 'schedule_lock_lead'").Scan(&n); err != nil {
		return fmt.Errorf("failed to inspect blocks: %w", err)
	}
	if n > 0 {
		return nil
	}
	if err := addColumnIfMissing(conn, "blocks", "schedule_lock_lead", "INTEGER NOT NULL DEFAULT -1"); err != nil {
		return err
	}
	_, err := conn.Exec(`UPDATE blocks SET schedule_lock_lead = 120
		WHERE id IN (SELECT block_id FROM schedules) OR id IN (SELECT block_id FROM place_rules)`)
	if err != nil {
		return fmt.Errorf("failed to keep the protection of scheduled blocks: %w", err)
	}
	return nil
}
