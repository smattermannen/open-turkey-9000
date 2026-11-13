// testmode.go — test mode (§3.2): a list in test mode reports what it would
// match in a page's title and text instead of acting on it, so thresholds
// can be tuned. Set per list; stored as a JSON list of list IDs in the
// setting "listtest". Before, it was set per block ("keywordtest"); those
// blocks' lists are moved over once.
package db

import (
	"encoding/json"
	"slices"
)

const (
	settingKeywordTest = "keywordtest" // the old per-block setting
	settingListTest    = "listtest"
)

// AccessPasswordSetting + area is the setting holding an access password's
// hash (see cli/access.go).
const AccessPasswordSetting = "access.password."

// TestLists returns the IDs of the lists in test mode.
func (d *DB) TestLists() []int {
	d.migrateTestBlocks()
	raw, err := d.GetSetting(settingListTest)
	if err != nil || raw == "" {
		return nil
	}
	var ids []int
	_ = json.Unmarshal([]byte(raw), &ids)
	return ids
}

// SetTestList turns test mode on or off for a list.
func (d *DB) SetTestList(id int, on bool) error {
	ids := slices.DeleteFunc(d.TestLists(), func(n int) bool { return n == id })
	if on {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	raw, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	return d.SetSetting(settingListTest, string(raw))
}

// migrateTestBlocks moves the old per-block test mode to the block's lists
// that read the title or text, once.
func (d *DB) migrateTestBlocks() {
	raw, err := d.GetSetting(settingKeywordTest)
	if err != nil || raw == "" {
		return
	}
	var names []string
	_ = json.Unmarshal([]byte(raw), &names)
	var ids []int
	if cur, _ := d.GetSetting(settingListTest); cur != "" {
		_ = json.Unmarshal([]byte(cur), &ids)
	}
	for _, name := range names {
		b, err := d.GetBlock(name)
		if err != nil {
			continue
		}
		for _, l := range b.Lists {
			if (l.Where.Title || l.Where.Text) && !slices.Contains(ids, l.ID) {
				ids = append(ids, l.ID)
			}
		}
	}
	slices.Sort(ids)
	if out, err := json.Marshal(ids); err == nil && d.SetSetting(settingListTest, string(out)) == nil {
		_ = d.SetSetting(settingKeywordTest, "")
	}
}
