package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/smattermannen/kc9k/internal/quota"
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
	var rolling string
	err = d.conn.QueryRow(`SELECT limit_seconds, used_seconds, day, open, session_end, last_tick, pending_limit, period, refill, max_sessions, sessions, last_use, rolling, roll_hour
		FROM quotas WHERE block_id = ?`, blockID).
		Scan(&s.Limit, &s.Used, &s.Day, &s.Open, &s.SessionEnd, &s.LastTick, &s.PendingLimit, &s.Period, &s.Refill,
			&s.MaxSessions, &s.Sessions, &s.LastUse, &rolling, &s.RollHour)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read quota of '%s': %w", name, err)
	}
	if rolling != "" {
		_ = json.Unmarshal([]byte(rolling), &s.Rolling)
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
		(block_id, limit_seconds, used_seconds, day, open, session_end, last_tick, pending_limit, period, refill,
		max_sessions, sessions, last_use, rolling, roll_hour)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		blockID, s.Limit, s.Used, s.Day, s.Open, s.SessionEnd, s.LastTick, s.PendingLimit, s.Period, s.Refill,
		s.MaxSessions, s.Sessions, s.LastUse, rollingJSON(s.Rolling), s.RollHour)
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
		session_end = ?, last_tick = ?, pending_limit = ?, period = ?, refill = ?,
		max_sessions = ?, sessions = ?, last_use = ?, rolling = ?, roll_hour = ?
		WHERE block_id = ? AND limit_seconds = ? AND used_seconds = ? AND day = ? AND open = ?
		AND session_end = ? AND last_tick = ? AND pending_limit = ?`,
		next.Limit, next.Used, next.Day, next.Open, next.SessionEnd, next.LastTick, next.PendingLimit, next.Period, next.Refill,
		next.MaxSessions, next.Sessions, next.LastUse, rollingJSON(next.Rolling), next.RollHour,
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

// rollingJSON stores a rolling allowance's hourly use.
func rollingJSON(b []int64) string {
	if len(b) == 0 {
		return ""
	}
	data, _ := json.Marshal(b)
	return string(data)
}
