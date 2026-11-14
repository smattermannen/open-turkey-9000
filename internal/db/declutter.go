// declutter.go — the declutter action was removed (SPEC 2026-11-14: it broke
// pages). Stored ones are dropped on open; imports skip them.
package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// removedActions are action names older versions stored or exported. They
// are dropped, never refused.
var removedActions = []string{"declutter"}

// IsRemovedAction reports whether name is an action that no longer exists,
// so imports can skip it instead of failing.
func IsRemovedAction(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, r := range removedActions {
		if r == name {
			return true
		}
	}
	return false
}

// migrateRemovedActions deletes removed actions from blacklists and phrase
// sets. One left with no action gets block, as a blacklist keeps at least
// one. Runs on every open; finds nothing once done.
func migrateRemovedActions(conn *sql.DB) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range []struct{ table, id, owner string }{
		{"web_list_actions", "list_id", "SELECT id FROM web_lists WHERE kind = '" + ListBlacklist + "'"},
		{"phrase_set_actions", "set_id", "SELECT id FROM phrase_sets"},
	} {
		for _, name := range removedActions {
			ids, err := legacyInts(tx, "SELECT "+t.id+" FROM "+t.table+" WHERE action = ?", name)
			if err != nil {
				return fmt.Errorf("failed to read %s actions: %w", name, err)
			}
			if len(ids) == 0 {
				continue
			}
			if _, err := tx.Exec("DELETE FROM "+t.table+" WHERE action = ?", name); err != nil {
				return fmt.Errorf("failed to remove %s actions: %w", name, err)
			}
			for _, id := range ids {
				if _, err := tx.Exec("INSERT INTO "+t.table+" ("+t.id+", action) SELECT ?, ? "+
					"WHERE ? IN ("+t.owner+") AND NOT EXISTS (SELECT 1 FROM "+t.table+" WHERE "+t.id+" = ?)",
					id, ActionBlock, id, id); err != nil {
					return fmt.Errorf("failed to give block to an emptied list: %w", err)
				}
			}
		}
	}
	return tx.Commit()
}

// legacyInts reads one integer column.
func legacyInts(tx *sql.Tx, query string, args ...any) ([]int, error) {
	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
