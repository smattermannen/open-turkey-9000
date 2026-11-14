package db

import "database/sql"

// scheduledSwitchOnDone marks that migrateScheduledSwitchOn ran.
const scheduledSwitchOnDone = "migrated.scheduled_switch_on"

// migrateScheduledSwitchOn runs once. Before 2026-11-15 the daemon switched
// blocks with windows or place rules on and off itself; now only the user
// does (§3.3, R-SCHED-1) and the schedule only lifts an active block. So
// every inactive block with windows or a place rule (and no allowance,
// which still switches its block) is switched on, and enforces exactly when
// its schedule said before. Blocks the daemon had switched on stay on.
func migrateScheduledSwitchOn(conn *sql.DB) error {
	var done int
	if err := conn.QueryRow("SELECT COUNT(*) FROM settings WHERE key = ?", scheduledSwitchOnDone).Scan(&done); err != nil || done > 0 {
		return err
	}
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO active_blocks (block_id, locked, lock_chars, locked_until, scheduled)
		SELECT b.id, 0, 0, 0, 0 FROM blocks b
		WHERE (EXISTS (SELECT 1 FROM schedules s WHERE s.block_id = b.id)
		       OR EXISTS (SELECT 1 FROM place_rules p WHERE p.block_id = b.id))
		  AND NOT EXISTS (SELECT 1 FROM active_blocks a WHERE a.block_id = b.id)
		  AND NOT EXISTS (SELECT 1 FROM quotas q WHERE q.block_id = b.id)`); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM settings WHERE key LIKE 'schedule.stopped.%'"); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO settings (key, value) VALUES (?, '1')", scheduledSwitchOnDone); err != nil {
		return err
	}
	return tx.Commit()
}
