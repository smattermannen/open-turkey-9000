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
	Style        string // the text lock's style (lock.Style*): "" random characters
	// Re-lock (§3.4): seconds after unlocking that the text and password
	// locks come on again; 0 means none. Unlocking lifts them until
	// LiftedUntil (trusted time) instead of removing them; turning the
	// setting off meanwhile removes them.
	Relock      int64
	LiftedUntil int64
	// Layered sequential locks (R-LOCK-6, layers.go): unlock order, a
	// duration timer lock waiting behind others, re-lock-lifted kinds.
	Order       string
	TimerAfter  int64
	LiftedKinds string
}

// asBlock views a setting lock as a block's locks, so layers.go judges both.
func (l SettingLock) asBlock() (*BlockDetail, LockExtras) {
	return &BlockDetail{Name: l.Name, Active: true, Locked: l.LockChars > 0, LockChars: l.LockChars, LockedUntil: l.LockedUntil},
		LockExtras{PasswordHash: l.PasswordHash, RestartBoot: l.RestartBoot, Style: l.Style, Relock: l.Relock,
			LiftedUntil: l.LiftedUntil, Order: l.Order, TimerAfter: l.TimerAfter, LiftedKinds: l.LiftedKinds}
}

// fromBlock copies back what layer code changed.
func (l *SettingLock) fromBlock(detail *BlockDetail, e LockExtras) {
	l.LockChars, l.LockedUntil = detail.LockChars, detail.LockedUntil
	if !detail.Locked {
		l.LockChars = 0
	}
	l.PasswordHash, l.RestartBoot, l.Style, l.Relock = e.PasswordHash, e.RestartBoot, e.Style, e.Relock
	l.LiftedUntil, l.Order, l.TimerAfter, l.LiftedKinds = e.LiftedUntil, e.Order, e.TimerAfter, e.LiftedKinds
}

// Layers returns the setting's locks in unlock order (R-LOCK-6).
func (l SettingLock) Layers() []string {
	detail, e := l.asBlock()
	return Layers(detail, e)
}

// LayersOn returns the locks still on, in unlock order; the first is next.
func (l SettingLock) LayersOn(trusted int64, bootID string) []string {
	detail, e := l.asBlock()
	return LayersOn(detail, e, trusted, bootID)
}

// LayerOn reports whether one kind of lock holds the setting (or waits).
func (l SettingLock) LayerOn(kind string, trusted int64, bootID string) bool {
	detail, e := l.asBlock()
	return LayerOn(kind, detail, e, trusted, bootID)
}

// Advance starts the first lock still on if it was waiting, and brings back
// re-lock-lifted locks whose time is over (AdvanceLayers).
func (l *SettingLock) Advance(trusted int64, bootID string) bool {
	detail, e := l.asBlock()
	if !AdvanceLayers(detail, &e, trusted, bootID) {
		return false
	}
	l.fromBlock(detail, e)
	return true
}

// LiftTextLayer unlocks the text or password lock (kind): gone without
// re-lock; with re-lock lifted, while the other still holds until that is
// unlocked too, then both for the re-lock time.
func (l *SettingLock) LiftTextLayer(kind string, trusted int64, bootID string) {
	detail, e := l.asBlock()
	LiftTextLayer(detail, &e, kind, trusted, bootID)
	l.fromBlock(detail, e)
}

// TextOrPasswordLocked reports whether a text or password lock holds the
// setting now (not lifted by a re-lock unlock).
func (l SettingLock) TextOrPasswordLocked(trusted int64) bool {
	return l.LayerOn(LayerText, trusted, "") || l.LayerOn(LayerPassword, trusted, "")
}

// PendingRelock reports whether the text and password locks are all lifted
// and will come on again by themselves.
func (l SettingLock) PendingRelock(trusted int64) bool {
	return (l.LockChars > 0 || l.PasswordHash != "") && l.Relock > 0 && !l.TextOrPasswordLocked(trusted)
}

// TimeLocked reports whether the time lock is running or waiting.
func (l SettingLock) TimeLocked(trusted int64) bool { return l.LockedUntil > trusted || l.TimerAfter > 0 }

// RestartLocked reports whether the lock lasts until a restart that hasn't
// happened yet (or waits behind other locks).
func (l SettingLock) RestartLocked(bootID string) bool {
	return l.RestartBoot == RestartPending || (l.RestartBoot != "" && l.RestartBoot == bootID)
}

