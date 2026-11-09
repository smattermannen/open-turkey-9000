package db

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestPageKeywords(t *testing.T) {
	d, err := OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.CreateBlock("b", ModeBlocklist); err != nil {
		t.Fatal(err)
	}
	if err := d.AddPageKeywords("b", []string{"NSFW", "nsfw", " 18+ ", ""}); err != nil {
		t.Fatal(err)
	}
	detail, err := d.GetBlock("b")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(detail.Keywords, []string{"18+", "NSFW"}) {
		t.Fatalf("keywords = %q; want 18+ and NSFW once each", detail.Keywords)
	}
	if detail.Thresholds != DefaultThresholds {
		t.Fatalf("new block thresholds = %+v; want all 1", detail.Thresholds)
	}
	if detail.Empty() {
		t.Fatal("a block with only page keywords counts as empty")
	}

	if err := d.SetThreshold("b", ActionBlock, 3); err != nil {
		t.Fatal(err)
	}
	if err := d.SetThreshold("b", ActionBlock, 0); err == nil {
		t.Error("threshold 0 accepted")
	}
	if err := d.SetThreshold("b", "bogus", 2); err == nil {
		t.Error("unknown action accepted")
	}
	// A queued raise applies like other loosening edits.
	if err := d.ApplyEdit("b", Edit{Action: EditSetThreshold, Values: []string{ActionGreyscale, "2"}}); err != nil {
		t.Fatal(err)
	}
	want := Thresholds{Block: 3, Greyscale: 2, NoImages: 1}
	if b, _ := d.GetBlock("b"); b.Thresholds != want {
		t.Fatalf("thresholds = %+v; want %+v", b.Thresholds, want)
	}

	if err := d.ActivateBlock("b", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	active, err := d.GetActiveBlocks()
	if err != nil || len(active) != 1 || len(active[0].Keywords) != 2 || active[0].Thresholds != want {
		t.Fatalf("active block: %+v %v", active, err)
	}
	if err := d.DuplicateBlock("b", "copy"); err != nil {
		t.Fatal(err)
	}
	if c, _ := d.GetBlock("copy"); len(c.Keywords) != 2 || c.Thresholds != want {
		t.Errorf("copy: keywords %q thresholds %+v", c.Keywords, c.Thresholds)
	}
	// Removing every keyword from a block with nothing else switches it off.
	if err := d.ApplyEdit("b", Edit{Action: EditRemoveKeywords, Values: []string{"18+", "NSFW"}}); err != nil {
		t.Fatal(err)
	}
	if b, _ := d.GetBlock("b"); len(b.Keywords) != 0 || b.Active {
		t.Errorf("after removing every keyword: %q active=%v", b.Keywords, b.Active)
	}
}
