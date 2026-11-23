// weblists.go — a block's website lists. A block has blacklists and
// exception lists, each with a name. Each list ticks where it looks (its
// scope): the page's address (URL), its title (the tab title), and its text.
// A list has no type: sites, URL entries and rendered terms share it. Each
// blacklist also chooses its actions (block, greyscale, no images), and for
// page text how many of its different entries a page must contain before
// each action applies. An exception list spares the pages it matches from
// every blacklist of its block.
//
// Before lists, a block had one list of sites and patterns, one of
// exceptions and one of page keywords, an allowlist mode, and its actions
// and thresholds for the whole block. migrateWebLists turns those into the
// lists "Blocked", "Exceptions" and "Rendered terms on page".
package db

import (
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/smattermannen/kc9k/internal/blocker"
)

// List kinds.
const (
	ListBlacklist  = "blacklist"
	ListExceptions = "exceptions"
)

// Lists have no types (§3.2): sites, URL entries and rendered terms share a
// list, and each entry is matched by the list's scope. With url, sites and
// URL entries match the address (and feed the layers) and rendered terms
// match words in the address (R-PHRASE-6); with title or text, every entry
// is a rendered term. Which entries are sites or URL entries is decided
// where they are used (blocker.IsAddressEntry, WebList.Addresses).

// CheckScope refuses a scope with no box ticked: a list looks somewhere.
func CheckScope(w Where) error {
	if !w.Any() {
		return fmt.Errorf("a list looks at the url, the title or the text")
	}
	return nil
}

// Where a list looks.
const (
	PlaceURL   = "url"
	PlaceTitle = "title"
	PlaceText  = "text"
	// PlaceAds: the ad networks a page loads (§3.9 ad trigger).
	PlaceAds = "ads"
)

// Places lists the places in display order.
var Places = []string{PlaceURL, PlaceTitle, PlaceText, PlaceAds}

// The lists a new block starts with, and those made from older blocks.
const (
	DefaultBlockedList    = "Blocked"
	DefaultExceptionsList = "Exceptions"
	DefaultWordsList      = "Rendered terms on page"
	// oldWordsList is DefaultWordsList's name before 2026-11-16.
	oldWordsList = "Page words"
)

// MaxListName bounds a list's name.
const MaxListName = 60

const webListsSchema = `CREATE TABLE IF NOT EXISTS web_lists (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id INTEGER NOT NULL,
    kind     TEXT NOT NULL,
    name     TEXT NOT NULL,
    in_url   BOOLEAN NOT NULL DEFAULT 1,
    in_title BOOLEAN NOT NULL DEFAULT 0,
    in_text  BOOLEAN NOT NULL DEFAULT 0,
    UNIQUE (block_id, name),
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS web_list_entries (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    list_id INTEGER NOT NULL,
    entry   TEXT NOT NULL,
    FOREIGN KEY (list_id) REFERENCES web_lists(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS web_list_actions (
    list_id  INTEGER NOT NULL,
    action   TEXT NOT NULL,
    distinct_min INTEGER NOT NULL DEFAULT 1,
    hits_min     INTEGER NOT NULL DEFAULT 1,
    param        TEXT NOT NULL DEFAULT '',
    minutes      INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (list_id, action),
    FOREIGN KEY (list_id) REFERENCES web_lists(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS web_list_entries_list ON web_list_entries (list_id);`

// Where holds the places a list looks.
type Where struct {
	URL   bool `json:"url"`
	Title bool `json:"title"`
	Text  bool `json:"text"`
	Ads   bool `json:"ads,omitempty"`
}

// IsPlace reports whether name is a place.
func IsPlace(name string) bool {
	for _, p := range Places {
		if p == name {
			return true
		}
	}
	return false
}

// Has reports whether the list looks at the named place.
func (w Where) Has(place string) bool {
	switch place {
	case PlaceURL:
		return w.URL
	case PlaceTitle:
		return w.Title
	case PlaceText:
		return w.Text
	case PlaceAds:
		return w.Ads
	}
	return false
}

// With returns a copy looking (or not) at the named place.
func (w Where) With(place string, on bool) Where {
	switch place {
	case PlaceURL:
		w.URL = on
	case PlaceTitle:
		w.Title = on
	case PlaceText:
		w.Text = on
	case PlaceAds:
		w.Ads = on
	}
	return w
}

