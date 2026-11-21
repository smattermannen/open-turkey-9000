// clock.go — trusted time (§3.4): the time timer locks are measured in.
//
// By default it is the real clock. With the tamper protection setting
// "block time and date changes" on (SettingTamperClock), while a timer lock
// (on a block or a setting) runs, trusted time only moves with
// CLOCK_BOOTTIME, which keeps counting during suspend and which `date` can't
// change; time spent powered off isn't counted then, since after a reboot
// nothing tells real downtime from a clock moved forward. With no timer lock
// running, it follows the clock either way.
//
// The clock check (§4.10 "check against hardware clock and internet time";
// SPEC §6 2026-11-18) gives powered-off time back once it is proven: once
// per boot while a timer lock runs, the daemon asks the internet for the
// time (enforce/clockcheck.go) and CheckClock keeps an anchor: real time,
// trusted time and the hardware clock at that moment. At the next anchor
// trusted time is raised to anchor trusted + real time passed, but never by
// more than the hardware clock says passed. The system clock plays no part,
// so moving it does nothing; without the internet nothing is given back.
// TrustedNow keeps the anchor as a floor (anchor trusted + boot clock since
// it), so a write racing the check can't lose what it gave back, and drops
// it whenever no timer lock runs (trusted time is the system clock then).
package db

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"
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
	Anchor   clockAnchor
}

// clockAnchor is the last proven moment: at internet time Real (unix
// seconds), trusted time was Trusted, the boot was Boot with the boot clock
// at Boottime, and the hardware clock read RTC (0: none).
type clockAnchor struct {
	Boot     string
	Boottime int64
	Real     int64
	Trusted  int64
	RTC      int64
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
	next.Anchor = prev.Anchor
	if prev.BootID == bootID && boottime > prev.Boottime {
		next.Trusted += boottime - prev.Boottime
	}
	if a := prev.Anchor; a.Boot == bootID && a.Real > 0 && boottime >= a.Boottime {
		if floor := a.Trusted + boottime - a.Boottime; floor > next.Trusted {
			next.Trusted = floor
		}
	}
	return next
}

// checkClock takes a new anchor at internet time real, with the boot clock
// at boottime and the hardware clock at rtc (rtcOK false: none). It returns
// the state and how many seconds of trusted time it gave back. ok is false
// when real is earlier than the last anchor's: nothing is changed then.
func checkClock(prev clockState, bootID string, boottime, real, rtc int64, rtcOK bool) (next clockState, credited int64, ok bool) {
	next = prev
	a := prev.Anchor
	if a.Real > 0 && real < a.Real {
		return prev, 0, false
	}
	if a.Real > 0 {
		candidate := a.Trusted + (real - a.Real)
		if rtcOK && a.RTC > 0 {
			if byRTC := a.Trusted + (rtc - a.RTC); byRTC < candidate {
				candidate = byRTC
			}
		}
		if candidate > next.Trusted {
			credited = candidate - next.Trusted
			next.Trusted = candidate
		}
	}
	if !rtcOK {
		rtc = 0
	}
	next.Anchor = clockAnchor{Boot: bootID, Boottime: boottime, Real: real, Trusted: next.Trusted, RTC: rtc}
	return next, credited, true
}

// TrustedNow advances and returns trusted time in unix seconds. Compare
// locked_until only against this value, never against the wall clock.
func (d *DB) TrustedNow(wall time.Time) (int64, error) {
	bootBytes, err := readBootID()
	if err != nil {
		return 0, fmt.Errorf("failed to read boot id: %w", err)
	}
	boottime, err := bootClock()
	if err != nil {
		return 0, err
	}
	bootID := strings.TrimSpace(string(bootBytes))

	var prev clockState
	prevPtr := &prev
	err = d.conn.QueryRow(`SELECT boot_id, boottime, trusted, anchor_boot, anchor_boottime, anchor_real,
		anchor_trusted, anchor_rtc FROM clock_state WHERE id = 1`).
		Scan(&prev.BootID, &prev.Boottime, &prev.Trusted, &prev.Anchor.Boot, &prev.Anchor.Boottime,
			&prev.Anchor.Real, &prev.Anchor.Trusted, &prev.Anchor.RTC)
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
		if lockActive, err = d.timerLockRunning(prev.Trusted); err != nil {
			return 0, err
		}
	}

	next := advanceClock(prevPtr, wall.Unix(), bootID, boottime, lockActive)
	_, err = d.conn.Exec(`INSERT INTO clock_state (id, boot_id, boottime, trusted) VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET boot_id = excluded.boot_id, boottime = excluded.boottime, trusted = excluded.trusted`,
		next.BootID, next.Boottime, next.Trusted)
	if err != nil {
		return 0, fmt.Errorf("failed to save clock state: %w", err)
	}
	if !lockActive {
		// Trusted time follows the system clock now, which may be set
		// anywhere: an anchor links only moments within one unbroken run of
		// timer locks, or setting the clock back before the next lock would
		// let the anchor shorten it.
		if _, err := d.conn.Exec(`UPDATE clock_state SET anchor_boot = '', anchor_boottime = 0, anchor_real = 0,
			anchor_trusted = 0, anchor_rtc = 0 WHERE id = 1 AND anchor_real != 0`); err != nil {
			return 0, fmt.Errorf("failed to save clock state: %w", err)
		}
	}
	return next.Trusted, nil
}

