package db

import (
	"database/sql"
	"fmt"

	"github.com/brunodcdo/open-turkey/internal/quota"
)

const quotaSchema = `CREATE TABLE IF NOT EXISTS quotas (
    block_id INTEGER PRIMARY KEY,
    limit_seconds INTEGER NOT NULL,
    used_seconds INTEGER NOT NULL,
    day TEXT NOT NULL,
    open BOOLEAN NOT NULL,
    session_end INTEGER NOT NULL,
    last_tick INTEGER NOT NULL,
    pending_limit INTEGER NOT NULL,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`

// GetQuota returns a block's quota, or nil if it has none.
func (d *DB) GetQuota(name string) (*quota.State, error) {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return nil, err
	}
	var s quota.State
	err = d.conn.QueryRow(`SELECT limit_seconds, used_seconds, day, open, session_end, last_tick, pending_limit
		FROM quotas WHERE block_id = ?`, blockID).
		Scan(&s.Limit, &s.Used, &s.Day, &s.Open, &s.SessionEnd, &s.LastTick, &s.PendingLimit)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read quota of '%s': %w", name, err)
	}
	return &s, nil
}

// SaveQuota creates or replaces a block's quota.
func (d *DB) SaveQuota(name string, s quota.State) error {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec(`INSERT OR REPLACE INTO quotas
		(block_id, limit_seconds, used_seconds, day, open, session_end, last_tick, pending_limit)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		blockID, s.Limit, s.Used, s.Day, s.Open, s.SessionEnd, s.LastTick, s.PendingLimit)
	if err != nil {
		return fmt.Errorf("failed to save quota of '%s': %w", name, err)
	}
	return nil
}

// UpdateQuotaIfUnchanged saves next only if the row still equals prev, and
// reports whether it did. The daemon uses it so a slow cycle can't overwrite a
// session the CLI opened or closed in the meantime.
func (d *DB) UpdateQuotaIfUnchanged(name string, prev, next quota.State) (bool, error) {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return false, err
	}
	result, err := d.conn.Exec(`UPDATE quotas SET limit_seconds = ?, used_seconds = ?, day = ?, open = ?,
		session_end = ?, last_tick = ?, pending_limit = ?
		WHERE block_id = ? AND limit_seconds = ? AND used_seconds = ? AND day = ? AND open = ?
		AND session_end = ? AND last_tick = ? AND pending_limit = ?`,
		next.Limit, next.Used, next.Day, next.Open, next.SessionEnd, next.LastTick, next.PendingLimit,
		blockID, prev.Limit, prev.Used, prev.Day, prev.Open, prev.SessionEnd, prev.LastTick, prev.PendingLimit)
	if err != nil {
		return false, fmt.Errorf("failed to update quota of '%s': %w", name, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

// DeleteQuota removes a block's quota.
func (d *DB) DeleteQuota(name string) error {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return err
	}
	if _, err := d.conn.Exec("DELETE FROM quotas WHERE block_id = ?", blockID); err != nil {
		return fmt.Errorf("failed to delete quota of '%s': %w", name, err)
	}
	return nil
}

// QuotaBlocks lists the names of blocks that have a quota.
func (d *DB) QuotaBlocks() ([]string, error) {
	rows, err := d.conn.Query("SELECT b.name FROM quotas q JOIN blocks b ON b.id = q.block_id ORDER BY b.name")
	if err != nil {
		return nil, fmt.Errorf("failed to list quotas: %w", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to read quota row: %w", err)
		}
		names = append(names, name)
	}
	return names, rows.Err()
}
