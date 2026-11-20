// settinglocks.go — locks on single settings (browser settings, tamper,
// strict mode, bypass prevention, freeze schedules).
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
	"strings"
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
	// Range lock (§3.4), as LockExtras: on at those times on those days.
	RangeDays   int
	RangeFrom   int
	RangeTo     int
	RangeInvert bool
	// Several locks of one kind and re-lock of the whole configuration
	// (§3.4), as LockExtras.Queue and Config: a setting's locks stack like
	// a block's (§6 2026-11-21 By Opus).
	Queue  []QueuedLock
	Config []QueuedLock
}

// asBlock views a setting lock as a block's locks, so layers.go judges both.
func (l SettingLock) asBlock() (*BlockDetail, LockExtras) {
	return &BlockDetail{Name: l.Name, Active: true, Locked: l.LockChars > 0, LockChars: l.LockChars, LockedUntil: l.LockedUntil},
		LockExtras{PasswordHash: l.PasswordHash, RestartBoot: l.RestartBoot, Style: l.Style, Relock: l.Relock,
			LiftedUntil: l.LiftedUntil, Order: l.Order, TimerAfter: l.TimerAfter, LiftedKinds: l.LiftedKinds,
			RangeDays: l.RangeDays, RangeFrom: l.RangeFrom, RangeTo: l.RangeTo, RangeInvert: l.RangeInvert,
			Queue: l.Queue, Config: l.Config}
}

// fromBlock copies back what layer code changed.
func (l *SettingLock) fromBlock(detail *BlockDetail, e LockExtras) {
	l.LockChars, l.LockedUntil = detail.LockChars, detail.LockedUntil
	if !detail.Locked {
		l.LockChars = 0
	}
	l.PasswordHash, l.RestartBoot, l.Style, l.Relock = e.PasswordHash, e.RestartBoot, e.Style, e.Relock
	l.LiftedUntil, l.Order, l.TimerAfter, l.LiftedKinds = e.LiftedUntil, e.Order, e.TimerAfter, e.LiftedKinds
	l.RangeDays, l.RangeFrom, l.RangeTo, l.RangeInvert = e.RangeDays, e.RangeFrom, e.RangeTo, e.RangeInvert
	l.Queue, l.Config = e.Queue, e.Config
}

// PlacedLayers is Layers without the queue (db.PlacedLayers).
func (l SettingLock) PlacedLayers() []string {
	detail, e := l.asBlock()
	return PlacedLayers(detail, e)
}

// Snapshot is the setting's locks as they would be given again (db.Snapshot).
func (l SettingLock) Snapshot() []QueuedLock {
	detail, e := l.asBlock()
	return Snapshot(detail, e)
}

// RelocksConfig reports whether re-lock brings back the whole configuration.
func (l SettingLock) RelocksConfig() bool {
	_, e := l.asBlock()
	return e.relocksConfig()
}

// RelockAt is when a setting whose locks are all unlocked locks again (its
// re-lock time is running), or 0.
func (l SettingLock) RelockAt(trusted int64) int64 {
	if l.Relock == 0 || l.LiftedUntil <= trusted || !l.PendingRelock(trusted) {
		return 0
	}
	return l.LiftedUntil
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

// PendingRelock reports whether the setting's locks are all unlocked and
// will come on again by themselves: its whole configuration once the
// re-lock time is over (§3.4), or, for locks set before settings kept it,
// the lifted text and password locks.
func (l SettingLock) PendingRelock(trusted int64) bool {
	if l.RelocksConfig() {
		return l.LiftedUntil != 0
	}
	return (l.LockChars > 0 || l.PasswordHash != "") && l.Relock > 0 && !l.TextOrPasswordLocked(trusted)
}

// TimeLocked reports whether the time lock is running or waiting.
func (l SettingLock) TimeLocked(trusted int64) bool {
	return l.LockedUntil > trusted || l.TimerAfter > 0
}

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
	var queue, config string
	err := d.conn.QueryRow(`SELECT locked_until, lock_chars, password, restart_boot, lock_style, relock, lifted_until,
		lock_order, timer_after, lifted_kinds, range_days, range_from, range_to, range_invert, lock_queue, lock_config
		FROM setting_locks WHERE name = ?`, name).Scan(&l.LockedUntil, &l.LockChars, &l.PasswordHash, &l.RestartBoot, &l.Style, &l.Relock, &l.LiftedUntil,
		&l.Order, &l.TimerAfter, &l.LiftedKinds, &l.RangeDays, &l.RangeFrom, &l.RangeTo, &l.RangeInvert, &queue, &config)
	if err == sql.ErrNoRows {
		return l, nil
	}
	if err != nil {
		return l, fmt.Errorf("failed to read the lock on %s: %w", name, err)
	}
	if l.Queue, err = decodeQueue(queue); err != nil {
		return l, fmt.Errorf("failed to read the lock on %s: %w", name, err)
	}
	if l.Config, err = decodeQueue(config); err != nil {
		return l, fmt.Errorf("failed to read the lock on %s: %w", name, err)
	}
	return l, nil
}

