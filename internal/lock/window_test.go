//go:build linux

package lock

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// keyLines writes what the window reports for text typed by hand, a key
// every 100 ms, each page ended with Enter.
func keyLines(pages ...string) string {
	var b strings.Builder
	at := 1000
	for _, p := range pages {
		for i := 0; i < len(p); i++ {
			fmt.Fprintf(&b, "k %d %d\n", p[i], at)
			at += 100
		}
		fmt.Fprintf(&b, "n %d\n", at)
	}
	return b.String()
}

func TestReadWindowKeys(t *testing.T) {
	pages := []string{"abc", "de"}
	if got, outcome, err := readWindowKeys(strings.NewReader(keyLines(pages...)), pages); err != nil || outcome != outcomeDone || got != "abcde" {
		t.Errorf("typed by hand = %q, %v, %v", got, outcome, err)
	}
	// Backspace takes the last key of the page back.
	in := "k 97 0\nk 120 100\nb 200\nk 98 300\nk 99 400\nn 500\nk 100 600\nk 101 700\nn 800\n"
	if got, _, _ := readWindowKeys(strings.NewReader(in), pages); got != "abcde" {
		t.Errorf("with a backspace = %q", got)
	}
	for in, want := range map[string]inputOutcome{
		"k 97 0\np\n": outcomePaste,     // Ctrl+V, middle click, a drop
		"k 97 0\ns\n": outcomeSynthetic, // XSendEvent
		"k 97 0\nc\n": outcomeCancel,    // Escape, Cancel
		"k 97 0\n":    outcomeCancel,    // closed before the end
		"k 97 0\nk 98 1\nk 99 2\nk 100 3\nk 101 4\n": outcomePaste, // five keys in 4 ms
	} {
		if _, got, err := readWindowKeys(strings.NewReader(in), pages); err != nil || got != want {
			t.Errorf("%q: outcome %v, %v; want %v", in, got, err, want)
		}
	}
}

// fakeWindow replaces the window with one that reports events.
func fakeWindow(t *testing.T, events func(pages []string) string) {
	old := StartWindow
	StartWindow = func(pages []string) (io.Reader, func() error, error) {
		return strings.NewReader(events(pages)), func() error { return nil }, nil
	}
	t.Cleanup(func() { StartWindow = old })
}

// R-LOCK-5, §4.6: without a terminal only the window's keys count; an
// answer on standard input is never read.
func TestChallengeWithoutTerminalUsesTheWindow(t *testing.T) {
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin; r.Close() })
	w.Close() // nothing piped: the old line reader would fail here

	fakeWindow(t, func(pages []string) string { return keyLines(pages...) })
	if ok, err := RunChallenge(30); err != nil || !ok {
		t.Errorf("typed in the window: %v, %v", ok, err)
	}
	fakeWindow(t, func(pages []string) string { return "k 97 0\np\n" })
	if ok, err := RunChallenge(30); err != nil || ok {
		t.Errorf("a paste in the window passed: %v, %v", ok, err)
	}
	old := StartWindow
	StartWindow = func([]string) (io.Reader, func() error, error) { return nil, nil, fmt.Errorf("no desktop") }
	defer func() { StartWindow = old }()
	if ok, err := RunChallenge(30); err == nil || ok {
		t.Errorf("without terminal and window: %v, %v", ok, err)
	}
}

// The real window on an X display (run under xvfb-run with xdotool and a
// python3 with GTK 3; KC9K_TEST_PYTHON names it when python3 lacks GTK).
func TestChallengeWindowOnX(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display")
	}
	if _, err := exec.LookPath("xdotool"); err != nil {
		t.Skip("xdotool not installed")
	}
	if p := os.Getenv("KC9K_TEST_PYTHON"); p != "" {
		old := windowPython
		windowPython = p
		defer func() { windowPython = old }()
	}
	if exec.Command(windowPython, "-c", "import gi; gi.require_version('Gtk', '3.0'); from gi.repository import Gtk").Run() != nil {
		t.Skip(windowPython + " has no GTK 3")
	}
	xdo := func(args ...string) {
		if out, err := exec.Command("xdotool", args...).CombinedOutput(); err != nil {
			t.Logf("xdotool %v: %v %s", args, err, out)
		}
	}
	run := func(text string, typeIt func(id string)) (string, inputOutcome) {
		pages := []string{text}
		events, wait, err := startWindow(pages)
		if err != nil {
			t.Fatal(err)
		}
		var once sync.Once
		stop := func() { once.Do(func() { wait() }) }
		defer stop()
		var id string
		for i := 0; i < 100 && id == ""; i++ {
			out, _ := exec.Command("xdotool", "search", "--name", "unlock challenge").Output()
			id = strings.TrimSpace(strings.Split(string(out), "\n")[0])
			time.Sleep(100 * time.Millisecond)
		}
		if id == "" {
			t.Fatal("the challenge window never showed")
		}
		xdo("windowfocus", "--sync", id)
		typeIt(id)
		timer := time.AfterFunc(3*time.Second, stop) // nothing more comes: closed, as cancelled
		defer timer.Stop()
		got, outcome, err := readWindowKeys(events, pages)
		if err != nil {
			t.Fatal(err)
		}
		return got, outcome
	}
	if got, outcome := run("aB3!x", func(string) {
		xdo("type", "--delay", "80", "aB3!x")
		xdo("key", "Return")
	}); outcome != outcomeDone || got != "aB3!x" {
		t.Errorf("typed at a human pace: %q, %v", got, outcome)
	}
	if _, outcome := run("abcdefgh", func(string) { xdo("type", "--delay", "0", "abcdefgh") }); outcome != outcomePaste {
		t.Errorf("typed in a burst: %v", outcome)
	}
	if _, outcome := run("abc", func(string) { xdo("key", "ctrl+v") }); outcome != outcomePaste {
		t.Errorf("Ctrl+V: %v", outcome)
	}
	// Keys sent to the window (XSendEvent) while another has the focus
	// count for nothing (a synthetic key the window gets says "s").
	if _, err := exec.LookPath("xterm"); err == nil {
		decoy := exec.Command("xterm")
		if decoy.Start() == nil {
			defer func() { decoy.Process.Kill(); decoy.Wait() }()
			if got, outcome := run("abc", func(id string) {
				out, _ := exec.Command("xdotool", "search", "--sync", "--class", "xterm").Output()
				xdo("windowfocus", "--sync", strings.TrimSpace(strings.Split(string(out), "\n")[0]))
				xdo("type", "--window", id, "abc")
				xdo("key", "--window", id, "Return")
			}); got != "" || (outcome != outcomeCancel && outcome != outcomeSynthetic) {
				t.Errorf("sent to the window: %q, %v", got, outcome)
			}
		}
	}
	if _, outcome := run("abc", func(string) { xdo("key", "Escape") }); outcome != outcomeCancel {
		t.Errorf("Escape: %v", outcome)
	}
}
