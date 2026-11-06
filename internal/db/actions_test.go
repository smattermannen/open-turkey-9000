package db

import (
	"path/filepath"
	"testing"
)

func TestBlockActions(t *testing.T) {
	d, err := OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if err := d.CreateBlock("b", ModeBlocklist); err != nil {
		t.Fatal(err)
	}
	if err := d.AddSites("b", []string{"reddit.com"}); err != nil {
		t.Fatal(err)
	}
	detail, err := d.GetBlock("b")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Actions != DefaultActions {
		t.Fatalf("new block actions = %+v; want only block", detail.Actions)
	}

	if err := d.SetBlockAction("b", ActionGreyscale, true); err != nil {
		t.Fatal(err)
	}
	if err := d.SetBlockAction("b", ActionBlock, false); err != nil {
		t.Fatal(err)
	}
	if err := d.SetBlockAction("b", ActionGreyscale, false); err == nil {
		t.Fatal("turning off the last action should be refused")
	}
	if err := d.SetBlockAction("b", "sparkles", true); err == nil {
		t.Fatal("an unknown action should be refused")
	}

	// A queued unset applies through ApplyEdit, and copies keep the actions.
	if err := d.SetBlockAction("b", ActionNoImages, true); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyEdit("b", Edit{Action: EditUnsetAction, Values: []string{ActionNoImages}}); err != nil {
		t.Fatal(err)
	}
	if err := d.ActivateBlock("b", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	active, err := d.GetActiveBlocks()
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].Actions != (Actions{Greyscale: true}) {
		t.Fatalf("active actions = %+v; want greyscale only", active)
	}
	if err := d.DuplicateBlock("b", "copy"); err != nil {
		t.Fatal(err)
	}
	copied, err := d.GetBlock("copy")
	if err != nil {
		t.Fatal(err)
	}
	if copied.Actions != (Actions{Greyscale: true}) {
		t.Errorf("copy actions = %+v; want greyscale only", copied.Actions)
	}
}

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
