package enforce

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
)

// Only a block's ticked actions get a threshold in its keyword rule, and a
// block with page keywords needs the extension.
func TestKeywordRules(t *testing.T) {
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.CreateBlock("b", db.ModeBlocklist); err != nil {
		t.Fatal(err)
	}
	if err := d.AddPageKeywords("b", []string{"nsfw", "18+"}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetBlockActions("b", db.Actions{Block: true, Greyscale: true}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetThreshold("b", db.ActionBlock, 2); err != nil {
		t.Fatal(err)
	}
	if err := d.AddExceptions("b", []string{"wikipedia.org"}); err != nil {
		t.Fatal(err)
	}
	if err := d.ActivateBlock("b", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	rules, err := BuildRules(d)
	if err != nil {
		t.Fatal(err)
	}
	want := []KeywordRule{{Keywords: []string{"18+", "nsfw"}, Block: 2, Greyscale: 1, NoImages: 0, Except: []string{"wikipedia.org"}}}
	if !reflect.DeepEqual(rules.Keywords, want) {
		t.Fatalf("keyword rules = %+v; want %+v", rules.Keywords, want)
	}
	active, err := d.GetActiveBlocks()
	if err != nil {
		t.Fatal(err)
	}
	if !ExtensionRequired(d, active) {
		t.Error("a block with page keywords doesn't require the extension")
	}
}
