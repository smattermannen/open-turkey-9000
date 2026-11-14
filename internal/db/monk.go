// monk.go — monk mode's state (R-MONK-1, R-MONK-2). Monk mode is a mode,
// not a block: all there is to it is when it ends, and what the network
// looked like before it was cut, so it can be put back.
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
)

const (
	// SettingMonkUntil is the trusted unix time monk mode ends; unset or 0
	// when it is off.
	SettingMonkUntil = "monk.until"
	// SettingMonkNet is the network state saved before monk mode cut the
	// connection (a MonkNet as JSON); unset when there is nothing to restore.
	SettingMonkNet = "monk.net"

	// legacyMonkBlock is the system-owned block monk mode used to be.
	legacyMonkBlock = "monk-mode"
	// settingMonkMigrated marks that the old block was looked for, so a
	// block the user names monk-mode later is left alone.
	settingMonkMigrated = "monk.migrated"
)

// MonkNet is the network as it was before monk mode cut it.
type MonkNet struct {
	// Wifi is whether the Wi-Fi radio was on.
	Wifi bool `json:"wifi"`
	// Ethernet says for each wired interface whether it was up.
	Ethernet map[string]bool `json:"ethernet"`
}

// MonkUntil returns the trusted unix time monk mode ends, 0 when it is off.
func (d *DB) MonkUntil() (int64, error) {
	value, err := d.GetSetting(SettingMonkUntil)
	if err != nil || value == "" {
		return 0, err
	}
	until, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("monk mode's end time is unreadable: %q", value)
	}
	return until, nil
}

// SetMonkUntil sets when monk mode ends; 0 switches it off.
func (d *DB) SetMonkUntil(until int64) error {
	if until <= 0 {
		return d.SetSetting(SettingMonkUntil, "")
	}
	return d.SetSetting(SettingMonkUntil, strconv.FormatInt(until, 10))
}

// ExtendMonk makes monk mode end at until, unless it already ends later:
// its time only ever grows (R-MONK-1). One statement, so the daemon and a
// command can't undo each other.
func (d *DB) ExtendMonk(until int64) error {
	_, err := d.conn.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
		WHERE CAST(settings.value AS INTEGER) < CAST(excluded.value AS INTEGER)`,
		SettingMonkUntil, strconv.FormatInt(until, 10))
	if err != nil {
		return fmt.Errorf("failed to start monk mode: %w", err)
	}
	return nil
}

// MonkNet returns the saved network state, nil when none is saved.
func (d *DB) MonkNet() (*MonkNet, error) {
	value, err := d.GetSetting(SettingMonkNet)
	if err != nil || value == "" {
		return nil, err
	}
	var net MonkNet
	if err := json.Unmarshal([]byte(value), &net); err != nil {
		return nil, fmt.Errorf("monk mode's saved network state is unreadable: %w", err)
	}
	if net.Ethernet == nil {
		net.Ethernet = map[string]bool{}
	}
	return &net, nil
}

// SaveMonkNet stores the network state to restore; nil forgets it.
func (d *DB) SaveMonkNet(net *MonkNet) error {
	if net == nil {
		return d.SetSetting(SettingMonkNet, "")
	}
	data, err := json.Marshal(net)
	if err != nil {
		return err
	}
	return d.SetSetting(SettingMonkNet, string(data))
}

// SaveMonkNetIfNone stores the network state unless one is saved already,
// and reports whether it did. The first state saved is the one from before
// the cut; whatever is read later (by the daemon, a second command, or
// after a restart) is the cut itself and must not replace it.
func (d *DB) SaveMonkNetIfNone(net MonkNet) (bool, error) {
	data, err := json.Marshal(net)
	if err != nil {
		return false, err
	}
	res, err := d.conn.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value WHERE settings.value = ''`,
		SettingMonkNet, string(data))
	if err != nil {
		return false, fmt.Errorf("failed to save the network state: %w", err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// migrateMonkMode removes the block monk mode used to be, once. One that is
// running keeps its end time: monk mode goes on as a mode until then.
func migrateMonkMode(conn *sql.DB) error {
	var n int
	if err := conn.QueryRow("SELECT COUNT(*) FROM settings WHERE key = ?", settingMonkMigrated).Scan(&n); err != nil || n > 0 {
		return err
	}
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var until int64
	err = tx.QueryRow(`SELECT MAX(ab.locked_until, ab.stop_at) FROM active_blocks ab
		JOIN blocks b ON b.id = ab.block_id WHERE b.name = ?`, legacyMonkBlock).Scan(&until)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to read the monk-mode block: %w", err)
	}
	if until > 0 {
		if _, err := tx.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value
			WHERE CAST(settings.value AS INTEGER) < CAST(excluded.value AS INTEGER)`,
			SettingMonkUntil, strconv.FormatInt(until, 10)); err != nil {
			return err
		}
	}
	// Its lists, active row and the rest go with it (ON DELETE CASCADE).
	if _, err := tx.Exec("DELETE FROM blocks WHERE name = ?", legacyMonkBlock); err != nil {
		return fmt.Errorf("failed to remove the monk-mode block: %w", err)
	}
	for _, key := range []string{"nepsis.ladder." + legacyMonkBlock, "schedule.stopped." + legacyMonkBlock} {
		if _, err := tx.Exec("DELETE FROM settings WHERE key = ?", key); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("INSERT INTO settings (key, value) VALUES (?, 'done')", settingMonkMigrated); err != nil {
		return err
	}
	return tx.Commit()
}
