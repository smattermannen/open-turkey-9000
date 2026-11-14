// combinations.go — word combinations, the old form of phrases, and their
// one-time move into phrase sets (R-PHRASE-1 to 10, SPEC §3.14: "word
// combination" is now "phrase").
//
// A word combination was an entry of a term list: boxes of words side by
// side, each a list in brackets with how many different words of it a match
// needs (":2"; 1 if left out), or one bare word: cars(red, big),
// (car, lambo*) (big, small, nice):2.
//
// Each combination becomes phrases: a box needing one word becomes a word
// set; a box needing N different words becomes N one-term word sets, once
// for every choice of N of its words (R-PHRASE-3: a word set gives one
// term), so (car) (big, small, nice):2 becomes [car] [big] [small],
// [car] [big] [nice] and [car] [small] [nice]. The phrases of a blacklist
// go to a new phrase set "<list> phrases" with the list's scope, test mode
// and actions, each at threshold 1; a term cluster's go to a new phrase
// cluster. A combination that could never match (a box needing more words
// than it has) is dropped. Combinations in exception lists stay where they
// are: there are no phrase exceptions.
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// MaxComboBoxes bounds the boxes of a combination, and the words one box
// may need.
const MaxComboBoxes = 9

// maxCombinationPhrases bounds how many phrases one combination becomes.
const maxCombinationPhrases = 200

// ComboBox is one box of a word combination.
type ComboBox struct {
	Terms []string
	Min   int
	Bare  bool // written without brackets
}

var comboGroup = regexp.MustCompile(`\(([^()]*)\)(?:\s*:\s*(\d+))?`)

func usableComboTerm(t string) bool {
	return strings.TrimSpace(strings.ReplaceAll(t, "*", "")) != ""
}

// ParseCombination returns a word combination's boxes, or false when k
// isn't a well-formed one.
func ParseCombination(k string) ([]ComboBox, bool) {
	k = strings.TrimSpace(k)
	if !strings.ContainsAny(k, "()") {
		return nil, false
	}
	var boxes []ComboBox
	bare := func(text string) bool {
		term := strings.TrimSpace(text)
		if term == "" {
			return true
		}
		if strings.ContainsAny(term, ",:()") || !usableComboTerm(term) {
			return false
		}
		boxes = append(boxes, ComboBox{Terms: []string{term}, Min: 1, Bare: true})
		return true
	}
	last := 0
	for _, m := range comboGroup.FindAllStringSubmatchIndex(k, -1) {
		if !bare(k[last:m[0]]) {
			return nil, false
		}
		box := ComboBox{Min: 1}
		for _, t := range strings.Split(k[m[2]:m[3]], ",") {
			t = strings.TrimSpace(t)
			if !usableComboTerm(t) {
				return nil, false
			}
			box.Terms = append(box.Terms, t)
		}
		if m[4] >= 0 {
			n, err := strconv.Atoi(k[m[4]:m[5]])
			if err != nil || n < 1 || n > MaxComboBoxes {
				return nil, false
			}
			box.Min = n
		}
		boxes = append(boxes, box)
		last = m[1]
	}
	if strings.ContainsAny(k[last:], "()") || !bare(k[last:]) {
		return nil, false
	}
	needed := 0
	for _, b := range boxes {
		needed += b.Min
	}
	return boxes, needed >= 2 && len(boxes) <= MaxComboBoxes
}

// choose returns every choice of n of the items, in order.
func choose(items []string, n int) [][]string {
	if n == 0 {
		return [][]string{{}}
	}
	var out [][]string
	for i := 0; i+n <= len(items); i++ {
		for _, rest := range choose(items[i+1:], n-1) {
			out = append(out, append([]string{items[i]}, rest...))
		}
	}
	return out
}

