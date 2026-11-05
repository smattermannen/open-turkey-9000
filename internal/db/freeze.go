package db

import (
	"database/sql"
	"fmt"
)

const freezeSchema = `CREATE TABLE IF NOT EXISTS freeze_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    until INTEGER NOT NULL
);`

// FreezeUntil returns the stored end of Frozen Chicken in unix seconds, or 0.
func (d *DB) FreezeUntil() (int64, error) {
	var until int64
	err := d.conn.QueryRow("SELECT until FROM freeze_state WHERE id = 1").Scan(&until)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to read freeze state: %w", err)
	}
	return until, nil
}

// FreezeMode returns how Frozen Chicken keeps the computer unusable.
func (d *DB) FreezeMode() (string, error) {
	var mode string
	err := d.conn.QueryRow("SELECT mode FROM freeze_state WHERE id = 1").Scan(&mode)
	if err == sql.ErrNoRows {
		return "lock", nil
	}
	return mode, err
}

// SetFreeze stores the end and mode of Frozen Chicken.
func (d *DB) SetFreeze(until int64, mode string) error {
	_, err := d.conn.Exec(`INSERT INTO freeze_state (id, until, mode) VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET until = excluded.until, mode = excluded.mode`, until, mode)
	if err != nil {
		return fmt.Errorf("failed to save freeze state: %w", err)
	}
	return nil
}
