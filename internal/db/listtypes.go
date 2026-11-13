// listtypes.go — splitting the lists from before list types (§3.2).
//
// A list used to tick the address, the title and the text freely. Now it is
// an address list or a term list. A list that looked at the address and at
// the page becomes two, once: the address list keeps its name, the id and
// the url tick; a new term list "<name> terms" gets the title and text ticks
// with the same entries and actions (and test mode). Together they match
// exactly what the one list did. Clusters are split the same way.
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

func migrateListTypes(conn *sql.DB) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	type mixed struct {
		id, owner   int
		kind, name  string
		title, text bool
	}
	read := func(query string) ([]mixed, error) {
		rows, err := tx.Query(query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []mixed
		for rows.Next() {
			var m mixed
			if err := rows.Scan(&m.id, &m.owner, &m.kind, &m.name, &m.title, &m.text); err != nil {
				return nil, err
			}
			out = append(out, m)
		}
		return out, rows.Err()
	}
	names := func(query string, args ...any) (map[string]bool, error) {
		rows, err := tx.Query(query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		taken := map[string]bool{}
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				return nil, err
			}
			taken[strings.ToLower(n)] = true
		}
		return taken, rows.Err()
	}

	lists, err := read(`SELECT id, block_id, kind, name, in_title, in_text FROM web_lists
		WHERE in_url = 1 AND (in_title = 1 OR in_text = 1) ORDER BY id`)
	if err != nil {
		return fmt.Errorf("list types: %w", err)
	}
	var testing []int
	if lists != nil {
		var raw string
		if err := tx.QueryRow("SELECT value FROM settings WHERE key = ?", settingListTest).Scan(&raw); err == nil && raw != "" {
			_ = json.Unmarshal([]byte(raw), &testing)
		}
	}
	for _, l := range lists {
		taken, err := names("SELECT name FROM web_lists WHERE block_id = ?", l.owner)
		if err != nil {
			return err
		}
		res, err := tx.Exec("INSERT INTO web_lists (block_id, kind, name, in_url, in_title, in_text) VALUES (?, ?, ?, 0, ?, ?)",
			l.owner, l.kind, freeListName(l.name+" terms", taken), l.title, l.text)
		if err != nil {
			return fmt.Errorf("list types: %w", err)
		}
		id64, _ := res.LastInsertId()
		id := int(id64)
		for _, q := range []string{
			"INSERT INTO web_list_entries (list_id, entry) SELECT ?, entry FROM web_list_entries WHERE list_id = ? ORDER BY id",
			`INSERT INTO web_list_actions (list_id, action, distinct_min, hits_min, param, minutes)
				SELECT ?, action, distinct_min, hits_min, param, minutes FROM web_list_actions WHERE list_id = ?`,
		} {
			if _, err := tx.Exec(q, id, l.id); err != nil {
				return fmt.Errorf("list types: %w", err)
			}
		}
		if _, err := tx.Exec("UPDATE web_lists SET in_title = 0, in_text = 0 WHERE id = ?", l.id); err != nil {
			return err
		}
		if slices.Contains(testing, l.id) {
			testing = append(testing, id)
		}
	}
	if len(lists) > 0 && testing != nil {
		raw, _ := json.Marshal(testing)
		if _, err := tx.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value`, settingListTest, string(raw)); err != nil {
			return err
		}
	}

	clusters, err := read(`SELECT id, 0, '', name, in_title, in_text FROM clusters
		WHERE in_url = 1 AND (in_title = 1 OR in_text = 1) ORDER BY id`)
	if err != nil {
		return fmt.Errorf("list types: %w", err)
	}
	for _, c := range clusters {
		taken, err := names("SELECT name FROM clusters")
		if err != nil {
			return err
		}
		res, err := tx.Exec("INSERT INTO clusters (name, in_url, in_title, in_text) VALUES (?, 0, ?, ?)",
			freeListName(c.name+" terms", taken), c.title, c.text)
		if err != nil {
			return fmt.Errorf("list types: %w", err)
		}
		id, _ := res.LastInsertId()
		if _, err := tx.Exec("INSERT INTO cluster_entries (cluster_id, entry) SELECT ?, entry FROM cluster_entries WHERE cluster_id = ? ORDER BY id",
			id, c.id); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE clusters SET in_title = 0, in_text = 0 WHERE id = ?", c.id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
