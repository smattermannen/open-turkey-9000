// pending.go — queued edits that loosen a block.
//
// A block with allow windows can't be loosened by a plain edit at any time
// (otherwise its own allow window would be the moment to dismantle it). The
// CLI instead queues the edit here with a time to apply it, and the daemon
// applies it once that time comes (see enforce.Tick). Queued edits can be
// listed and cancelled; they are deleted with their block.
package db

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/smattermannen/kc9k/internal/schedule"
)

// Edit actions that can be queued.
const (
	EditRemoveApps     = "remove-app"
	EditRemoveTitles   = "remove-title"
	EditRemoveBlock    = "remove-block"
	EditAddSchedule    = "add-schedule"
	EditRemoveSchedule = "remove-schedule"

	// Edits to a website list, named by List (its id).
	EditListRemove       = "list-remove"        // Values: entries, from a blacklist
	EditListAdd          = "list-add"           // Values: entries, to an exception list
	EditListWhere        = "list-where"         // Values: place, "on" or "off"
	EditListUnsetAction  = "list-unset-action"  // Values: action
	EditListSetThreshold = "list-set-threshold" // Values: action, number
	EditListDelete       = "list-delete"        // a blacklist
)

// Edits queued before website lists. They still apply, to the lists made from
// the old ones (see migrateWebLists).
const (
	legacyAddSites       = "add-site" // only queued for allowlist blocks: an allowed site
	legacyRemoveSites    = "remove-site"
	legacyRemovePatterns = "remove-pattern"
	legacyAddExceptions  = "add-exception"
	legacySetMode        = "set-mode"
	legacyUnsetAction    = "unset-action"
	legacyRemoveKeywords = "remove-keyword"
	legacySetThreshold   = "set-threshold"
)

// Edit is one change to a block. Values holds the apps, titles or list
// entries (or what the action's comment says); Window the schedule, for the
// schedule actions; List and ListName the list, for list edits.
type Edit struct {
	Action   string          `json:"-"`
	Values   []string        `json:"values,omitempty"`
	Window   schedule.Window `json:"window"`
	List     int             `json:"list,omitempty"`
	ListName string          `json:"list_name,omitempty"`
}

// String describes the edit for listings, e.g. "remove-app steam".
func (e Edit) String() string {
	switch e.Action {
	case EditRemoveBlock:
		return "remove the block"
	case EditAddSchedule, EditRemoveSchedule:
		return fmt.Sprintf("%s %s", e.Action, e.Window)
	case EditListDelete:
		return fmt.Sprintf("delete the list %q", e.ListName)
	}
	s := e.Action
	if e.ListName != "" {
		s += fmt.Sprintf(" %q", e.ListName)
	}
	for _, v := range e.Values {
		s += " " + v
	}
	return s
}

// PendingEdit is a queued edit. Num is its 1-based position in the queue
// (ordered by when it applies), as shown by "pending list"; ApplyAt is unix
// seconds.
type PendingEdit struct {
	ID        int
	Num       int
	BlockName string
	Edit      Edit
	ApplyAt   int64
}

// QueueEdit queues an edit to a block, to be applied at applyAt (unix seconds).
func (d *DB) QueueEdit(blockName string, e Edit, applyAt int64) error {
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("failed to encode edit: %w", err)
	}
	_, err = d.conn.Exec(
		"INSERT INTO pending_edits (block_id, action, payload, apply_at) VALUES (?, ?, ?, ?)",
		blockID, e.Action, string(payload), applyAt,
	)
	if err != nil {
		return fmt.Errorf("failed to queue edit to block '%s': %w", blockName, err)
	}
	return nil
}

