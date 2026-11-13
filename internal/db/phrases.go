// phrases.go — phrase sets (§3.1, §3.2, R-PHRASE-1 to 10): named sets of
// phrases inside a block, each with a scope (url, title, text), its own
// actions with a threshold (how many different phrases must match), and
// test mode, like a blacklist. A phrase is word sets, each a list of terms
// and necessary or optional, and a length; it is stored as JSON.
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

const phrasesSchema = `CREATE TABLE IF NOT EXISTS phrase_sets (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id INTEGER NOT NULL,
    name     TEXT NOT NULL,
    in_url   BOOLEAN NOT NULL DEFAULT 0,
    in_title BOOLEAN NOT NULL DEFAULT 0,
    in_text  BOOLEAN NOT NULL DEFAULT 1,
    test     BOOLEAN NOT NULL DEFAULT 0,
    UNIQUE (block_id, name),
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS phrase_set_actions (
    set_id    INTEGER NOT NULL,
    action    TEXT NOT NULL,
    threshold INTEGER NOT NULL DEFAULT 1,
    param     TEXT NOT NULL DEFAULT '',
    minutes   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (set_id, action),
    FOREIGN KEY (set_id) REFERENCES phrase_sets(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS phrases (
    id     INTEGER PRIMARY KEY AUTOINCREMENT,
    set_id INTEGER NOT NULL,
    spec   TEXT NOT NULL,
    FOREIGN KEY (set_id) REFERENCES phrase_sets(id) ON DELETE CASCADE
);`

// WordSet is one place of a phrase: the terms that may fill it.
type WordSet struct {
	Terms     []string `json:"terms"`
	Necessary bool     `json:"necessary"`
}

// Phrase is word sets and a length (0: the number of necessary sets).
type Phrase struct {
	Sets   []WordSet `json:"sets"`
	Length int       `json:"length,omitempty"`
}

// MaxWordSets bounds a phrase.
const MaxWordSets = 9

// Normalize makes the first two word sets necessary (R-PHRASE-1), trims
// and lowercases the terms, and drops empty ones.
func (p Phrase) Normalize() Phrase {
	out := Phrase{Length: p.Length}
	for i, s := range p.Sets {
		set := WordSet{Necessary: i < 2 || s.Necessary}
		for _, t := range s.Terms {
			if t = strings.ToLower(strings.Join(strings.Fields(t), " ")); t != "" && strings.Trim(t, "*-") != "" {
				set.Terms = append(set.Terms, t)
			}
		}
		out.Sets = append(out.Sets, set)
	}
	return out
}

// Necessary counts the necessary word sets.
func (p Phrase) Necessary() int {
	n := 0
	for i, s := range p.Sets {
		if i < 2 || s.Necessary {
			n++
		}
	}
	return n
}

// Check refuses a phrase that can't be used (R-PHRASE-1, R-PHRASE-8).
func (p Phrase) Check() error {
	if len(p.Sets) < 2 {
		return fmt.Errorf("a phrase needs at least two word sets, such as [eat, nice] [hello, grandpa]")
	}
	if len(p.Sets) > MaxWordSets {
		return fmt.Errorf("a phrase has at most %d word sets", MaxWordSets)
	}
	into, out, middle := 0, 0, false
	for _, s := range p.Sets {
		if len(s.Terms) == 0 {
			return fmt.Errorf("a word set is empty")
		}
		for _, t := range s.Terms {
			if len(t) > 100 {
				return fmt.Errorf("the term %q is too long", t[:20]+"…")
			}
			in, o := strings.HasPrefix(t, "-"), strings.HasSuffix(t, "-") && len(t) > 1
			if in {
				into++
			}
			if o {
				out++
			}
			middle = middle || (in && o)
		}
	}
	if (into > 0) != (out > 0) {
		return fmt.Errorf("a bridge needs both ends: a term ending in - (big-) and one starting with - (-gorilla) in another word set")
	}
	if middle && len(p.Sets) < 3 {
		return fmt.Errorf("a middle link (-strong-) needs at least three word sets")
	}
	if p.Length != 0 && (p.Length < p.Necessary() || p.Length > len(p.Sets)) {
		return fmt.Errorf("the phrase length is from %d (its necessary word sets) to %d (all its word sets)", p.Necessary(), len(p.Sets))
	}
	return nil
}

// String writes a phrase as it is typed: [a, b] [c] [d]? with :N for a
// length other than the default.
func (p Phrase) String() string {
	var parts []string
	for i, s := range p.Sets {
		part := "[" + strings.Join(s.Terms, ", ") + "]"
		if i >= 2 && !s.Necessary {
			part += "?"
		}
		parts = append(parts, part)
	}
	out := strings.Join(parts, " ")
	if p.Length != 0 && p.Length != p.Necessary() {
		out += fmt.Sprintf(" :%d", p.Length)
	}
	return out
}

