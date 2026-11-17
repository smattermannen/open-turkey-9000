package cli

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

// testPicture writes a small PNG; shade makes each one a different file.
func testPicture(t *testing.T, name string, shade uint8) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{shade, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func guiStateNow(t *testing.T) guiState {
	t.Helper()
	var s guiState
	if err := json.Unmarshal([]byte(capture(t, "gui-state")), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func stateMode(s guiState, name string) *guiMode {
	for i := range s.Modes {
		if s.Modes[i].Name == name {
			return &s.Modes[i]
		}
	}
	return nil
}

// R-MODE-5 (owner 2026-11-17): a mode's wallpapers, laid out as circadian
// rhythm's; Frozen Chicken and monk mode take them too; the override
// setting is one, seen from both sides; a picture goes when nothing uses it.
func TestModeWallpaperCommands(t *testing.T) {
	d := testDB(t)
	school := testPicture(t, "school.png", 1)
	mustRun(t, "mode", "create", "School")
	mustRun(t, "mode", "wallpaper", "School", "add", school, "07:00", "16:00")
	mustRun(t, "mode", "wallpaper", "School", "add", school, "16:00", "18:00")
	mustRun(t, "mode", "wallpaper", "School", "time", "2", "16:00", "19:00")
	m := stateMode(guiStateNow(t), "School")
	if m == nil || len(m.Wallpapers) != 2 || m.Wallpapers[0].Name != "school.png" || m.Wallpapers[1].To != "19:00" {
		t.Fatalf("its wallpapers in the interface's state: %+v", m)
	}
	path := m.Wallpapers[0].Path
	if out := capture(t, "mode", "wallpaper", "School"); !strings.Contains(out, "1: school.png  from 07:00 to 16:00") {
		t.Errorf("listed: %q", out)
	}
	for _, bad := range [][]string{
		{"mode", "wallpaper", "School", "time", "3", "08:00", "09:00"},
		{"mode", "wallpaper", "School", "add", school, "7", "16:00"},
		{"mode", "wallpaper", "Nowhere", "add", school, "07:00", "16:00"},
		{"mode", "wallpaper", "School", "remove"},
	} {
		if runCLI(t, bad...) == nil {
			t.Errorf("accepted: %v", bad)
		}
	}
	mustRun(t, "mode", "wallpaper", "freeze", "add", school, "22:00", "06:00")
	mustRun(t, "mode", "wallpaper", "monk", "add", school, "00:00", "00:00")
	s := guiStateNow(t)
	if fc := stateMode(s, db.FrozenChickenName); fc == nil || len(fc.Wallpapers) != 1 {
		t.Errorf("Frozen Chicken: %+v", fc)
	}
	monk, _ := d.MonkMode()
	if m := stateMode(s, monk.Name); m == nil || len(m.Wallpapers) != 1 {
		t.Errorf("monk mode: %+v", m)
	}
	// One picture, used four times: it goes with the last use.
	mustRun(t, "mode", "wallpaper", "School", "remove", "1")
	mustRun(t, "mode", "wallpaper", "School", "remove", "1")
	mustRun(t, "mode", "wallpaper", "freeze", "remove", "1")
	if _, err := os.Stat(path); err != nil {
		t.Fatal("the picture went while monk mode still shows it")
	}
	mustRun(t, "mode", "wallpaper", "monk", "remove", "1")
	if _, err := os.Stat(path); err == nil {
		t.Error("the picture stayed when nothing used it")
	}

	// Which wins: one setting, in gui-state and circadian rhythm's view.
	if s := guiStateNow(t); s.Wallpaper.OverrideModes || s.Wallpaper.Default != enforce.WallpaperLatest {
		t.Errorf("defaults: %+v", s.Wallpaper)
	}
	mustRun(t, "wallpaper", "priority", "circadian")
	if !guiStateNow(t).Wallpaper.OverrideModes || !circadianJSON(t).OverrideModes {
		t.Error("circadian rhythm first isn't seen from both sides")
	}
	mustRun(t, "wallpaper", "priority", "modes")
	if circadianJSON(t).OverrideModes {
		t.Error("modes first again")
	}

	// The default: a picture of its own, which goes when replaced.
	plain := testPicture(t, "plain.png", 2)
	mustRun(t, "wallpaper", "default", "custom", plain)
	w := guiStateNow(t).Wallpaper
	if w.Default != enforce.WallpaperCustom || w.CustomName != "plain.png" {
		t.Fatalf("custom default: %+v", w)
	}
	if _, err := os.Stat(w.CustomPath); err != nil {
		t.Fatal(err)
	}
	if out := capture(t, "wallpaper"); !strings.Contains(out, "Modes' wallpapers override") || !strings.Contains(out, "Default: plain.png") {
		t.Errorf("shown: %q", out)
	}
	// Another default keeps the picture for when Custom is chosen again; a
	// new picture replaces it.
	mustRun(t, "wallpaper", "default", "circadian")
	if _, err := os.Stat(w.CustomPath); err != nil {
		t.Error("the custom picture went with another default")
	}
	mustRun(t, "wallpaper", "default", "custom")
	if s := guiStateNow(t).Wallpaper; s.Default != enforce.WallpaperCustom || s.CustomName != "plain.png" {
		t.Errorf("custom again: %+v", s)
	}
	mustRun(t, "wallpaper", "default", "custom", testPicture(t, "grey.png", 5))
	if _, err := os.Stat(w.CustomPath); err == nil {
		t.Error("the replaced picture stayed")
	}
	testDB(t)
	for _, bad := range [][]string{{"wallpaper", "default", "custom"}, {"wallpaper", "default", "sometimes"},
		{"wallpaper", "priority", "both"}, {"wallpaper", "default", "latest", plain}} {
		if runCLI(t, bad...) == nil {
			t.Errorf("accepted: %v", bad)
		}
	}
	// The sleep hook's command runs without the access passwords and does
	// nothing on a test's database.
	mustRun(t, "wallpaper", "before-sleep")
}

// Project files carry a mode's wallpapers and custom actions, and the
// wallpaper settings; a file whose custom actions open or run something is
// imported only once that is confirmed (R-CUSTOM-1).
func TestProjectModeWallpapersAndCustoms(t *testing.T) {
	notTerminal(t)
	testDB(t)
	school := testPicture(t, "school.png", 3)
	plain := testPicture(t, "plain.png", 4)
	mustRun(t, "block", "create", "lessons")
	mustRun(t, "mode", "create", "School")
	mustRun(t, "mode", "add", "School", "lessons")
	mustRun(t, "mode", "wallpaper", "School", "add", school, "07:00", "16:00")
	mustRun(t, "mode", "custom", "School", "add", "run libreoffice --writer")
	mustRun(t, "mode", "wallpaper", "freeze", "add", school, "22:00", "06:00")
	mustRun(t, "wallpaper", "default", "custom", plain)
	mustRun(t, "wallpaper", "priority", "circadian")
	out := capture(t, "project", "export", "--share")
	file := filepath.Join(t.TempDir(), "p.json")
	if err := os.WriteFile(file, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}

	d2 := testDB(t)
	dry := capture(t, "project", "import", "--dry-run", file)
	for _, want := range []string{"Would: add a custom action that can run libreoffice --writer",
		"Would: give mode 'School' 1 wallpapers", "Would: set the wallpaper settings (default: plain.png)"} {
		if !strings.Contains(dry, want) {
			t.Errorf("the dry run doesn't say %q:\n%s", want, dry)
		}
	}
	err := runCLI(t, "project", "import", file)
	if err == nil || !strings.Contains(err.Error(), "--yes") || !strings.Contains(err.Error(), "run libreoffice --writer") {
		t.Fatalf("imported custom actions without confirmation: %v", err)
	}
	if _, err := d2.GetMode("School"); err == nil {
		t.Fatal("something was written before the confirmation")
	}
	capture(t, "project", "import", file, "--yes")
	m, err := d2.GetMode("School")
	if err != nil {
		t.Fatal(err)
	}
	ws := enforce.ModeWallpapers(d2, m.ID)
	if len(ws) != 1 || ws[0].From != 7*60 {
		t.Fatalf("wallpapers %+v", ws)
	}
	if _, err := os.Stat(filepath.Join(enforce.WallpapersDir(), ws[0].Image)); err != nil {
		t.Errorf("picture not stored: %v", err)
	}
	if c, _ := d2.ModeCustom("School"); !strings.Contains(c, "libreoffice") {
		t.Errorf("custom %q", c)
	}
	s := enforce.LoadWallpaperSettings(d2)
	if !s.CircadianFirst || s.Default != enforce.WallpaperCustom || s.CustomName != "plain.png" {
		t.Errorf("settings %+v", s)
	}
	if len(enforce.ModeWallpapers(d2, enforce.FrozenChickenWallpapers)) != 1 {
		t.Error("Frozen Chicken's wallpapers")
	}
}

// A backup carries hotkeys that run actions (owner 2026-11-17): imported with
// their blocks' new names, left out when what they use isn't there.
func TestBackupHotkeyActions(t *testing.T) {
	notTerminal(t)
	d := testDB(t)
	mustRun(t, "block", "create", "work")
	mustRun(t, "mode", "create", "Evening")
	if _, err := d.AddHotkey(db.Hotkey{Keys: "<Primary><Alt>p", Block: "work", Actions: []db.HotkeyAction{
		{Kind: db.HotkeyStart, Target: "work"}, {Kind: db.HotkeyModeOn, Target: "Evening"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.AddHotkey(db.Hotkey{Keys: "<Primary><Alt>c", Actions: []db.HotkeyAction{
		{Kind: db.HotkeyCustom, Param: `[{"open":"https://example.org"}]`}}}); err != nil {
		t.Fatal(err)
	}
	out := capture(t, "project", "export")
	file := filepath.Join(t.TempDir(), "b.json")
	if err := os.WriteFile(file, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	d2 := testDB(t)
	mustRun(t, "block", "create", "work") // the file's 'work' comes in as 'work-2'
	capture(t, "project", "import", file, "--yes")
	keys, _ := d2.Hotkeys()
	byKeys := map[string]db.Hotkey{}
	for _, h := range keys {
		byKeys[h.Keys] = h
	}
	p, ok := byKeys["<Primary><Alt>p"]
	if !ok || p.Actions[0].Target != "work-2" || p.Actions[1].Target != "Evening" {
		t.Errorf("the hotkey of actions: %+v (all %+v)", p, keys)
	}
	if c, ok := byKeys["<Primary><Alt>c"]; !ok || c.Actions[0].Kind != db.HotkeyCustom {
		t.Errorf("the hotkey that opens a site: %+v", keys)
	}
}