// timerLockRunning reports whether a timer lock runs at trusted time now.
// Setting locks and monk mode count too, or moving the clock forward would
// end them.
func (d *DB) timerLockRunning(now int64) (bool, error) {
	var n int
	if err := d.conn.QueryRow(`SELECT (SELECT COUNT(*) FROM active_blocks WHERE locked_until > ?) +
		(SELECT COUNT(*) FROM setting_locks WHERE locked_until > ?) +
		(SELECT COUNT(*) FROM settings WHERE key = ? AND CAST(value AS INTEGER) > ?)`,
		now, now, SettingMonkUntil, now).Scan(&n); err != nil {
		return false, fmt.Errorf("failed to check timer locks: %w", err)
	}
	return n > 0, nil
}

// readBootID is this boot's ID: the file at bootIDPath when set (Linux,
// tests), else the platform's own (bootid_windows.go).
func readBootID() ([]byte, error) {
	if bootIDPath != "" {
		return os.ReadFile(bootIDPath)
	}
	return platformBootID()
}

// ClockCheckDue reports whether the daemon should take the clock check's
// anchor now: block time and date changes is on, a timer lock runs, and
// none was taken in this boot.
func (d *DB) ClockCheckDue(wall time.Time) (bool, error) {
	if protect, err := d.GetSetting(SettingTamperClock); err != nil || protect != "on" {
		return false, err
	}
	trusted, err := d.TrustedNow(wall)
	if err != nil {
		return false, err
	}
	if running, err := d.timerLockRunning(trusted); err != nil || !running {
		return false, err
	}
	bootBytes, err := readBootID()
	if err != nil {
		return false, fmt.Errorf("failed to read boot id: %w", err)
	}
	var boot string
	if err := d.conn.QueryRow("SELECT anchor_boot FROM clock_state WHERE id = 1").Scan(&boot); err != nil {
		return false, fmt.Errorf("failed to read clock state: %w", err)
	}
	return boot != strings.TrimSpace(string(bootBytes)), nil
}

// CheckClock takes the clock check's anchor at internet time real, with the
// hardware clock at rtc (rtcOK false: no hardware clock). It returns the
// seconds of powered-off time given back.
func (d *DB) CheckClock(wall, real time.Time, rtc int64, rtcOK bool) (int64, error) {
	if _, err := d.TrustedNow(wall); err != nil {
		return 0, err
	}
	bootBytes, err := readBootID()
	if err != nil {
		return 0, fmt.Errorf("failed to read boot id: %w", err)
	}
	boottime, err := bootClock()
	if err != nil {
		return 0, err
	}
	var prev clockState
	if err := d.conn.QueryRow(`SELECT boot_id, boottime, trusted, anchor_boot, anchor_boottime, anchor_real,
		anchor_trusted, anchor_rtc FROM clock_state WHERE id = 1`).
		Scan(&prev.BootID, &prev.Boottime, &prev.Trusted, &prev.Anchor.Boot, &prev.Anchor.Boottime,
			&prev.Anchor.Real, &prev.Anchor.Trusted, &prev.Anchor.RTC); err != nil {
		return 0, fmt.Errorf("failed to read clock state: %w", err)
	}
	next, credited, ok := checkClock(prev, strings.TrimSpace(string(bootBytes)), boottime, real.Unix(), rtc, rtcOK)
	if !ok {
		return 0, fmt.Errorf("internet time %s is before the last check's", real.UTC().Format(time.RFC3339))
	}
	a := next.Anchor
	// MAX: a TrustedNow racing this one may have moved trusted on meanwhile.
	_, err = d.conn.Exec(`UPDATE clock_state SET trusted = MAX(trusted, ?), anchor_boot = ?, anchor_boottime = ?,
		anchor_real = ?, anchor_trusted = ?, anchor_rtc = ? WHERE id = 1`,
		next.Trusted, a.Boot, a.Boottime, a.Real, a.Trusted, a.RTC)
	if err != nil {
		return 0, fmt.Errorf("failed to save clock check: %w", err)
	}
	return credited, nil
}