// InForce reports whether any lock still holds the setting.
func (l SettingLock) InForce(trusted int64, bootID string) bool {
	return len(l.LayersOn(trusted, bootID)) > 0
}

// GetSettingLock returns a setting's locks, or an unlocked SettingLock.
func (d *DB) GetSettingLock(name string) (SettingLock, error) {
	l := SettingLock{Name: name}
	err := d.conn.QueryRow(`SELECT locked_until, lock_chars, password, restart_boot, lock_style, relock, lifted_until,
		lock_order, timer_after, lifted_kinds
		FROM setting_locks WHERE name = ?`, name).Scan(&l.LockedUntil, &l.LockChars, &l.PasswordHash, &l.RestartBoot, &l.Style, &l.Relock, &l.LiftedUntil,
		&l.Order, &l.TimerAfter, &l.LiftedKinds)
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
	_, err := d.conn.Exec(`INSERT INTO setting_locks (name, locked_until, lock_chars, password, restart_boot, lock_style, relock, lifted_until,
			lock_order, timer_after, lifted_kinds)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET locked_until = excluded.locked_until, lock_chars = excluded.lock_chars,
			password = excluded.password, restart_boot = excluded.restart_boot, lock_style = excluded.lock_style,
			relock = excluded.relock, lifted_until = excluded.lifted_until, lock_order = excluded.lock_order,
			timer_after = excluded.timer_after, lifted_kinds = excluded.lifted_kinds`,
		l.Name, l.LockedUntil, l.LockChars, l.PasswordHash, l.RestartBoot, l.Style, l.Relock, l.LiftedUntil,
		l.Order, l.TimerAfter, l.LiftedKinds)
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
	rows, err := d.conn.Query(`SELECT name, locked_until, lock_chars, password, restart_boot, lock_style, relock, lifted_until,
		lock_order, timer_after, lifted_kinds
		FROM setting_locks ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("failed to list setting locks: %w", err)
	}
	defer rows.Close()
	var out []SettingLock
	for rows.Next() {
		var l SettingLock
		if err := rows.Scan(&l.Name, &l.LockedUntil, &l.LockChars, &l.PasswordHash, &l.RestartBoot, &l.Style, &l.Relock, &l.LiftedUntil,
			&l.Order, &l.TimerAfter, &l.LiftedKinds); err != nil {
			return nil, fmt.Errorf("failed to read a setting lock: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// UnsupportedBrowsers names the tamper protection setting "block unsupported
// browsers" (§4.10) and its setting lock. SettingUnsupportedBrowsers stores
// it, "on" or "off".
const (
	UnsupportedBrowsers        = "unsupported-browsers"
	SettingUnsupportedBrowsers = "tamper." + UnsupportedBrowsers
)

// Before 2026-11-14 the setting was the browser setting force-librewolf.
const (
	oldForceLibreWolf        = "force-librewolf"
	oldForceLibreWolfSetting = "browser." + oldForceLibreWolf
)

// migrateForceLibreWolf carries force-librewolf over to block unsupported
// browsers once: its on/off state, and its lock row unchanged (time,
// challenge and its style, password, restart boot), so a running lock holds
// exactly as before.
func migrateForceLibreWolf(conn *sql.DB) error {
	// Every open runs this: write only when there is something to move, so
	// opening never waits on, or fails beside, another writer.
	var n int
	if err := conn.QueryRow(`SELECT (SELECT COUNT(*) FROM settings WHERE key = ?) + (SELECT COUNT(*) FROM setting_locks WHERE name = ?)`,
		oldForceLibreWolfSetting, oldForceLibreWolf).Scan(&n); err != nil || n == 0 {
		return err
	}
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var value string
	err = tx.QueryRow("SELECT value FROM settings WHERE key = ?", oldForceLibreWolfSetting).Scan(&value)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == nil {
		if _, err := tx.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)", SettingUnsupportedBrowsers, value); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM settings WHERE key = ?", oldForceLibreWolfSetting); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("UPDATE OR IGNORE setting_locks SET name = ? WHERE name = ?", UnsupportedBrowsers, oldForceLibreWolf); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM setting_locks WHERE name = ?", oldForceLibreWolf); err != nil {
		return err
	}
	return tx.Commit()
}
