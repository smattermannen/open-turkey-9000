// automation.go — a block's automation settings (§3.6, owner 2026-11-16):
// enabled or disabled by default, and, with windows and places, whether
// switching needs both. Its windows and places are kept where they always
// were (schedules, place_rules); a block's windows turn it off exactly when
// it is enabled by default (schedules.allow follows the default).
package db

import (
	"database/sql"
	"fmt"
)

const automationSchema = `CREATE TABLE IF NOT EXISTS automation (
    block_id   INTEGER PRIMARY KEY,
    default_on BOOLEAN NOT NULL DEFAULT 0,
    need_both  BOOLEAN NOT NULL DEFAULT 0,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`

// AutomationSettings are a block's default and its both/either choice.
type AutomationSettings struct {
	DefaultOn bool `json:"default_on"` // enabled by default: windows and places turn it off
	NeedBoth  bool `json:"need_both"`  // with windows and places: switching needs both
}

// Automation returns a block's settings (disabled by default and either,
// when none were set).
func (d *DB) Automation(block string) (AutomationSettings, error) {
	blockID, err := d.getBlockID(block)
	if err != nil {
		return AutomationSettings{}, err
	}
	var s AutomationSettings
	err = d.conn.QueryRow("SELECT default_on, need_both FROM automation WHERE block_id = ?", blockID).Scan(&s.DefaultOn, &s.NeedBoth)
	if err == sql.ErrNoRows {
		return AutomationSettings{}, nil
	}
	if err != nil {
		return AutomationSettings{}, fmt.Errorf("failed to read the automation of '%s': %w", block, err)
	}
	return s, nil
}

// Automations returns every block's settings that differ from the defaults,
// by block name.
func (d *DB) Automations() (map[string]AutomationSettings, error) {
	rows, err := d.conn.Query("SELECT b.name, a.default_on, a.need_both FROM automation a JOIN blocks b ON b.id = a.block_id")
	if err != nil {
		return nil, fmt.Errorf("failed to read automation: %w", err)
	}
	defer rows.Close()
	out := map[string]AutomationSettings{}
	for rows.Next() {
		var name string
		var s AutomationSettings
		if err := rows.Scan(&name, &s.DefaultOn, &s.NeedBoth); err != nil {
			return nil, fmt.Errorf("failed to read automation: %w", err)
		}
		out[name] = s
	}
	return out, rows.Err()
}

// SetAutomation stores a block's settings; its windows follow the default
// (a block enabled by default has allow windows).
func (d *DB) SetAutomation(block string, s AutomationSettings) error {
	blockID, err := d.getBlockID(block)
	if err != nil {
		return err
	}
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO automation (block_id, default_on, need_both) VALUES (?, ?, ?)
		ON CONFLICT(block_id) DO UPDATE SET default_on = excluded.default_on, need_both = excluded.need_both`,
		blockID, s.DefaultOn, s.NeedBoth); err != nil {
		return fmt.Errorf("failed to save the automation of '%s': %w", block, err)
	}
	if _, err := tx.Exec("UPDATE schedules SET allow = ? WHERE block_id = ?", s.DefaultOn, blockID); err != nil {
		return fmt.Errorf("failed to save the automation of '%s': %w", block, err)
	}
	return tx.Commit()
}

// migrateAutomation gives a block with allow windows (from before 2026-11-16)
// the setting they meant: enabled by default, its windows turning it off.
func migrateAutomation(conn *sql.DB) error {
	_, err := conn.Exec(`INSERT OR IGNORE INTO automation (block_id, default_on, need_both)
		SELECT DISTINCT block_id, 1, 0 FROM schedules WHERE allow = 1`)
	return err
}
