package db

import "fmt"

// Templates (§3.2): ready-made block definitions the user saved, as block
// export JSON. KC9K ships none (decision 2026-11-14: no categories).
const templatesSchema = `CREATE TABLE IF NOT EXISTS templates (
    name TEXT PRIMARY KEY COLLATE NOCASE,
    data TEXT NOT NULL
);`

// SaveTemplate stores (or replaces) a template.
func (d *DB) SaveTemplate(name, data string) error {
	if _, err := d.conn.Exec("INSERT OR REPLACE INTO templates (name, data) VALUES (?, ?)", name, data); err != nil {
		return fmt.Errorf("failed to save block preset '%s': %w", name, err)
	}
	return nil
}

// Template returns a template's data.
func (d *DB) Template(name string) (string, error) {
	var data string
	if err := d.conn.QueryRow("SELECT data FROM templates WHERE name = ?", name).Scan(&data); err != nil {
		return "", fmt.Errorf("no block preset '%s' (see 'kc9k block-preset')", name)
	}
	return data, nil
}

// Templates lists template names.
func (d *DB) Templates() ([]string, error) {
	rows, err := d.conn.Query("SELECT name FROM templates ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// RemoveTemplate deletes a template.
func (d *DB) RemoveTemplate(name string) error {
	res, err := d.conn.Exec("DELETE FROM templates WHERE name = ?", name)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("no block preset '%s'", name)
	}
	return nil
}