// ParsePhrase reads "[eat, nice] [hello, grandpa] [today]? :3": word sets
// in brackets, a ? after an optional one (from the third on), and :N for
// the phrase length.
func ParsePhrase(text string) (Phrase, error) {
	var p Phrase
	rest := strings.TrimSpace(text)
	if i := strings.LastIndex(rest, ":"); i >= 0 && !strings.Contains(rest[i:], "]") {
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(rest[i+1:]), "%d", &n); err != nil || n < 1 {
			return p, fmt.Errorf("after : comes the phrase length, a number")
		}
		p.Length = n
		rest = strings.TrimSpace(rest[:i])
	}
	for rest != "" {
		if !strings.HasPrefix(rest, "[") {
			return p, fmt.Errorf("write a phrase as word sets in brackets: [eat, nice] [hello, grandpa]")
		}
		end := strings.Index(rest, "]")
		if end < 0 {
			return p, fmt.Errorf("a [ has no ]")
		}
		set := WordSet{Necessary: true}
		for _, t := range strings.Split(rest[1:end], ",") {
			if t = strings.TrimSpace(t); t != "" {
				set.Terms = append(set.Terms, t)
			}
		}
		rest = strings.TrimSpace(rest[end+1:])
		if strings.HasPrefix(rest, "?") {
			set.Necessary = false
			rest = strings.TrimSpace(rest[1:])
		}
		p.Sets = append(p.Sets, set)
	}
	p = p.Normalize()
	return p, p.Check()
}

// PhraseSet is a block's phrase set.
type PhraseSet struct {
	ID      int
	Name    string
	Where   Where
	Test    bool
	Actions []ListAction // Distinct is the threshold: different phrases
	Phrases []Phrase
}

// Action returns one of the set's actions.
func (s PhraseSet) Action(name string) (ListAction, bool) {
	for _, a := range s.Actions {
		if a.Name == name {
			return a, true
		}
	}
	return ListAction{}, false
}

// PhraseSets returns a block's phrase sets, in the order they were made.
func (d *DB) PhraseSets(block string) ([]PhraseSet, error) {
	blockID, err := d.getBlockID(block)
	if err != nil {
		return nil, err
	}
	rows, err := d.conn.Query("SELECT id, name, in_url, in_title, in_text, test FROM phrase_sets WHERE block_id = ? ORDER BY id", blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to read phrase sets: %w", err)
	}
	var out []PhraseSet
	for rows.Next() {
		var s PhraseSet
		if err := rows.Scan(&s.ID, &s.Name, &s.Where.URL, &s.Where.Title, &s.Where.Text, &s.Test); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, s)
	}
	rows.Close()
	for i := range out {
		if err := d.fillPhraseSet(&out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// PhraseSetByID returns a phrase set with the name of its block.
func (d *DB) PhraseSetByID(id int) (PhraseSet, string, error) {
	var s PhraseSet
	var block string
	err := d.conn.QueryRow(`SELECT s.id, s.name, s.in_url, s.in_title, s.in_text, s.test, b.name
		FROM phrase_sets s JOIN blocks b ON b.id = s.block_id WHERE s.id = ?`, id).
		Scan(&s.ID, &s.Name, &s.Where.URL, &s.Where.Title, &s.Where.Text, &s.Test, &block)
	if err == sql.ErrNoRows {
		return s, "", fmt.Errorf("there is no phrase set %d", id)
	}
	if err != nil {
		return s, "", err
	}
	return s, block, d.fillPhraseSet(&s)
}

func (d *DB) fillPhraseSet(s *PhraseSet) error {
	rows, err := d.conn.Query("SELECT action, threshold, param, minutes FROM phrase_set_actions WHERE set_id = ? ORDER BY action", s.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		a := ListAction{Hits: 1}
		if err := rows.Scan(&a.Name, &a.Distinct, &a.Param, &a.Minutes); err != nil {
			rows.Close()
			return err
		}
		s.Actions = append(s.Actions, a)
	}
	rows.Close()
	rows, err = d.conn.Query("SELECT spec FROM phrases WHERE set_id = ? ORDER BY id", s.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return err
		}
		var p Phrase
		if json.Unmarshal([]byte(raw), &p) == nil {
			s.Phrases = append(s.Phrases, p)
		}
	}
	return rows.Err()
}

// FindPhraseSet returns a block's phrase set by name.
func (d *DB) FindPhraseSet(block, name string) (PhraseSet, error) {
	sets, err := d.PhraseSets(block)
	if err != nil {
		return PhraseSet{}, err
	}
	for _, s := range sets {
		if strings.EqualFold(s.Name, name) {
			return s, nil
		}
	}
	return PhraseSet{}, fmt.Errorf("block '%s' has no phrase set '%s'", block, name)
}

// CreatePhraseSet adds a phrase set that blocks what it matches.
func (d *DB) CreatePhraseSet(block, name string, where Where) (int, error) {
	if err := CheckListName(name); err != nil {
		return 0, err
	}
	if !where.Any() {
		return 0, fmt.Errorf("a phrase set looks at the address, the title or the text")
	}
	blockID, err := d.getBlockID(block)
	if err != nil {
		return 0, err
	}
	res, err := d.conn.Exec("INSERT INTO phrase_sets (block_id, name, in_url, in_title, in_text) VALUES (?, ?, ?, ?, ?)",
		blockID, name, where.URL, where.Title, where.Text)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, fmt.Errorf("the block already has a phrase set named '%s'", name)
		}
		return 0, fmt.Errorf("failed to create the phrase set: %w", err)
	}
	id, _ := res.LastInsertId()
	if _, err := d.conn.Exec("INSERT INTO phrase_set_actions (set_id, action) VALUES (?, ?)", id, ActionBlock); err != nil {
		return 0, err
	}
	return int(id), nil
}

