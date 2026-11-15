package cli

import (
	"strings"
	"testing"
)

// §3.11: a block password hides a block's contents until typed; viewing
// only, locks unaffected.
func TestBlockPassword(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "secret.example")
	if err := runWith(t, 0, []string{"open1"}, "block", "password", "b", "set"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"wrong"}, "list", "export", "b", "Blocked"); err == nil {
		t.Error("list exported with a wrong password")
	}
	if err := runWith(t, 0, []string{"open1"}, "list", "export", "b", "Blocked"); err != nil {
		t.Errorf("right password refused: %v", err)
	}
	state := capture(t, "gui-state")
	if strings.Contains(state, "secret.example") || !strings.Contains(state, `"hidden":true`) {
		t.Error("gui-state shows a hidden block's entries")
	}
	if err := runWith(t, 0, []string{"open1"}, "gui-state", "--reveal", "b"); err != nil {
		t.Fatal(err)
	}
	// Adding entries and starting need no password; it isn't a lock.
	mustRun(t, "block", "add", "b", "more.example")
	mustRun(t, "start", "b")
	mustRun(t, "stop", "b")
	if err := runWith(t, 0, []string{"wrong"}, "block", "password", "b", "remove"); err == nil {
		t.Error("removed with a wrong password")
	}
	if err := runWith(t, 0, []string{"open1"}, "block", "password", "b", "remove"); err != nil {
		t.Fatal(err)
	}
	if h, _ := d.BlockViewPassword("b"); h != "" {
		t.Error("still set")
	}
}