// Any reports whether the list looks anywhere.
func (w Where) Any() bool { return w.URL || w.Title || w.Text || w.Ads }

// freeListName returns name, or name 2, name 3… when taken (lower case).
func freeListName(name string, taken map[string]bool) string {
	candidate := name
	for n := 2; taken[strings.ToLower(candidate)]; n++ {
		candidate = fmt.Sprintf("%s %d", name, n)
	}
	taken[strings.ToLower(candidate)] = true
	return candidate
}

// List returns the places, in display order.
func (w Where) List() []string {
	var out []string
	for _, p := range Places {
		if w.Has(p) {
			out = append(out, p)
		}
	}
	return out
}

// String names the places, e.g. "url, text".
func (w Where) String() string {
	if !w.Any() {
		return "nowhere"
	}
	return strings.Join(w.List(), ", ")
}

// ListAction is one action a blacklist takes on the pages it matches. For
// page text, Distinct is how many of the list's different entries a page must
// contain, and Hits how many times they must occur in all (repeats counted);
// both must be met. A match in the address or title always counts. Param
// and Minutes are what the action needs (see CheckActionParams).
type ListAction struct {
	Name     string `json:"action"`
	Distinct int    `json:"distinct"`
	Hits     int    `json:"hits"`
	// Rate: matches per 1,000 words (tenths of a percent) a page's text
	// needs, on pages of at least RateMinWords words; 0 means none (§3.9).
	Rate    int    `json:"rate,omitempty"`
	Param   string `json:"param,omitempty"`
	Minutes int    `json:"minutes,omitempty"`
}

// RateMinWords is the shortest page a rate is judged on (§3.9).
const RateMinWords = 200

// MaxRate is the highest rate: 100 %.
const MaxRate = 1000

// WebList is one of a block's website lists.
type WebList struct {
	ID      int
	Kind    string
	Name    string
	Where   Where
	Entries []string
	Actions []ListAction // blacklists only, in ActionNames order
}

// Addresses returns the list's sites and URL entries if it looks at the url
// (blocker.IsAddressEntry); they match the whole address.
func (l WebList) Addresses() []string {
	out := []string{}
	if l.Where.URL {
		for _, e := range l.Entries {
			if blocker.IsAddressEntry(e) {
				out = append(out, e)
			}
		}
	}
	return out
}

// URLTerms returns the list's rendered terms if it looks at the url: the
// entries that aren't sites or URL entries, matched as words in the address.
func (l WebList) URLTerms() []string {
	out := []string{}
	if l.Where.URL {
		for _, e := range l.Entries {
			if !blocker.IsAddressEntry(e) {
				out = append(out, e)
			}
		}
	}
	return out
}

// IsExceptions reports whether the list is an exception list.
func (l WebList) IsExceptions() bool { return l.Kind == ListExceptions }

// Action returns the named action, if the list takes it.
func (l WebList) Action(name string) (ListAction, bool) {
	for _, a := range l.Actions {
		if a.Name == name {
			return a, true
		}
	}
	return ListAction{}, false
}

// HasAction reports whether the list takes the named action.
func (l WebList) HasAction(name string) bool {
	_, ok := l.Action(name)
	return ok
}

// ActionNames returns the names of the list's actions.
func (l WebList) ActionNames() []string {
	out := []string{}
	for _, a := range l.Actions {
		out = append(out, a.Name)
	}
	return out
}

// Enforces reports whether a blacklist does anything: it has entries, looks
// somewhere and takes an action.
func (l WebList) Enforces() bool {
	return !l.IsExceptions() && len(l.Entries) > 0 && l.Where.Any() && len(l.Actions) > 0
}

// Lists is a block's website lists.
type Lists []WebList

// Find returns the named list.
func (ls Lists) Find(name string) (WebList, bool) {
	for _, l := range ls {
		if l.Name == name {
			return l, true
		}
	}
	return WebList{}, false
}

// Blacklists returns the blacklists.
func (ls Lists) Blacklists() Lists {
	var out Lists
	for _, l := range ls {
		if !l.IsExceptions() {
			out = append(out, l)
		}
	}
	return out
}

