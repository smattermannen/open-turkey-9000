// clock.go — a time source for time locks that survives clock changes.
//
// While a time lock is active, trusted time only moves with CLOCK_BOOTTIME,
// which keeps counting during suspend and which `date` cannot change. Time
// spent powered off is not counted: after a reboot nothing distinguishes real
// downtime from a clock moved forward. With no time lock active, trusted time
// simply follows the wall clock.
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

	lockActive := false
	if prevPtr != nil {
		var n int
		if err := d.conn.QueryRow("SELECT COUNT(*) FROM active_blocks WHERE locked_until > ?", prev.Trusted).Scan(&n); err != nil {
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
