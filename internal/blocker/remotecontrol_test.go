package blocker

import (
	"strings"
	"testing"
)

func nul(parts ...string) []byte { return []byte(strings.Join(parts, "\x00")) }

func TestParseRemoteControl(t *testing.T) {
	cases := []struct {
		name    string
		cmdline []byte
		environ []byte
		want    bool
	}{
		{"plain browser", nul("/usr/lib/librewolf/librewolf", "https://example.com"), nul("HOME=/home/u"), false},
		{"marionette", nul("librewolf", "--marionette"), nil, true},
		{"single dash", nul("librewolf", "-marionette"), nil, true},
		{"system access", nul("librewolf", "--remote-allow-system-access"), nil, true},
		{"debugging port with value", nul("librewolf", "--remote-debugging-port=9222"), nil, true},
		{"debugger server", nul("librewolf", "-start-debugger-server", "6000"), nil, true},
		{"browser toolbox at start", nul("librewolf", "-jsdebugger"), nil, true},
		{"marionette by environment", nul("librewolf"), nul("HOME=/h", "MOZ_MARIONETTE=1"), true},
		{"empty environment value", nul("librewolf"), nul("MOZ_MARIONETTE="), false},
		{"a URL that mentions it", nul("librewolf", "https://example.com/--marionette"), nil, false},
		{"lookalike flag", nul("librewolf", "--marionettes"), nil, false},
	}
	for _, c := range cases {
		if got := parseRemoteControl(c.cmdline, c.environ); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestRemoteControlNotAllowedInReleaseBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	// In a normal build there is no way to allow it; in a devmaster build it
	// depends on a root-owned marker file this test doesn't create.
	if remoteControlAllowed() {
		t.Error("remote control allowed without the testing marker")
	}
}
