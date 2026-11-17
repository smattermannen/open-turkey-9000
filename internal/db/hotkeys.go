// hotkeys.go — key combinations that start a block, optionally with locks
// (see cli/hotkeys.go).
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const hotkeysSchema = `CREATE TABLE IF NOT EXISTS hotkeys (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    keys     TEXT NOT NULL UNIQUE,
    block    TEXT NOT NULL,
    lock_for TEXT NOT NULL DEFAULT '',
    no_confirm INTEGER NOT NULL DEFAULT 0,
    lock_args TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL DEFAULT '',
    actions TEXT NOT NULL DEFAULT ''
);`

// Hotkey action kinds (§3.9 Hotkey action; owner 2026-11-17): what a press
// does, in order.
const (
	HotkeyStart        = "start"         // Target: a block; LockArgs: start's lock flags
	HotkeyStop         = "stop"          // Target: a block (refused while locked)
	HotkeyModeOn       = "mode-on"       // Target: a mode
	HotkeyModeOff      = "mode-off"      // Target: a mode (locked blocks stay on)
	HotkeyModeRun      = "mode-run"      // Target: a mode; Minutes: it runs, locked (R-MODE-4)
	HotkeyMonk         = "monk"          // Minutes: monk mode, locked
	HotkeyFreeze       = "freeze"        // Minutes; Param: the freeze mode (lock, logout, shutdown, apps)
	HotkeyCloseBrowser = "close-browser" //
	HotkeyLockScreen   = "lock-screen"   // Minutes: through Frozen Chicken (0: lock once)
	HotkeyLogout       = "logout"        // Minutes: stay logged out (0: once)
	HotkeyShutdown     = "shutdown"      // Minutes: stay off (0: once)
	HotkeyCustom       = "custom"        // Param: what it opens or runs (CustomParam)
)

// HotkeyKinds lists them in display order.
var HotkeyKinds = []string{HotkeyStart, HotkeyStop, HotkeyModeOn, HotkeyModeOff, HotkeyModeRun, HotkeyMonk,
	HotkeyFreeze, HotkeyCloseBrowser, HotkeyLockScreen, HotkeyLogout, HotkeyShutdown, HotkeyCustom}

// HotkeyAction is one thing a hotkey does.
type HotkeyAction struct {
	Kind     string   `json:"kind"`
	Target   string   `json:"target,omitempty"`
	Minutes  int      `json:"minutes,omitempty"`
	Param    string   `json:"param,omitempty"`
	LockArgs []string `json:"lock_args,omitempty"`
}

// Locks reports whether the action sets a lock (R-LOCK-12: confirmed).
func (a HotkeyAction) Locks() bool {
	switch a.Kind {
	case HotkeyStart:
		return len(a.LockArgs) > 0
	case HotkeyModeRun, HotkeyMonk, HotkeyFreeze:
		return true
	case HotkeyLockScreen, HotkeyLogout, HotkeyShutdown:
		return a.Minutes > 0
	}
	return false
}

// Weakens reports whether the action could weaken something (it needs the
// settings password when the hotkey is added).
func (a HotkeyAction) Weakens() bool { return a.Kind == HotkeyStop || a.Kind == HotkeyModeOff }

// CheckHotkeyAction checks an action's own values (whether its block or mode
// exists is checked when it is added and when it runs).
func CheckHotkeyAction(a HotkeyAction) error {
	needsTarget := map[string]bool{HotkeyStart: true, HotkeyStop: true, HotkeyModeOn: true, HotkeyModeOff: true, HotkeyModeRun: true}
	switch {
	case !slices.Contains(HotkeyKinds, a.Kind):
		return fmt.Errorf("unknown hotkey action %q (use %s)", a.Kind, strings.Join(HotkeyKinds, ", "))
	case needsTarget[a.Kind] && strings.TrimSpace(a.Target) == "":
		return fmt.Errorf("%s needs a %s", a.Kind, map[bool]string{true: "block", false: "mode"}[a.Kind == HotkeyStart || a.Kind == HotkeyStop])
	case !needsTarget[a.Kind] && a.Target != "":
		return fmt.Errorf("%s takes no block or mode", a.Kind)
	case len(a.LockArgs) > 0 && a.Kind != HotkeyStart:
		return fmt.Errorf("only start takes locks")
	}
	switch a.Kind {
	case HotkeyModeRun, HotkeyMonk:
		if a.Minutes < 1 || a.Minutes > MaxMonkMinutes {
			return fmt.Errorf("%s lasts from 1 minute to %d days", a.Kind, MaxMonkMinutes/(24*60))
		}
	case HotkeyFreeze:
		if a.Minutes < 1 || a.Minutes > MaxFreezeMinutes {
			return fmt.Errorf("Frozen Chicken lasts from 1 minute to a week")
		}
		if a.Param != "lock" && a.Param != "logout" && a.Param != "shutdown" && a.Param != "apps" {
			return fmt.Errorf("Frozen Chicken locks the screen, logs out, shuts down or keeps to allowed apps (lock, logout, shutdown, apps)")
		}
		return nil
	case HotkeyLockScreen, HotkeyLogout, HotkeyShutdown:
		if a.Minutes < 0 || a.Minutes > MaxFreezeMinutes {
			return fmt.Errorf("%s lasts at most a week", a.Kind)
		}
	case HotkeyCustom:
		return CheckCustomParam(a.Param)
	default:
		if a.Minutes != 0 {
			return fmt.Errorf("%s takes no time", a.Kind)
		}
	}
	if a.Param != "" {
		return fmt.Errorf("%s takes nothing more", a.Kind)
	}
	return nil
}

