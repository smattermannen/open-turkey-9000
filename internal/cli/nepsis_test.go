package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
)

// tinyPNG is a 1x1 PNG.
var tinyPNG = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82")

func TestActionSettings(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "list", "add", "b", "Blocked", "reddit.com")
	if err := runCLI(t, "list", "action", "b", "Blocked", "monk", "on"); err == nil {
		t.Error("monk without a duration was accepted")
	}
	if err := runCLI(t, "list", "action", "b", "Blocked", "redirect", "on", "--url", "javascript:alert(1)"); err == nil {
		t.Error("a non-web redirect was accepted")
	}
	mustRun(t, "list", "action", "b", "Blocked", "monk", "on", "--for", "30m")
	mustRun(t, "list", "action", "b", "Blocked", "redirect", "on", "--url", "https://example.com/focus")
	mustRun(t, "list", "action", "b", "Blocked", "close-tab", "on")

	file := filepath.Join(t.TempDir(), "calm.png")
	if err := os.WriteFile(file, tinyPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "list", "action", "b", "Blocked", "image", "on", "--image", file)
	l := list(t, d, "b", "Blocked")
	img, _ := l.Action("image")
	if !db.IsImageName(img.Param) {
		t.Fatalf("image param = %q", img.Param)
	}
	if _, err := os.Stat(filepath.Join(db.ImagesDir(), img.Param)); err != nil {
		t.Errorf("the image wasn't stored: %v", err)
	}
	if err := os.WriteFile(file, []byte("not a picture"), 0o600); err == nil {
		if err := runCLI(t, "list", "action", "b", "Blocked", "image", "on", "--image", file); err == nil {
			t.Error("a file that isn't a picture was accepted")
		}
	}

	// Held: a longer monk mode is allowed, a shorter one or another address
	// isn't.
	if err := d.ActivateBlock("b", true, 300, 0, false); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"list", "action", "b", "Blocked", "monk", "on", "--for", "10m"},
		{"list", "action", "b", "Blocked", "redirect", "on", "--url", "https://youtube.com/"},
		{"list", "action", "b", "Blocked", "close-tab", "off"},
	} {
		if err := runCLI(t, args...); err == nil || !strings.Contains(err.Error(), "held") {
			t.Errorf("%s on a held block: %v; want refused", strings.Join(args, " "), err)
		}
	}
	monk, _ := list(t, d, "b", "Blocked").Action("monk")
	if monk.Minutes != 30 {
		t.Errorf("monk minutes = %d", monk.Minutes)
	}
	// Refusals are logged for bypass prevention.
	if n, _ := d.CountEvents("b", 0); n != 3 {
		t.Errorf("%d refusals logged; want 3", n)
	}
}

func TestNepsisCommands(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "list", "add", "b", "Blocked", "reddit.com")
	mustRun(t, "nepsis", "set", "--block", "b", "--count", "10", "--within", "5m", "--then", "monk", "--then-for", "1h")
	if err := runCLI(t, "nepsis", "set", "--count", "5", "--within", "5m", "--then", "monk"); err == nil {
		t.Error("monk without --then-for was accepted")
	}
	if err := d.ActivateBlock("b", true, 300, 0, false); err != nil {
		t.Fatal(err)
	}
	// Stricter is allowed while held; looser isn't.
	mustRun(t, "nepsis", "set", "--block", "b", "--count", "5", "--within", "10m", "--then", "monk", "--then-for", "2h")
	for _, args := range [][]string{
		{"nepsis", "off", "--block", "b"},
		{"nepsis", "set", "--block", "b", "--count", "20", "--within", "10m", "--then", "monk", "--then-for", "2h"},
		{"nepsis", "set", "--block", "b", "--count", "5", "--within", "10m", "--then", "close-browser"},
	} {
		if err := runCLI(t, args...); err == nil {
			t.Errorf("%s on a held block was allowed", strings.Join(args, " "))
		}
	}
	if n, _ := d.BlockNepsis("b"); n != (db.Nepsis{Count: 5, Minutes: 10, Consequence: "monk", ActionMinutes: 120}) {
		t.Errorf("block bypass prevention = %+v", n)
	}

	// The global one with a time lock: looser is refused until it ends.
	mustRun(t, "nepsis", "set", "--count", "20", "--within", "10m", "--then", "lock-screen", "--then-for", "15m", "--for", "2h")
	if err := runCLI(t, "nepsis", "off"); err == nil || !strings.Contains(err.Error(), "locked until") {
		t.Errorf("turning off a time-locked bypass prevention: %v", err)
	}
	mustRun(t, "nepsis", "set", "--count", "10", "--within", "10m", "--then", "lock-screen", "--then-for", "30m")
	if n, _ := d.GlobalNepsis(); n.Count != 10 || n.ActionMinutes != 30 {
		t.Errorf("global bypass prevention = %+v", n)
	}
	if err := runCLI(t, "nepsis", "set", "--block", "b", "--count", "5", "--within", "10m", "--then", "monk", "--then-for", "2h", "--lock"); err == nil {
		t.Error("a lock on a block's bypass prevention was accepted")
	}
}
