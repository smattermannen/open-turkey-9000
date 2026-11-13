package db

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
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
	err = d.conn.QueryRow(`SELECT lock_password, restart_boot, lock_delay, stop_at, lock_style, delay_until, relock, lifted_until
		FROM active_blocks WHERE block_id = ?`, blockID).Scan(&e.PasswordHash, &e.RestartBoot, &e.Delay, &e.StopAt, &e.Style, &e.DelayUntil,
		&e.Relock, &e.LiftedUntil)
	if err == sql.ErrNoRows {
		return LockExtras{}, nil
	}
	if err != nil {
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
		relock = ?, lifted_until = ? WHERE block_id = ?`, e.PasswordHash, e.RestartBoot, e.Delay, e.StopAt, e.Style, e.DelayUntil,
		e.Relock, e.LiftedUntil, blockID)
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
