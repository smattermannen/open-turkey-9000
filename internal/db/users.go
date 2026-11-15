// users.go — affected users (§3.2): the local accounts a block applies to.
// None listed: every account.
package db

import (
	"fmt"
	"slices"
	"strings"
)

func splitUsers(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// BlockUsers returns the accounts a block applies to; nil: all.
func (d *DB) BlockUsers(name string) ([]string, error) {
	id, err := d.getBlockID(name)
	if err != nil {
		return nil, err
	}
	var users string
	if err := d.conn.QueryRow("SELECT users FROM blocks WHERE id = ?", id).Scan(&users); err != nil {
		return nil, fmt.Errorf("failed to read the users of '%s': %w", name, err)
	}
	return splitUsers(users), nil
}

// SetBlockUsers sets the accounts a block applies to; none: all.
func (d *DB) SetBlockUsers(name string, users []string) error {
	id, err := d.getBlockID(name)
	if err != nil {
		return err
	}
	users = slices.Clone(users)
	slices.Sort(users)
	users = slices.Compact(users)
	if _, err := d.conn.Exec("UPDATE blocks SET users = ? WHERE id = ?", strings.Join(users, ","), id); err != nil {
		return fmt.Errorf("failed to set the users of '%s': %w", name, err)
	}
	return nil
}

// AppliesTo reports whether a block applies to user ("" unknown: yes).
func (b ActiveBlockDetail) AppliesTo(user string) bool {
	return len(b.Users) == 0 || user == "" || slices.Contains(b.Users, user)
}
