package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
)

// §3.2 Preset, §4.4 (owner 2026-11-17): a block's lists (blacklists,
// exceptions or both, with or without actions), its automation and its
// windows are saved as presets and copied into other blocks and modes,
// through the gates; presets are copies both ways.
func TestPresets(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "block", "add-exception", "b", "x.com/ok")
	mustRun(t, "list", "create", "b", "Words", "--text")
	mustRun(t, "list", "add", "b", "Words", "casino")
	mustRun(t, "list", "action", "b", "Words", "greyscale", "on")
	mustRun(t, "list", "action", "b", "Words", "block", "off")
	mustRun(t, "schedule", "add", "b", "--days", "mon", "--from", "09:00", "--to", "10:00")
	mustRun(t, "preset", "save", "lists", "b", "All", "--actions")
	mustRun(t, "preset", "save", "lists", "b", "Black", "--blacklists")
	mustRun(t, "preset", "save", "automation", "b", "Mornings")
	mustRun(t, "preset", "save", "schedule", "b", "Monday")
	if out := capture(t, "preset"); !strings.Contains(out, "Lists presets") || !strings.Contains(out, "Automation presets") ||
		!strings.Contains(out, "Schedule presets") || !strings.Contains(out, "mon 09:00-10:00") {
		t.Errorf("preset: %s", out)
	}
	mustRun(t, "block", "add", "b", "later.com") // a copy: the preset doesn't change
	mustRun(t, "block", "create", "c")
	mustRun(t, "preset", "use", "All", "c")
	mustRun(t, "preset", "use", "Mornings", "c")
	c, _ := d.GetBlock("c")
	blocked, _ := c.Lists.Find("Blocked 2")
	words, _ := c.Lists.Find("Words")
	exc, _ := c.Lists.Find("Exceptions 2")
	if !slices.Equal(blocked.Entries, []string{"reddit.com"}) || !slices.Contains(exc.Entries, "x.com/ok") ||
		len(words.Actions) != 1 || words.Actions[0].Name != db.ActionGreyscale {
		t.Errorf("copied lists: blocked %v, exceptions %v, words %+v", blocked.Entries, exc.Entries, words.Actions)
	}
	if rows, _ := d.ListSchedules("c"); len(rows) != 1 || rows[0].StartMin != 9*60 {
		t.Errorf("copied automation: %+v", rows)
	}
	mustRun(t, "mode", "create", "M")
	mustRun(t, "preset", "use", "Monday", "M")
	if m, _ := d.GetMode("M"); len(m.Spec.Own) != 1 {
		t.Errorf("schedule preset on a mode: %+v", m.Spec)
	}
	// Locked: an exception list weakens, blacklists don't.
	mustRun(t, "block", "create", "e")
	mustRun(t, "block", "add", "e", "e.com")
	mustRun(t, "start", "e", "--for", "1h", "--yes")
	if err := runCLI(t, "preset", "use", "All", "e"); err == nil || !strings.Contains(err.Error(), "exception") {
		t.Errorf("exceptions copied into a locked block: %v", err)
	}
	mustRun(t, "preset", "use", "Black", "e")
	mustRun(t, "preset", "rename", "Black", "Blacklists")
	mustRun(t, "preset", "delete", "Blacklists")
	if _, err := d.FindPreset("", "Black"); err == nil {
		t.Error("deleted preset still there")
	}
}
