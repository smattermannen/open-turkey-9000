package enforce

import (
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
)

// blacklist makes a blacklist for tests, taking the given actions (block if
// none) at one entry each.
func blacklist(name string, where db.Where, entries ...string) db.WebList {
	return db.WebList{Kind: db.ListBlacklist, Name: name, Where: where, Entries: entries,
		Actions: []db.ListAction{{Name: db.ActionBlock, Distinct: 1}}}
}

func withActions(l db.WebList, actions ...db.ListAction) db.WebList {
	l.Actions = actions
	return l
}

func exceptionList(name string, where db.Where, entries ...string) db.WebList {
	return db.WebList{Kind: db.ListExceptions, Name: name, Where: where, Entries: entries}
}

// addEntries adds entries to a block's list in the database.
func addEntries(d *db.DB, block, list string, entries ...string) error {
	id, err := d.ListID(block, list)
	if err != nil {
		return err
	}
	return d.AddListEntries(id, entries)
}

func mustAdd(t *testing.T, d *db.DB, block, list string, entries ...string) {
	t.Helper()
	if err := addEntries(d, block, list, entries...); err != nil {
		t.Fatal(err)
	}
}
