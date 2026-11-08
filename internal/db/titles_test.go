package db

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestTitles(t *testing.T) {
	d, err := OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.CreateBlock("b", ModeBlocklist); err != nil {
		t.Fatal(err)
	}
	if err := d.AddTitles("b", []string{"YouTube", "youtube", " Twitch ", ""}); err != nil {
		t.Fatal(err)
	}
	detail, err := d.GetBlock("b")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(detail.Titles, []string{"Twitch", "YouTube"}) {
		t.Fatalf("titles = %q; want Twitch and YouTube once each", detail.Titles)
	}
	if err := d.ActivateBlock("b", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	active, err := d.GetActiveBlocks()
	if err != nil || len(active) != 1 || len(active[0].Titles) != 2 {
		t.Fatalf("active titles: %+v %v", active, err)
	}
	if err := d.DuplicateBlock("b", "copy"); err != nil {
		t.Fatal(err)
	}
	if c, _ := d.GetBlock("copy"); len(c.Titles) != 2 {
		t.Errorf("copy titles = %q", c.Titles)
	}
	// A queued removal applies, and a block left with nothing switches off.
	if err := d.ApplyEdit("b", Edit{Action: EditRemoveTitles, Values: []string{"Twitch", "YouTube"}}); err != nil {
		t.Fatal(err)
	}
	if b, _ := d.GetBlock("b"); len(b.Titles) != 0 || b.Active {
		t.Errorf("after removing every title: %q active=%v", b.Titles, b.Active)
	}
}
