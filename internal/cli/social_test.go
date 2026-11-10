package cli

import (
	"reflect"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

func TestSocialSettings(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	if err := runCLI(t, "social", "b", "youtube:no-such-thing", "on"); err == nil {
		t.Error("an unknown setting was accepted")
	}
	mustRun(t, "social", "b", "youtube:no-shorts", "on")
	mustRun(t, "social", "b", "instagram:messages-only", "on")
	detail, _ := d.GetBlock("b")
	if !reflect.DeepEqual(detail.Social, []string{"instagram:messages-only", "youtube:no-shorts"}) || detail.Empty() {
		t.Fatalf("social = %v, empty = %v", detail.Social, detail.Empty())
	}
	// A block with only settings can run, needs the extension, and hands them to it.
	if err := d.ActivateBlock("b", true, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	active, _ := d.GetActiveBlocks()
	if !enforce.ExtensionRequired(d, active) {
		t.Error("a block with social media settings doesn't require the extension")
	}
	if rules, _ := enforce.BuildRules(d); !reflect.DeepEqual(rules.Social, detail.Social) {
		t.Errorf("rules.social = %v", rules.Social)
	}
	// Locked: more is allowed, less is not.
	mustRun(t, "social", "b", "youtube:no-comments", "on")
	if err := runCLI(t, "social", "b", "youtube:no-shorts", "off"); err == nil {
		t.Error("a setting was turned off on a locked block")
	}
	// Letting a playlist through adds exceptions: refused while locked.
	if err := runCLI(t, "social", "b", "playlist", "PLabcdefghij12", "on"); err == nil {
		t.Error("a playlist was let through a locked block")
	}
	if detail, _ = d.GetBlock("b"); len(detail.Social) != 3 {
		t.Errorf("a refused playlist left something behind: %v", detail.Social)
	}
	if err := d.DeactivateBlock("b"); err != nil {
		t.Fatal(err)
	}

	mustRun(t, "block", "add", "b", "youtube.com")
	mustRun(t, "social", "b", "playlist", "https://www.youtube.com/playlist?list=PLabcdefghij12&si=x", "on")
	mustRun(t, "social", "b", "playlist", "PLzyxwvutsrq98", "on")
	detail, _ = d.GetBlock("b")
	l, _ := detail.Lists.Find(db.DefaultExceptionsList)
	if len(l.Entries) != 3 { // two playlist pages and the video pages
		t.Errorf("exceptions = %q", l.Entries)
	}
	if err := runCLI(t, "social", "b", "playlist", "not a playlist", "on"); err == nil {
		t.Error("nonsense was accepted as a playlist")
	}
	// Removing one keeps the video pages open for the other; removing both closes them.
	mustRun(t, "social", "b", "playlist", "PLabcdefghij12", "off")
	detail, _ = d.GetBlock("b")
	if l, _ = detail.Lists.Find(db.DefaultExceptionsList); len(l.Entries) != 2 {
		t.Errorf("after removing one playlist: %q", l.Entries)
	}
	mustRun(t, "social", "b", "playlist", "PLzyxwvutsrq98", "off")
	detail, _ = d.GetBlock("b")
	if l, _ = detail.Lists.Find(db.DefaultExceptionsList); len(l.Entries) != 0 || len(detail.Social) != 3 {
		t.Errorf("after removing both: exceptions %q, social %v", l.Entries, detail.Social)
	}
}