// SetSettingLock stores a setting's locks.
func (d *DB) SetSettingLock(l SettingLock) error {
	_, err := d.conn.Exec(`INSERT INTO setting_locks (name, locked_until, lock_chars, password, restart_boot, lock_style, relock, lifted_until,
			lock_order, timer_after, lifted_kinds, range_days, range_from, range_to, range_invert, lock_queue, lock_config)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET locked_until = excluded.locked_until, lock_chars = excluded.lock_chars,
			password = excluded.password, restart_boot = excluded.restart_boot, lock_style = excluded.lock_style,
			relock = excluded.relock, lifted_until = excluded.lifted_until, lock_order = excluded.lock_order,
			timer_after = excluded.timer_after, lifted_kinds = excluded.lifted_kinds, range_days = excluded.range_days,
			range_from = excluded.range_from, range_to = excluded.range_to, range_invert = excluded.range_invert,
			lock_queue = excluded.lock_queue, lock_config = excluded.lock_config`,
		l.Name, l.LockedUntil, l.LockChars, l.PasswordHash, l.RestartBoot, l.Style, l.Relock, l.LiftedUntil,
		l.Order, l.TimerAfter, l.LiftedKinds, l.RangeDays, l.RangeFrom, l.RangeTo, l.RangeInvert, encodeQueue(l.Queue), encodeQueue(l.Config))
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
		lock_order, timer_after, lifted_kinds, range_days, range_from, range_to, range_invert, lock_queue, lock_config
		FROM setting_locks ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("failed to list setting locks: %w", err)
	}
	defer rows.Close()
	var out []SettingLock
	for rows.Next() {
		var l SettingLock
		var queue, config string
		if err := rows.Scan(&l.Name, &l.LockedUntil, &l.LockChars, &l.PasswordHash, &l.RestartBoot, &l.Style, &l.Relock, &l.LiftedUntil,
			&l.Order, &l.TimerAfter, &l.LiftedKinds, &l.RangeDays, &l.RangeFrom, &l.RangeTo, &l.RangeInvert, &queue, &config); err != nil {
			return nil, fmt.Errorf("failed to read a setting lock: %w", err)
		}
		var err error
		if l.Queue, err = decodeQueue(queue); err != nil {
			return nil, fmt.Errorf("failed to read a setting lock: %w", err)
		}
		if l.Config, err = decodeQueue(config); err != nil {
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

// OldSafeSearch is the one SafeSearch browser setting before each engine
// had its own (§6 2026-11-18); SafeSearchSettings are those (the same as
// enforce's SafeSearchSetting of each engine).
const OldSafeSearch = "safesearch"

var SafeSearchSettings = []string{"safesearch-google", "safesearch-bing", "safesearch-duckduckgo", "safesearch-youtube"}

// migrateSafeSearch carries the one SafeSearch setting over to each
// engine's once: its on/off state to every engine, and its lock row to
// every engine unchanged (a lock of each, so a running lock holds every
// engine exactly as before; nothing is weakened).
func migrateSafeSearch(conn *sql.DB) error {
	var n int
	if err := conn.QueryRow(`SELECT (SELECT COUNT(*) FROM settings WHERE key = ?) + (SELECT COUNT(*) FROM setting_locks WHERE name = ?)`,
		"browser."+OldSafeSearch, OldSafeSearch).Scan(&n); err != nil || n == 0 {
		return err
	}
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var value string
	err = tx.QueryRow("SELECT value FROM settings WHERE key = ?", "browser."+OldSafeSearch).Scan(&value)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	var columns []string // every column but name, so each copy is the whole lock
	rows, qerr := tx.Query("SELECT name FROM pragma_table_info('setting_locks')")
	if qerr != nil {
		return qerr
	}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			rows.Close()
			return err
		}
		if c != "name" {
			columns = append(columns, c)
		}
	}
	rows.Close()
	list := strings.Join(columns, ", ")
	for _, name := range SafeSearchSettings {
		if err == nil {
			if _, err := tx.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)", "browser."+name, value); err != nil {
				return err
			}
		}
		if _, err := tx.Exec("INSERT OR IGNORE INTO setting_locks (name, "+list+") SELECT ?, "+list+
			" FROM setting_locks WHERE name = ?", name, OldSafeSearch); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("DELETE FROM settings WHERE key = ?", "browser."+OldSafeSearch); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM setting_locks WHERE name = ?", OldSafeSearch); err != nil {
		return err
	}
	return tx.Commit()
}
