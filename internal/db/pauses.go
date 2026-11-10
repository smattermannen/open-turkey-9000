// pauses.go — a block lifted for a while: it stays active, with its locks,
// and is enforced again by itself when the time is up. A break taken from
// an allowance and an unlocked block waiting to relock are both pauses.
// Times are trusted unix seconds (clock.go).
package db

import (
	"database/sql"
	"fmt"
)

const pausesSchema = `CREATE TABLE IF NOT EXISTS block_pauses (
    block_id INTEGER PRIMARY KEY,
    until    INTEGER NOT NULL,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`

// PausedUntil returns when a block's pause ends, 0 without one.
func (d *DB) PausedUntil(name string) (int64, error) {
	var until int64
	err := d.conn.QueryRow(`SELECT p.until FROM block_pauses p JOIN blocks b ON b.id = p.block_id WHERE b.name = ?`, name).Scan(&until)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to read the pause of '%s': %w", name, err)
	}
	return until, nil
}

// SetPause lifts a block until a time; 0 ends the pause.
func (d *DB) SetPause(name string, until int64) error {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return err
	}
	if until <= 0 {
		_, err = d.conn.Exec("DELETE FROM block_pauses WHERE block_id = ?", blockID)
	} else {
		_, err = d.conn.Exec(`INSERT INTO block_pauses (block_id, until) VALUES (?, ?)
			ON CONFLICT(block_id) DO UPDATE SET until = excluded.until`, blockID, until)
	}
	if err != nil {
		return fmt.Errorf("failed to set the pause of '%s': %w", name, err)
	}
	return nil
}