// CombinationPhrases turns a word combination's boxes into phrases (see the
// top of this file). No phrases and no error: it could never match.
func CombinationPhrases(boxes []ComboBox) ([]Phrase, error) {
	// For each box, the ways it can fill word sets.
	var options [][][]WordSet
	for _, b := range boxes {
		var terms []string
		for _, t := range b.Terms {
			t = strings.ToLower(strings.Join(strings.Fields(t), " "))
			if strings.HasPrefix(t, "-") || strings.HasSuffix(t, "-") {
				return nil, fmt.Errorf("the word %q would read as a bridge", t)
			}
			if !slices.Contains(terms, t) {
				terms = append(terms, t)
			}
		}
		if b.Min == 1 {
			options = append(options, [][]WordSet{{{Terms: terms, Necessary: true}}})
			continue
		}
		var ways [][]WordSet
		for _, pick := range choose(terms, b.Min) {
			var sets []WordSet
			for _, t := range pick {
				sets = append(sets, WordSet{Terms: []string{t}, Necessary: true})
			}
			ways = append(ways, sets)
		}
		if len(ways) == 0 {
			return nil, nil // the box needs more words than it has
		}
		options = append(options, ways)
	}
	phrases := []Phrase{{}}
	for _, ways := range options {
		var next []Phrase
		for _, p := range phrases {
			for _, w := range ways {
				next = append(next, Phrase{Sets: append(slices.Clone(p.Sets), w...)})
			}
		}
		if len(next) > maxCombinationPhrases {
			return nil, fmt.Errorf("it would become more than %d phrases", maxCombinationPhrases)
		}
		phrases = next
	}
	for i := range phrases {
		phrases[i] = phrases[i].Normalize()
		if err := phrases[i].Check(); err != nil {
			return nil, err
		}
	}
	return phrases, nil
}

// freeName returns name, or name with a number, that taken (lower case)
// doesn't hold.
func freeName(name string, taken map[string]bool) string {
	out := name
	for n := 2; taken[strings.ToLower(out)]; n++ {
		out = fmt.Sprintf("%s %d", name, n)
	}
	taken[strings.ToLower(out)] = true
	return out
}

