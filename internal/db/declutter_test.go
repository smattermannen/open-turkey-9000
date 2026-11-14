package db

import (
	"reflect"
	"testing"
)

// The declutter action is gone (SPEC 2026-11-14): stored ones are dropped
// when the database is opened, and a blacklist or phrase set left with no
// action gets block. Opening again changes nothing.
func TestMigrateRemovedActions(t *testing.T) {
	d, path := openTest(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	blocked, err := d.ListID("b", DefaultBlockedList)
	if err != nil {
		t.Fatal(err)
	}
	words, err := d.CreateList("b", ListBlacklist, "Words", Where{Text: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.AddPhraseSet("b", PhraseSet{Name: "P", Where: Where{Text: true}}); err != nil {
		t.Fatal(err)
	}
	var set int
	if err := d.conn.QueryRow("SELECT id FROM phrase_sets WHERE name = 'P'").Scan(&set); err != nil {
		t.Fatal(err)
	}
	// As an older version stored them: Blocked with block and declutter,
	// Words and P with declutter alone (and a threshold).
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.conn.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	mustExec("INSERT INTO web_list_actions (list_id, action) VALUES (?, 'declutter')", blocked)
	mustExec("DELETE FROM web_list_actions WHERE list_id = ?", words)
	mustExec("INSERT INTO web_list_actions (list_id, action, distinct_min) VALUES (?, 'declutter', 3)", words)
	mustExec("DELETE FROM phrase_set_actions WHERE set_id = ?", set)
	mustExec("INSERT INTO phrase_set_actions (set_id, action) VALUES (?, 'declutter')", set)
	d.Close()

	for round := 0; round < 2; round++ {
		d, err := OpenDB(path)
		if err != nil {
			t.Fatal(err)
		}
		lists, err := d.GetBlock("b")
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range lists.Lists.Blacklists() {
			var names []string
			for _, a := range l.Actions {
				names = append(names, a.Name)
			}
			if !reflect.DeepEqual(names, []string{ActionBlock}) {
				t.Errorf("round %d: %s actions = %v; want [block]", round, l.Name, names)
			}
		}
		var actions []string
		rows, err := d.conn.Query("SELECT action FROM phrase_set_actions WHERE set_id = ?", set)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var a string
			rows.Scan(&a)
			actions = append(actions, a)
		}
		rows.Close()
		if !reflect.DeepEqual(actions, []string{ActionBlock}) {
			t.Errorf("round %d: phrase set actions = %v; want [block]", round, actions)
		}
		var n int
		d.conn.QueryRow("SELECT COUNT(*) FROM web_list_actions WHERE action = 'declutter'").Scan(&n)
		if n != 0 {
			t.Errorf("round %d: %d declutter actions left", round, n)
		}
		d.Close()
	}
}
