// nepsis.go — bypass detection ("nepsis", watchfulness). When KC9K stops
// many things within a short time, someone is likely hunting for a way
// around it. Bypass detection counts the events its detectors pick
// (events.go) and climbs a ladder of steps, each with its own trigger and
// actions: a warning first, say, then monk mode, then a shutdown. After a
// quiet time without firing, it falls back a step.
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
	"slices"
	"strings"
	"time"
)

// What a step of bypass detection can do.
const (
	ConsequenceWarning      = "warning"       // a notification
	ConsequenceCloseTab     = "close-tab"     // the tab of the page that tripped it
	ConsequenceCloseBrowser = "close-browser" //
	ConsequenceLockScreen   = "lock-screen"   // Minutes: how long (required)
	ConsequenceLogout       = "logout"        // Minutes: stay logged out (0: once)
	ConsequenceShutdown     = "shutdown"      // Minutes: stay off (0: once)
	ConsequenceMonk         = "monk"          // Minutes: how long (required)
)

// Consequences lists them in display order, mildest first.
var Consequences = []string{ConsequenceWarning, ConsequenceCloseTab, ConsequenceCloseBrowser,
	ConsequenceLockScreen, ConsequenceLogout, ConsequenceShutdown, ConsequenceMonk}

// Detectors are the event kinds bypass detection can count.
var Detectors = []string{EventPage, EventApp, EventTitle, EventBrowser, EventRefused, EventTamper}

// Limits on bypass detection's own numbers.
const (
	MaxNepsisCount   = 1000
	MaxNepsisMinutes = 24 * 60
	MaxNepsisSteps   = 10
)