// ExceptionLists returns the exception lists.
func (ls Lists) ExceptionLists() Lists {
	var out Lists
	for _, l := range ls {
		if l.IsExceptions() {
			out = append(out, l)
		}
	}
	return out
}

// Enforces reports whether any blacklist does something.
func (ls Lists) Enforces() bool {
	for _, l := range ls {
		if l.Enforces() {
			return true
		}
	}
	return false
}

// ExceptionAddresses returns the sites and URL entries of the exception
// lists that look at the url.
func (ls Lists) ExceptionAddresses() []string {
	var out []string
	for _, l := range ls {
		if l.IsExceptions() {
			out = append(out, l.Addresses()...)
		}
	}
	return out
}

// ExceptionTerms returns the rendered terms of the exception lists that look
// at the place: for the url the entries that aren't sites or URL entries
// (matched as words in the decoded address, R-PHRASE-6), for the title and
// text every entry.
func (ls Lists) ExceptionTerms(place string) []string {
	var out []string
	for _, l := range ls {
		if !l.IsExceptions() {
			continue
		}
		if place == PlaceURL {
			out = append(out, l.URLTerms()...)
		} else if l.Where.Has(place) {
			out = append(out, l.Entries...)
		}
	}
	return out
}

// CheckListName refuses names that are empty, too long or multi-line.
func CheckListName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("a list needs a name")
	}
	if name != strings.TrimSpace(name) || len(name) > MaxListName || strings.ContainsAny(name, "\n\r\t") {
		return fmt.Errorf("invalid list name %q (one line, at most %d characters, no spaces around it)", name, MaxListName)
	}
	return nil
}

// querier is what both *sql.DB and *sql.Tx offer.
type querier interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

func getBlockLists(q querier, blockID int) (Lists, error) {
	return cachedBlockLists(q, blockID, nil)
}

