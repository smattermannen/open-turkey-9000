// settinglocks.go — locks on single browser settings.
//
// A browser setting (SafeSearch, web-only, ...) can be locked on its own,
// independently of any block: with a time lock, the typing challenge, a
// password, or until the computer restarts. While a lock is in force the
// setting can't be turned off. Time locks use trusted time (clock.go), like
// block time locks.
package db

import (
	"database/sql"
	"fmt"
)

const settingLocksSchema = `CREATE TABLE IF NOT EXISTS setting_locks (
    name         TEXT PRIMARY KEY,
    locked_until INTEGER NOT NULL DEFAULT 0,
    lock_chars   INTEGER NOT NULL DEFAULT 0,
    password     TEXT NOT NULL DEFAULT '',
    restart_boot TEXT NOT NULL DEFAULT ''
);`

// SettingLock holds the locks on one setting. The zero value is unlocked.
type SettingLock struct {
	Name         string
	LockedUntil  int64  // trusted unix time; 0 means no time lock
	LockChars    int    // typing challenge length; 0 means none
	PasswordHash string // "" means no password
	RestartBoot  string // the boot the lock lasts for; "" means none
}

// TimeLocked reports whether the time lock is still running.
func (l SettingLock) TimeLocked(trusted int64) bool { return l.LockedUntil > trusted }

// RestartLocked reports whether the lock lasts until a restart that hasn't
// happened yet.
func (l SettingLock) RestartLocked(bootID string) bool {
	return l.RestartBoot != "" && l.RestartBoot == bootID
}

// InForce reports whether any lock still holds the setting.
func (l SettingLock) InForce(trusted int64, bootID string) bool {
	return l.TimeLocked(trusted) || l.LockChars > 0 || l.PasswordHash != "" || l.RestartLocked(bootID)
}

// GetSettingLock returns a setting's locks, or an unlocked SettingLock.
func (d *DB) GetSettingLock(name string) (SettingLock, error) {
	l := SettingLock{Name: name}
	err := d.conn.QueryRow(`SELECT locked_until, lock_chars, password, restart_boot
		FROM setting_locks WHERE name = ?`, name).Scan(&l.LockedUntil, &l.LockChars, &l.PasswordHash, &l.RestartBoot)
	if err == sql.ErrNoRows {
		return l, nil
	}
	if err != nil {
		return l, fmt.Errorf("failed to read the lock on %s: %w", name, err)
	}
	return l, nil
}

// SetSettingLock stores a setting's locks.
func (d *DB) SetSettingLock(l SettingLock) error {
	_, err := d.conn.Exec(`INSERT INTO setting_locks (name, locked_until, lock_chars, password, restart_boot)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET locked_until = excluded.locked_until, lock_chars = excluded.lock_chars,
			password = excluded.password, restart_boot = excluded.restart_boot`,
		l.Name, l.LockedUntil, l.LockChars, l.PasswordHash, l.RestartBoot)
	if err != nil {
		return fmt.Errorf("failed to save the lock on %s: %w", l.Name, err)
	}
	return nil
}

// ClearSettingLock removes every lock from a setting.
func (d *DB) ClearSettingLock(name string) error {
	if _, err := d.conn.Exec("DELETE FROM setting_locks WHERE name = ?", name); err != nil {
		return fmt.Errorf("failed to remove the lock on %s: %w", name, err)
	}
	return nil
}

// SettingLocks returns every stored setting lock, in name order. Some may no
// longer be in force; check InForce.
func (d *DB) SettingLocks() ([]SettingLock, error) {
	rows, err := d.conn.Query(`SELECT name, locked_until, lock_chars, password, restart_boot
		FROM setting_locks ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("failed to list setting locks: %w", err)
	}
	defer rows.Close()
	var out []SettingLock
	for rows.Next() {
		var l SettingLock
		if err := rows.Scan(&l.Name, &l.LockedUntil, &l.LockChars, &l.PasswordHash, &l.RestartBoot); err != nil {
			return nil, fmt.Errorf("failed to read a setting lock: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
