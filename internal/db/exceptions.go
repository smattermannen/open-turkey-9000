package db

import "fmt"

const exceptionsSchema = `CREATE TABLE IF NOT EXISTS exceptions (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id INTEGER NOT NULL,
    pattern  TEXT NOT NULL,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`

func (d *DB) getBlockExceptions(blockID int) ([]string, error) {
	rows, err := d.conn.Query("SELECT pattern FROM exceptions WHERE block_id = ? ORDER BY pattern", blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block exceptions: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("failed to read exception: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AddExceptions adds exceptions to a block, skipping ones it already has.
func (d *DB) AddExceptions(blockName string, exceptions []string) error {
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return err
	}
	have, err := d.getBlockExceptions(blockID)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(have))
	for _, e := range have {
		seen[e] = true
	}
	for _, e := range exceptions {
		if seen[e] {
			continue
		}
		seen[e] = true
		if _, err := d.conn.Exec("INSERT INTO exceptions (block_id, pattern) VALUES (?, ?)", blockID, e); err != nil {
			return fmt.Errorf("failed to add exception '%s': %w", e, err)
		}
	}
	return nil
}

// RemoveExceptions removes exceptions from a block.
func (d *DB) RemoveExceptions(blockName string, exceptions []string) error {
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return err
	}
	for _, e := range exceptions {
		result, err := d.conn.Exec("DELETE FROM exceptions WHERE block_id = ? AND pattern = ?", blockID, e)
		if err != nil {
			return fmt.Errorf("failed to remove exception '%s': %w", e, err)
		}
		if n, _ := result.RowsAffected(); n == 0 {
			return fmt.Errorf("exception '%s' not found in block '%s'", e, blockName)
		}
	}
	return nil
}
