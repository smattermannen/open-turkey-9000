// nepsis.go — bypass prevention ("nepsis", watchfulness). When KC9K stops
// many things within a short time, someone is likely hunting for a way
// around it; a consequence the user chose then makes them stop and shift:
// close the browser, lock the screen, log out, shut down, or monk mode, some
// of them for a while.
//
// It can be set for the whole computer (stored as a setting, and lockable
// like a browser option) and for each block (it counts that block's events
// while the block is on, and loosening it is gated like any edit to the
// block). When both are set, both apply.
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Consequences bypass prevention can take.
const (
	ConsequenceCloseBrowser = "close-browser"
	ConsequenceLockScreen   = "lock-screen" // Minutes: how long (required)
	ConsequenceLogout       = "logout"      // Minutes: stay logged out (0: once)
	ConsequenceShutdown     = "shutdown"    // Minutes: stay off (0: once)
	ConsequenceMonk         = "monk"        // Minutes: how long (required)
)

// Consequences lists them in display order, mildest first.
var Consequences = []string{ConsequenceCloseBrowser, ConsequenceLockScreen, ConsequenceLogout,
	ConsequenceShutdown, ConsequenceMonk}

// Limits on bypass prevention's own numbers.
const (
	MaxNepsisCount   = 1000
	MaxNepsisMinutes = 24 * 60
)

// SettingNepsis holds the global bypass prevention, as JSON; its locks are
// the setting lock named NepsisLockName.
const (
	SettingNepsis  = "nepsis"
	NepsisLockName = "nepsis"
)

const nepsisSchema = `CREATE TABLE IF NOT EXISTS block_nepsis (
    block_id       INTEGER PRIMARY KEY,
    count          INTEGER NOT NULL,
    minutes        INTEGER NOT NULL,
    consequence    TEXT NOT NULL,
    action_minutes INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`

// Nepsis is one bypass prevention: when Count events happen within Minutes,
// take Consequence (for ActionMinutes, where it lasts). Count 0 is off.
type Nepsis struct {
	Count         int    `json:"count"`
	Minutes       int    `json:"minutes"`
	Consequence   string `json:"consequence"`
	ActionMinutes int    `json:"action_minutes"`
}

// On reports whether it is set.
func (n Nepsis) On() bool { return n.Count > 0 }

// Window is how far back events count.
func (n Nepsis) Window() time.Duration { return time.Duration(n.Minutes) * time.Minute }

// IsConsequence reports whether name is a consequence.
func IsConsequence(name string) bool {
	for _, c := range Consequences {
		if c == name {
			return true
		}
	}
	return false
}

// Check reports what is wrong with a bypass prevention that is on.
func (n Nepsis) Check() error {
	if n.Count < 1 || n.Count > MaxNepsisCount {
		return fmt.Errorf("the count is a number from 1 to %d", MaxNepsisCount)
	}
	if n.Minutes < 1 || n.Minutes > MaxNepsisMinutes {
		return fmt.Errorf("the window is from 1 minute to %d hours", MaxNepsisMinutes/60)
	}
	return CheckConsequence(n.Consequence, n.ActionMinutes)
}

// CheckConsequence checks a consequence and its minutes.
func CheckConsequence(name string, minutes int) error {
	switch name {
	case ConsequenceCloseBrowser:
		if minutes != 0 {
			return fmt.Errorf("close-browser takes no duration")
		}
	case ConsequenceLockScreen:
		if minutes < 1 || minutes > MaxFreezeMinutes {
			return fmt.Errorf("lock-screen lasts from 1 minute to %d hours", MaxFreezeMinutes/60)
		}
	case ConsequenceLogout, ConsequenceShutdown:
		if minutes < 0 || minutes > MaxFreezeMinutes {
			return fmt.Errorf("%s lasts at most %d hours (0: just once)", name, MaxFreezeMinutes/60)
		}
	case ConsequenceMonk:
		if minutes < 1 || minutes > MaxMonkMinutes {
			return fmt.Errorf("monk mode lasts from 1 minute to %d days", MaxMonkMinutes/(24*60))
		}
	default:
		return fmt.Errorf("unknown consequence %q", name)
	}
	return nil
}

// StricterThan reports whether n can only be stricter than old: it is on
// when old was off, or it fires on fewer events, over a longer window, with
// the same consequence lasting at least as long. Anything else loosens.
func (n Nepsis) StricterThan(old Nepsis) bool {
	if !n.On() {
		return !old.On()
	}
	if !old.On() {
		return true
	}
	return n.Count <= old.Count && n.Minutes >= old.Minutes &&
		n.Consequence == old.Consequence && n.ActionMinutes >= old.ActionMinutes
}

// GlobalNepsis returns the bypass prevention for the whole computer.
func (d *DB) GlobalNepsis() (Nepsis, error) {
	var n Nepsis
	value, err := d.GetSetting(SettingNepsis)
	if err != nil || value == "" {
		return n, err
	}
	if err := json.Unmarshal([]byte(value), &n); err != nil {
		return Nepsis{}, fmt.Errorf("bad bypass prevention setting: %w", err)
	}
	return n, nil
}

// SetGlobalNepsis stores the bypass prevention for the whole computer.
func (d *DB) SetGlobalNepsis(n Nepsis) error {
	if !n.On() {
		return d.SetSetting(SettingNepsis, "")
	}
	if err := n.Check(); err != nil {
		return err
	}
	data, err := json.Marshal(n)
	if err != nil {
		return err
	}
	return d.SetSetting(SettingNepsis, string(data))
}

// BlockNepsis returns a block's bypass prevention.
func (d *DB) BlockNepsis(blockName string) (Nepsis, error) {
	var n Nepsis
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return n, err
	}
	err = d.conn.QueryRow("SELECT count, minutes, consequence, action_minutes FROM block_nepsis WHERE block_id = ?", blockID).
		Scan(&n.Count, &n.Minutes, &n.Consequence, &n.ActionMinutes)
	if err == sql.ErrNoRows {
		return Nepsis{}, nil
	}
	if err != nil {
		return n, fmt.Errorf("failed to read bypass prevention: %w", err)
	}
	return n, nil
}

// SetBlockNepsis stores a block's bypass prevention; one that is off is
// removed.
func (d *DB) SetBlockNepsis(blockName string, n Nepsis) error {
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return err
	}
	if !n.On() {
		_, err = d.conn.Exec("DELETE FROM block_nepsis WHERE block_id = ?", blockID)
		return err
	}
	if err := n.Check(); err != nil {
		return err
	}
	_, err = d.conn.Exec(`INSERT INTO block_nepsis (block_id, count, minutes, consequence, action_minutes) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(block_id) DO UPDATE SET count = excluded.count, minutes = excluded.minutes,
		consequence = excluded.consequence, action_minutes = excluded.action_minutes`,
		blockID, n.Count, n.Minutes, n.Consequence, n.ActionMinutes)
	if err != nil {
		return fmt.Errorf("failed to save bypass prevention: %w", err)
	}
	return nil
}
