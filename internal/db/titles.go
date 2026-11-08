// titles.go — window-title keywords: a block closes any app whose window
// title contains one of them (see blocker.TitleGuard).
package db

import (
	"fmt"
	"strings"
)

const titlesSchema = `CREATE TABLE IF NOT EXISTS titles (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id INTEGER NOT NULL,
    keyword  TEXT NOT NULL,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`

func (d *DB) getBlockTitles(blockID int) ([]string, error) {
	rows, err := d.conn.Query("SELECT keyword FROM titles WHERE block_id = ? ORDER BY keyword", blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch window titles: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("failed to read a window title: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// AddTitles adds window-title keywords to a block, skipping ones it has.
func (d *DB) AddTitles(blockName string, keywords []string) error {
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return err
	}
	have, err := d.getBlockTitles(blockID)
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
		if _, err := d.conn.Exec("INSERT INTO titles (block_id, keyword) VALUES (?, ?)", blockID, k); err != nil {
			return fmt.Errorf("failed to add window title %q: %w", k, err)
		}
	}
	return nil
}

// RemoveTitles removes window-title keywords from a block.
func (d *DB) RemoveTitles(blockName string, keywords []string) error {
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return err
	}
	for _, k := range keywords {
		if _, err := d.conn.Exec("DELETE FROM titles WHERE block_id = ? AND keyword = ?", blockID, k); err != nil {
			return fmt.Errorf("failed to remove window title %q: %w", k, err)
		}
	}
	return nil
}
