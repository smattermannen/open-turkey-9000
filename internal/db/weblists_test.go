package db

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"
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
		!reflect.DeepEqual(blocked.Actions, []ListAction{{Name: ActionBlock, Distinct: 1}}) {
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
	want := []ListAction{{Name: ActionBlock, Distinct: 3}, {Name: ActionGreyscale, Distinct: 1}}
	if l, _ := d.GetList(id); !reflect.DeepEqual(l.Actions, want) {
		t.Errorf("actions = %+v; want %+v", l.Actions, want)
	}

	// Queued edits name the list by id, so they survive a rename.
	if err := d.RenameList(id, "Page words"); err != nil {
		t.Fatal(err)
	}
	for _, e := range []Edit{
		{Action: EditListSetThreshold, List: id, Values: []string{ActionGreyscale, "2"}},
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
		!reflect.DeepEqual(l.Actions, []ListAction{{Name: ActionGreyscale, Distinct: 2}}) {
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
	both := []ListAction{{Name: ActionBlock, Distinct: 1}, {Name: ActionGreyscale, Distinct: 1}}
	want := Lists{
		{Kind: ListBlacklist, Name: DefaultBlockedList, Where: Where{URL: true}, Entries: []string{"reddit.com", "*casino*"}, Actions: both},
		{Kind: ListBlacklist, Name: DefaultWordsList, Where: Where{Text: true}, Entries: []string{"nsfw"},
			Actions: []ListAction{{Name: ActionBlock, Distinct: 2}, {Name: ActionGreyscale, Distinct: 1}}},
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
