// testmode.go — keyword test mode: for the blocks named here, the extension
// shows what their title and text lists find on each page instead of acting
// on it, so thresholds can be tuned. Stored as a JSON list of block names in
// the setting "keywordtest".
package db

import (
	"encoding/json"
	"slices"
)

const settingKeywordTest = "keywordtest"

// AccessPasswordSetting + area is the setting holding an access password's
// hash (see cli/access.go).
const AccessPasswordSetting = "access.password."

// TestBlocks returns the blocks in keyword test mode.
func (d *DB) TestBlocks() []string {
	raw, err := d.GetSetting(settingKeywordTest)
	if err != nil || raw == "" {
		return nil
	}
	var names []string
	_ = json.Unmarshal([]byte(raw), &names)
	return names
}

// SetTestBlock turns keyword test mode on or off for a block.
func (d *DB) SetTestBlock(name string, on bool) error {
	names := slices.DeleteFunc(d.TestBlocks(), func(n string) bool { return n == name })
	if on {
		names = append(names, name)
	}
	raw, err := json.Marshal(names)
	if err != nil {
		return err
	}
	return d.SetSetting(settingKeywordTest, string(raw))
}
