package db

import "fmt"

// Block password (§3.11): an optional password per block that hides its
// contents until typed. Viewing only: it changes no lock and nothing that
// is blocked.

// BlockViewPassword returns a block's password hash, or "".
func (d *DB) BlockViewPassword(name string) (string, error) {
	id, err := d.getBlockID(name)
	if err != nil {
		return "", err
	}
	var hash string
	if err := d.conn.QueryRow("SELECT view_password FROM blocks WHERE id = ?", id).Scan(&hash); err != nil {
		return "", fmt.Errorf("failed to read the block password of '%s': %w", name, err)
	}
	return hash, nil
}

// SetBlockViewPassword stores a block's password hash ("" removes it).
func (d *DB) SetBlockViewPassword(name, hash string) error {
	id, err := d.getBlockID(name)
	if err != nil {
		return err
	}
	if _, err := d.conn.Exec("UPDATE blocks SET view_password = ? WHERE id = ?", hash, id); err != nil {
		return fmt.Errorf("failed to save the block password of '%s': %w", name, err)
	}
	return nil
}
