// pagekeywords.go — keywords looked for in a page's text. A block acts on a
// page (blocks it, greys it, hides its images) once the page contains enough
// of the block's distinct keywords: its threshold for that action.
package db

import (
	"fmt"
	"strings"
)

const pageKeywordsSchema = `CREATE TABLE IF NOT EXISTS page_keywords (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id INTEGER NOT NULL,
    keyword  TEXT NOT NULL,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS keyword_thresholds (
    block_id INTEGER NOT NULL,
    action   TEXT NOT NULL,
    count    INTEGER NOT NULL,
    PRIMARY KEY (block_id, action),
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`

// MaxThreshold caps a keyword threshold; a block can't have more keywords
// than this be useful anyway.
const MaxThreshold = 1000

// Thresholds holds, per website action, how many of a block's distinct page
// keywords a page must contain before the action applies to it.
type Thresholds struct {
	Block     int `json:"block"`
	Greyscale int `json:"greyscale"`
	NoImages  int `json:"no-images"`
}

// DefaultThresholds act on the first keyword found.
var DefaultThresholds = Thresholds{Block: 1, Greyscale: 1, NoImages: 1}

// Get returns the threshold for the named action.
func (t Thresholds) Get(action string) int {
	switch action {
	case ActionBlock:
		return t.Block
	case ActionGreyscale:
		return t.Greyscale
	case ActionNoImages:
		return t.NoImages
	}
	return 0
}

// With returns a copy with the named action's threshold set to n.
func (t Thresholds) With(action string, n int) Thresholds {
	switch action {
	case ActionBlock:
		t.Block = n
	case ActionGreyscale:
		t.Greyscale = n
	case ActionNoImages:
		t.NoImages = n
	}
	return t
}

func (d *DB) getBlockPageKeywords(blockID int) ([]string, error) {
	rows, err := d.conn.Query("SELECT keyword FROM page_keywords WHERE block_id = ? ORDER BY keyword", blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page keywords: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("failed to read a page keyword: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (d *DB) getBlockThresholds(blockID int) (Thresholds, error) {
	t := DefaultThresholds
	rows, err := d.conn.Query("SELECT action, count FROM keyword_thresholds WHERE block_id = ?", blockID)
	if err != nil {
		return t, fmt.Errorf("failed to fetch keyword thresholds: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var action string
		var n int
		if err := rows.Scan(&action, &n); err != nil {
			return t, fmt.Errorf("failed to read a keyword threshold: %w", err)
		}
		if IsAction(action) && n >= 1 {
			t = t.With(action, n)
		}
	}
	return t, rows.Err()
}

// AddPageKeywords adds page keywords to a block, skipping ones it has
// (ignoring case).
func (d *DB) AddPageKeywords(blockName string, keywords []string) error {
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return err
	}
	have, err := d.getBlockPageKeywords(blockID)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, k := range have {
		seen[strings.ToLower(k)] = true
	}
	for _, k := range keywords {
		k = strings.TrimSpace(k)
		if k == "" || seen[strings.ToLower(k)] {
			continue
		}
		seen[strings.ToLower(k)] = true
		if _, err := d.conn.Exec("INSERT INTO page_keywords (block_id, keyword) VALUES (?, ?)", blockID, k); err != nil {
			return fmt.Errorf("failed to add page keyword %q: %w", k, err)
		}
	}
	return nil
}

// RemovePageKeywords removes page keywords from a block.
func (d *DB) RemovePageKeywords(blockName string, keywords []string) error {
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return err
	}
	for _, k := range keywords {
		if _, err := d.conn.Exec("DELETE FROM page_keywords WHERE block_id = ? AND keyword = ?", blockID, k); err != nil {
			return fmt.Errorf("failed to remove page keyword %q: %w", k, err)
		}
	}
	return nil
}

// SetThreshold sets how many distinct page keywords trigger an action.
func (d *DB) SetThreshold(blockName, action string, n int) error {
	if !IsAction(action) {
		return fmt.Errorf("unknown action %q (use %s)", action, strings.Join(ActionNames, ", "))
	}
	if n < 1 || n > MaxThreshold {
		return fmt.Errorf("a threshold is a number from 1 to %d", MaxThreshold)
	}
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec(`INSERT INTO keyword_thresholds (block_id, action, count) VALUES (?, ?, ?)
		ON CONFLICT(block_id, action) DO UPDATE SET count = excluded.count`, blockID, action, n)
	if err != nil {
		return fmt.Errorf("failed to set the %s threshold: %w", action, err)
	}
	return nil
}