// cachedBlockLists is getBlockLists, taking entries from cache when they
// haven't changed.
func cachedBlockLists(q querier, blockID int, cache *entryCache) (Lists, error) {
	rows, err := q.Query(`SELECT id, kind, name, in_url, in_title, in_text, in_ads, entries_version FROM web_lists
		WHERE block_id = ? ORDER BY CASE kind WHEN 'blacklist' THEN 0 ELSE 1 END, id`, blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch website lists: %w", err)
	}
	var lists Lists
	var versions []int64
	for rows.Next() {
		var l WebList
		var version int64
		if err := rows.Scan(&l.ID, &l.Kind, &l.Name, &l.Where.URL, &l.Where.Title, &l.Where.Text, &l.Where.Ads, &version); err != nil {
			rows.Close()
			return nil, fmt.Errorf("failed to read a website list: %w", err)
		}
		lists = append(lists, l)
		versions = append(versions, version)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range lists {
		if lists[i].Entries, err = cache.entries(q, lists[i].ID, versions[i]); err != nil {
			return nil, err
		}
		if !lists[i].IsExceptions() {
			if lists[i].Actions, err = listActions(q, lists[i].ID); err != nil {
				return nil, err
			}
		}
	}
	return lists, nil
}

// entryCache keeps lists' entries by their entries_version, which triggers
// set to a new random number whenever a list's entries change, in whatever
// program (entryVersionTriggers). The daemon reads the active blocks
// several times a cycle: a 100,000-site list took 100 ms each time
// (principle 9; 2026-11-21, desktop).
type entryCache struct {
	sync.Mutex
	lists map[int]cachedEntries
}

type cachedEntries struct {
	version int64
	entries []string
}

// entries returns a list's entries, from the cache when its version is
// the one cached. A nil cache always reads. Version 0 (not set yet) is
// never cached.
func (c *entryCache) entries(q querier, listID int, version int64) ([]string, error) {
	if c != nil && version != 0 {
		c.Lock()
		hit, ok := c.lists[listID]
		c.Unlock()
		if ok && hit.version == version {
			return slices.Clone(hit.entries), nil
		}
	}
	// Read after the version: the entries are at least that new, so a
	// change in between is read again next time, never missed.
	entries, err := listEntries(q, listID)
	if err != nil || c == nil || version == 0 {
		return entries, err
	}
	c.Lock()
	if c.lists == nil {
		c.lists = map[int]cachedEntries{}
	}
	c.lists[listID] = cachedEntries{version: version, entries: slices.Clone(entries)}
	c.Unlock()
	return entries, nil
}

// entryVersionTriggers keep web_lists.entries_version new: any insert,
// change or removal of an entry gives its list a new random version, and so
// does a new list. Random, not counted up: a database put back from a copy
// never repeats a version the cache holds.
const entryVersionTriggers = `
CREATE TRIGGER IF NOT EXISTS web_list_entries_ins AFTER INSERT ON web_list_entries BEGIN
    UPDATE web_lists SET entries_version = random() WHERE id = NEW.list_id;
END;
CREATE TRIGGER IF NOT EXISTS web_list_entries_del AFTER DELETE ON web_list_entries BEGIN
    UPDATE web_lists SET entries_version = random() WHERE id = OLD.list_id;
END;
CREATE TRIGGER IF NOT EXISTS web_list_entries_upd AFTER UPDATE ON web_list_entries BEGIN
    UPDATE web_lists SET entries_version = random() WHERE id IN (OLD.list_id, NEW.list_id);
END;
CREATE TRIGGER IF NOT EXISTS web_lists_new AFTER INSERT ON web_lists BEGIN
    UPDATE web_lists SET entries_version = random() WHERE id = NEW.id;
END;`

// migrateEntryVersions adds the triggers and gives lists from before them a
// version.
func migrateEntryVersions(conn *sql.DB) error {
	if _, err := conn.Exec(entryVersionTriggers); err != nil {
		return fmt.Errorf("entry versions: %w", err)
	}
	var unset int
	if err := conn.QueryRow("SELECT COUNT(*) FROM web_lists WHERE entries_version = 0").Scan(&unset); err != nil || unset == 0 {
		return err
	}
	_, err := conn.Exec("UPDATE web_lists SET entries_version = random() WHERE entries_version = 0")
	return err
}

func listEntries(q querier, listID int) ([]string, error) {
	rows, err := q.Query("SELECT entry FROM web_list_entries WHERE list_id = ? ORDER BY id", listID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch list entries: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, fmt.Errorf("failed to read a list entry: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func listActions(q querier, listID int) ([]ListAction, error) {
	rows, err := q.Query("SELECT action, distinct_min, hits_min, rate_min, param, minutes FROM web_list_actions WHERE list_id = ?", listID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch list actions: %w", err)
	}
	defer rows.Close()
	found := map[string]ListAction{}
	for rows.Next() {
		var a ListAction
		if err := rows.Scan(&a.Name, &a.Distinct, &a.Hits, &a.Rate, &a.Param, &a.Minutes); err != nil {
			return nil, fmt.Errorf("failed to read a list action: %w", err)
		}
		a.Distinct, a.Hits = max(a.Distinct, 1), max(a.Hits, 1)
		found[a.Name] = a
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []ListAction{}
	for _, name := range ActionNames {
		if a, ok := found[name]; ok {
			out = append(out, a)
		}
	}
	return out, nil
}

// ListID returns the id of a block's named list.
func (d *DB) ListID(blockName, listName string) (int, error) {
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return 0, err
	}
	var id int
	err = d.conn.QueryRow("SELECT id FROM web_lists WHERE block_id = ? AND name = ?", blockID, listName).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("block '%s' has no list named '%s'", blockName, listName)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to find list '%s': %w", listName, err)
	}
	return id, nil
}

// GetList returns one list by id.
func (d *DB) GetList(listID int) (WebList, error) {
	var l WebList
	err := d.conn.QueryRow("SELECT id, kind, name, in_url, in_title, in_text, in_ads FROM web_lists WHERE id = ?", listID).
		Scan(&l.ID, &l.Kind, &l.Name, &l.Where.URL, &l.Where.Title, &l.Where.Text, &l.Where.Ads)
	if err == sql.ErrNoRows {
		return l, fmt.Errorf("the list no longer exists")
	}
	if err != nil {
		return l, fmt.Errorf("failed to fetch list: %w", err)
	}
	if l.Entries, err = listEntries(d.conn, l.ID); err != nil {
		return l, err
	}
	if !l.IsExceptions() {
		l.Actions, err = listActions(d.conn, l.ID)
	}
	return l, err
}

func createList(q querier, blockID int, kind, name string, where Where) (int, error) {
	if kind != ListBlacklist && kind != ListExceptions {
		return 0, fmt.Errorf("invalid list kind %q", kind)
	}
	if err := CheckListName(name); err != nil {
		return 0, err
	}
	if err := CheckScope(where); err != nil {
		return 0, err
	}
	if listNameTaken(q, blockID, 0, name) {
		return 0, fmt.Errorf("the block already has a list named '%s' (names differ only in case)", name)
	}
	res, err := q.Exec("INSERT INTO web_lists (block_id, kind, name, in_url, in_title, in_text, in_ads) VALUES (?, ?, ?, ?, ?, ?, ?)",
		blockID, kind, name, where.URL, where.Title, where.Text, where.Ads)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, fmt.Errorf("the block already has a list named '%s'", name)
		}
		return 0, fmt.Errorf("failed to create list '%s': %w", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if kind == ListBlacklist {
		if _, err := q.Exec("INSERT INTO web_list_actions (list_id, action, distinct_min) VALUES (?, ?, 1)", id, ActionBlock); err != nil {
			return 0, fmt.Errorf("failed to set the list's action: %w", err)
		}
	}
	return int(id), nil
}

// listNameTaken reports whether another list of the block (not except) has
// the name in any case: lists are found by name without regard to case, as
// phrase sets are (phraseSetNameTaken).
func listNameTaken(q querier, blockID, except int, name string) bool {
	var n int
	err := q.QueryRow("SELECT COUNT(*) FROM web_lists WHERE block_id = ? AND id != ? AND lower(name) = lower(?)",
		blockID, except, name).Scan(&n)
	return err == nil && n > 0
}

// CreateList adds a list to a block. A new blacklist blocks what it
// matches.
func (d *DB) CreateList(blockName, kind, name string, where Where) (int, error) {
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return 0, err
	}
	return createList(d.conn, blockID, kind, name, where)
}

// RenameList renames a list. Queued edits name lists by id, so they follow.
func (d *DB) RenameList(listID int, name string) error {
	if err := CheckListName(name); err != nil {
		return err
	}
	var blockID int
	if err := d.conn.QueryRow("SELECT block_id FROM web_lists WHERE id = ?", listID).Scan(&blockID); err == nil &&
		listNameTaken(d.conn, blockID, listID, name) {
		return fmt.Errorf("the block already has a list named '%s' (names differ only in case)", name)
	}
	if _, err := d.conn.Exec("UPDATE web_lists SET name = ? WHERE id = ?", name, listID); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("the block already has a list named '%s'", name)
		}
		return fmt.Errorf("failed to rename the list: %w", err)
	}
	return nil
}

