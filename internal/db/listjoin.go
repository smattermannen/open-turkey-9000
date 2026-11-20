// listjoin.go — rejoining lists split by list types (§3.2).
//
// Lists had types for a while: an address list looked only at the url, a
// term list at the title and/or text. An upgrade then split each list that
// looked at both into "<name>" (url) and "<name> terms" (title/text, same
// entries and actions). Lists have no types now, so such a pair is joined
// back into "<name>" looking at the url and where the terms list looked,
// when that changes nothing about what they match or do:
//   - same block and kind; "<name>" looks only at the url, "<name> terms"
//     (that exact name, in any case) not at the url;
//   - the same entries (ignoring case and order);
//   - for blacklists, the same actions with the same threshold, hits,
//     parameter and minutes;
//   - neither in test mode (a joined list in test mode would stop acting on
//     addresses; one out of it would act on terms that were only reported);
//   - no queued edit and no recording names "<name> terms".
//
// "<name>" keeps its id, so queued edits and test mode follow it; "<name>
// terms" is removed. Clusters are joined the same way (no actions, no test
// mode). Pairs that differ stay as two lists. It runs at every open and
// writes nothing once no pair is left.
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"
)

// joinable is a list or cluster as the join compares it.
type joinable struct {
	id, owner  int
	kind, name string
	where      Where
	entries    []string // lower case, sorted
	actions    []ListAction
}

// joinPair is one pair to join: url keeps its id, terms goes.
type joinPair struct{ url, terms joinable }

func migrateJoinLists(conn *sql.DB) error {
	// Find the pairs first without a transaction: a transaction that reads
	// and then writes fails at once with "database is locked" when another
	// command wrote in between, so write only when there is something to do.
	lists, clusters, err := findJoinPairs(conn)
	if err != nil {
		return fmt.Errorf("joining lists: %w", err)
	}
	if len(lists)+len(clusters) == 0 {
		return nil
	}
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if lists, clusters, err = findJoinPairs(tx); err != nil {
		return fmt.Errorf("joining lists: %w", err)
	}
	for _, p := range lists {
		if _, err := tx.Exec("UPDATE web_lists SET in_title = ?, in_text = ? WHERE id = ?",
			p.terms.where.Title, p.terms.where.Text, p.url.id); err != nil {
			return fmt.Errorf("joining lists: %w", err)
		}
		for _, q := range []string{
			"DELETE FROM web_list_entries WHERE list_id = ?",
			"DELETE FROM web_list_actions WHERE list_id = ?",
			"DELETE FROM web_lists WHERE id = ?",
		} {
			if _, err := tx.Exec(q, p.terms.id); err != nil {
				return fmt.Errorf("joining lists: %w", err)
			}
		}
		log.Printf("lists '%s' and '%s' joined into '%s' (looks at: %s)", p.url.name, p.terms.name, p.url.name,
			Where{URL: true, Title: p.terms.where.Title, Text: p.terms.where.Text})
	}
	for _, p := range clusters {
		if _, err := tx.Exec("UPDATE clusters SET in_title = ?, in_text = ? WHERE id = ?",
			p.terms.where.Title, p.terms.where.Text, p.url.id); err != nil {
			return fmt.Errorf("joining list presets: %w", err)
		}
		for _, q := range []string{
			"DELETE FROM cluster_entries WHERE cluster_id = ?",
			"DELETE FROM clusters WHERE id = ?",
		} {
			if _, err := tx.Exec(q, p.terms.id); err != nil {
				return fmt.Errorf("joining list presets: %w", err)
			}
		}
		log.Printf("list presets '%s' and '%s' joined into '%s'", p.url.name, p.terms.name, p.url.name)
	}
	return tx.Commit()
}