// SettingNepsis holds the global bypass detection, as JSON; its locks are
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
    config         TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`

// NepsisAction is one thing a step does, for Minutes where that applies.
type NepsisAction struct {
	Kind    string `json:"action"`
	Minutes int    `json:"minutes,omitempty"`
}

// NepsisStep is one rung of the ladder: when Count events happen within
// Minutes, take Actions.
type NepsisStep struct {
	Count   int            `json:"count"`
	Minutes int            `json:"minutes"`
	Actions []NepsisAction `json:"actions"`
}

// Window is how far back events count for the step.
func (s NepsisStep) Window() time.Duration { return time.Duration(s.Minutes) * time.Minute }

// Nepsis is one bypass detection. Detectors are the event kinds that count
// (none: all). Steps run in order: the first time it trips, step 1; the next
// time, step 2; past the last, the last again. After QuietMinutes without
// tripping it falls back a step (0: never). No steps is off.
type Nepsis struct {
	Detectors    []string     `json:"detectors,omitempty"`
	Steps        []NepsisStep `json:"steps,omitempty"`
	QuietMinutes int          `json:"quiet_minutes,omitempty"`

	// One step, as stored before the ladder; read only (see normalize).
	Count         int    `json:"count,omitempty"`
	Minutes       int    `json:"minutes,omitempty"`
	Consequence   string `json:"consequence,omitempty"`
	ActionMinutes int    `json:"action_minutes,omitempty"`
}

// On reports whether it is set.
func (n Nepsis) On() bool { return len(n.Steps) > 0 }

// normalize turns the one step stored before the ladder into a ladder. That
// consequence closed the browsers first, so the step does too.
func (n Nepsis) normalize() Nepsis {
	if len(n.Steps) == 0 && n.Count > 0 {
		actions := []NepsisAction{{Kind: ConsequenceCloseBrowser}}
		if n.Consequence != ConsequenceCloseBrowser {
			actions = append(actions, NepsisAction{Kind: n.Consequence, Minutes: n.ActionMinutes})
		}
		n.Steps = []NepsisStep{{Count: n.Count, Minutes: n.Minutes, Actions: actions}}
	}
	n.Count, n.Minutes, n.Consequence, n.ActionMinutes = 0, 0, "", 0
	return n
}

// Counts reports whether events of a kind count.
func (n Nepsis) Counts(kind string) bool {
	return len(n.Detectors) == 0 || slices.Contains(n.Detectors, kind)
}

// Step returns the step for a level (0-based), the last one past the end.
func (n Nepsis) Step(level int) NepsisStep {
	return n.Steps[min(max(level, 0), len(n.Steps)-1)]
}

// IsConsequence reports whether name is a consequence.
func IsConsequence(name string) bool { return slices.Contains(Consequences, name) }

// Check reports what is wrong with a bypass detection that is on.
func (n Nepsis) Check() error {
	if len(n.Steps) > MaxNepsisSteps {
		return fmt.Errorf("at most %d steps", MaxNepsisSteps)
	}
	for _, d := range n.Detectors {
		if !slices.Contains(Detectors, d) {
			return fmt.Errorf("unknown detector %q (use %s)", d, strings.Join(Detectors, ", "))
		}
	}
	if n.QuietMinutes < 0 || n.QuietMinutes > 30*24*60 {
		return fmt.Errorf("the quiet time is at most 30 days")
	}
	for i, s := range n.Steps {
		if s.Count < 1 || s.Count > MaxNepsisCount {
			return fmt.Errorf("step %d: the count is a number from 1 to %d", i+1, MaxNepsisCount)
		}
		if s.Minutes < 1 || s.Minutes > MaxNepsisMinutes {
			return fmt.Errorf("step %d: the window is from 1 minute to %d hours", i+1, MaxNepsisMinutes/60)
		}
		if len(s.Actions) == 0 {
			return fmt.Errorf("step %d needs at least one action", i+1)
		}
		for _, a := range s.Actions {
			if err := CheckConsequence(a.Kind, a.Minutes); err != nil {
				return fmt.Errorf("step %d: %w", i+1, err)
			}
		}
	}
	return nil
}

// CheckConsequence checks a consequence and its minutes.
func CheckConsequence(name string, minutes int) error {
	switch name {
	case ConsequenceWarning, ConsequenceCloseTab, ConsequenceCloseBrowser:
		if minutes != 0 {
			return fmt.Errorf("%s takes no duration", name)
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
		return fmt.Errorf("unknown action %q (use %s)", name, strings.Join(Consequences, ", "))
	}
	return nil
}

// actionsStricter reports whether actions take every action of old, each for
// at least as long.
func actionsStricter(actions, old []NepsisAction) bool {
	for _, o := range old {
		found := false
		for _, a := range actions {
			if a.Kind == o.Kind && a.Minutes >= o.Minutes {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// StricterThan reports whether n can only be stricter than old: it is on
// when old was off; or it counts every detector old did, falls back no
// sooner, and each of old's steps is matched at the same place by one that
// trips on no more events, over at least as long, taking every action of
// old's for at least as long (more steps at the end are fine). Anything
// else loosens.
func (n Nepsis) StricterThan(old Nepsis) bool {
	if !n.On() {
		return !old.On()
	}
	if !old.On() {
		return true
	}
	if len(n.Detectors) > 0 {
		if len(old.Detectors) == 0 {
			return false
		}
		for _, d := range old.Detectors {
			if !slices.Contains(n.Detectors, d) {
				return false
			}
		}
	}
	if old.QuietMinutes == 0 && n.QuietMinutes != 0 || old.QuietMinutes != 0 && n.QuietMinutes != 0 && n.QuietMinutes < old.QuietMinutes {
		return false
	}
	if len(n.Steps) < len(old.Steps) {
		return false
	}
	for i, o := range old.Steps {
		s := n.Steps[i]
		if s.Count > o.Count || s.Minutes < o.Minutes || !actionsStricter(s.Actions, o.Actions) {
			return false
		}
	}
	return true
}

// GlobalNepsis returns the bypass detection for the whole computer.
func (d *DB) GlobalNepsis() (Nepsis, error) {
	var n Nepsis
	value, err := d.GetSetting(SettingNepsis)
	if err != nil || value == "" {
		return n, err
	}
	if err := json.Unmarshal([]byte(value), &n); err != nil {
		return Nepsis{}, fmt.Errorf("bad bypass detection setting: %w", err)
	}
	return n.normalize(), nil
}

// SetGlobalNepsis stores the bypass detection for the whole computer.
func (d *DB) SetGlobalNepsis(n Nepsis) error {
	n = n.normalize()
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

// BlockNepsis returns a block's bypass detection.
func (d *DB) BlockNepsis(blockName string) (Nepsis, error) {
	var n Nepsis
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return n, err
	}
	var config string
	err = d.conn.QueryRow("SELECT count, minutes, consequence, action_minutes, config FROM block_nepsis WHERE block_id = ?", blockID).
		Scan(&n.Count, &n.Minutes, &n.Consequence, &n.ActionMinutes, &config)
	if err == sql.ErrNoRows {
		return Nepsis{}, nil
	}
	if err != nil {
		return n, fmt.Errorf("failed to read bypass detection: %w", err)
	}
	if config != "" {
		n = Nepsis{}
		if err := json.Unmarshal([]byte(config), &n); err != nil {
			return Nepsis{}, fmt.Errorf("bad bypass detection of '%s': %w", blockName, err)
		}
	}
	return n.normalize(), nil
}

// SetBlockNepsis stores a block's bypass detection; one that is off is
// removed.
func (d *DB) SetBlockNepsis(blockName string, n Nepsis) error {
	n = n.normalize()
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
	data, err := json.Marshal(n)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec(`INSERT INTO block_nepsis (block_id, count, minutes, consequence, action_minutes, config)
		VALUES (?, 0, 0, '', 0, ?) ON CONFLICT(block_id) DO UPDATE SET count = 0, minutes = 0,
		consequence = '', action_minutes = 0, config = excluded.config`, blockID, string(data))
	if err != nil {
		return fmt.Errorf("failed to save bypass detection: %w", err)
	}
	return nil
}
