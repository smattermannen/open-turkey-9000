package cli

import (
	"testing"
)

// R-WEAK-4: with no weakening on, while anything is locked, no setting can
// be weakened. Every setting that can be weakened is tried here.
func TestNoWeakeningEverySetting(t *testing.T) {
	notTerminal(t)
	testDB(t)
	exempt := t.TempDir()
	// Set everything up strong, unlocked.
	setup := [][]string{
		{"browser", "safesearch", "on"},
		{"tamper", "clock", "on"},
		{"tamper", "file-urls", "on"},
		{"strict", "on"},
		{"bypass", "set", "--count", "3", "--within", "10m", "--then", "warning"},
		{"use-counting", "strict"},
		{"dns", "quad9"},
		{"no-weakening", "on"},
		{"block", "create", "held"},
		{"block", "add", "held", "reddit.com"},
		{"start", "held", "--for", "1h", "--yes"},
	}
	for _, args := range setup {
		mustRun(t, args...)
	}
	if err := runWith(t, 0, []string{"access1"}, "access", "set", "stats"); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"browser", "safesearch", "off"},
		{"tamper", "clock", "off"},
		{"tamper", "file-urls", "off"},
		{"strict", "off"},
		{"bypass", "off"},
		{"use-counting", "normal"},
		{"dns", "network"},
		{"strict", "exempt", "add", exempt},
		{"no-weakening", "off"},
	} {
		if err := runCLI(t, args...); err == nil {
			t.Errorf("%v: weakened while locked", args)
		}
	}
	if err := runWith(t, 1, []string{"access1"}, "access", "remove", "stats"); err == nil {
		t.Error("access password removed while locked")
	}
}
