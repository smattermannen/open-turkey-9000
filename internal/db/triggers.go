// triggers.go — what a block's apps, its window titles and each of its
// place rules do when they fire (owner, 2026-11-16; §1 "any trigger to any
// action"): one action set per group, like a list's actions. Without a
// stored set a group does what it always did: apps and titles are closed, a
// place rule enables its block there.
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const triggerActionsSchema = `CREATE TABLE IF NOT EXISTS trigger_actions (
    block_id INTEGER NOT NULL,
    trigger  TEXT NOT NULL,
    actions  TEXT NOT NULL,
    PRIMARY KEY (block_id, trigger),
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`

// The trigger groups, and their own actions.
const (
	TriggerApps   = "apps"
	TriggerTitles = "titles"
	// A place rule's group is PlaceTrigger(place id).

	TriggerClose  = "close"  // apps and titles: close the program or the window
	TriggerEnable = "enable" // a place rule: the block is enabled while there
)

// PlaceTrigger names the trigger group of a place rule.
func PlaceTrigger(placeID int) string { return "place:" + strconv.Itoa(placeID) }

// TriggerConsequences are the device actions every trigger group can take.
var TriggerConsequences = []string{ConsequenceCloseBrowser, ConsequenceLockScreen, ConsequenceLogout,
	ConsequenceShutdown, ConsequenceMonk}

// TriggerAction is one action of a trigger group.
type TriggerAction struct {
	Name    string `json:"name"`
	Minutes int    `json:"minutes,omitempty"`
}

// IsPlaceTrigger reports whether a group is a place rule's.
func IsPlaceTrigger(group string) bool { return strings.HasPrefix(group, "place:") }

// ValidTriggerGroup reports whether group names a trigger group.
func ValidTriggerGroup(group string) bool {
	if group == TriggerApps || group == TriggerTitles {
		return true
	}
	id, err := strconv.Atoi(strings.TrimPrefix(group, "place:"))
	return IsPlaceTrigger(group) && err == nil && id > 0
}

// DefaultTriggerActions is what a group does when the user set nothing.
func DefaultTriggerActions(group string) []TriggerAction {
	if IsPlaceTrigger(group) {
		return []TriggerAction{{Name: TriggerEnable}}
	}
	return []TriggerAction{{Name: TriggerClose}}
}

// CheckTriggerAction checks an action for a group.
func CheckTriggerAction(group string, a TriggerAction) error {
	switch a.Name {
	case TriggerClose:
		if IsPlaceTrigger(group) {
			return fmt.Errorf("a place rule has no %q action (it enables the block: %q)", TriggerClose, TriggerEnable)
		}
	case TriggerEnable:
		if !IsPlaceTrigger(group) {
			return fmt.Errorf("only a place rule has the %q action", TriggerEnable)
		}
	default:
		known := false
		for _, c := range TriggerConsequences {
			known = known || c == a.Name
		}
		if !known {
			own := TriggerClose
			if IsPlaceTrigger(group) {
				own = TriggerEnable
			}
			return fmt.Errorf("unknown action %q (use %s, %s)", a.Name, own, strings.Join(TriggerConsequences, ", "))
		}
		return CheckConsequence(a.Name, a.Minutes)
	}
	if a.Minutes != 0 {
		return fmt.Errorf("%s takes no duration", a.Name)
	}
	return nil
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

// forgetPlaceTriggers drops the action sets of a place's rules.
func (d *DB) forgetPlaceTriggers(placeID int, blockID int) error {
	query, args := "DELETE FROM trigger_actions WHERE trigger = ?", []any{PlaceTrigger(placeID)}
	if blockID > 0 {
		query += " AND block_id = ?"
		args = append(args, blockID)
	}
	_, err := d.conn.Exec(query, args...)
	return err
}