// DeletePhraseSet removes a phrase set with its phrases.
func (d *DB) DeletePhraseSet(id int) error {
	_, err := d.conn.Exec("DELETE FROM phrase_sets WHERE id = ?", id)
	return err
}

// RenamePhraseSet renames a phrase set.
func (d *DB) RenamePhraseSet(id int, name string) error {
	if err := CheckListName(name); err != nil {
		return err
	}
	if _, err := d.conn.Exec("UPDATE phrase_sets SET name = ? WHERE id = ?", name, id); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("the block already has a phrase set named '%s'", name)
		}
		return err
	}
	return nil
}

// SetPhraseSetWhere sets a phrase set's scope.
func (d *DB) SetPhraseSetWhere(id int, where Where) error {
	if !where.Any() {
		return fmt.Errorf("a phrase set looks at the address, the title or the text")
	}
	_, err := d.conn.Exec("UPDATE phrase_sets SET in_url = ?, in_title = ?, in_text = ? WHERE id = ?", where.URL, where.Title, where.Text, id)
	return err
}

// SetPhraseSetTest turns test mode on or off.
func (d *DB) SetPhraseSetTest(id int, on bool) error {
	_, err := d.conn.Exec("UPDATE phrase_sets SET test = ? WHERE id = ?", on, id)
	return err
}

// SetPhraseSetAction turns an action on (threshold 1) or off.
func (d *DB) SetPhraseSetAction(id int, action string, on bool, param string, minutes int) error {
	if !on {
		_, err := d.conn.Exec("DELETE FROM phrase_set_actions WHERE set_id = ? AND action = ?", id, action)
		return err
	}
	_, err := d.conn.Exec(`INSERT INTO phrase_set_actions (set_id, action, param, minutes) VALUES (?, ?, ?, ?)
		ON CONFLICT(set_id, action) DO UPDATE SET param = excluded.param, minutes = excluded.minutes`, id, action, param, minutes)
	return err
}

// SetPhraseSetThreshold sets how many different phrases an action needs.
func (d *DB) SetPhraseSetThreshold(id int, action string, n int) error {
	if n < 1 || n > MaxThreshold {
		return fmt.Errorf("the threshold is from 1 to %d", MaxThreshold)
	}
	res, err := d.conn.Exec("UPDATE phrase_set_actions SET threshold = ? WHERE set_id = ? AND action = ?", n, id, action)
	if err != nil {
		return err
	}
	if k, _ := res.RowsAffected(); k == 0 {
		return fmt.Errorf("the phrase set doesn't have the action %s", action)
	}
	return nil
}

// AddPhrase adds a phrase to a set, unless it has it already; it reports
// whether it was new.
func (d *DB) AddPhrase(id int, p Phrase) (bool, error) {
	p = p.Normalize()
	if err := p.Check(); err != nil {
		return false, err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return false, err
	}
	var n int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM phrases WHERE set_id = ? AND spec = ?", id, string(raw)).Scan(&n); err != nil || n > 0 {
		return false, err
	}
	_, err = d.conn.Exec("INSERT INTO phrases (set_id, spec) VALUES (?, ?)", id, string(raw))
	return err == nil, err
}

// RemovePhrase removes a phrase from a set; it reports whether it was there.
func (d *DB) RemovePhrase(id int, p Phrase) (bool, error) {
	raw, err := json.Marshal(p.Normalize())
	if err != nil {
		return false, err
	}
	res, err := d.conn.Exec("DELETE FROM phrases WHERE set_id = ? AND spec = ?", id, string(raw))
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
