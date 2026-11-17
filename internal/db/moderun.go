package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// ModeRun is a mode of blocks run by a Mode action (§3.9, R-MODE-4; owner
// 2026-11-17): its blocks enforce until Until (trusted time), locked,
// whatever its automation says; the blocks that were off are switched off
// again when it ends. Monk mode's and Frozen Chicken's runs are their own
// (SettingMonkUntil, the freeze).
type ModeRun struct {
	Mode      string   `json:"mode"`
	Until     int64    `json:"until"`
	OffBefore []string `json:"off_before"`
}

const settingModeRuns = "mode.runs"

// ModeRuns returns the runs stored, ended ones too until the daemon ends
// them.
func (d *DB) ModeRuns() []ModeRun {
	raw, err := d.GetSetting(settingModeRuns)
	if err != nil || raw == "" {
		return nil
	}
	var runs []ModeRun
	if json.Unmarshal([]byte(raw), &runs) != nil {
		return nil
	}
	return runs
}

// SaveModeRuns stores the runs.
func (d *DB) SaveModeRuns(runs []ModeRun) error {
	if len(runs) == 0 {
		return d.SetSetting(settingModeRuns, "")
	}
	raw, err := json.Marshal(runs)
	if err != nil {
		return err
	}
	return d.SetSetting(settingModeRuns, string(raw))
}

// ModeRunOf returns the run of a mode, by name.
func ModeRunOf(runs []ModeRun, mode string) (int, bool) {
	for i, r := range runs {
		if strings.EqualFold(r.Mode, mode) {
			return i, true
		}
	}
	return -1, false
}

// ModeCustom returns what a mode opens or runs when it becomes enabled (§3.9
// Custom action; owner 2026-11-17), as a custom action's parameter; "" when
// nothing.
func (d *DB) ModeCustom(name string) (string, error) {
	var param string
	err := d.conn.QueryRow("SELECT custom FROM modes WHERE name = ? COLLATE NOCASE", name).Scan(&param)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("no mode named '%s'", name)
	}
	if err != nil {
		return "", err
	}
	return param, nil
}

// SetModeCustom stores what a mode opens or runs when it becomes enabled
// ("" for nothing).
func (d *DB) SetModeCustom(name, param string) error {
	if param != "" {
		if err := CheckCustomParam(param); err != nil {
			return err
		}
	}
	res, err := d.conn.Exec("UPDATE modes SET custom = ? WHERE name = ? COLLATE NOCASE", param, name)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("no mode named '%s'", name)
	}
	return nil
}
