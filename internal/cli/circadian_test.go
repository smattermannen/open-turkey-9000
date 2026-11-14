package cli

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/circadian"
	"github.com/smattermannen/kc9k/internal/enforce"
)

func circadianJSON(t *testing.T) circadianView {
	t.Helper()
	var view circadianView
	if err := json.Unmarshal([]byte(capture(t, "circadian", "show", "--json")), &view); err != nil {
		t.Fatal(err)
	}
	return view
}

// §4.10: the clips, through the commands the interface uses.
func TestCircadianClips(t *testing.T) {
	d := testDB(t)
	circadianNow = func() time.Time { return time.Date(2026, 11, 14, 10, 0, 0, 0, time.Local) }
	t.Cleanup(func() { circadianNow = time.Now })

	view := circadianJSON(t)
	if view.On || len(view.Lanes) != 3 || view.Lanes[0].Name != "backlight" || view.Lanes[2].Name != "kelvin" ||
		len(view.Presets) < 3 || view.Minute != 600 || view.Dark.Mode != "system" || view.Hardware.Daemon {
		t.Fatalf("default view %+v", view)
	}
	mustRun(t, "circadian", "on")
	mustRun(t, "circadian", "lane", "backlight", "on", "--points", "08:00=40 12:00=80~0.5, 20:00=40")
	view = circadianJSON(t)
	l := view.Lanes[0]
	if !view.On || !l.On || len(l.Points) != 3 || l.Points[1].Tension != 0.5 || !l.HasValue || l.Value != 60 {
		t.Fatalf("backlight %+v", l)
	}
	if view.Lanes[1].On || view.Lanes[2].On {
		t.Error("the other clips were switched on too")
	}
	mustRun(t, "circadian", "point", "add", "backlight", "10:00", "70", "--tension", "-0.25")
	mustRun(t, "circadian", "point", "remove", "backlight", "12:00")
	if got := circadian.FormatPoints(enforce.CircadianConfig(d).Lanes["backlight"].Points); got != "08:00=40 10:00=70~-0.25 20:00=40" {
		t.Errorf("points %q", got)
	}
	mustRun(t, "circadian", "lane", "backlight", "off")
	if c := enforce.CircadianConfig(d); c.Lanes["backlight"].On || len(c.Lanes["backlight"].Points) != 3 {
		t.Error("off lost the points or didn't switch off")
	}
	mustRun(t, "circadian", "preset", "candlelight")
	if got := enforce.CircadianConfig(d).Lanes["kelvin"].Points[0].Value; got != 1900 {
		t.Errorf("preset kelvin starts at %v", got)
	}
	for _, bad := range [][]string{
		{"circadian", "lane", "volume", "on"},
		{"circadian", "lane", "kelvin", "--points", "08:00=200"},
		{"circadian", "lane", "kelvin", "--points", "25:00=3000"},
		{"circadian", "point", "add", "software", "08:00", "5"},
		{"circadian", "point", "add", "software", "08:00", "50", "--tension", "2"},
		{"circadian", "point", "remove", "software", "03:33"},
		{"circadian", "preset", "disco"},
	} {
		if runCLI(t, bad...) == nil {
			t.Errorf("%v was accepted", bad)
		}
	}
	mustRun(t, "circadian", "off")
	if enforce.CircadianConfig(d).On {
		t.Error("still on")
	}
}

// §4.10: circadian settings are never locked: every change goes through
// while a block is locked and with no weakening during active locks on.
func TestCircadianIsNeverLocked(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "work")
	mustRun(t, "block", "add", "work", "example.com")
	mustRun(t, "start", "work", "--for", "1h", "--yes")
	mustRun(t, "no-weakening", "on")
	mustRun(t, "circadian", "on")
	mustRun(t, "circadian", "lane", "kelvin", "on")
	mustRun(t, "circadian", "dark", "dark")
	mustRun(t, "circadian", "dark-pages", "on")
	mustRun(t, "circadian", "dark-pages", "off")
	mustRun(t, "circadian", "dark", "system")
	mustRun(t, "circadian", "lane", "kelvin", "off")
	mustRun(t, "circadian", "off")
	if enforce.CircadianConfig(d).On {
		t.Error("still on")
	}
}

