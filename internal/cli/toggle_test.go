package cli

import (
	"testing"

	"github.com/smattermannen/kc9k/internal/enforce"
)

// §4.12: toggle switches a block on (no locks) or off; a locked block stays
// on. §4.2: a duplicate starts inactive.
func TestToggleAndDuplicate(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "toggle", "b")
	if detail, _ := d.GetBlock("b"); !detail.Active || detail.Locked || detail.LockedUntil != 0 {
		t.Fatalf("toggle on: %+v", detail)
	}
	mustRun(t, "toggle", "b")
	if active, _ := d.IsBlockActive("b"); active {
		t.Fatal("toggle off left it on")
	}
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runCLI(t, "toggle", "b"); err == nil {
		t.Error("toggle switched a locked block off")
	}
	mustRun(t, "block", "duplicate", "b", "copy")
	copied, err := d.GetBlock("copy")
	if err != nil || copied.Active || len(copied.Lists) != 2 {
		t.Errorf("duplicate of an active locked block: %+v, %v", copied, err)
	}
}

// §4.10: notification choices from the CLI.
func TestNotificationsCommand(t *testing.T) {
	d := testDB(t)
	mustRun(t, "notifications", "window-end", "off")
	mustRun(t, "notifications", "advance", "5,2")
	o := enforce.LoadNoticeOptions(d)
	if o.On("window-end") || !o.On("timer-end") || len(o.Advance) != 2 || o.Advance[0] != 2 {
		t.Errorf("options = %+v", o)
	}
	if err := runCLI(t, "notifications", "advance", "0"); err == nil {
		t.Error("advance 0 accepted")
	}
	if err := runCLI(t, "notifications", "nonsense", "on"); err == nil {
		t.Error("unknown kind accepted")
	}
	mustRun(t, "notifications", "window-end", "on")
	if !enforce.LoadNoticeOptions(d).On("window-end") {
		t.Error("not back on")
	}
}