// ListPendingEdits returns every queued edit, soonest first.
func (d *DB) ListPendingEdits() ([]PendingEdit, error) {
	rows, err := d.conn.Query(`
		SELECT p.id, b.name, p.action, p.payload, p.apply_at
		FROM pending_edits p
		JOIN blocks b ON b.id = p.block_id
		ORDER BY p.apply_at, p.id
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending edits: %w", err)
	}
	defer rows.Close()

	var out []PendingEdit
	for rows.Next() {
		p := PendingEdit{Num: len(out) + 1}
		var payload string
		if err := rows.Scan(&p.ID, &p.BlockName, &p.Edit.Action, &payload, &p.ApplyAt); err != nil {
			return nil, fmt.Errorf("failed to read pending edit: %w", err)
		}
		if err := json.Unmarshal([]byte(payload), &p.Edit); err != nil {
			return nil, fmt.Errorf("failed to decode pending edit %d: %w", p.ID, err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeletePendingEdit removes a queued edit by its row ID.
func (d *DB) DeletePendingEdit(id int) error {
	if _, err := d.conn.Exec("DELETE FROM pending_edits WHERE id = ?", id); err != nil {
		return fmt.Errorf("failed to delete pending edit: %w", err)
	}
	return nil
}

// ApplyEdit performs an edit on a block. Removing a block deactivates it
// first; any other edit that leaves an active block with nothing to enforce
// deactivates it, as the CLI does.
func (d *DB) ApplyEdit(blockName string, e Edit) error {
	var err error
	switch e.Action {
	case EditRemoveApps:
		err = d.RemoveApps(blockName, e.Values)
	case EditRemoveTitles:
		err = d.RemoveTitles(blockName, e.Values)
	case EditListRemove, EditListAdd, EditListWhere, EditListUnsetAction, EditListSetThreshold, EditListDelete:
		err = d.applyListEdit(e)
	case legacyAddSites, legacyRemoveSites, legacyRemovePatterns, legacyAddExceptions,
		legacySetMode, legacyUnsetAction, legacyRemoveKeywords, legacySetThreshold:
		err = d.applyLegacyEdit(blockName, e)
	case EditAddSchedule:
		err = d.AddSchedule(blockName, e.Window)
	case EditRemoveSchedule:
		err = d.removeScheduleWindow(blockName, e.Window)
	case EditRemoveBlock:
		active, err := d.IsBlockActive(blockName)
		if err != nil {
			return err
		}
		if active {
			if err := d.DeactivateBlock(blockName); err != nil {
				return err
			}
		}
		return d.RemoveBlock(blockName)
	default:
		return fmt.Errorf("unknown edit action %q", e.Action)
	}
	if err != nil {
		return err
	}

	detail, err := d.GetBlock(blockName)
	if err != nil {
		return err
	}
	if detail.Active && detail.Empty() {
		return d.DeactivateBlock(blockName)
	}
	return nil
}

// applyListEdit performs an edit to one website list.
func (d *DB) applyListEdit(e Edit) error {
	list, err := d.GetList(e.List)
	if err != nil {
		return err
	}
	switch e.Action {
	case EditListRemove:
		return d.RemoveListEntries(list.ID, e.Values)
	case EditListAdd:
		return d.AddListEntries(list.ID, e.Values)
	case EditListWhere:
		if len(e.Values) != 2 || !IsPlace(e.Values[0]) {
			return fmt.Errorf("list-where needs a place and on or off")
		}
		return d.SetListWhere(list.ID, list.Where.With(e.Values[0], e.Values[1] == "on"))
	case EditListUnsetAction:
		if len(e.Values) != 1 {
			return fmt.Errorf("list-unset-action needs exactly one action")
		}
		return d.SetListAction(list.ID, e.Values[0], false)
	case EditListSetThreshold:
		if len(e.Values) != 2 {
			return fmt.Errorf("list-set-threshold needs an action and a number")
		}
		n, err := strconv.Atoi(e.Values[1])
		if err != nil {
			return fmt.Errorf("bad threshold %q", e.Values[1])
		}
		return d.SetListThreshold(list.ID, e.Values[0], n)
	case EditListDelete:
		return d.DeleteList(list.ID)
	}
	return fmt.Errorf("unknown list edit %q", e.Action)
}

// applyLegacyEdit performs an edit queued before website lists on the lists
// made from the old ones.
func (d *DB) applyLegacyEdit(blockName string, e Edit) error {
	listFor := func(name string) (int, error) { return d.ListID(blockName, name) }
	switch e.Action {
	case legacySetMode:
		return fmt.Errorf("blocks no longer have modes")
	case legacyAddSites, legacyAddExceptions:
		id, err := listFor(DefaultExceptionsList)
		if err != nil {
			return err
		}
		return d.AddListEntries(id, e.Values)
	case legacyRemoveSites, legacyRemovePatterns:
		id, err := listFor(DefaultBlockedList)
		if err != nil {
			return err
		}
		return d.RemoveListEntries(id, e.Values)
	case legacyRemoveKeywords:
		id, err := listFor(DefaultWordsList)
		if err != nil {
			return err
		}
		return d.RemoveListEntries(id, e.Values)
	case legacySetThreshold:
		id, err := listFor(DefaultWordsList)
		if err != nil {
			return err
		}
		return d.applyListEdit(Edit{Action: EditListSetThreshold, List: id, Values: e.Values})
	case legacyUnsetAction:
		detail, err := d.GetBlock(blockName)
		if err != nil {
			return err
		}
		if len(e.Values) != 1 {
			return fmt.Errorf("unset-action needs exactly one action")
		}
		for _, l := range detail.Lists.Blacklists() {
			if err := d.SetListAction(l.ID, e.Values[0], false); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("unknown edit action %q", e.Action)
}
