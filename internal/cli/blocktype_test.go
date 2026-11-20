package cli

import (
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

// §3.3: a block is continuous with no schedule and no place rules, else
// automated; in a mode, by its mode's automation. Breaks and an allowance
// lift a block but never change its type.
func TestBlockTypeContinuousOrAutomated(t *testing.T) {
	testDB(t)
	notTerminal(t)
	oldScan := enforce.ScanWifi
	enforce.ScanWifi = func(bool) ([]db.Network, error) {
		return []db.Network{{BSSID: "AA:AA:AA:AA:AA:01"}, {BSSID: "AA:AA:AA:AA:AA:02"}}, nil
	}
	t.Cleanup(func() { enforce.ScanWifi = oldScan })
	continuous := func(name string) bool {
		t.Helper()
		for _, b := range guiStateWith(t).Blocks {
			if b.Name == name {
				return b.Continuous
			}
		}
		t.Fatalf("no block %s", name)
		return false
	}
	mustRun(t, "block", "create", "b")
	if !continuous("b") {
		t.Error("a new block isn't continuous")
	}
	mustRun(t, "allowance", "set", "b", "1h")
	mustRun(t, "break", "delay", "b", "10m", "5m")
	if !continuous("b") {
		t.Error("an allowance made the block automated")
	}
	mustRun(t, "place", "add", "home")
	mustRun(t, "place", "rule", "b", "home", "on")
	if continuous("b") {
		t.Error("a place rule leaves it continuous")
	}
	mustRun(t, "place", "rule", "b", "home", "off")
	if !continuous("b") {
		t.Error("without its place rule it isn't continuous again")
	}
	mustRun(t, "schedule", "add", "b", "--days", "mon-fri", "--from", "09:00", "--to", "17:00")
	if continuous("b") {
		t.Error("a window leaves it continuous")
	}
	mustRun(t, "block", "create", "c")
	mustRun(t, "mode", "create", "M")
	mustRun(t, "schedule", "add", "M", "--days", "sat", "--from", "10:00", "--to", "12:00")
	mustRun(t, "mode", "add", "M", "c")
	if continuous("c") {
		t.Error("a block in an automated mode is continuous")
	}
}