// Hotkey starts Block when Keys (a GTK accelerator such as <Primary><Alt>s)
// are pressed, with the locks of LockArgs: start's lock flags in unlock
// order, e.g. ["--lock", "--for", "1h"]. Older hotkeys have only LockFor, a
// timer lock's duration (see Args).
type Hotkey struct {
	ID       int      `json:"id"`
	Keys     string   `json:"keys"`
	Block    string   `json:"block"`
	LockFor  string   `json:"lock_for,omitempty"`
	LockArgs []string `json:"lock_args,omitempty"`
	// PasswordHash is the password lock's, given when the hotkey was added.
	// Never shown (gui-state); backups carry it separately.
	PasswordHash string `json:"-"`
	// NoConfirm: "Run without confirmation". Off, pressing the hotkey asks
	// before it sets its lock; on, the lock was confirmed only when the
	// hotkey was set up (R-LOCK-12).
	NoConfirm bool `json:"no_confirm"`
	// Actions: what a press does, in order (owner 2026-11-17). Older hotkeys
	// have none: they start Block with LockArgs (see Steps).
	Actions []HotkeyAction `json:"actions,omitempty"`
}

// Steps returns what a press does: its actions, or, for an older hotkey,
// starting its block with its locks.
func (h Hotkey) Steps() []HotkeyAction {
	if len(h.Actions) > 0 {
		return h.Actions
	}
	if h.Block == "" {
		return nil
	}
	return []HotkeyAction{{Kind: HotkeyStart, Target: h.Block, LockArgs: h.Args()}}
}

// Args returns the hotkey's lock flags: LockArgs, or a timer lock of
// LockFor for older hotkeys.
func (h Hotkey) Args() []string {
	if len(h.LockArgs) > 0 {
		return h.LockArgs
	}
	if h.LockFor != "" {
		return []string{"--for", h.LockFor}
	}
	return nil
}

// migrateHotkeys adds the newer columns to older databases: their hotkeys
// ask at each press, and keep their timer lock in lock_for.
func migrateHotkeys(conn *sql.DB) error {
	for _, c := range [][2]string{
		{"no_confirm", "INTEGER NOT NULL DEFAULT 0"},
		{"lock_args", "TEXT NOT NULL DEFAULT ''"},
		{"password_hash", "TEXT NOT NULL DEFAULT ''"},
		{"actions", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := addColumnIfMissing(conn, "hotkeys", c[0], c[1]); err != nil {
			return err
		}
	}
	return nil
}

func encodeLockArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	b, _ := json.Marshal(args)
	return string(b)
}

func decodeLockArgs(s string) []string {
	var args []string
	if s != "" {
		_ = json.Unmarshal([]byte(s), &args)
	}
	return args
}

// AddHotkey saves a hotkey and returns its ID.
func (d *DB) AddHotkey(h Hotkey) (int, error) {
	if h.Block != "" {
		if _, err := d.getBlockID(h.Block); err != nil {
			return 0, err
		}
	}
	actions := ""
	if len(h.Actions) > 0 {
		raw, _ := json.Marshal(h.Actions)
		actions = string(raw)
	}
	result, err := d.conn.Exec("INSERT INTO hotkeys (keys, block, lock_for, no_confirm, lock_args, password_hash, actions) VALUES (?, ?, ?, ?, ?, ?, ?)",
		h.Keys, h.Block, h.LockFor, h.NoConfirm, encodeLockArgs(h.LockArgs), h.PasswordHash, actions)
	if err != nil {
		return 0, fmt.Errorf("failed to add the hotkey (is %s already in use?): %w", h.Keys, err)
	}
	id, err := result.LastInsertId()
	return int(id), err
}

// RemoveHotkey deletes the hotkey with this ID.
func (d *DB) RemoveHotkey(id int) error {
	result, err := d.conn.Exec("DELETE FROM hotkeys WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to remove hotkey %d: %w", id, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("no hotkey %d (see 'kc9k hotkey list')", id)
	}
	return nil
}

// Hotkeys lists every hotkey, oldest first.
func (d *DB) Hotkeys() ([]Hotkey, error) {
	rows, err := d.conn.Query("SELECT id, keys, block, lock_for, no_confirm, lock_args, password_hash, actions FROM hotkeys ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("failed to list hotkeys: %w", err)
	}
	defer rows.Close()
	out := []Hotkey{}
	for rows.Next() {
		var h Hotkey
		var args, actions string
		if err := rows.Scan(&h.ID, &h.Keys, &h.Block, &h.LockFor, &h.NoConfirm, &args, &h.PasswordHash, &actions); err != nil {
			return nil, fmt.Errorf("failed to read a hotkey: %w", err)
		}
		h.LockArgs = decodeLockArgs(args)
		if actions != "" {
			_ = json.Unmarshal([]byte(actions), &h.Actions)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// GetHotkey returns the hotkey with this ID.
func (d *DB) GetHotkey(id int) (Hotkey, error) {
	var h Hotkey
	var args, actions string
	err := d.conn.QueryRow("SELECT id, keys, block, lock_for, no_confirm, lock_args, password_hash, actions FROM hotkeys WHERE id = ?", id).
		Scan(&h.ID, &h.Keys, &h.Block, &h.LockFor, &h.NoConfirm, &args, &h.PasswordHash, &actions)
	if err != nil {
		return h, fmt.Errorf("no hotkey %d (see 'kc9k hotkey list')", id)
	}
	h.LockArgs = decodeLockArgs(args)
	if actions != "" {
		_ = json.Unmarshal([]byte(actions), &h.Actions)
	}
	return h, nil
}
