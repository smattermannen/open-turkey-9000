package db

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/schedule"
)

func openTest(t *testing.T) (*DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.db")
	d, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d, path
}

func TestNewBlockLists(t *testing.T) {
	d, _ := openTest(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	b, err := d.GetBlock("b")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Lists) != 2 {
		t.Fatalf("lists = %+v; want Blocked and Exceptions", b.Lists)
	}
	blocked, exceptions := b.Lists[0], b.Lists[1]
	if blocked.Name != DefaultBlockedList || blocked.IsExceptions() || blocked.Where != (Where{URL: true}) ||
		!reflect.DeepEqual(blocked.Actions, []ListAction{{Name: ActionBlock, Distinct: 1, Hits: 1}}) {
		t.Errorf("Blocked = %+v", blocked)
	}
	if exceptions.Name != DefaultExceptionsList || !exceptions.IsExceptions() || len(exceptions.Actions) != 0 {
		t.Errorf("Exceptions = %+v", exceptions)
	}
	if !b.Empty() {
		t.Error("a new block isn't empty")
	}
}

func TestListEdits(t *testing.T) {
	d, _ := openTest(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	id, err := d.CreateList("b", ListBlacklist, "Words", Where{Text: true, Title: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateList("b", ListBlacklist, "Words", Where{URL: true}); err == nil {
		t.Error("a second list named Words was accepted")
	}
	if _, err := d.CreateList("b", ListBlacklist, " ", Where{URL: true}); err == nil {
		t.Error("a blank name was accepted")
	}
	if err := d.AddListEntries(id, []string{"NSFW", "nsfw", " 18+ ", ""}); err != nil {
		t.Fatal(err)
	}
	l, err := d.GetList(id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(l.Entries, []string{"NSFW", "18+"}) {
		t.Fatalf("entries = %q; want NSFW and 18+ once each", l.Entries)
	}
	if err := d.SetListAction(id, ActionGreyscale, true); err != nil {
		t.Fatal(err)
	}
	if err := d.SetListThreshold(id, ActionBlock, 3); err != nil {
		t.Fatal(err)
	}
	if err := d.SetListThreshold(id, ActionNoImages, 2); err == nil {
		t.Error("a threshold for an action the list doesn't take was accepted")
	}
	if err := d.SetListThreshold(id, ActionBlock, 0); err == nil {
		t.Error("threshold 0 accepted")
	}
	want := []ListAction{{Name: ActionBlock, Distinct: 3, Hits: 1}, {Name: ActionGreyscale, Distinct: 1, Hits: 1}}
	if l, _ := d.GetList(id); !reflect.DeepEqual(l.Actions, want) {
		t.Errorf("actions = %+v; want %+v", l.Actions, want)
	}

	// Queued edits name the list by id, so they survive a rename.
	if err := d.RenameList(id, "Page words"); err != nil {
		t.Fatal(err)
	}
	for _, e := range []Edit{
		{Action: EditListSetThreshold, List: id, Values: []string{ActionGreyscale, "2"}},
		{Action: EditListSetHits, List: id, Values: []string{ActionGreyscale, "4"}},
		{Action: EditListUnsetAction, List: id, Values: []string{ActionBlock}},
		{Action: EditListWhere, List: id, Values: []string{PlaceTitle, "off"}},
		{Action: EditListRemove, List: id, Values: []string{"18+"}},
	} {
		if err := d.ApplyEdit("b", e); err != nil {
			t.Fatalf("%s: %v", e, err)
		}
	}
	l, _ = d.GetList(id)
	if l.Name != "Page words" || l.Where != (Where{Text: true}) || !reflect.DeepEqual(l.Entries, []string{"NSFW"}) ||
		!reflect.DeepEqual(l.Actions, []ListAction{{Name: ActionGreyscale, Distinct: 2, Hits: 4}}) {
		t.Errorf("after queued edits: %+v", l)
	}

	// Copies keep every list.
	if err := d.DuplicateBlock("b", "copy"); err != nil {
		t.Fatal(err)
	}
	orig, _ := d.GetBlock("b")
	copied, _ := d.GetBlock("copy")
	for i := range orig.Lists {
		orig.Lists[i].ID, copied.Lists[i].ID = 0, 0
	}
	if !reflect.DeepEqual(orig.Lists, copied.Lists) {
		t.Errorf("copy lists = %+v; want %+v", copied.Lists, orig.Lists)
	}

	// Removing the last entry of an active block with nothing else
	// switches it off.
	if err := d.ActivateBlock("b", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyEdit("b", Edit{Action: EditListRemove, List: id, Values: []string{"NSFW"}}); err != nil {
		t.Fatal(err)
	}
	if b, _ := d.GetBlock("b"); b.Active {
		t.Error("an emptied block stayed on")
	}
}

// TestMigrateWebLists turns a block from before lists into lists, as an
// upgraded install opens its database.
func TestMigrateWebLists(t *testing.T) {
	d, path := openTest(t)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.conn.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec("INSERT INTO blocks (name, mode, act_block, act_greyscale, lists_ready) VALUES ('old', 'blocklist', 1, 1, 0)")
	exec("INSERT INTO blocks (name, mode, lists_ready) VALUES ('allow', 'allowlist', 0)")
	exec("INSERT INTO sites (block_id, domain) SELECT id, 'reddit.com' FROM blocks WHERE name = 'old'")
	exec("INSERT INTO patterns (block_id, pattern) SELECT id, '*casino*' FROM blocks WHERE name = 'old'")
	exec("INSERT INTO exceptions (block_id, pattern) SELECT id, 'reddit.com/r/golang' FROM blocks WHERE name = 'old'")
	exec("INSERT INTO page_keywords (block_id, keyword) SELECT id, 'nsfw' FROM blocks WHERE name = 'old'")
	exec("INSERT INTO keyword_thresholds (block_id, action, count) SELECT id, 'block', 2 FROM blocks WHERE name = 'old'")
	exec("INSERT INTO sites (block_id, domain) SELECT id, 'work.com' FROM blocks WHERE name = 'allow'")
	exec("INSERT INTO patterns (block_id, pattern) SELECT id, '*ads*' FROM blocks WHERE name = 'allow'")
	exec("INSERT INTO pending_edits (block_id, action, payload, apply_at) SELECT id, 'remove-site', '{\"values\":[\"reddit.com\"]}', 0 FROM blocks WHERE name = 'old'")
	d.Close()

	d, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	old, err := d.GetBlock("old")
	if err != nil {
		t.Fatal(err)
	}
	both := []ListAction{{Name: ActionBlock, Distinct: 1, Hits: 1}, {Name: ActionGreyscale, Distinct: 1, Hits: 1}}
	want := Lists{
		{Kind: ListBlacklist, Name: DefaultBlockedList, Where: Where{URL: true}, Entries: []string{"reddit.com", "*casino*"}, Actions: both},
		{Kind: ListBlacklist, Name: DefaultWordsList, Where: Where{Text: true}, Entries: []string{"nsfw"},
			Actions: []ListAction{{Name: ActionBlock, Distinct: 2, Hits: 1}, {Name: ActionGreyscale, Distinct: 1, Hits: 1}}},
		{Kind: ListExceptions, Name: DefaultExceptionsList, Where: Where{URL: true}, Entries: []string{"reddit.com/r/golang"}},
	}
	for i := range old.Lists {
		old.Lists[i].ID = 0
	}
	if !reflect.DeepEqual(old.Lists, want) {
		t.Errorf("old block lists:\n got %+v\nwant %+v", old.Lists, want)
	}

	allow, _ := d.GetBlock("allow")
	blocked, _ := allow.Lists.Find(DefaultBlockedList)
	excepted, _ := allow.Lists.Find(DefaultExceptionsList)
	if !reflect.DeepEqual(blocked.Entries, []string{"*.*", "*ads*"}) || !reflect.DeepEqual(excepted.Entries, []string{"work.com"}) {
		t.Errorf("allowlist block: blocked %q, exceptions %q", blocked.Entries, excepted.Entries)
	}

	// An edit queued before lists still applies, to the list made from the
	// old one.
	pending, err := d.ListPendingEdits()
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending = %+v, %v", pending, err)
	}
	if err := d.ApplyEdit("old", pending[0].Edit); err != nil {
		t.Fatal(err)
	}
	old, _ = d.GetBlock("old")
	if l, _ := old.Lists.Find(DefaultBlockedList); !reflect.DeepEqual(l.Entries, []string{"*casino*"}) {
		t.Errorf("after the queued edit, Blocked = %q", l.Entries)
	}

	// Opening again doesn't migrate twice.
	d.Close()
	if d, err = OpenDB(path); err != nil {
		t.Fatal(err)
	}
	if again, _ := d.GetBlock("old"); len(again.Lists) != 3 {
		t.Errorf("lists after reopening = %d; want 3", len(again.Lists))
	}
}

// Two connections writing at once wait for each other instead of failing,
// and every connection enforces foreign keys (deleting a block deletes its
// lists).
func TestConcurrentWritesAndCascade(t *testing.T) {
	d, path := openTest(t)
	other, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	tx, err := d.conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("INSERT INTO blocks (name, lists_ready) VALUES ('held', 1)"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error)
	go func() { done <- other.CreateBlock("waiting") }()
	time.Sleep(200 * time.Millisecond)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("a write during another one failed: %v", err)
	}

	for i := 0; i < 4; i++ { // spread over the pool's connections
		name := fmt.Sprintf("b%d", i)
		if err := d.CreateBlock(name); err != nil {
			t.Fatal(err)
		}
		if err := d.RemoveBlock(name); err != nil {
			t.Fatal(err)
		}
	}
	var lists int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM web_lists WHERE block_id NOT IN (SELECT id FROM blocks)").Scan(&lists); err != nil {
		t.Fatal(err)
	}
	if lists != 0 {
		t.Errorf("%d lists outlived their block", lists)
	}
}

// R-LIST-3: every new block has the exception file://*.
func TestNewBlockSparesLocalFiles(t *testing.T) {
	d, _ := openTest(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	b, err := d.GetBlock("b")
	if err != nil {
		t.Fatal(err)
	}
	l, ok := b.Lists.Find(DefaultExceptionsList)
	if !ok || len(l.Entries) != 1 || l.Entries[0] != FileException {
		t.Errorf("exceptions of a new block = %+v", l)
	}
	if !b.Empty() {
		t.Error("the default exception made a new block enforce something")
	}
}

// Upgrading keeps the protection blocks with schedules had: they get a
// schedule lock with the old 2-hour lead; other blocks get none.
func TestScheduleLockMigration(t *testing.T) {
	d, path := openTest(t)
	for _, name := range []string{"sched", "plain"} {
		if err := d.CreateBlock(name); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.AddSchedule("sched", schedule.Window{Days: 127, Start: 540, End: 600}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("ALTER TABLE blocks DROP COLUMN schedule_lock_lead"); err != nil {
		t.Fatal(err)
	}
	d.Close()
	d, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if on, lead, _ := d.ScheduleLock("sched"); !on || lead != 120 {
		t.Errorf("scheduled block: lock %v, lead %d", on, lead)
	}
	if on, _, _ := d.ScheduleLock("plain"); on {
		t.Error("a block without schedules got a schedule lock")
	}
	if err := d.CreateBlock("new"); err != nil {
		t.Fatal(err)
	}
	if on, _, _ := d.ScheduleLock("new"); on {
		t.Error("a new block got a schedule lock")
	}
}

// §3.2: test mode is per list; a block in the old per-block test mode
// gets its title and text lists in test mode.
func TestTestModeMovesToLists(t *testing.T) {
	d, _ := openTest(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	words, err := d.CreateList("b", ListBlacklist, "Words", Where{Text: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetSetting(settingKeywordTest, `["b"]`); err != nil {
		t.Fatal(err)
	}
	if got := d.TestLists(); len(got) != 1 || got[0] != words {
		t.Errorf("test lists = %v; want only Words (%d)", got, words)
	}
	if v, _ := d.GetSetting(settingKeywordTest); v != "" {
		t.Error("the old setting stayed")
	}
}

// §3.2: lists have no types. A pair split by the old type upgrade, "<name>"
// (url) and "<name> terms" (title/text), is joined back into "<name>" when
// that changes nothing; any difference keeps both. Opening again changes
// nothing more. An old database still has the type column.
func TestJoinSplitLists(t *testing.T) {
	d, path := openTest(t)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.conn.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	// The column an older version had, with its values.
	exec("ALTER TABLE web_lists ADD COLUMN type TEXT NOT NULL DEFAULT ''")
	exec("ALTER TABLE clusters ADD COLUMN type TEXT NOT NULL DEFAULT ''")
	entries := []string{"reddit.com", "*hentai*", "porn"}
	pair := func(block, name string, kind string, termEntries []string, termsWhere Where) (int, int) {
		t.Helper()
		if _, err := d.GetBlock(block); err != nil {
			if err := d.CreateBlock(block); err != nil {
				t.Fatal(err)
			}
		}
		url, err := d.CreateList(block, kind, name, Where{URL: true})
		if err != nil {
			t.Fatal(err)
		}
		terms, err := d.CreateList(block, kind, name+" terms", termsWhere)
		if err != nil {
			t.Fatal(err)
		}
		if err := d.AddListEntries(url, entries); err != nil {
			t.Fatal(err)
		}
		if err := d.AddListEntries(terms, termEntries); err != nil {
			t.Fatal(err)
		}
		for _, id := range []int{url, terms} {
			if kind == ListBlacklist {
				if err := d.SetListAction(id, ActionGreyscale, true); err != nil {
					t.Fatal(err)
				}
				if err := d.SetListThreshold(id, ActionGreyscale, 3); err != nil {
					t.Fatal(err)
				}
			}
		}
		exec("UPDATE web_lists SET type = 'address' WHERE id = ?", url)
		exec("UPDATE web_lists SET type = 'term' WHERE id = ?", terms)
		return url, terms
	}
	// Joined: same entries in another case and order.
	same := []string{"PORN", "reddit.com", "*Hentai*"}
	joinedID, _ := pair("join", "Mine", ListBlacklist, same, Where{Title: true, Text: true})
	exceptID, _ := pair("join", "Spare", ListExceptions, same, Where{Text: true})
	// Kept apart: other entries, another threshold, test mode, a queued edit,
	// another kind, another name.
	pair("entries", "Mine", ListBlacklist, []string{"reddit.com", "porn"}, Where{Text: true})
	_, threshold := pair("threshold", "Mine", ListBlacklist, same, Where{Text: true})
	if err := d.SetListThreshold(threshold, ActionGreyscale, 2); err != nil {
		t.Fatal(err)
	}
	_, testID := pair("test", "Mine", ListBlacklist, same, Where{Text: true})
	if err := d.SetTestList(testID, true); err != nil {
		t.Fatal(err)
	}
	_, queued := pair("queued", "Mine", ListBlacklist, same, Where{Text: true})
	if err := d.QueueEdit("queued", Edit{Action: EditListAdd, List: queued, ListName: "Mine terms", Values: []string{"x"}}, time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	pair("kind", "Mine", ListBlacklist, nil, Where{Text: true})
	if id, err := d.CreateList("kind", ListExceptions, "Other terms", Where{Text: true}); err != nil {
		t.Fatal(err)
	} else if err := d.AddListEntries(id, entries); err != nil {
		t.Fatal(err)
	}
	if id, err := d.CreateList("kind", ListBlacklist, "Other", Where{URL: true}); err != nil {
		t.Fatal(err)
	} else if err := d.AddListEntries(id, entries); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateList("kind", ListBlacklist, "Third terms 2", Where{Text: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateList("kind", ListBlacklist, "Third", Where{URL: true}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name  string
		where Where
		items []string
	}{{"mix", Where{URL: true}, entries}, {"mix terms", Where{Text: true}, same},
		{"apart", Where{URL: true}, entries}, {"apart terms", Where{Title: true}, []string{"porn"}}} {
		if err := d.CreateCluster(c.name, c.where); err != nil {
			t.Fatal(err)
		}
		cl, _ := d.GetCluster(c.name)
		if _, err := d.AddClusterEntries(cl.ID, c.items); err != nil {
			t.Fatal(err)
		}
	}

	type state map[string]Lists
	read := func() state {
		t.Helper()
		out := state{}
		for _, name := range []string{"join", "entries", "threshold", "test", "queued", "kind"} {
			b, err := d.GetBlock(name)
			if err != nil {
				t.Fatal(err)
			}
			out[name] = b.Lists
		}
		return out
	}
	names := func(ls Lists) []string {
		var out []string
		for _, l := range ls {
			out = append(out, l.Name)
		}
		return out
	}
	before := read()
	d.Close()
	var err error
	if d, err = OpenDB(path); err != nil {
		t.Fatal(err)
	}
	after := read()

	joined := after["join"]
	if got := names(joined); !reflect.DeepEqual(got, []string{DefaultBlockedList, "Mine", DefaultExceptionsList, "Spare"}) {
		t.Fatalf("joined block lists = %q", got)
	}
	mine, _ := joined.Find("Mine")
	if mine.ID != joinedID || mine.Where != (Where{URL: true, Title: true, Text: true}) ||
		!reflect.DeepEqual(mine.Entries, entries) {
		t.Errorf("joined list = %+v", mine)
	}
	if a, ok := mine.Action(ActionGreyscale); !ok || a.Distinct != 3 || !mine.HasAction(ActionBlock) {
		t.Errorf("joined list actions = %+v", mine.Actions)
	}
	spare, _ := joined.Find("Spare")
	if spare.ID != exceptID || spare.Where != (Where{URL: true, Text: true}) {
		t.Errorf("joined exception list = %+v", spare)
	}
	for _, name := range []string{"entries", "threshold", "test", "queued", "kind"} {
		if !reflect.DeepEqual(after[name], before[name]) {
			t.Errorf("block %s changed:\n got %+v\nwant %+v", name, after[name], before[name])
		}
	}
	if got := d.TestLists(); !reflect.DeepEqual(got, []int{testID}) {
		t.Errorf("test lists = %v", got)
	}
	if c, err := d.GetCluster("mix"); err != nil || c.Where != (Where{URL: true, Text: true}) || !reflect.DeepEqual(c.Entries, entries) {
		t.Errorf("joined list preset = %+v, %v", c, err)
	}
	if ok, _ := d.ClusterExists("mix terms"); ok {
		t.Error("the joined list preset's terms half stayed")
	}
	for _, name := range []string{"apart", "apart terms"} {
		if ok, _ := d.ClusterExists(name); !ok {
			t.Errorf("list preset %s was joined", name)
		}
	}

	// Idempotent: opening again changes nothing.
	d.Close()
	if d, err = OpenDB(path); err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if again := read(); !reflect.DeepEqual(again, after) {
		t.Errorf("a second open changed lists:\n got %+v\nwant %+v", again, after)
	}
}

// §3.2: one list holds sites, URL entries and rendered terms. With the url
// scope, sites and URL entries match the address and the rest are words in
// it; with title or text every entry is a rendered term.
func TestMixedListEntries(t *testing.T) {
	l := WebList{Kind: ListExceptions, Where: Where{URL: true, Text: true},
		Entries: []string{"reddit.com", "reddit.com/r/x", "*hot*girl*", "*.xxx", "*.*", "porn", "long cat", "18+", "**"}}
	if got, want := l.Addresses(), []string{"reddit.com", "reddit.com/r/x", "*hot*girl*", "*.xxx", "*.*"}; !reflect.DeepEqual(got, want) {
		t.Errorf("addresses = %q; want %q", got, want)
	}
	if got, want := l.URLTerms(), []string{"porn", "long cat", "18+", "**"}; !reflect.DeepEqual(got, want) {
		t.Errorf("url terms = %q; want %q", got, want)
	}
	ls := Lists{l}
	if got := ls.ExceptionTerms(PlaceText); !reflect.DeepEqual(got, l.Entries) {
		t.Errorf("text exceptions = %q", got)
	}
	if got := ls.ExceptionTerms(PlaceTitle); got != nil {
		t.Errorf("title exceptions = %q", got)
	}
	l.Where = Where{Text: true}
	if len(l.Addresses())+len(l.URLTerms()) != 0 {
		t.Error("a list not looking at the url gave url entries")
	}
}

// A list name that differs from another list's of the block only in case is
// refused, on create and on rename; renaming a list to its own name in another
// case is fine.
func TestListNamesIgnoreCase(t *testing.T) {
	d, _ := openTest(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	where := Where{URL: true}
	id, err := d.CreateList("b", ListBlacklist, "News", where)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateList("b", ListBlacklist, "news", where); err == nil {
		t.Error("created 'news' next to 'News'")
	}
	other, err := d.CreateList("b", ListBlacklist, "Other", where)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.RenameList(other, "NEWS"); err == nil {
		t.Error("renamed 'Other' to 'NEWS' next to 'News'")
	}
	if err := d.RenameList(id, "NEWS"); err != nil {
		t.Errorf("changing a list's own name's case: %v", err)
	}
}

// Lists of one block that differ only in case, from before the rule, are
// separated on open: the later one becomes "<name>-2".
func TestListCaseNamesMigrated(t *testing.T) {
	d, path := openTest(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	where := Where{URL: true}
	if _, err := d.CreateList("b", ListBlacklist, "News", where); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateList("b", ListBlacklist, "news-2", where); err != nil {
		t.Fatal(err)
	}
	// Plant the old pair behind the rule's back.
	if _, err := d.conn.Exec("INSERT INTO web_lists (block_id, kind, name, in_url) SELECT block_id, kind, 'NEWS', 1 FROM web_lists WHERE name = 'News'"); err != nil {
		t.Fatal(err)
	}
	d.Close()
	d2, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d2.Close()
	b, err := d2.GetBlock("b")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, l := range b.Lists {
		names = append(names, l.Name)
	}
	got := strings.Join(names, ",")
	for _, want := range []string{"News", "news-2", "NEWS-3"} {
		if !strings.Contains(got, want) {
			t.Errorf("lists %s lack %s", got, want)
		}
	}
}

// §3.14: the default page text list was "Page words"; every open renames
// it to "Rendered terms on page", unless the block already has that name.
func TestWordsListRenamed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	d, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range []string{"a", "both"} {
		if err := d.CreateBlock(b); err != nil {
			t.Fatal(err)
		}
		if _, err := d.CreateList(b, ListBlacklist, "Page words", Where{Text: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.CreateList("both", ListBlacklist, "rendered terms on page", Where{Text: true}); err != nil {
		t.Fatal(err)
	}
	d.Close()
	if d, err = OpenDB(path); err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	a, _ := d.GetBlock("a")
	if _, ok := a.Lists.Find(DefaultWordsList); !ok {
		t.Errorf("a's lists: %+v", a.Lists)
	}
	both, _ := d.GetBlock("both")
	if _, ok := both.Lists.Find("Page words"); !ok {
		t.Error("renamed onto a list the block already had")
	}
}

// Principle 9 (huge lists never slow the computer): every kc9k command opens
// the database, and the open looks for split lists to join. With a
// 100,000-site list it read and deduplicated every entry one by one: over a
// minute per command (2026-11-21, desktop). Now only a pair's entries are
// read; a pair of huge lists still joins.
func TestPrinciple9_OpenWithHugeList(t *testing.T) {
	d, path := openTest(t)
	if err := d.CreateBlock("big"); err != nil {
		t.Fatal(err)
	}
	b, err := d.GetBlock("big")
	if err != nil {
		t.Fatal(err)
	}
	sites := make([]string, 100000)
	for i := range sites {
		sites[i] = fmt.Sprintf("site%d.example", i)
	}
	if err := d.AddListEntries(b.Lists[0].ID, sites); err != nil {
		t.Fatal(err)
	}
	// A split pair of 20,000 entries each, in a different order and case.
	url, err := d.CreateList("big", ListBlacklist, "Pair", Where{URL: true})
	if err != nil {
		t.Fatal(err)
	}
	terms, err := d.CreateList("big", ListBlacklist, "Pair terms", Where{Text: true})
	if err != nil {
		t.Fatal(err)
	}
	pair := sites[:20000]
	upper := make([]string, len(pair))
	for i, s := range pair {
		upper[len(pair)-1-i] = strings.ToUpper(s)
	}
	if err := d.AddListEntries(url, pair); err != nil {
		t.Fatal(err)
	}
	if err := d.AddListEntries(terms, upper); err != nil {
		t.Fatal(err)
	}
	d.Close()

	start := time.Now()
	d2, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d2.Close()
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("opening took %s", took)
	}
	if _, err := d2.GetList(terms); err == nil {
		t.Error("the pair of 20,000 entries wasn't joined")
	}
	start = time.Now()
	d3, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	d3.Close()
	if took := time.Since(start); took > time.Second {
		t.Errorf("opening again took %s", took)
	}
}

// Principle 9: the daemon keeps lists' entries between reads (entryCache),
// and a change written by another program (the CLI) is seen at the next
// read: added, removed, a list removed and another made.
func TestPrinciple9_EntryCacheFollowsChanges(t *testing.T) {
	daemon, path := openTest(t)
	cli, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	if err := cli.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	if err := cli.ActivateBlock("b", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	entries := func() []string {
		t.Helper()
		active, err := daemon.GetActiveBlocks()
		if err != nil || len(active) != 1 {
			t.Fatalf("%v %v", active, err)
		}
		var out []string
		for _, l := range active[0].Lists {
			if !l.IsExceptions() {
				out = append(out, l.Entries...)
			}
		}
		slices.Sort(out)
		return out
	}
	b, _ := cli.GetBlock("b")
	blocked := b.Lists[0].ID
	if err := cli.AddListEntries(blocked, []string{"a.example", "b.example"}); err != nil {
		t.Fatal(err)
	}
	if got := entries(); !slices.Equal(got, []string{"a.example", "b.example"}) {
		t.Fatalf("first read %v", got)
	}
	got := entries()
	got[0] = "changed by a caller" // a caller's change stays its own
	if got := entries(); !slices.Equal(got, []string{"a.example", "b.example"}) {
		t.Fatalf("cached read %v", got)
	}
	if err := cli.AddListEntries(blocked, []string{"c.example"}); err != nil {
		t.Fatal(err)
	}
	if got := entries(); !slices.Contains(got, "c.example") {
		t.Errorf("added: %v", got)
	}
	if err := cli.RemoveListEntries(blocked, []string{"a.example"}); err != nil {
		t.Fatal(err)
	}
	if got := entries(); slices.Contains(got, "a.example") {
		t.Errorf("removed: %v", got)
	}
	extra, err := cli.CreateList("b", ListBlacklist, "Extra", Where{URL: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.AddListEntries(extra, []string{"e.example"}); err != nil {
		t.Fatal(err)
	}
	if got := entries(); !slices.Contains(got, "e.example") {
		t.Errorf("new list: %v", got)
	}
	if err := cli.DeleteList(extra); err != nil {
		t.Fatal(err)
	}
	other, err := cli.CreateList("b", ListBlacklist, "Other", Where{URL: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.AddListEntries(other, []string{"d.example"}); err != nil {
		t.Fatal(err)
	}
	if got := entries(); !slices.Equal(got, []string{"b.example", "c.example", "d.example"}) {
		t.Errorf("list removed, another made: %v", got)
	}
	var unset int
	if err := daemon.conn.QueryRow("SELECT COUNT(*) FROM web_lists WHERE entries_version = 0").Scan(&unset); err != nil || unset != 0 {
		t.Errorf("%d lists without a version (%v)", unset, err)
	}
}
