// triggers.go — what a block's apps and its window titles do when they fire
// (owner, 2026-11-16; §1 "any trigger to any action"): one action set per
// group, like a list's actions. Without a stored set a group does what it
// always did: apps and windows are closed. A place rule has no actions: it
// only enables or lifts its block (§3.6).
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const triggerActionsSchema = `CREATE TABLE IF NOT EXISTS trigger_actions (
    block_id INTEGER NOT NULL,
    trigger  TEXT NOT NULL,
    actions  TEXT NOT NULL,
    PRIMARY KEY (block_id, trigger),
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`

// The trigger groups, and their own action.
const (
	TriggerApps   = "apps"
	TriggerTitles = "titles"

	TriggerClose = "close" // close the program or the window
)

// TriggerConsequences are the device actions every trigger group can take.
var TriggerConsequences = []string{ConsequenceCloseBrowser, ConsequenceLockScreen, ConsequenceLogout,
	ConsequenceShutdown, ConsequenceMonk}

// TriggerAction is one action of a trigger group.
type TriggerAction struct {
	Name    string `json:"name"`
	Minutes int    `json:"minutes,omitempty"`
}

// ValidTriggerGroup reports whether group names a trigger group.
func ValidTriggerGroup(group string) bool { return group == TriggerApps || group == TriggerTitles }

// DefaultTriggerActions is what a group does when the user set nothing.
func DefaultTriggerActions(string) []TriggerAction { return []TriggerAction{{Name: TriggerClose}} }

// CheckTriggerAction checks an action for a group.
func CheckTriggerAction(_ string, a TriggerAction) error {
	if a.Name == TriggerClose {
		if a.Minutes != 0 {
			return fmt.Errorf("%s takes no duration", a.Name)
		}
		return nil
	}
	if !slices.Contains(TriggerConsequences, a.Name) {
		return fmt.Errorf("unknown action %q (use %s, %s)", a.Name, TriggerClose, strings.Join(TriggerConsequences, ", "))
	}
	return CheckConsequence(a.Name, a.Minutes)
}

// FindTriggerAction returns the action of that name in actions.
func FindTriggerAction(actions []TriggerAction, name string) (TriggerAction, bool) {
	for _, a := range actions {
		if a.Name == name {
			return a, true
		}
	}
	return TriggerAction{}, false
}

// TriggerActions returns what a block's trigger group does.
func (d *DB) TriggerActions(block, group string) ([]TriggerAction, error) {
	blockID, err := d.getBlockID(block)
	if err != nil {
		return nil, err
	}
	var stored string
	err = d.conn.QueryRow("SELECT actions FROM trigger_actions WHERE block_id = ? AND trigger = ?", blockID, group).Scan(&stored)
	if err == sql.ErrNoRows {
		return DefaultTriggerActions(group), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read the actions of '%s': %w", block, err)
	}
	actions := []TriggerAction{}
	if err := json.Unmarshal([]byte(stored), &actions); err != nil {
		return DefaultTriggerActions(group), nil
	}
	return actions, nil
}

// SetTriggerActions stores what a block's trigger group does; nil goes back
// to the group's default.
func (d *DB) SetTriggerActions(block, group string, actions []TriggerAction) error {
	blockID, err := d.getBlockID(block)
	if err != nil {
		return err
	}
	if actions == nil {
		_, err = d.conn.Exec("DELETE FROM trigger_actions WHERE block_id = ? AND trigger = ?", blockID, group)
	} else {
		data, _ := json.Marshal(actions)
		_, err = d.conn.Exec(`INSERT INTO trigger_actions (block_id, trigger, actions) VALUES (?, ?, ?)
			ON CONFLICT(block_id, trigger) DO UPDATE SET actions = excluded.actions`, blockID, group, string(data))
	}
	if err != nil {
		return fmt.Errorf("failed to save the actions of '%s': %w", block, err)
	}
	return nil
}

// migratePlaceTriggers drops the action sets place rules had for a day
// (2026-11-16: withdrawn, a place rule only enables or lifts its block) and
// what the daemon noted about arriving.
func migratePlaceTriggers(conn *sql.DB) error {
	if _, err := conn.Exec("DELETE FROM trigger_actions WHERE trigger LIKE 'place:%'"); err != nil {
		return err
	}
	_, err := conn.Exec("DELETE FROM settings WHERE key LIKE 'place.here.%'")
	return err
}