func TestCircadianWallpapers(t *testing.T) {
	d := testDB(t)
	circadianNow = func() time.Time { return time.Date(2026, 11, 14, 10, 0, 0, 0, time.Local) }
	t.Cleanup(func() { circadianNow = time.Now })
	picture := filepath.Join(t.TempDir(), "dawn.png")
	f, err := os.Create(picture)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	f.Close()
	mustRun(t, "circadian", "wallpaper", "add", picture, "06:00", "12:00")
	mustRun(t, "circadian", "wallpaper", "add", picture, "22:00", "02:00")
	view := circadianJSON(t)
	if len(view.Wallpapers) != 2 || view.Wallpapers[0].Name != "dawn.png" || view.Wallpapers[0].From != "06:00" ||
		!view.Wallpapers[0].Active || view.Wallpapers[1].Active || view.Wallpapers[1].To != "02:00" {
		t.Fatalf("wallpapers %+v", view.Wallpapers)
	}
	info, err := os.Stat(view.Wallpapers[0].Path)
	if err != nil || info.Mode().Perm() != 0o644 || !strings.HasSuffix(view.Wallpapers[0].Path, ".png") {
		t.Fatalf("stored file: %v %v", info, err)
	}
	mustRun(t, "circadian", "wallpaper", "time", "1", "13:00", "14:00")
	if w := enforce.CircadianConfig(d).Wallpapers[0]; w.From != 13*60 || w.To != 14*60 {
		t.Errorf("times %+v", w)
	}
	mustRun(t, "circadian", "wallpaper", "remove", "1")
	if _, err := os.Stat(view.Wallpapers[0].Path); err != nil {
		t.Error("the file went although another time still uses it")
	}
	mustRun(t, "circadian", "wallpaper", "remove", "1")
	if _, err := os.Stat(view.Wallpapers[0].Path); err == nil {
		t.Error("the file stayed")
	}
	text := filepath.Join(t.TempDir(), "notes.txt")
	_ = os.WriteFile(text, []byte("not a picture"), 0o644)
	for _, bad := range [][]string{
		{"circadian", "wallpaper", "add", text, "06:00", "12:00"},
		{"circadian", "wallpaper", "add", picture, "6", "12:00"},
		{"circadian", "wallpaper", "remove", "1"},
		{"circadian", "wallpaper", "time", "0", "06:00", "12:00"},
	} {
		if runCLI(t, bad...) == nil {
			t.Errorf("%v was accepted", bad)
		}
	}
}

func TestCircadianDarkModeAndPages(t *testing.T) {
	d := testDB(t)
	circadianNow = func() time.Time { return time.Date(2026, 11, 14, 22, 0, 0, 0, time.Local) }
	t.Cleanup(func() { circadianNow = time.Now })
	mustRun(t, "circadian", "on")
	mustRun(t, "circadian", "dark", "light-between", "07:00", "20:00")
	if v := circadianJSON(t).Dark; v.Mode != "scheduled" || v.LightFrom != "07:00" || v.LightTo != "20:00" || v.Now != "dark" {
		t.Errorf("dark %+v", v)
	}
	mustRun(t, "circadian", "dark", "light")
	if v := circadianJSON(t).Dark; v.Mode != "light" || v.Now != "light" {
		t.Errorf("dark %+v", v)
	}
	mustRun(t, "circadian", "dark", "system")
	mustRun(t, "circadian", "dark-pages", "on")
	mustRun(t, "circadian", "exempt", "add", "https://Docs.Example.com/page", "maps.example.org")
	mustRun(t, "circadian", "exempt", "add", "docs.example.com")
	c := enforce.CircadianConfig(d)
	if !c.DarkPages || strings.Join(c.DarkExempt, " ") != "docs.example.com maps.example.org" {
		t.Errorf("dark pages %v, exempt %v", c.DarkPages, c.DarkExempt)
	}
	mustRun(t, "circadian", "exempt", "remove", "maps.example.org")
	if v := circadianJSON(t); len(v.DarkExempt) != 1 || !v.DarkPages {
		t.Errorf("view %+v", v)
	}
	for _, bad := range [][]string{
		{"circadian", "dark", "purple"},
		{"circadian", "dark", "light-between", "07:00", "07:00"},
		{"circadian", "dark-pages", "maybe"},
		{"circadian", "exempt", "add", "not a site"},
		{"circadian", "exempt", "remove", "nowhere.example"},
	} {
		if runCLI(t, bad...) == nil {
			t.Errorf("%v was accepted", bad)
		}
	}
}

// Preview, resume and the night light switch never reach the hardware of
// the computer the tests run on (the database comes from KC9K_DB).
func TestCircadianPreviewAndResume(t *testing.T) {
	d := testDB(t)
	real := turnOffNightLight
	turnOffNightLight = func() error { t.Error("the desktop's night light was touched"); return nil }
	t.Cleanup(func() { turnOffNightLight = real })
	if runCLI(t, "circadian", "preview", "kelvin", "3000") == nil {
		t.Error("previewed a clip that is off")
	}
	mustRun(t, "circadian", "on")
	mustRun(t, "circadian", "lane", "kelvin", "on")
	mustRun(t, "circadian", "preview", "kelvin", "3000")
	if runCLI(t, "circadian", "preview", "kelvin", "20") == nil {
		t.Error("20 K accepted")
	}
	mustRun(t, "circadian", "preview", "kelvin", "--end")
	mustRun(t, "circadian", "nightlight", "off")
	// A pause the daemon noted shows on the clip and goes with resume.
	until := time.Now().Add(time.Hour).Unix()
	if err := d.SetSetting("circadian.paused", fmt.Sprintf(`{"gamma":%d}`, until)); err != nil {
		t.Fatal(err)
	}
	view := circadianJSON(t)
	if view.Lanes[2].PausedUntil != until || view.Lanes[1].PausedUntil != until || view.Lanes[0].PausedUntil != 0 {
		t.Fatalf("pauses %+v", view.Lanes)
	}
	if runCLI(t, "circadian", "preview", "kelvin", "3000") == nil {
		t.Error("previewed a paused clip")
	}
	mustRun(t, "circadian", "resume", "kelvin")
	if circadianJSON(t).Lanes[2].PausedUntil != 0 {
		t.Error("still paused")
	}
}

func TestCircadianAccessArea(t *testing.T) {
	if got := commandAreas["circadian"]; len(got) != 1 || got[0] != areaSettings {
		t.Errorf("circadian belongs to %v", got)
	}
}