// migrateCombinations moves the word combinations of blacklists and term
// clusters into phrase sets and phrase clusters. It runs at every open and
// finds nothing once they have moved; an old file imported later is moved
// the next time the database is opened.
func migrateCombinations(conn *sql.DB) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	type entry struct {
		id     int
		text   string
		phrase []Phrase
	}
	// combos reads the combination entries of query's rows, grouped by
	// owner (a list or a cluster), and turns each into phrases.
	combos := func(query string) (map[int][]entry, []int, error) {
		rows, err := tx.Query(query)
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close()
		out, order := map[int][]entry{}, []int{}
		for rows.Next() {
			var owner int
			var e entry
			if err := rows.Scan(&owner, &e.id, &e.text); err != nil {
				return nil, nil, err
			}
			boxes, ok := ParseCombination(e.text)
			if !ok {
				continue
			}
			if e.phrase, err = CombinationPhrases(boxes); err != nil {
				log.Printf("word combination %q stays as it is: %v", e.text, err)
				continue
			}
			if _, seen := out[owner]; !seen {
				order = append(order, owner)
			}
			out[owner] = append(out[owner], e)
		}
		return out, order, rows.Err()
	}
	phraseSpecs := func(entries []entry) ([]string, error) {
		var out []string
		for _, e := range entries {
			for _, p := range e.phrase {
				raw, err := json.Marshal(p)
				if err != nil {
					return nil, err
				}
				if !slices.Contains(out, string(raw)) {
					out = append(out, string(raw))
				}
			}
		}
		return out, nil
	}

	lists, order, err := combos(`SELECT l.id, e.id, e.entry FROM web_list_entries e JOIN web_lists l ON l.id = e.list_id
		WHERE l.kind = 'blacklist' AND l.type = 'term' AND e.entry LIKE '%(%' ORDER BY e.id`)
	if err != nil {
		return fmt.Errorf("word combinations: %w", err)
	}
	var testing []int
	var raw string
	if err := tx.QueryRow("SELECT value FROM settings WHERE key = ?", settingListTest).Scan(&raw); err == nil && raw != "" {
		_ = json.Unmarshal([]byte(raw), &testing)
	}
	for _, listID := range order {
		var blockID int
		var name string
		var where Where
		if err := tx.QueryRow("SELECT block_id, name, in_url, in_title, in_text FROM web_lists WHERE id = ?", listID).
			Scan(&blockID, &name, &where.URL, &where.Title, &where.Text); err != nil {
			return err
		}
		taken := map[string]bool{}
		rows, err := tx.Query("SELECT name FROM phrase_sets WHERE block_id = ?", blockID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var n string
			if rows.Scan(&n) == nil {
				taken[strings.ToLower(n)] = true
			}
		}
		rows.Close()
		specs, err := phraseSpecs(lists[listID])
		if err != nil {
			return err
		}
		if len(specs) > 0 {
			res, err := tx.Exec("INSERT INTO phrase_sets (block_id, name, in_url, in_title, in_text, test) VALUES (?, ?, ?, ?, ?, ?)",
				blockID, freeName(name+" phrases", taken), where.URL, where.Title, where.Text, slices.Contains(testing, listID))
			if err != nil {
				return fmt.Errorf("word combinations: %w", err)
			}
			setID, _ := res.LastInsertId()
			// The list's actions, each at threshold 1: one phrase is enough.
			if _, err := tx.Exec(`INSERT INTO phrase_set_actions (set_id, action, threshold, param, minutes)
				SELECT ?, action, 1, param, minutes FROM web_list_actions WHERE list_id = ?`, setID, listID); err != nil {
				return err
			}
			for _, spec := range specs {
				if _, err := tx.Exec("INSERT INTO phrases (set_id, spec) VALUES (?, ?)", setID, spec); err != nil {
					return err
				}
			}
		}
		for _, e := range lists[listID] {
			if _, err := tx.Exec("DELETE FROM web_list_entries WHERE id = ?", e.id); err != nil {
				return err
			}
		}
		log.Printf("word combinations of list '%s' moved into phrases (%d)", name, len(specs))
	}

	clusters, order, err := combos(`SELECT c.id, e.id, e.entry FROM cluster_entries e JOIN clusters c ON c.id = e.cluster_id
		WHERE c.phrases = 0 AND c.type = 'term' AND e.entry LIKE '%(%' ORDER BY e.id`)
	if err != nil {
		return fmt.Errorf("word combinations: %w", err)
	}
	for _, clusterID := range order {
		var name string
		var where Where
		if err := tx.QueryRow("SELECT name, in_url, in_title, in_text FROM clusters WHERE id = ?", clusterID).
			Scan(&name, &where.URL, &where.Title, &where.Text); err != nil {
			return err
		}
		taken := map[string]bool{}
		rows, err := tx.Query("SELECT name FROM clusters")
		if err != nil {
			return err
		}
		for rows.Next() {
			var n string
			if rows.Scan(&n) == nil {
				taken[strings.ToLower(n)] = true
			}
		}
		rows.Close()
		var texts []string
		for _, e := range clusters[clusterID] {
			for _, p := range e.phrase {
				if t := p.String(); !slices.Contains(texts, t) {
					texts = append(texts, t)
				}
			}
		}
		if len(texts) > 0 {
			res, err := tx.Exec("INSERT INTO clusters (name, in_url, in_title, in_text, phrases, type) VALUES (?, ?, ?, ?, 1, 'term')",
				freeName(name+" phrases", taken), where.URL, where.Title, where.Text)
			if err != nil {
				return fmt.Errorf("word combinations: %w", err)
			}
			id, _ := res.LastInsertId()
			for _, t := range texts {
				if _, err := tx.Exec("INSERT INTO cluster_entries (cluster_id, entry) VALUES (?, ?)", id, t); err != nil {
					return err
				}
			}
		}
		for _, e := range clusters[clusterID] {
			if _, err := tx.Exec("DELETE FROM cluster_entries WHERE id = ?", e.id); err != nil {
				return err
			}
		}
		log.Printf("word combinations of cluster '%s' moved into phrases (%d)", name, len(texts))
	}
	return tx.Commit()
}

// MigrateCombinations moves word combinations into phrases now (after an
// import of an old file), instead of at the next open.
func (d *DB) MigrateCombinations() error {
	return migrateCombinations(d.conn)
}
