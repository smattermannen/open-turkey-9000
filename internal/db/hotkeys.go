// hotkeys.go — key combinations that start a block, optionally with a time
// lock (see cli/hotkeys.go).
package db

import (
	"database/sql"
	"fmt"
)

const hotkeysSchema = `CREATE TABLE IF NOT EXISTS hotkeys (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    keys     TEXT NOT NULL UNIQUE,
    block    TEXT NOT NULL,
    lock_for TEXT NOT NULL DEFAULT '',
    no_confirm INTEGER NOT NULL DEFAULT 0
);`

// Hotkey starts Block when Keys (a GTK accelerator such as <Primary><Alt>s)
// are pressed, time-locked for LockFor (a duration such as 1h) unless empty.
type Hotkey struct {
	ID      int    `json:"id"`
	Keys    string `json:"keys"`
	Block   string `json:"block"`
	LockFor string `json:"lock_for"`
	// NoConfirm: "Run without confirmation". Off, pressing the hotkey asks
	// before it sets its lock; on, the lock was confirmed only when the
	// hotkey was set up (R-LOCK-12).
	NoConfirm bool `json:"no_confirm"`
}

// migrateHotkeys adds the no_confirm column to older databases; their
// hotkeys ask at each press.
func migrateHotkeys(conn *sql.DB) error {
	return addColumnIfMissing(conn, "hotkeys", "no_confirm", "INTEGER NOT NULL DEFAULT 0")
}

// AddHotkey saves a hotkey and returns its ID.
func (d *DB) AddHotkey(h Hotkey) (int, error) {
	if _, err := d.getBlockID(h.Block); err != nil {
		return 0, err
	}
	result, err := d.conn.Exec("INSERT INTO hotkeys (keys, block, lock_for, no_confirm) VALUES (?, ?, ?, ?)", h.Keys, h.Block, h.LockFor, h.NoConfirm)
	if err != nil {
		return 0, fmt.Errorf("failed to add the hotkey (is %s already in use?): %w", h.Keys, err)
	}
	id, err := result.LastInsertId()
	return int(id), err
}

// RemoveHotkey deletes the hotkey with this ID.
func (d *DB) RemoveHotkey(id int) error {
	result, err := d.conn.Exec("DELETE FROM hotkeys WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to remove hotkey %d: %w", id, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("no hotkey %d (see 'kc9k hotkey list')", id)
	}
	return nil
}

// Hotkeys lists every hotkey, oldest first.
func (d *DB) Hotkeys() ([]Hotkey, error) {
	rows, err := d.conn.Query("SELECT id, keys, block, lock_for, no_confirm FROM hotkeys ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("failed to list hotkeys: %w", err)
	}
	defer rows.Close()
	out := []Hotkey{}
	for rows.Next() {
		var h Hotkey
		if err := rows.Scan(&h.ID, &h.Keys, &h.Block, &h.LockFor, &h.NoConfirm); err != nil {
			return nil, fmt.Errorf("failed to read a hotkey: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// GetHotkey returns the hotkey with this ID.
func (d *DB) GetHotkey(id int) (Hotkey, error) {
	var h Hotkey
	err := d.conn.QueryRow("SELECT id, keys, block, lock_for, no_confirm FROM hotkeys WHERE id = ?", id).
		Scan(&h.ID, &h.Keys, &h.Block, &h.LockFor, &h.NoConfirm)
	if err != nil {
		return h, fmt.Errorf("no hotkey %d (see 'kc9k hotkey list')", id)
	}
	return h, nil
}