// DeleteList removes a list with its entries.
func (d *DB) DeleteList(listID int) error {
	if _, err := d.conn.Exec("DELETE FROM web_lists WHERE id = ?", listID); err != nil {
		return fmt.Errorf("failed to remove the list: %w", err)
	}
	return nil
}

// SetListWhere sets where a list looks.
func (d *DB) SetListWhere(listID int, where Where) error {
	if err := CheckScope(where); err != nil {
		return err
	}
	_, err := d.conn.Exec("UPDATE web_lists SET in_url = ?, in_title = ?, in_text = ?, in_ads = ? WHERE id = ?",
		where.URL, where.Title, where.Text, where.Ads, listID)
	if err != nil {
		return fmt.Errorf("failed to set where the list looks: %w", err)
	}
	return nil
}

func addListEntries(q querier, listID int, entries []string) error {
	have, err := listEntries(q, listID)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, e := range have {
		seen[strings.ToLower(e)] = true
	}
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" || seen[strings.ToLower(e)] {
			continue
		}
		seen[strings.ToLower(e)] = true
		if _, err := q.Exec("INSERT INTO web_list_entries (list_id, entry) VALUES (?, ?)", listID, e); err != nil {
			return fmt.Errorf("failed to add %q: %w", e, err)
		}
	}
	return nil
}

