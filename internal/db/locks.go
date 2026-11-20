package db

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const settingsSchema = `CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);`

const passwordIterations = 200000

// LockExtras are the locks beyond the character and time locks.
type LockExtras struct {
	PasswordHash string
	RestartBoot  string // the boot the lock lasts for
	Delay        int64  // seconds between asking to stop and the block stopping
	StopAt       int64  // trusted unix time the block switches off by itself (monk mode)
	DelayUntil   int64  // trusted unix time the delay lock is unlocked, once asked; 0: not asked
	// Relock (seconds) is the re-lock option of the text and password locks
	// (§3.4): unlocking lifts them until LiftedUntil (trusted unix time),
	// then they are on again by themselves.
	Relock      int64
	LiftedUntil int64
	Style       string // the text lock's style (lock.Style*): "" random characters
	// Layered sequential locks (layers.go): the unlock order ("text,timer"),
	// a timer lock's duration waiting for the locks before it, and which
	// text/password locks re-lock has lifted.
	Order       string
	TimerAfter  int64
	LiftedKinds string
	// Range lock (§3.4): on from RangeFrom to RangeTo (minutes of the day;
	// To before From runs past midnight) on RangeDays (schedule day bits);
	// RangeInvert: on outside those times instead. RangeDays 0: none.
	RangeDays   int
	RangeFrom   int
	RangeTo     int
	RangeInvert bool
	// Queue: locks of a kind the block already has, added behind its other
	// locks (§3.4, several of one kind; owner 2026-11-20). Each waits until
	// every lock before it is gone, then takes its kind's place
	// (AdvanceLayers). Stored as JSON in lock_queue.
	Queue []QueuedLock
	// Relifted: text/password locks re-lock has lifted whose place a later
	// lock of their kind has taken; they come back with it, before it, when
	// re-lock puts the locks on again (AdvanceLayers). JSON in lock_relifted.
	Relifted []QueuedLock
}

// QueuedLock is one lock waiting in LockExtras.Queue.
type QueuedLock struct {
	Kind         string `json:"kind"`
	Chars        int    `json:"chars,omitempty"`      // text lock
	Style        string `json:"style,omitempty"`      // text lock
	PasswordHash string `json:"password,omitempty"`   // password lock
	For          int64  `json:"for,omitempty"`        // timer lock: seconds from when it starts
	Until        int64  `json:"until,omitempty"`      // timer lock: until a date (trusted unix time)
	Delay        int64  `json:"delay,omitempty"`      // delay lock: seconds
	RangeDays    int    `json:"range_days,omitempty"` // range lock (LockExtras.Range*)
	RangeFrom    int    `json:"range_from,omitempty"`
	RangeTo      int    `json:"range_to,omitempty"`
	RangeInvert  bool   `json:"range_invert,omitempty"`
}

func encodeQueue(q []QueuedLock) string {
	if len(q) == 0 {
		return ""
	}
	data, _ := json.Marshal(q)
	return string(data)
}

func decodeQueue(s string) ([]QueuedLock, error) {
	if s == "" {
		return nil, nil
	}
	var q []QueuedLock
	if err := json.Unmarshal([]byte(s), &q); err != nil {
		return nil, fmt.Errorf("unreadable lock queue: %w", err)
	}
	return q, nil
}

// BootID identifies the current boot.
func BootID() (string, error) {
	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", fmt.Errorf("failed to read boot id: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// HashPassword returns a salted hash for storage.
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, 32)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(key), nil
}

// CheckPassword reports whether password matches a stored hash.
func CheckPassword(stored, password string) bool {
	saltHex, keyHex, ok := strings.Cut(stored, ":")
	if !ok {
		return false
	}
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(keyHex)
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, 32)
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

// LiftTextAndPasswordLocks unlocks an active block's text and password
// locks; the block stays on (§3.4: a lock is only lifted).
func (d *DB) LiftTextAndPasswordLocks(name string) error {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return err
	}
	if _, err := d.conn.Exec(`UPDATE active_blocks SET locked = 0, lock_chars = 0, lock_password = '', lock_style = ''
		WHERE block_id = ?`, blockID); err != nil {
		return fmt.Errorf("failed to unlock '%s': %w", name, err)
	}
	return nil
}

// Lifted reports whether the text and password locks are lifted for now by
// their re-lock option.
func (e LockExtras) Lifted(trusted int64) bool { return e.LiftedUntil > trusted }

// GetLockExtras returns an active block's extra locks, or zero values.
func (d *DB) GetLockExtras(name string) (LockExtras, error) {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return LockExtras{}, err
	}
	var e LockExtras
	var queue, relifted string
	err = d.conn.QueryRow(`SELECT lock_password, restart_boot, lock_delay, stop_at, lock_style, delay_until, relock, lifted_until,
		lock_order, timer_after, lifted_kinds, range_days, range_from, range_to, range_invert, lock_queue, lock_relifted
		FROM active_blocks WHERE block_id = ?`, blockID).Scan(&e.PasswordHash, &e.RestartBoot, &e.Delay, &e.StopAt, &e.Style, &e.DelayUntil,
		&e.Relock, &e.LiftedUntil, &e.Order, &e.TimerAfter, &e.LiftedKinds, &e.RangeDays, &e.RangeFrom, &e.RangeTo, &e.RangeInvert, &queue, &relifted)
	if err == sql.ErrNoRows {
		return LockExtras{}, nil
	}
	if err != nil {
		return LockExtras{}, fmt.Errorf("failed to read locks of '%s': %w", name, err)
	}
	if e.Queue, err = decodeQueue(queue); err != nil {
		return LockExtras{}, fmt.Errorf("failed to read locks of '%s': %w", name, err)
	}
	if e.Relifted, err = decodeQueue(relifted); err != nil {
		return LockExtras{}, fmt.Errorf("failed to read locks of '%s': %w", name, err)
	}
	return e, nil
}

// SetLockExtras stores an active block's extra locks.
func (d *DB) SetLockExtras(name string, e LockExtras) error {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec(`UPDATE active_blocks SET lock_password = ?, restart_boot = ?, lock_delay = ?, stop_at = ?, lock_style = ?, delay_until = ?,
		relock = ?, lifted_until = ?, lock_order = ?, timer_after = ?, lifted_kinds = ?,
		range_days = ?, range_from = ?, range_to = ?, range_invert = ?, lock_queue = ?, lock_relifted = ? WHERE block_id = ?`,
		e.PasswordHash, e.RestartBoot, e.Delay, e.StopAt, e.Style, e.DelayUntil,
		e.Relock, e.LiftedUntil, e.Order, e.TimerAfter, e.LiftedKinds,
		e.RangeDays, e.RangeFrom, e.RangeTo, e.RangeInvert, encodeQueue(e.Queue), encodeQueue(e.Relifted), blockID)
	if err != nil {
		return fmt.Errorf("failed to save locks of '%s': %w", name, err)
	}
	return nil
}

// GetSetting returns a setting, or "" if unset.
func (d *DB) GetSetting(key string) (string, error) {
	var value string
	err := d.conn.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to read setting %s: %w", key, err)
	}
	return value, nil
}

// SetSetting stores a setting.
func (d *DB) SetSetting(key, value string) error {
	_, err := d.conn.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	if err != nil {
		return fmt.Errorf("failed to save setting %s: %w", key, err)
	}
	return nil
}
