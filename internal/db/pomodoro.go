package db

import (
	"database/sql"
	"fmt"
	"github.com/smattermannen/kc9k/internal/quota"
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
	// Long and Every: every Every-th break lasts Long seconds (0: none).
	Long, Every int64
}

// Cycle returns the rhythm for the time arithmetic in package quota.
func (p Pomodoro) Cycle() quota.Cycle {
	return quota.Cycle{On: p.On, Off: p.Off, Long: p.Long, Every: p.Every, Anchor: p.Anchor}
}

// GetPomodoro returns a block's cycle, or nil if it has none.
func (d *DB) GetPomodoro(name string) (*Pomodoro, error) {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return nil, err
	}
	var p Pomodoro
	err = d.conn.QueryRow("SELECT on_seconds, off_seconds, anchor, long_seconds, every_n FROM pomodoro WHERE block_id = ?", blockID).
		Scan(&p.On, &p.Off, &p.Anchor, &p.Long, &p.Every)
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
	_, err = d.conn.Exec("INSERT OR REPLACE INTO pomodoro (block_id, on_seconds, off_seconds, anchor, long_seconds, every_n) VALUES (?, ?, ?, ?, ?, ?)",
		blockID, p.On, p.Off, p.Anchor, p.Long, p.Every)
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
		if err := d.AddTitles(copyName, detail.Titles); err != nil {
			return err
		}
	}
	n, err := d.BlockNepsis(name)
	if err != nil {
		return err
	}
	return d.SetBlockNepsis(copyName, n)
}
