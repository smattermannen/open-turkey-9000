// clock.go — trusted time (§3.4): the time timer locks are measured in.
//
// By default it is the real clock. With the tamper protection setting
// "block time and date changes" on (SettingTamperClock), while a timer lock
// (on a block or a setting) runs, trusted time only moves with
// CLOCK_BOOTTIME, which keeps counting during suspend and which `date` can't
// change; time spent powered off isn't counted then, since after a reboot
// nothing tells real downtime from a clock moved forward. With no timer lock
// running, it follows the clock either way.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const clockSchema = `CREATE TABLE IF NOT EXISTS clock_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    boot_id TEXT NOT NULL,
    boottime INTEGER NOT NULL,
    trusted INTEGER NOT NULL
);`

type clockState struct {
	BootID   string
	Boottime int64
	Trusted  int64
}

// SettingTamperClock is "on" when clock changes are ignored while a timer
// lock runs (§4.10, tamper protection). Installs from before the setting
// get it on, as that is how they worked; new ones start with it off.
const SettingTamperClock = "tamper.clock"

// migrateTamperClock decides the setting once: on where trusted time was
// already in use.
func migrateTamperClock(conn *sql.DB) error {
	var n int
	if err := conn.QueryRow("SELECT COUNT(*) FROM settings WHERE key = ?", SettingTamperClock).Scan(&n); err != nil || n > 0 {
		return err
	}
	if err := conn.QueryRow("SELECT COUNT(*) FROM clock_state").Scan(&n); err != nil {
		return err
	}
	value := "off"
	if n > 0 {
		value = "on"
	}
	_, err := conn.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", SettingTamperClock, value)
	return err
}

// advanceClock computes the next state. prev is nil on first use.
func advanceClock(prev *clockState, wall int64, bootID string, boottime int64, lockActive bool) clockState {
	next := clockState{BootID: bootID, Boottime: boottime, Trusted: wall}
	if prev == nil || !lockActive {
		return next
	}
	next.Trusted = prev.Trusted
	if prev.BootID == bootID && boottime > prev.Boottime {
		next.Trusted += boottime - prev.Boottime
	}
	return next
}

// TrustedNow advances and returns trusted time in unix seconds. Compare
// locked_until only against this value, never against the wall clock.
func (d *DB) TrustedNow(wall time.Time) (int64, error) {
	bootBytes, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return 0, fmt.Errorf("failed to read boot id: %w", err)
	}
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_BOOTTIME, &ts); err != nil {
		return 0, fmt.Errorf("failed to read boot clock: %w", err)
	}
	bootID := strings.TrimSpace(string(bootBytes))

	var prev clockState
	prevPtr := &prev
	err = d.conn.QueryRow("SELECT boot_id, boottime, trusted FROM clock_state WHERE id = 1").
		Scan(&prev.BootID, &prev.Boottime, &prev.Trusted)
	if err == sql.ErrNoRows {
		prevPtr = nil
	} else if err != nil {
		return 0, fmt.Errorf("failed to read clock state: %w", err)
	}

	protect, err := d.GetSetting(SettingTamperClock)
	if err != nil {
		return 0, err
	}
	lockActive := false
	if prevPtr != nil && protect == "on" {
		var n int
		// Setting locks count too, or moving the clock forward would end them.
		if err := d.conn.QueryRow(`SELECT (SELECT COUNT(*) FROM active_blocks WHERE locked_until > ?) +
			(SELECT COUNT(*) FROM setting_locks WHERE locked_until > ?)`, prev.Trusted, prev.Trusted).Scan(&n); err != nil {
			return 0, fmt.Errorf("failed to check time locks: %w", err)
		}
		lockActive = n > 0
	}

	next := advanceClock(prevPtr, wall.Unix(), bootID, int64(ts.Sec), lockActive)
	_, err = d.conn.Exec(`INSERT INTO clock_state (id, boot_id, boottime, trusted) VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET boot_id = excluded.boot_id, boottime = excluded.boottime, trusted = excluded.trusted`,
		next.BootID, next.Boottime, next.Trusted)
	if err != nil {
		return 0, fmt.Errorf("failed to save clock state: %w", err)
	}
	return next.Trusted, nil
}