// AddListEntries adds entries to a list, skipping ones it has (ignoring case).
// One transaction: written one by one, 5,000 entries took 215 s on a
// spinning disk (2026-11-21, desktop), the daemon meanwhile finding the
// database locked.
func (d *DB) AddListEntries(listID int, entries []string) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := addListEntries(tx, listID, entries); err != nil {
		return err
	}
	return tx.Commit()
}

// RemoveListEntries removes entries from a list, in one transaction.
func (d *DB) RemoveListEntries(listID int, entries []string) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, e := range entries {
		if _, err := tx.Exec("DELETE FROM web_list_entries WHERE list_id = ? AND entry = ?", listID, e); err != nil {
			return fmt.Errorf("failed to remove %q: %w", e, err)
		}
	}
	return tx.Commit()
}

// ReplaceListEntry swaps one entry of a list for another in one
// transaction, so a changed entry is never lost or doubled (R-WEAK-5).
func (d *DB) ReplaceListEntry(listID int, old, entry string) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM web_list_entries WHERE list_id = ? AND entry = ?", listID, old); err != nil {
		return fmt.Errorf("failed to remove %q: %w", old, err)
	}
	if err := addListEntries(tx, listID, []string{entry}); err != nil {
		return err
	}
	return tx.Commit()
}

// SetListAction turns one of a blacklist's actions on or off. Turning one on
// starts it at one entry for page text; an action that needs an address,
// an image or minutes is turned on with SetListActionParams instead.
func (d *DB) SetListAction(listID int, action string, on bool) error {
	if !IsAction(action) {
		return fmt.Errorf("unknown action %q (use %s)", action, strings.Join(ActionNames, ", "))
	}
	var err error
	if on {
		if err := CheckActionParams(action, "", 0); err != nil {
			return err
		}
		_, err = d.conn.Exec("INSERT OR IGNORE INTO web_list_actions (list_id, action, distinct_min) VALUES (?, ?, 1)", listID, action)
	} else {
		_, err = d.conn.Exec("DELETE FROM web_list_actions WHERE list_id = ? AND action = ?", listID, action)
	}
	if err != nil {
		return fmt.Errorf("failed to set the %s action: %w", action, err)
	}
	return nil
}

// SetListActionParams turns an action on with its address, image or minutes,
// or changes them if it is on.
func (d *DB) SetListActionParams(listID int, action, param string, minutes int) error {
	if !IsAction(action) {
		return fmt.Errorf("unknown action %q (use %s)", action, strings.Join(ActionNames, ", "))
	}
	if err := CheckActionParams(action, param, minutes); err != nil {
		return err
	}
	_, err := d.conn.Exec(`INSERT INTO web_list_actions (list_id, action, distinct_min, param, minutes) VALUES (?, ?, 1, ?, ?)
		ON CONFLICT(list_id, action) DO UPDATE SET param = excluded.param, minutes = excluded.minutes`,
		listID, action, param, minutes)
	if err != nil {
		return fmt.Errorf("failed to set the %s action: %w", action, err)
	}
	return nil
}

// SetListThreshold sets how many different entries of a blacklist a page's
// text must contain before the action applies. The action must be on.
func (d *DB) SetListThreshold(listID int, action string, n int) error {
	return d.setListCount(listID, action, "distinct_min", n)
}

// SetListHits sets how many times in all a blacklist's entries must occur in
// a page's text before the action applies. The action must be on.
func (d *DB) SetListHits(listID int, action string, n int) error {
	return d.setListCount(listID, action, "hits_min", n)
}

// SetListRate sets the rate (per 1,000 words; 0: none) a page's text needs
// for a blacklist's action (§3.9). The action must be on.
func (d *DB) SetListRate(listID int, action string, perMille int) error {
	if perMille < 0 || perMille > MaxRate {
		return fmt.Errorf("a rate is from 0.1 to 100 percent")
	}
	res, err := d.conn.Exec("UPDATE web_list_actions SET rate_min = ? WHERE list_id = ? AND action = ?", perMille, listID, action)
	if err != nil {
		return fmt.Errorf("failed to set the %s rate: %w", action, err)
	}
	if count, _ := res.RowsAffected(); count == 0 {
		return fmt.Errorf("the list doesn't take the %s action; turn it on first", action)
	}
	return nil
}

