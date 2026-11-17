package db

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Presets (§3.2 Preset, §4.4; owner 2026-11-17): saved copies outside
// blocks, never enforced. List and phrase set presets are kept as clusters
// (clusters.go), block presets as templates (templates.go); the other types
// are kept here, as JSON. Using one copies it; saving one copies too.
const presetsSchema = `CREATE TABLE IF NOT EXISTS presets (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    kind TEXT NOT NULL,
    name TEXT NOT NULL COLLATE NOCASE,
    data TEXT NOT NULL,
    UNIQUE (kind, name)
);`

// Kinds of presets kept here.
const (
	PresetLists      = "lists"      // a block's lists (blacklists, exceptions or both), with or without actions
	PresetAutomation = "automation" // windows, schedules, places and settings (AutomationSpec)
	PresetSchedule   = "schedule"   // weekly windows
	PresetHotkey     = "hotkey"     // a hotkey: keys and actions
)

// PresetKinds are the kinds kept here, in the order the Presets page shows
// them.
var PresetKinds = []string{PresetLists, PresetAutomation, PresetSchedule, PresetHotkey}

// Preset is one preset kept here.
type Preset struct {
	ID   int             `json:"id"`
	Kind string          `json:"kind"`
	Name string          `json:"name"`
	Data json.RawMessage `json:"data"`
}

// ListsPreset is what a lists preset holds: lists as a block has them.
type ListsPreset struct {
	Lists   []WebList `json:"lists"`
	Actions bool      `json:"actions"` // the lists' actions were saved too
}

// SavePreset stores a preset, replacing one of the kind and name.
func (d *DB) SavePreset(kind, name string, data any) error {
	if err := CheckListName(name); err != nil {
		return err
	}
	if !validPresetKind(kind) {
		return fmt.Errorf("no kind of preset %q", kind)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec(`INSERT INTO presets (kind, name, data) VALUES (?, ?, ?)
		ON CONFLICT(kind, name) DO UPDATE SET data = excluded.data`, kind, name, string(raw))
	if err != nil {
		return fmt.Errorf("failed to save the preset '%s': %w", name, err)
	}
	return nil
}

func validPresetKind(kind string) bool {
	for _, k := range PresetKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// Presets returns the presets kept here, of a kind ("" for all), by kind
// and name.
func (d *DB) Presets(kind string) ([]Preset, error) {
	rows, err := d.conn.Query("SELECT id, kind, name, data FROM presets WHERE ? = '' OR kind = ? ORDER BY kind, name", kind, kind)
	if err != nil {
		return nil, fmt.Errorf("failed to read presets: %w", err)
	}
	defer rows.Close()
	out := []Preset{}
	for rows.Next() {
		var p Preset
		var data string
		if err := rows.Scan(&p.ID, &p.Kind, &p.Name, &data); err != nil {
			return nil, err
		}
		p.Data = json.RawMessage(data)
		out = append(out, p)
	}
	return out, rows.Err()
}

// FindPreset returns a preset by name (case ignored), of a kind or of any
// kind kept here when kind is "".
func (d *DB) FindPreset(kind, name string) (Preset, error) {
	all, err := d.Presets(kind)
	if err != nil {
		return Preset{}, err
	}
	var found []Preset
	for _, p := range all {
		if strings.EqualFold(p.Name, name) {
			found = append(found, p)
		}
	}
	switch len(found) {
	case 0:
		return Preset{}, fmt.Errorf("no preset named '%s'", name)
	case 1:
		return found[0], nil
	}
	return Preset{}, fmt.Errorf("presets of several kinds are named '%s': say which with --kind", name)
}

// RenamePreset renames a preset kept here.
func (d *DB) RenamePreset(id int, name string) error {
	if err := CheckListName(name); err != nil {
		return err
	}
	if _, err := d.conn.Exec("UPDATE presets SET name = ? WHERE id = ?", name, id); err != nil {
		return fmt.Errorf("failed to rename the preset: %w", err)
	}
	return nil
}

// DeletePreset deletes a preset kept here.
func (d *DB) DeletePreset(id int) error {
	_, err := d.conn.Exec("DELETE FROM presets WHERE id = ?", id)
	return err
}
