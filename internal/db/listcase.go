// listcase.go — lists of one block whose names differ only in case.
//
// Lists are found by name without regard to case, so of two such lists one
// was unreachable. New ones are refused (listNameTaken); on open, the later
// of an existing pair (higher id) is renamed "<name>-2" (or -3, … when taken).
package db

import (
	"fmt"
	"log"
	"strings"
)

func migrateListCaseNames(q querier) error {
	rows, err := q.Query("SELECT id, block_id, name FROM web_lists ORDER BY block_id, id")
	if err != nil {
		return fmt.Errorf("failed to read list names: %w", err)
	}
	type row struct {
		id, block int
		name      string
	}
	var all []row
	taken := map[int]map[string]bool{} // block -> lowercase names in use
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.block, &r.name); err != nil {
			rows.Close()
			return err
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, r := range all {
		if taken[r.block] == nil {
			taken[r.block] = map[string]bool{}
		}
		taken[r.block][strings.ToLower(r.name)] = true
	}
	seen := map[int]map[string]bool{}
	for _, r := range all {
		if seen[r.block] == nil {
			seen[r.block] = map[string]bool{}
		}
		low := strings.ToLower(r.name)
		if !seen[r.block][low] {
			seen[r.block][low] = true
			continue
		}
		var name string
		for n := 2; ; n++ {
			suffix := fmt.Sprintf("-%d", n)
			base := r.name
			if len(base)+len(suffix) > MaxListName {
				base = base[:MaxListName-len(suffix)]
			}
			name = base + suffix
			if !taken[r.block][strings.ToLower(name)] {
				break
			}
		}
		if _, err := q.Exec("UPDATE web_lists SET name = ? WHERE id = ?", name, r.id); err != nil {
			return fmt.Errorf("failed to rename list '%s': %w", r.name, err)
		}
		taken[r.block][strings.ToLower(name)] = true
		seen[r.block][strings.ToLower(name)] = true
		log.Printf("list '%s' renamed '%s': another list of the block had the same name in other case", r.name, name)
	}
	return nil
}
