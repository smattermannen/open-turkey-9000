package db

import "testing"

func TestParseActions(t *testing.T) {
	if a, err := ParseActions(nil); err != nil || a != DefaultActions {
		t.Errorf("empty = %+v, %v; want the default", a, err)
	}
	if a, err := ParseActions([]string{"Greyscale", " no-images"}); err != nil || a != (Actions{Greyscale: true, NoImages: true}) {
		t.Errorf("parsed = %+v, %v", a, err)
	}
	if _, err := ParseActions([]string{"blur"}); err == nil {
		t.Error("an unknown action should be refused")
	}
}

// Declutter is gone (the owner's decision, 2026-11-14): a stored declutter
// action is dropped when the database opens, and old files naming it read
// without it.
func TestDeclutterRemoved(t *testing.T) {
	d, path := openTest(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	id, err := d.CreateList("b", ListBlacklist, "Words", Where{Text: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec(`INSERT INTO web_list_actions (list_id, action) VALUES (?, 'declutter')`, id); err != nil {
		t.Fatal(err)
	}
	d.Close()
	if d, err = OpenDB(path); err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var n int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM web_list_actions WHERE action = 'declutter'`).Scan(&n); err != nil || n != 0 {
		t.Errorf("declutter actions left: %d, %v", n, err)
	}
	if IsAction("declutter") {
		t.Error("declutter is still an action")
	}
	a, err := ParseActions([]string{"declutter", "greyscale"})
	if err != nil || !a.Has(ActionGreyscale) {
		t.Errorf("ParseActions with declutter: %v, %v", a, err)
	}
}