// findJoinPairs returns the list pairs and cluster pairs that can be joined
// without changing anything.
func findJoinPairs(q querier) (lists, clusters []joinPair, err error) {
	busyLists, busyClusters, err := joinBusy(q)
	if err != nil {
		return nil, nil, err
	}
	var testing []int
	var raw string
	if err := q.QueryRow("SELECT value FROM settings WHERE key = ?", settingListTest).Scan(&raw); err == nil && raw != "" {
		_ = json.Unmarshal([]byte(raw), &testing)
	}

	all, err := readJoinables(q, `SELECT id, block_id, kind, name, in_url, in_title, in_text FROM web_lists ORDER BY id`,
		"SELECT entry FROM web_list_entries WHERE list_id = ?", true)
	if err != nil {
		return nil, nil, err
	}
	for _, p := range pairUp(all) {
		if p.url.kind == p.terms.kind && slices.Equal(p.url.entries, p.terms.entries) &&
			slices.Equal(p.url.actions, p.terms.actions) &&
			!slices.Contains(testing, p.url.id) && !slices.Contains(testing, p.terms.id) &&
			!busyLists[p.terms.id] {
			lists = append(lists, p)
		}
	}

	all, err = readJoinables(q, `SELECT id, 0, '', name, in_url, in_title, in_text FROM clusters WHERE phrases = 0 ORDER BY id`,
		"SELECT entry FROM cluster_entries WHERE cluster_id = ?", false)
	if err != nil {
		return nil, nil, err
	}
	for _, p := range pairUp(all) {
		if slices.Equal(p.url.entries, p.terms.entries) && !busyClusters[strings.ToLower(p.terms.name)] {
			clusters = append(clusters, p)
		}
	}
	return lists, clusters, nil
}

// pairUp finds, per owner, each list looking only at the url with a list
// named "<its name> terms" not looking at the url.
func pairUp(all []joinable) []joinPair {
	byName := map[string]joinable{}
	for _, j := range all {
		byName[fmt.Sprint(j.owner)+"\x00"+strings.ToLower(j.name)] = j
	}
	var out []joinPair
	for _, j := range all {
		if j.where != (Where{URL: true}) {
			continue
		}
		terms, ok := byName[fmt.Sprint(j.owner)+"\x00"+strings.ToLower(j.name+" terms")]
		if !ok || terms.where.URL || !terms.where.Any() {
			continue
		}
		out = append(out, joinPair{url: j, terms: terms})
	}
	return out
}

// readJoinables reads lists or clusters with their entries (and, for
// blacklists, actions).
func readJoinables(q querier, query, entriesQuery string, withActions bool) ([]joinable, error) {
	rows, err := q.Query(query)
	if err != nil {
		return nil, err
	}
	var out []joinable
	for rows.Next() {
		var j joinable
		if err := rows.Scan(&j.id, &j.owner, &j.kind, &j.name, &j.where.URL, &j.where.Title, &j.where.Text); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, j)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		entries, err := legacyStrings(q, entriesQuery, out[i].id)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e = strings.ToLower(e); !slices.Contains(out[i].entries, e) {
				out[i].entries = append(out[i].entries, e)
			}
		}
		slices.Sort(out[i].entries)
		if withActions && out[i].kind == ListBlacklist {
			if out[i].actions, err = listActions(q, out[i].id); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// joinBusy returns the lists named by queued edits and the clusters (lower
// case names) and lists a recording adds to: those aren't joined.
func joinBusy(q querier) (map[int]bool, map[string]bool, error) {
	lists, clusters := map[int]bool{}, map[string]bool{}
	rows, err := q.Query("SELECT payload FROM pending_edits")
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var payload string
		if rows.Scan(&payload) != nil {
			continue
		}
		var e Edit
		if json.Unmarshal([]byte(payload), &e) == nil && e.List != 0 {
			lists[e.List] = true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	var raw string
	if err := q.QueryRow("SELECT value FROM settings WHERE key = ?", SettingRecording).Scan(&raw); err == nil && raw != "" {
		var r Recording
		if json.Unmarshal([]byte(raw), &r) == nil {
			if r.ListID != 0 {
				lists[r.ListID] = true
			}
			if r.Cluster != "" {
				clusters[strings.ToLower(r.Cluster)] = true
			}
		}
	}
	return lists, clusters, nil
}
