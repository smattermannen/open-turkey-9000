package enforce

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/circadian"
	"github.com/smattermannen/kc9k/internal/db"
)

// wallpaperMode makes a mode with one block, switched on, and a wallpaper
// all day; with places, the mode is enabled at them.
func wallpaperMode(t *testing.T, d *db.DB, mode, block, image string, places ...int) db.Mode {
	t.Helper()
	if err := d.CreateBlock(block); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateMode(mode); err != nil {
		t.Fatal(err)
	}
	m, _ := d.GetMode(mode)
	spec := db.AutomationSpec{Places: places}
	if err := d.SetModeAutomation(m.ID, spec); err != nil {
		t.Fatal(err)
	}
	if err := d.SetBlockMode(block, m.ID); err != nil {
		t.Fatal(err)
	}
	if err := d.SetBlockAutomation(block, spec); err != nil {
		t.Fatal(err)
	}
	if err := d.ActivateBlock(block, false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := SetModeWallpapers(d, m.ID, []circadian.Wallpaper{{Image: image, Name: image}}); err != nil {
		t.Fatal(err)
	}
	return m
}

// seenWifi makes the networks in range *seen.
func seenWifi(t *testing.T, seen *[]db.Network) {
	t.Helper()
	old := ScanWifi
	ScanWifi = func(bool) ([]db.Network, error) { return *seen, nil }
	t.Cleanup(func() { ScanWifi = old; ForgetSeenNetworks() })
	ForgetSeenNetworks()
}

func shown(f *fakeScreen) string {
	if len(f.wallpapers) == 0 {
		return ""
	}
	return filepath.Base(f.wallpapers[len(f.wallpapers)-1])
}

// R-MODE-5: an enabled mode's wallpaper shows over circadian rhythm's,
// unless circadian rhythm overrides modes; a mode that isn't enabled shows
// none.
func TestModeWallpaperOverCircadian(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	none := []db.Network{}
	seenWifi(t, &none)
	circadianWith(t, d, nil, func(c *circadian.Config) {
		c.Wallpapers = []circadian.Wallpaper{{Image: "dawn.png", Name: "dawn.png"}}
	})
	wallpaperMode(t, d, "Work", "work", "work.png")
	e := NewCircadian(d)
	e.Step(at(10, 0, 0))
	if shown(f) != "work.png" {
		t.Fatalf("shown %q, want the mode's", f.wallpapers)
	}
	if err := SaveWallpaperSettings(d, WallpaperSettings{CircadianFirst: true, Default: WallpaperLatest}); err != nil {
		t.Fatal(err)
	}
	e.Step(at(10, 0, 5))
	if shown(f) != "dawn.png" {
		t.Errorf("with circadian rhythm first: %q", f.wallpapers)
	}
	if err := SaveWallpaperSettings(d, WallpaperSettings{Default: WallpaperLatest}); err != nil {
		t.Fatal(err)
	}
	e.Step(at(10, 0, 10))
	if err := d.DeactivateBlock("work"); err != nil {
		t.Fatal(err)
	}
	e.Step(at(10, 0, 15))
	if shown(f) != "dawn.png" || len(f.wallpapers) != 4 {
		t.Errorf("the mode switched off still shows: %q", f.wallpapers)
	}
	if slices.Contains(f.hooks, true) {
		t.Errorf("a sleep hook without a mode switched by places: %v", f.hooks)
	}
}

// R-MODE-5: of several enabled modes, the one enabled last wins.
func TestModeWallpaperLatestEnabledWins(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	none := []db.Network{}
	seenWifi(t, &none)
	wallpaperMode(t, d, "Work", "work", "work.png")
	e := NewCircadian(d)
	e.Step(at(9, 0, 0))
	wallpaperMode(t, d, "Exam", "exam", "exam.png")
	e.Step(at(9, 0, 5))
	if shown(f) != "exam.png" {
		t.Fatalf("the mode enabled later doesn't win: %q", f.wallpapers)
	}
	if err := d.DeactivateBlock("exam"); err != nil {
		t.Fatal(err)
	}
	e.Step(at(9, 0, 10))
	if shown(f) != "work.png" {
		t.Errorf("back to the other: %q", f.wallpapers)
	}
}

// R-MODE-5 (the owner's example): at home the anime wallpaper, at school
// the school one. Without a known network neither can be told, so the
// default shows (here a plain picture), at once at boot and after waking,
// not after one look without networks a moment after the place was known;
// the latest wallpaper as the default changes nothing. Circadian rhythm's
// home wallpaper doesn't show either: a mode might override it.
func TestModeWallpaperUnknownPlace(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	home := []db.Network{{BSSID: "AA:00:00:00:00:01"}, {BSSID: "AA:00:00:00:00:02"}}
	school := []db.Network{{BSSID: "BB:00:00:00:00:01"}, {BSSID: "BB:00:00:00:00:02"}}
	var seen []db.Network
	seenWifi(t, &seen)
	for name, nets := range map[string][]db.Network{"home": home, "school": school} {
		if _, err := d.SavePlace(name, nets); err != nil {
			t.Fatal(err)
		}
	}
	h, _ := d.GetPlace("home")
	s, _ := d.GetPlace("school")
	circadianWith(t, d, nil, func(c *circadian.Config) {
		c.Wallpapers = []circadian.Wallpaper{{Image: "anime.png", Name: "anime.png"}}
	})
	wallpaperMode(t, d, "Home", "home", "anime.png", h.ID)
	wallpaperMode(t, d, "School", "school", "school.png", s.ID)
	if err := SaveWallpaperSettings(d, WallpaperSettings{Default: WallpaperCustom, Custom: "plain.png", CustomName: "plain.png"}); err != nil {
		t.Fatal(err)
	}
	// Boot, no network yet: the default, and the engine looks every second.
	e := NewCircadian(d)
	start := time.Now()
	e.Step(start)
	if shown(f) != "plain.png" {
		t.Fatalf("at boot without networks: %q", f.wallpapers)
	}
	if !e.lookingForPlace(start.Add(2 * time.Second)) {
		t.Error("not looking for the place every second")
	}
	if !slices.Equal(f.hooks, []bool{true}) {
		t.Errorf("sleep hook %v", f.hooks)
	}
	// The school's networks: its wallpaper.
	seen = school
	ForgetSeenNetworks()
	e.Step(start.Add(3 * time.Second))
	if shown(f) != "school.png" || e.lookingForPlace(start.Add(4*time.Second)) {
		t.Fatalf("at school: %q", f.wallpapers)
	}
	// One look without networks: nothing changes for a minute.
	seen = nil
	ForgetSeenNetworks()
	e.Step(start.Add(20 * time.Second))
	if len(f.wallpapers) != 2 {
		t.Errorf("one look without networks changed the wallpaper: %q", f.wallpapers)
	}
	e.Step(start.Add(90 * time.Second))
	if shown(f) != "plain.png" {
		t.Errorf("long without networks: %q", f.wallpapers)
	}
	// Home: the anime wallpaper. The lid closes; it wakes at school with no
	// network yet: the default at once.
	seen = home
	ForgetSeenNetworks()
	e.Step(start.Add(100 * time.Second))
	if shown(f) != "anime.png" {
		t.Fatalf("at home: %q", f.wallpapers)
	}
	seen, f.slept = nil, 3600
	e.Step(start.Add(2 * time.Hour))
	if shown(f) != "plain.png" {
		t.Errorf("woken without networks: %q", f.wallpapers)
	}
	seen = school
	ForgetSeenNetworks()
	e.Step(start.Add(2*time.Hour + time.Second))
	if shown(f) != "school.png" {
		t.Errorf("woken at school: %q", f.wallpapers)
	}
	// The latest wallpaper as the default: without networks nothing changes,
	// and back at school the school wallpaper isn't given again.
	if err := SaveWallpaperSettings(d, WallpaperSettings{Default: WallpaperLatest}); err != nil {
		t.Fatal(err)
	}
	n := len(f.wallpapers)
	seen, f.slept = nil, 7200
	e.Step(start.Add(3 * time.Hour))
	seen = school
	ForgetSeenNetworks()
	e.Step(start.Add(3*time.Hour + time.Second))
	if len(f.wallpapers) != n {
		t.Errorf("with the latest wallpaper as default: %q", f.wallpapers[n:])
	}
	if last := f.hooks[len(f.hooks)-1]; last {
		t.Errorf("the sleep hook stays with the latest wallpaper as default: %v", f.hooks)
	}
}

// R-MODE-5: before sleep the default goes on the desktop (the sleep hook);
// after waking, what should show is given afresh.
func TestWallpaperBeforeSleep(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	school := []db.Network{{BSSID: "BB:00:00:00:00:01"}, {BSSID: "BB:00:00:00:00:02"}}
	seen := school
	seenWifi(t, &seen)
	if _, err := d.SavePlace("school", school); err != nil {
		t.Fatal(err)
	}
	s, _ := d.GetPlace("school")
	wallpaperMode(t, d, "School", "school", "school.png", s.ID)
	if err := SaveWallpaperSettings(d, WallpaperSettings{Default: WallpaperCustom, Custom: "plain.png", CustomName: "plain.png"}); err != nil {
		t.Fatal(err)
	}
	e := NewCircadian(d)
	now := time.Now()
	e.Step(now)
	if shown(f) != "school.png" {
		t.Fatalf("at school: %q", f.wallpapers)
	}
	if err := WallpaperBeforeSleep(d, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if shown(f) != "plain.png" {
		t.Fatalf("before sleep: %q", f.wallpapers)
	}
	// Still awake a moment: the engine leaves it.
	e.Step(now.Add(2 * time.Second))
	if shown(f) != "plain.png" {
		t.Errorf("undone before sleeping: %q", f.wallpapers)
	}
	// Woken at school: given afresh.
	f.slept = 600
	e.Step(now.Add(time.Hour))
	if shown(f) != "school.png" {
		t.Errorf("after waking at school: %q", f.wallpapers)
	}
	// Nothing to do with the latest wallpaper as default.
	n := len(f.wallpapers)
	if err := SaveWallpaperSettings(d, WallpaperSettings{Default: WallpaperLatest}); err != nil {
		t.Fatal(err)
	}
	if err := WallpaperBeforeSleep(d, now.Add(2*time.Hour)); err != nil || len(f.wallpapers) != n {
		t.Errorf("latest: %v %q", err, f.wallpapers[n:])
	}
}

// A stored picture stays while circadian rhythm, a mode or the default
// uses it.
func TestWallpaperInUse(t *testing.T) {
	d := testDB(t)
	circadianWith(t, d, nil, func(c *circadian.Config) {
		c.Wallpapers = []circadian.Wallpaper{{Image: "a.png"}}
	})
	m := wallpaperMode(t, d, "Work", "work", "b.png")
	if err := SetModeWallpapers(d, FrozenChickenWallpapers, []circadian.Wallpaper{{Image: "d.png"}}); err != nil {
		t.Fatal(err)
	}
	if err := SaveWallpaperSettings(d, WallpaperSettings{Default: WallpaperCustom, Custom: "c.png"}); err != nil {
		t.Fatal(err)
	}
	for image, want := range map[string]bool{"a.png": true, "b.png": true, "c.png": true, "d.png": true, "e.png": false} {
		if WallpaperInUse(d, image) != want {
			t.Errorf("%s in use: %v", image, !want)
		}
	}
	if err := SetModeWallpapers(d, m.ID, nil); err != nil || WallpaperInUse(d, "b.png") {
		t.Errorf("removed: %v", err)
	}
	if err := SaveWallpaperSettings(d, WallpaperSettings{Default: WallpaperCustom}); err == nil {
		t.Error("custom without a picture")
	}
}