// setListCount sets one of an action's counts (column is a constant).
func (d *DB) setListCount(listID int, action, column string, n int) error {
	if !IsAction(action) {
		return fmt.Errorf("unknown action %q (use %s)", action, strings.Join(ActionNames, ", "))
	}
	if n < 1 || n > MaxThreshold {
		return fmt.Errorf("a threshold is a number from 1 to %d", MaxThreshold)
	}
	res, err := d.conn.Exec("UPDATE web_list_actions SET "+column+" = ? WHERE list_id = ? AND action = ?", n, listID, action)
	if err != nil {
		return fmt.Errorf("failed to set the %s threshold: %w", action, err)
	}
	if count, _ := res.RowsAffected(); count == 0 {
		return fmt.Errorf("the list doesn't take the %s action; turn it on first", action)
	}
	return nil
}

// copyLists copies a block's lists into another block.
func (d *DB) copyLists(from Lists, toBlock string) error {
	blockID, err := d.getBlockID(toBlock)
	if err != nil {
		return err
	}
	if _, err := d.conn.Exec("DELETE FROM web_lists WHERE block_id = ?", blockID); err != nil {
		return err
	}
	for _, l := range from {
		id, err := createList(d.conn, blockID, l.Kind, l.Name, l.Where)
		if err != nil {
			return err
		}
		if err := addListEntries(d.conn, id, l.Entries); err != nil {
			return err
		}
		if !l.IsExceptions() {
			if _, err := d.conn.Exec("DELETE FROM web_list_actions WHERE list_id = ?", id); err != nil {
				return err
			}
			for _, a := range l.Actions {
				if _, err := d.conn.Exec("INSERT INTO web_list_actions (list_id, action, distinct_min, hits_min, rate_min, param, minutes) VALUES (?, ?, ?, ?, ?, ?, ?)",
					id, a.Name, max(a.Distinct, 1), max(a.Hits, 1), min(max(a.Rate, 0), MaxRate), a.Param, a.Minutes); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// SetLists replaces a block's lists (used by import).
func (d *DB) SetLists(blockName string, lists Lists) error {
	return d.copyLists(lists, blockName)
}

// createDefaultLists gives a new block an empty blacklist and exception list.
func createDefaultLists(q querier, blockID int) error {
	if _, err := createList(q, blockID, ListBlacklist, DefaultBlockedList, Where{URL: true}); err != nil {
		return err
	}
	id, err := createList(q, blockID, ListExceptions, DefaultExceptionsList, Where{URL: true})
	if err != nil {
		return err
	}
	// R-LIST-3: local files stay reachable in every new block.
	return addListEntries(q, int(id), []string{FileException})
}

// FileException is the exception every new block starts with (R-LIST-3).
const FileException = "file://*"

// legacyStrings reads one column of an old per-block table.
func legacyStrings(q querier, query string, blockID int) ([]string, error) {
	rows, err := q.Query(query, blockID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// migrateWebLists turns the sites, patterns, exceptions and page keywords of
// blocks from before lists into lists, once per block:
//   - "Blocked" (address): the sites and patterns, with the block's actions.
//     An allowlist block becomes *.* plus its patterns.
//   - "Exceptions" (address): the exceptions, plus an allowlist's sites.
//   - "Rendered terms on page" (text), if it had any: the keywords, with the block's
//     actions at its thresholds.
func migrateWebLists(conn *sql.DB) error {
	if err := addColumnIfMissing(conn, "blocks", "lists_ready", "BOOLEAN NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	for _, c := range []struct{ col, def string }{
		{"hits_min", "INTEGER NOT NULL DEFAULT 1"},
		{"param", "TEXT NOT NULL DEFAULT ''"},
		{"minutes", "INTEGER NOT NULL DEFAULT 0"},
	} {
		if err := addColumnIfMissing(conn, "web_list_actions", c.col, c.def); err != nil {
			return err
		}
	}
	ids, err := func() ([]int, error) {
		rows, err := conn.Query("SELECT id FROM blocks WHERE lists_ready = 0")
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var ids []int
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		return ids, rows.Err()
	}()
	if err != nil {
		return fmt.Errorf("failed to find blocks to move to lists: %w", err)
	}
	for _, id := range ids {
		if err := migrateBlockLists(conn, id); err != nil {
			return fmt.Errorf("failed to move block %d to lists: %w", id, err)
		}
	}
	return nil
}

func migrateBlockLists(conn *sql.DB, blockID int) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var mode string
	var actions Actions
	if err := tx.QueryRow("SELECT mode, act_block, act_greyscale, act_no_images FROM blocks WHERE id = ?", blockID).
		Scan(&mode, &actions.Block, &actions.Greyscale, &actions.NoImages); err != nil {
		return err
	}
	sites, err := legacyStrings(tx, "SELECT domain FROM sites WHERE block_id = ? ORDER BY id", blockID)
	if err != nil {
		return err
	}
	patterns, err := legacyStrings(tx, "SELECT pattern FROM patterns WHERE block_id = ? ORDER BY id", blockID)
	if err != nil {
		return err
	}
	exceptions, err := legacyStrings(tx, "SELECT pattern FROM exceptions WHERE block_id = ? ORDER BY id", blockID)
	if err != nil {
		return err
	}
	keywords, err := legacyStrings(tx, "SELECT keyword FROM page_keywords WHERE block_id = ? ORDER BY id", blockID)
	if err != nil {
		return err
	}
	thresholds := DefaultThresholds
	if rows, err := tx.Query("SELECT action, count FROM keyword_thresholds WHERE block_id = ?", blockID); err == nil {
		for rows.Next() {
			var action string
			var n int
			if rows.Scan(&action, &n) == nil && IsAction(action) && n >= 1 {
				thresholds = thresholds.With(action, n)
			}
		}
		rows.Close()
	}
	if !actions.Any() {
		actions = DefaultActions
	}

	blocked := append(append([]string{}, sites...), patterns...)
	allowed := exceptions
	if mode == ModeAllowlist {
		blocked = append([]string{"*.*"}, patterns...)
		allowed = append(append([]string{}, sites...), exceptions...)
	}

	setActions := func(listID int, withThresholds bool) error {
		if _, err := tx.Exec("DELETE FROM web_list_actions WHERE list_id = ?", listID); err != nil {
			return err
		}
		for _, name := range actions.List() {
			n := 1
			if withThresholds {
				n = thresholds.Get(name)
			}
			if _, err := tx.Exec("INSERT INTO web_list_actions (list_id, action, distinct_min) VALUES (?, ?, ?)", listID, name, n); err != nil {
				return err
			}
		}
		return nil
	}

	id, err := createList(tx, blockID, ListBlacklist, DefaultBlockedList, Where{URL: true})
	if err != nil {
		return err
	}
	if err := addListEntries(tx, id, blocked); err != nil {
		return err
	}
	if err := setActions(id, false); err != nil {
		return err
	}
	if id, err = createList(tx, blockID, ListExceptions, DefaultExceptionsList, Where{URL: true}); err != nil {
		return err
	}
	if err := addListEntries(tx, id, allowed); err != nil {
		return err
	}
	if len(keywords) > 0 {
		if id, err = createList(tx, blockID, ListBlacklist, DefaultWordsList, Where{Text: true}); err != nil {
			return err
		}
		if err := addListEntries(tx, id, keywords); err != nil {
			return err
		}
		if err := setActions(id, true); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("UPDATE blocks SET lists_ready = 1, mode = ? WHERE id = ?", ModeBlocklist, blockID); err != nil {
		return err
	}
	return tx.Commit()
}

// migrateWordsListName renames the default page text list from its old name
// "Page words" to DefaultWordsList (the glossary's words, §3.14), in every
// block that has no list of the new name yet.
func migrateWordsListName(conn *sql.DB) error {
	_, err := conn.Exec(`UPDATE web_lists SET name = ? WHERE name = ? AND NOT EXISTS
		(SELECT 1 FROM web_lists w WHERE w.block_id = web_lists.block_id AND lower(w.name) = lower(?))`,
		DefaultWordsList, oldWordsList, DefaultWordsList)
	if err != nil {
		return fmt.Errorf("renaming the Page words lists: %w", err)
	}
	return nil
}
