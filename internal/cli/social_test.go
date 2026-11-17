package cli

import (
	"reflect"
	"slices"
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
	// Letting a playlist through is an exception: refused while locked.
	if err := runCLI(t, "social", "b", "playlist", "PLabcdefghij12", "on"); err == nil {
		t.Error("a playlist was let through a locked block")
	}
	if err := runCLI(t, "block", "add-exception", "b", "youtube.com/playlist?list=PLabcdefghij12"); err == nil {
		t.Error("a playlist's address was added to a locked block's exceptions")
	}
	if detail, _ = d.GetBlock("b"); len(detail.Social) != 3 || len(detail.Lists.Playlists()) != 0 {
		t.Errorf("a refused playlist left something behind: %v %v", detail.Social, detail.Lists.Playlists())
	}
	if err := d.DeactivateBlock("b"); err != nil {
		t.Fatal(err)
	}
}

// §3.2 Exception list (owner 2026-11-17): a YouTube playlist is let through
// by its address in an exception list, kept one way however it is pasted;
// enforcement adds the restriction to its videos and the video pages, never
// stored; "social … playlist" is an alias for the exception.
func TestPlaylistException(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "youtube.com")
	mustRun(t, "social", "b", "playlist", "https://www.youtube.com/playlist?list=PLabcdefghij12&si=x", "on")
	mustRun(t, "block", "add-exception", "b", "https://m.youtube.com/playlist?list=PLzyxwvutsrq98")
	if err := runCLI(t, "social", "b", "playlist", "not a playlist", "on"); err == nil {
		t.Error("nonsense was accepted as a playlist")
	}
	detail, _ := d.GetBlock("b")
	l, _ := detail.Lists.Find(db.DefaultExceptionsList)
	want := []string{db.FileException, "youtube.com/playlist?list=PLabcdefghij12", "youtube.com/playlist?list=PLzyxwvutsrq98"}
	if !reflect.DeepEqual(l.Entries, want) || len(detail.Social) != 0 {
		t.Fatalf("exceptions = %q, social = %v", l.Entries, detail.Social)
	}
	// Enforced: the extension gets both playlists, and the video pages are
	// among the exceptions; nothing of it is stored.
	mustRun(t, "start", "b")
	active, _ := d.GetActiveBlocks()
	al, _ := active[0].Lists.Find(db.DefaultExceptionsList)
	if !reflect.DeepEqual(active[0].Social, []string{db.PlaylistPrefix + "PLabcdefghij12", db.PlaylistPrefix + "PLzyxwvutsrq98"}) ||
		!slices.Contains(al.Entries, db.VideoPages) {
		t.Errorf("enforced: social %v, exceptions %q", active[0].Social, al.Entries)
	}
	if rules, _ := enforce.BuildRules(d); len(rules.Social) != 2 {
		t.Errorf("rules.social = %v", rules.Social)
	}
	if detail, _ = d.GetBlock("b"); slices.Contains(detail.Lists[1].Entries, db.VideoPages) {
		t.Errorf("the video pages were stored: %q", detail.Lists[1].Entries)
	}
	mustRun(t, "stop", "b")
	// Removing: by the alias or as an entry; the video pages go with the last.
	mustRun(t, "social", "b", "playlist", "PLabcdefghij12", "off")
	mustRun(t, "block", "remove-exception", "b", "https://www.youtube.com/playlist?list=PLzyxwvutsrq98")
	mustRun(t, "start", "b")
	active, _ = d.GetActiveBlocks()
	if al, _ = active[0].Lists.Find(db.DefaultExceptionsList); !reflect.DeepEqual(al.Entries, []string{db.FileException}) || len(active[0].Social) != 0 {
		t.Errorf("after removing both: exceptions %q, social %v", al.Entries, active[0].Social)
	}
	if err := runCLI(t, "social", "b", "playlist", "PLabcdefghij12", "off"); err == nil {
		t.Error("a playlist the block doesn't have was removed")
	}
}
