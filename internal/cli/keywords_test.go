package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
)

// runCLI runs a kc9k command line against the test database.
func runCLI(t *testing.T, args ...string) error {
	t.Helper()
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

// A threshold can always be lowered, but raising one loosens the block:
// refused while it is locked.
func TestThresholdGating(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	t.Setenv("KC9K_DB", path)
	d, err := db.OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.CreateBlock("b", db.ModeBlocklist); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "block", "add-keyword", "b", "nsfw", "18+", "*porn*"); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "block", "add-keyword", "b", "***"); err == nil {
		t.Error("a keyword of only wildcards was accepted")
	}
	if err := runCLI(t, "block", "threshold", "b", "block", "3"); err != nil {
		t.Fatalf("raising on an inactive block: %v", err)
	}
	if err := runCLI(t, "block", "threshold", "b", "block", "0"); err == nil {
		t.Error("threshold 0 accepted")
	}

	// Locked: raising is refused (the check runs before anything is applied).
	if err := d.ActivateBlock("b", true, 300, 0, false); err != nil {
		t.Fatal(err)
	}
	err = runCLI(t, "block", "threshold", "b", "block", "5")
	if err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("raising on a locked block: %v; want refused as locked", err)
	}
	err = runCLI(t, "block", "remove-keyword", "b", "nsfw")
	if err == nil {
		t.Fatal("removing a keyword from a locked block was allowed")
	}
	if b, _ := d.GetBlock("b"); b.Thresholds.Block != 3 || len(b.Keywords) != 3 {
		t.Fatalf("after refused edits: threshold %d, keywords %q", b.Thresholds.Block, b.Keywords)
	}
}
