package db

import (
	"database/sql"
	"fmt"
)

const pomodoroSchema = `CREATE TABLE IF NOT EXISTS pomodoro (
    block_id INTEGER PRIMARY KEY,
    on_seconds INTEGER NOT NULL,
    off_seconds INTEGER NOT NULL,
    anchor INTEGER NOT NULL,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`

// Pomodoro is a block's work and break cycle.
type Pomodoro struct {
	On, Off, Anchor int64
}

// GetPomodoro returns a block's cycle, or nil if it has none.
func (d *DB) GetPomodoro(name string) (*Pomodoro, error) {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return nil, err
	}
	var p Pomodoro
	err = d.conn.QueryRow("SELECT on_seconds, off_seconds, anchor FROM pomodoro WHERE block_id = ?", blockID).
		Scan(&p.On, &p.Off, &p.Anchor)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read Pomodoro of '%s': %w", name, err)
	}
	return &p, nil
}

// SetPomodoro creates or replaces a block's cycle.
func (d *DB) SetPomodoro(name string, p Pomodoro) error {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec("INSERT OR REPLACE INTO pomodoro (block_id, on_seconds, off_seconds, anchor) VALUES (?, ?, ?, ?)",
		blockID, p.On, p.Off, p.Anchor)
	if err != nil {
		return fmt.Errorf("failed to save Pomodoro of '%s': %w", name, err)
	}
	return nil
}

// ClearPomodoro removes a block's cycle.
func (d *DB) ClearPomodoro(name string) error {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return err
	}
	if _, err := d.conn.Exec("DELETE FROM pomodoro WHERE block_id = ?", blockID); err != nil {
		return fmt.Errorf("failed to clear Pomodoro of '%s': %w", name, err)
	}
	return nil
}

// RenameBlock changes a block's name.
func (d *DB) RenameBlock(oldName, newName string) error {
	blockID, err := d.getBlockID(oldName)
	if err != nil {
		return err
	}
	if _, err := d.conn.Exec("UPDATE blocks SET name = ? WHERE id = ?", newName, blockID); err != nil {
		return fmt.Errorf("failed to rename '%s': %w", oldName, err)
	}
	// Hotkeys name their block, so they follow it.
	if _, err := d.conn.Exec("UPDATE hotkeys SET block = ? WHERE block = ?", newName, oldName); err != nil {
		return fmt.Errorf("failed to update hotkeys for '%s': %w", newName, err)
	}
	return nil
}

// DuplicateBlock copies a block's lists into a new inactive block.
func (d *DB) DuplicateBlock(name, copyName string) error {
	detail, err := d.GetBlock(name)
	if err != nil {
		return err
	}
	if err := d.CreateBlock(copyName); err != nil {
		return err
	}
	if err := d.copyLists(detail.Lists, copyName); err != nil {
		return err
	}
	if len(detail.Apps) > 0 {
		if err := d.AddApps(copyName, detail.Apps); err != nil {
			return err
		}
	}
	if len(detail.Titles) > 0 {
		return d.AddTitles(copyName, detail.Titles)
	}
	return nil
}
