package cli

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
)

// runWith runs a command with the given lines on standard input, the first
// access of them announced as access passwords, as the interface does.
func runWith(t *testing.T, access int, lines []string, args ...string) error {
	t.Helper()
	stdinLines = bufio.NewReader(strings.NewReader(strings.Join(lines, "\n") + "\n"))
	accessGiven, accessLinesRed, settingsPasswordChecked = nil, false, false
	if access > 0 {
		args = append([]string{"--access-lines", strconv.Itoa(access)}, args...)
	}
	err := runCLI(t, args...)
	_ = rootCmd.PersistentFlags().Set("access-lines", "0")
	return err
}

func notTerminal(t *testing.T) {
	old := isTerminalInput
	isTerminalInput = func() bool { return false }
	t.Cleanup(func() {
		isTerminalInput = old
		stdinLines = bufio.NewReader(os.Stdin)
		accessGiven, accessLinesRed, settingsPasswordChecked = nil, false, false
	})
}

func TestAccessPasswords(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	must := func(access int, lines []string, args ...string) {
		t.Helper()
		if err := runWith(t, access, lines, args...); err != nil {
			t.Fatalf("%s: %v", strings.Join(args, " "), err)
		}
	}
	refused := func(want string, access int, lines []string, args ...string) {
		t.Helper()
		err := runWith(t, access, lines, args...)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v; want %q", strings.Join(args, " "), err, want)
		}
	}

	must(0, nil, "block", "create", "b")
	// One password for two pages, another for a third.
	must(0, []string{"alpha1"}, "access", "set", "blocks", "stats")
	must(0, []string{"beta22"}, "access", "set", "device")
	if a, _ := d.GetSetting(settingAccessPrefix + areaBlocks); a == "" {
		t.Fatal("no hash stored")
	} else if b, _ := d.GetSetting(settingAccessPrefix + areaStats); a != b {
		t.Error("areas set together don't share a hash")
	}

	// A page's commands need its password; other pages stay open.
	refused("needs the access password (blocks)", 0, nil, "block", "list")
	refused("needs the access password (blocks)", 0, nil, "block", "add", "b", "example.com")
	must(1, []string{"alpha1"}, "block", "add", "b", "example.com")
	must(0, nil, "browser")
	refused("needs the access password (device)", 0, nil, "strict", "on")
	refused("wrong access password (device)", 1, []string{"alpha1"}, "strict", "on")
	// A command on two pages runs when either is open.
	must(0, nil, "status")
	must(0, nil, "stats", "--json")
	must(0, []string{"gamma3"}, "access", "set", "dashboard")
	refused("needs the access password", 0, nil, "status")
	must(1, []string{"alpha1"}, "status")
	must(1, []string{"gamma3"}, "stats", "--json")

	// The app password comes on top of the page's.
	must(0, []string{"omega4"}, "access", "set", "app")
	refused("needs the access password (app)", 0, nil, "browser")
	refused("wrong access password (blocks)", 1, []string{"omega4"}, "block", "list")
	must(2, []string{"omega4", "alpha1"}, "block", "list")
	// The daemon's and a hotkey's commands never ask.
	for _, path := range []string{"daemon", "hotkey run", "gui-state"} {
		c, _, err := rootCmd.Find(strings.Fields(path))
		if err != nil {
			t.Fatal(err)
		}
		if _, asks := areasOf(c); asks {
			t.Errorf("%s asks for an access password", path)
		}
	}

	// Changing or removing one needs the one it has.
	refused("access password (blocks)", 1, []string{"omega4", "newpass"}, "access", "set", "blocks")
	must(2, []string{"omega4", "alpha1", "newpass"}, "access", "set", "blocks")
	must(2, []string{"omega4", "newpass"}, "block", "list")
	refused("access password (stats)", 1, []string{"omega4"}, "access", "remove", "stats")
	must(2, []string{"omega4", "alpha1"}, "access", "remove", "stats")
	must(1, []string{"omega4"}, "stats", "--json")

	// Wrong tries are logged.
	n, err := d.CountEventsOf("", 0, []string{db.EventAccess})
	if err != nil {
		t.Fatal(err)
	}
	if n < 2 {
		t.Errorf("%d access events logged; want at least 2", n)
	}

	// With the settings password set, changes need it too, after the access lines.
	must(1, []string{"omega4", "settings1"}, "password", "set")
	refused("settings password", 2, []string{"omega4", "beta22"}, "access", "remove", "device")
	must(2, []string{"omega4", "beta22", "settings1"}, "access", "remove", "device")
}

func guiStateWith(t *testing.T, passwords ...string) guiState {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := runWith(t, len(passwords), passwords, "gui-state")
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if runErr != nil {
		t.Fatalf("gui-state: %v", runErr)
	}
	var s guiState
	if err := json.Unmarshal(out, &s); err != nil {
		t.Fatalf("gui-state printed %q: %v", out, err)
	}
	return s
}

func TestGuiStateHidesLockedPages(t *testing.T) {
	testDB(t)
	notTerminal(t)
	if err := runWith(t, 0, nil, "block", "create", "b"); err != nil {
		t.Fatal(err)
	}
	if s := guiStateWith(t); len(s.Blocks) != 1 || s.Access[areaBlocks] != "none" {
		t.Fatalf("without passwords: %d blocks, access %v", len(s.Blocks), s.Access)
	}
	for _, set := range [][]string{{"alpha1", "blocks", "dashboard"}, {"beta22", "settings"}} {
		if err := runWith(t, 0, set[:1], append([]string{"access", "set"}, set[1:]...)...); err != nil {
			t.Fatal(err)
		}
	}
	s := guiStateWith(t)
	if len(s.Blocks) != 0 || len(s.Browser) != 0 || s.Access[areaBlocks] != "locked" || s.Access[areaStats] != "none" {
		t.Errorf("locked: %d blocks, %d browser options, access %v", len(s.Blocks), len(s.Browser), s.Access)
	}
	s = guiStateWith(t, "alpha1")
	if len(s.Blocks) != 1 || len(s.Browser) != 0 || s.Access[areaBlocks] != "open" || s.Access[areaSettings] != "locked" || len(s.AccessUnused) != 0 {
		t.Errorf("blocks open: %d blocks, %d browser options, access %v, unused %v", len(s.Blocks), len(s.Browser), s.Access, s.AccessUnused)
	}
	if s = guiStateWith(t, "alpha1", "beta22"); len(s.Browser) == 0 || s.Access[areaSettings] != "open" {
		t.Errorf("all given: access %v", s.Access)
	}
	if s = guiStateWith(t, "alpha1", "nonsense"); len(s.AccessUnused) != 1 || s.AccessUnused[0] != 1 {
		t.Error("a password that opens nothing wasn't reported")
	}
	// The app password locks every page, whatever else is given.
	if err := runWith(t, 1, []string{"beta22", "omega4"}, "access", "set", "app"); err != nil {
		t.Fatal(err)
	}
	if s = guiStateWith(t, "alpha1"); len(s.Blocks) != 0 || s.Access[areaStats] != "locked" {
		t.Errorf("app locked: %d blocks, access %v", len(s.Blocks), s.Access)
	}
	if s = guiStateWith(t, "omega4", "alpha1"); len(s.Blocks) != 1 || s.Access[areaStats] != "none" {
		t.Errorf("app open: %d blocks, access %v", len(s.Blocks), s.Access)
	}
}
