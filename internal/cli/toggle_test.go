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

// §3.7: strict use counting on is free; back to normal weakens, refused
// while locked with no weakening on (R-WEAK-4).
func TestUseCounting(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "use-counting", "strict")
	if !enforce.StrictUseOn(d) {
		t.Fatal("not on")
	}
	if r, _ := enforce.BuildRules(d); !r.StrictUse {
		t.Error("not in the rules")
	}
	mustRun(t, "no-weakening", "on")
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runCLI(t, "use-counting", "normal"); err == nil {
		t.Error("weakened while locked with no weakening on")
	}
}

// §3.8: adding an exempt folder weakens strict mode; removing is free.
func TestStrictExempt(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	dir := t.TempDir()
	mustRun(t, "strict", "exempt", "add", dir)
	if got := enforce.ExemptFolders(d); len(got) != 1 || got[0] != dir {
		t.Fatalf("exempt = %v", got)
	}
	if err := runCLI(t, "strict", "exempt", "add", "/usr/bin"); err == nil {
		t.Error("a system folder was exempted")
	}
	mustRun(t, "strict", "on", "--for", "1h", "--yes")
	other := t.TempDir()
	if err := runCLI(t, "strict", "exempt", "add", other); err == nil {
		t.Error("exempted a folder while strict mode is locked")
	}
	mustRun(t, "strict", "exempt", "remove", dir)
	if got := enforce.ExemptFolders(d); len(got) != 0 {
		t.Errorf("after remove: %v", got)
	}
}
