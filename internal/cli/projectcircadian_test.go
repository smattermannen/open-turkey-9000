package cli

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/enforce"
)

// ROADMAP 2.1: project files carry the circadian rhythm and its wallpapers;
// import sets them only where none are set.
func TestProjectCircadian(t *testing.T) {
	notTerminal(t)
	testDB(t)
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	pic := filepath.Join(t.TempDir(), "dawn.png")
	if err := os.WriteFile(pic, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "circadian", "wallpaper", "add", pic, "06:00", "12:00")
	out := capture(t, "project", "export", "--share")
	if !strings.Contains(out, `"circadian"`) || !strings.Contains(out, `"wallpapers"`) {
		t.Fatalf("no circadian part in the file")
	}
	file := filepath.Join(t.TempDir(), "p.json")
	if err := os.WriteFile(file, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	d2 := testDB(t)
	capture(t, "project", "import", file)
	c := enforce.CircadianConfig(d2)
	if len(c.Wallpapers) != 1 {
		t.Fatalf("wallpapers = %+v", c.Wallpapers)
	}
	if _, err := os.Stat(filepath.Join(enforce.WallpapersDir(), c.Wallpapers[0].Image)); err != nil {
		t.Errorf("picture not stored: %v", err)
	}
	// A picture that isn't what its name says is refused.
	if _, err := checkedWallpaper("0000.png", "iVBORw0KGgo="); err == nil {
		t.Error("a damaged wallpaper was accepted")
	}
}
