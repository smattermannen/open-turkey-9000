// hotkeys.go — key combinations that start a block, optionally with locks
// (see cli/hotkeys.go).
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

const hotkeysSchema = `CREATE TABLE IF NOT EXISTS hotkeys (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    keys     TEXT NOT NULL UNIQUE,
    block    TEXT NOT NULL,
    lock_for TEXT NOT NULL DEFAULT '',
    no_confirm INTEGER NOT NULL DEFAULT 0,
    lock_args TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL DEFAULT ''
);`

// Hotkey starts Block when Keys (a GTK accelerator such as <Primary><Alt>s)
// are pressed, with the locks of LockArgs: start's lock flags in unlock
// order, e.g. ["--lock", "--for", "1h"]. Older hotkeys have only LockFor, a
// timer lock's duration (see Args).
type Hotkey struct {
	ID       int      `json:"id"`
	Keys     string   `json:"keys"`
	Block    string   `json:"block"`
	LockFor  string   `json:"lock_for,omitempty"`
	LockArgs []string `json:"lock_args,omitempty"`
	// PasswordHash is the password lock's, given when the hotkey was added.
	// Never shown (gui-state); backups carry it separately.
	PasswordHash string `json:"-"`
	// NoConfirm: "Run without confirmation". Off, pressing the hotkey asks
	// before it sets its lock; on, the lock was confirmed only when the
	// hotkey was set up (R-LOCK-12).
	NoConfirm bool `json:"no_confirm"`
}

// Args returns the hotkey's lock flags: LockArgs, or a timer lock of
// LockFor for older hotkeys.
func (h Hotkey) Args() []string {
	if len(h.LockArgs) > 0 {
		return h.LockArgs
	}
	if h.LockFor != "" {
		return []string{"--for", h.LockFor}
	}
	return nil
}

// migrateHotkeys adds the newer columns to older databases: their hotkeys
// ask at each press, and keep their timer lock in lock_for.
func migrateHotkeys(conn *sql.DB) error {
	for _, c := range [][2]string{
		{"no_confirm", "INTEGER NOT NULL DEFAULT 0"},
		{"lock_args", "TEXT NOT NULL DEFAULT ''"},
		{"password_hash", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := addColumnIfMissing(conn, "hotkeys", c[0], c[1]); err != nil {
			return err
		}
	}
	return nil
}

func encodeLockArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	b, _ := json.Marshal(args)
	return string(b)
}

func decodeLockArgs(s string) []string {
	var args []string
	if s != "" {
		_ = json.Unmarshal([]byte(s), &args)
	}
	return args
}

// AddHotkey saves a hotkey and returns its ID.
func (d *DB) AddHotkey(h Hotkey) (int, error) {
	if _, err := d.getBlockID(h.Block); err != nil {
		return 0, err
	}
	result, err := d.conn.Exec("INSERT INTO hotkeys (keys, block, lock_for, no_confirm, lock_args, password_hash) VALUES (?, ?, ?, ?, ?, ?)",
		h.Keys, h.Block, h.LockFor, h.NoConfirm, encodeLockArgs(h.LockArgs), h.PasswordHash)
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
	rows, err := d.conn.Query("SELECT id, keys, block, lock_for, no_confirm, lock_args, password_hash FROM hotkeys ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("failed to list hotkeys: %w", err)
	}
	defer rows.Close()
	out := []Hotkey{}
	for rows.Next() {
		var h Hotkey
		var args string
		if err := rows.Scan(&h.ID, &h.Keys, &h.Block, &h.LockFor, &h.NoConfirm, &args, &h.PasswordHash); err != nil {
			return nil, fmt.Errorf("failed to read a hotkey: %w", err)
		}
		h.LockArgs = decodeLockArgs(args)
		out = append(out, h)
	}
	return out, rows.Err()
}

// GetHotkey returns the hotkey with this ID.
func (d *DB) GetHotkey(id int) (Hotkey, error) {
	var h Hotkey
	var args string
	err := d.conn.QueryRow("SELECT id, keys, block, lock_for, no_confirm, lock_args, password_hash FROM hotkeys WHERE id = ?", id).
		Scan(&h.ID, &h.Keys, &h.Block, &h.LockFor, &h.NoConfirm, &args, &h.PasswordHash)
	if err != nil {
		return h, fmt.Errorf("no hotkey %d (see 'kc9k hotkey list')", id)
	}
	h.LockArgs = decodeLockArgs(args)
	return h, nil
}
