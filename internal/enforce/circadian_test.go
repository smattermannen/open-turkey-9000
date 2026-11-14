package enforce

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/circadian"
	"github.com/smattermannen/kc9k/internal/db"
)

// fakeScreen stands in for the hardware of the circadian rhythm (§4.10).
type fakeScreen struct {
	backlight  int // of 1000
	external   int // of 100
	red, blue  []uint16
	gammaSets  int
	noX        bool
	wallpapers []string
	darks      []bool
	users      int
	nightLight bool
}

func plain256() []uint16 {
	r := make([]uint16, 256)
	for i := range r {
		r[i] = uint16(math.Round(65535 * float64(i) / 255))
	}
	return r
}

func near(a, b []uint16) bool {
	for i := range a {
		if math.Abs(float64(a[i])-float64(b[i])) > 1024 {
			return false
		}
	}
	return true
}

// someoneSetsGamma is another program (a night light) changing the ramps.
func (f *fakeScreen) someoneSetsGamma() {
	f.red, _, f.blue = circadian.Ramp(256, 1234, 0.77)
}

// fakeCircadian replaces every piece of hardware and lets the engine run
// although the database is a test's.
func fakeCircadian(t *testing.T) *fakeScreen {
	t.Helper()
	f := &fakeScreen{backlight: 500, external: 50, red: plain256(), blue: plain256(), users: 1}
	dry, backlights, external, ddc := circadianDry, circadianBacklights, circadianExternal, circadianDDC
	readGamma, setGamma := circadianReadGamma, circadianSetGamma
	setWallpaper, setDark, nightLight := circadianSetWallpaper, circadianSetDark, circadianNightLight
	t.Cleanup(func() {
		circadianDry, circadianBacklights, circadianExternal, circadianDDC = dry, backlights, external, ddc
		circadianReadGamma, circadianSetGamma = readGamma, setGamma
		circadianSetWallpaper, circadianSetDark, circadianNightLight = setWallpaper, setDark, nightLight
	})
	circadianDry = func() bool { return false }
	circadianBacklights = func() []circadianLevel {
		return []circadianLevel{{Name: "panel", Max: 1000,
			Get: func() (int, error) { return f.backlight, nil },
			Set: func(v int) error { f.backlight = v; return nil }}}
	}
	circadianExternal = func() []circadianLevel {
		return []circadianLevel{{Name: "monitor", Max: 100,
			Get: func() (int, error) { return f.external, nil },
			Set: func(v int) error { f.external = v; return nil }}}
	}
	circadianDDC = func() bool { return true }
	circadianReadGamma = func(want blocker.RampFunc) blocker.GammaReading {
		if f.noX {
			return blocker.GammaUnknown
		}
		if want != nil {
			if r, _, b := want(256); near(r, f.red) && near(b, f.blue) {
				return blocker.GammaOurs
			}
		}
		if near(plain256(), f.red) && near(plain256(), f.blue) {
			return blocker.GammaIdentity
		}
		return blocker.GammaOther
	}
	circadianSetGamma = func(ramp blocker.RampFunc) (int, error) {
		if f.noX {
			return 0, nil
		}
		f.gammaSets++
		f.red, f.blue = plain256(), plain256()
		if ramp != nil {
			f.red, _, f.blue = ramp(256)
		}
		return 1, nil
	}
	circadianSetWallpaper = func(path string) (int, error) {
		f.wallpapers = append(f.wallpapers, path)
		return f.users, nil
	}
	circadianSetDark = func(dark bool) (int, error) {
		f.darks = append(f.darks, dark)
		return f.users, nil
	}
	circadianNightLight = func() bool { return f.nightLight }
	return f
}

// at is a time of day on a fixed date.
func at(hour, minute, second int) time.Time {
	return time.Date(2026, 11, 14, hour, minute, second, 0, time.Local)
}

// circadianWith stores a config that is on, with the given clips on and
// every other clip off.
func circadianWith(t *testing.T, d *db.DB, lanes map[string][]circadian.Point, change func(*circadian.Config)) {
	t.Helper()
	c := circadian.Default()
	c.On = true
	for name, l := range c.Lanes {
		l.Points, l.On = lanes[name], lanes[name] != nil
	}
	if change != nil {
		change(&c)
	}
	if err := SetCircadianConfig(d, c); err != nil {
		t.Fatal(err)
	}
}

// Backlight 40 % at 08:00, 80 % at 12:00, 40 % at 20:00.
var testBacklight = map[string][]circadian.Point{
	circadian.LaneBacklight: {{Minute: 8 * 60, Value: 40}, {Minute: 12 * 60, Value: 80}, {Minute: 20 * 60, Value: 40}},
}

func TestCircadianBacklightFollowsTheClip(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	circadianWith(t, d, testBacklight, nil)
	e := NewCircadian(d)
	e.Step(at(8, 0, 0))
	if f.backlight != 400 || f.external != 40 {
		t.Fatalf("at 08:00: backlight %d of 1000, monitor %d", f.backlight, f.external)
	}
	// Past a point: the clip rises on towards 80 % at 12:00.
	e.Step(at(10, 0, 0))
	if f.backlight != 600 {
		t.Errorf("at 10:00: backlight %d, want 600", f.backlight)
	}
	e.Step(at(12, 0, 30))
	if f.backlight != 800 || f.external != 80 {
		t.Errorf("at 12:00: backlight %d, monitor %d", f.backlight, f.external)
	}
	if s, ok := ReadCircadianStatus(d); !ok || len(s.Backlights) != 1 || len(s.External) != 1 || !s.DDCUtil {
		t.Errorf("status %+v %v", s, ok)
	}
}

// Never override the user: a level changed by hand pauses its clip until
// the clip's next point; KC9K never puts the level back.
func TestCircadianNeverOverridesTheUser(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	circadianWith(t, d, testBacklight, nil)
	e := NewCircadian(d)
	e.Step(at(9, 0, 0))
	if f.backlight != 500 {
		t.Fatalf("backlight %d", f.backlight)
	}
	f.backlight = 900 // the user's brightness keys
	e.Step(at(9, 0, 5))
	e.Step(at(11, 0, 0))
	if f.backlight != 900 {
		t.Fatalf("KC9K put the level back: %d", f.backlight)
	}
	pauses := CircadianPauses(d, at(11, 0, 0))
	if got := time.Unix(pauses[PauseBacklight], 0); !got.Equal(at(12, 0, 0)) {
		t.Fatalf("paused until %v, want the next point at 12:00", got)
	}
	if LanePausedUntil(pauses, circadian.LaneBacklight) == 0 || LanePausedUntil(pauses, circadian.LaneKelvin) != 0 {
		t.Errorf("pauses %v", pauses)
	}
	// The monitor wasn't touched by hand: it carries on.
	if f.external != 70 {
		t.Errorf("monitor %d, want 70", f.external)
	}
	// At the next point the clip takes over again.
	e.Step(at(12, 0, 1))
	if f.backlight != 800 {
		t.Errorf("after the next point: %d, want 800", f.backlight)
	}
	if len(CircadianPauses(d, at(12, 0, 1))) != 0 {
		t.Error("still paused")
	}
}

func TestCircadianResume(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	circadianWith(t, d, testBacklight, nil)
	e := NewCircadian(d)
	e.Step(at(9, 0, 0))
	f.backlight = 900
	e.Step(at(9, 0, 5))
	if len(CircadianPauses(d, at(9, 0, 5))) != 1 {
		t.Fatal("not paused")
	}
	if err := ResumeCircadian(d, circadian.LaneBacklight, at(9, 0, 10)); err != nil {
		t.Fatal(err)
	}
	e.Step(at(9, 0, 10))
	if f.backlight != 500 || len(CircadianPauses(d, at(9, 0, 10))) != 0 {
		t.Errorf("after resume: backlight %d, pauses %v", f.backlight, CircadianPauses(d, at(9, 0, 10)))
	}
}

// The desktop dims an idle screen and lights it again: back at KC9K's own
// level, the clip carries on.
func TestCircadianResumesWhenTheLevelComesBack(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	circadianWith(t, d, testBacklight, nil)
	e := NewCircadian(d)
	e.Step(at(9, 0, 0))
	f.backlight = 150
	e.Step(at(9, 0, 5))
	if f.backlight != 150 {
		t.Fatal("overridden")
	}
	f.backlight = 500
	e.Step(at(9, 30, 0))
	if f.backlight != 550 || len(CircadianPauses(d, at(9, 30, 0))) != 0 {
		t.Errorf("backlight %d, pauses %v", f.backlight, CircadianPauses(d, at(9, 30, 0)))
	}
}

// A pause outlives the daemon: a new engine doesn't touch a paused level.
func TestCircadianPauseSurvivesRestart(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	circadianWith(t, d, testBacklight, nil)
	e := NewCircadian(d)
	e.Step(at(9, 0, 0))
	f.backlight = 900
	e.Step(at(9, 0, 5))
	NewCircadian(d).Step(at(9, 1, 0))
	if f.backlight != 900 {
		t.Errorf("a restarted daemon overrode the user: %d", f.backlight)
	}
}

var testGamma = map[string][]circadian.Point{
	circadian.LaneKelvin:   {{Minute: 8 * 60, Value: 6500}, {Minute: 20 * 60, Value: 3000}},
	circadian.LaneSoftware: {{Minute: 0, Value: 100}},
}

func TestCircadianGamma(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	circadianWith(t, d, testGamma, nil)
	e := NewCircadian(d)
	// Plain ramps: apply.
	e.Step(at(20, 0, 0))
	warm, _, cold := circadian.Ramp(256, 3000, 1)
	if !near(f.red, warm) || !near(f.blue, cold) {
		t.Fatal("3000 K wasn't applied")
	}
	// Ours and nothing changed: nothing is written.
	sets := f.gammaSets
	e.Step(at(20, 0, 0))
	if f.gammaSets != sets {
		t.Error("the same ramp was written again")
	}
	// The screens went back to plain ramps (a monitor switched off and on):
	// apply again.
	f.red, f.blue = plain256(), plain256()
	e.Step(at(20, 0, 5))
	if !near(f.blue, cold) {
		t.Error("not applied again after plain ramps")
	}
	// Someone else's ramps: pause until the next point of either clip
	// (software brightness at 00:00), and leave them.
	f.someoneSetsGamma()
	theirs := append([]uint16{}, f.blue...)
	e.Step(at(20, 0, 10))
	e.Step(at(23, 0, 0))
	if !near(f.blue, theirs) {
		t.Fatal("KC9K overrode someone else's ramps")
	}
	pauses := CircadianPauses(d, at(23, 0, 0))
	if got := time.Unix(pauses[PauseGamma], 0); !got.Equal(at(0, 0, 0).AddDate(0, 0, 1)) {
		t.Errorf("paused until %v, want midnight", got)
	}
	if LanePausedUntil(pauses, circadian.LaneKelvin) == 0 || LanePausedUntil(pauses, circadian.LaneSoftware) == 0 ||
		LanePausedUntil(pauses, circadian.LaneBacklight) != 0 {
		t.Errorf("pauses %v", pauses)
	}
	if s, _ := ReadCircadianStatus(d); s.Gamma != "other" {
		t.Errorf("status gamma %q", s.Gamma)
	}
	// Resume takes the screens over.
	if err := ResumeCircadian(d, "", at(23, 0, 5)); err != nil {
		t.Fatal(err)
	}
	e.Step(at(23, 0, 5))
	if near(f.blue, theirs) || len(CircadianPauses(d, at(23, 0, 5))) != 0 {
		t.Error("resume didn't apply the clip")
	}
}

// Right after the daemon starts, ramps that aren't plain are most likely
// KC9K's own from before: they are taken over, not taken for a manual change.
func TestCircadianGammaAfterStart(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	circadianWith(t, d, testGamma, nil)
	f.someoneSetsGamma()
	NewCircadian(d).Step(at(20, 0, 0))
	_, _, cold := circadian.Ramp(256, 3000, 1)
	if !near(f.blue, cold) || len(CircadianPauses(d, at(20, 0, 0))) != 0 {
		t.Error("not taken over")
	}
}

func TestCircadianGammaRestoredOnceWhenOff(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	circadianWith(t, d, testGamma, nil)
	e := NewCircadian(d)
	e.Step(at(20, 0, 0))
	c := CircadianConfig(d)
	c.On = false
	if err := SetCircadianConfig(d, c); err != nil {
		t.Fatal(err)
	}
	e.Step(at(20, 0, 5))
	if !near(f.blue, plain256()) {
		t.Fatal("plain ramps weren't restored")
	}
	// Off: someone else's ramps are theirs.
	f.someoneSetsGamma()
	sets := f.gammaSets
	e.Step(at(20, 0, 10))
	NewCircadian(d).Step(at(20, 0, 15))
	if f.gammaSets != sets {
		t.Error("restored more than once")
	}
}

func TestCircadianNoXSession(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	circadianWith(t, d, testGamma, nil)
	f.noX = true
	e := NewCircadian(d)
	e.Step(at(20, 0, 0))
	if s, _ := ReadCircadianStatus(d); s.Gamma != "unknown" {
		t.Errorf("status gamma %q", s.Gamma)
	}
	f.noX = false
	e.Step(at(20, 0, 5))
	if f.gammaSets != 1 {
		t.Errorf("applied %d times once the session was there", f.gammaSets)
	}
}

func TestCircadianWallpaperAndDarkMode(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	circadianWith(t, d, nil, func(c *circadian.Config) {
		c.Wallpapers = []circadian.Wallpaper{
			{Image: "day.jpg", From: 8 * 60, To: 18 * 60},
			{Image: "night.jpg", From: 22 * 60, To: 6 * 60},
		}
		c.Dark = circadian.Dark{Mode: circadian.DarkScheduled, LightFrom: 7 * 60, LightTo: 20 * 60}
	})
	e := NewCircadian(d)
	e.Step(at(9, 0, 0))
	e.Step(at(9, 0, 5))
	e.Step(at(17, 0, 0))
	// Given once: what the user picks by hand afterwards stays.
	if len(f.wallpapers) != 1 || !strings.HasSuffix(f.wallpapers[0], "/wallpapers/day.jpg") {
		t.Fatalf("wallpapers %v", f.wallpapers)
	}
	if len(f.darks) != 1 || f.darks[0] {
		t.Fatalf("dark mode %v", f.darks)
	}
	e.Step(at(19, 0, 0)) // no wallpaper's time
	e.Step(at(22, 30, 0))
	if len(f.wallpapers) != 2 || !strings.HasSuffix(f.wallpapers[1], "night.jpg") {
		t.Errorf("wallpapers %v", f.wallpapers)
	}
	if len(f.darks) != 2 || !f.darks[1] {
		t.Errorf("dark mode %v", f.darks)
	}
	// The next day the day wallpaper is given again.
	e.Step(at(9, 0, 0).AddDate(0, 0, 1))
	if len(f.wallpapers) != 3 {
		t.Errorf("wallpapers %v", f.wallpapers)
	}
}

// Nobody logged in: nothing is recorded as given, and it is tried again at
// most once a minute.
func TestCircadianDesktopRetry(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	circadianWith(t, d, nil, func(c *circadian.Config) { c.Dark.Mode = circadian.DarkDark })
	f.users = 0
	e := NewCircadian(d)
	e.Step(at(9, 0, 0))
	e.Step(at(9, 0, 5))
	e.Step(at(9, 0, 30))
	if len(f.darks) != 1 {
		t.Fatalf("tried %d times within a minute", len(f.darks))
	}
	f.users = 1
	e.Step(at(9, 1, 0))
	e.Step(at(9, 2, 0))
	if len(f.darks) != 2 {
		t.Errorf("tried %d times", len(f.darks))
	}
}

func TestCircadianSystemDarkModeIsLeftAlone(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	circadianWith(t, d, testBacklight, nil)
	NewCircadian(d).Step(at(9, 0, 0))
	if len(f.darks) != 0 || len(f.wallpapers) != 0 || f.gammaSets != 0 {
		t.Errorf("touched: dark %v, wallpapers %v, gamma %d", f.darks, f.wallpapers, f.gammaSets)
	}
}

func TestCircadianOffTouchesNothing(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	NewCircadian(d).Step(at(9, 0, 0))
	if f.backlight != 500 || f.external != 50 || f.gammaSets != 0 || len(f.darks)+len(f.wallpapers) != 0 {
		t.Errorf("touched %+v", f)
	}
}

// A preview drives the hardware itself; the engine stays away and takes
// what the preview wrote for its own afterwards.
func TestCircadianPreview(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	lanes := map[string][]circadian.Point{}
	for k, v := range testBacklight {
		lanes[k] = v
	}
	for k, v := range testGamma {
		lanes[k] = v
	}
	circadianWith(t, d, lanes, nil)
	e := NewCircadian(d)
	e.Step(at(9, 0, 0))
	if err := PreviewCircadian(d, circadian.LaneBacklight, 20, false, at(9, 0, 1)); err != nil {
		t.Fatal(err)
	}
	if f.backlight != 200 {
		t.Fatalf("preview wrote %d", f.backlight)
	}
	e.Step(at(9, 0, 5))
	if f.backlight != 200 || len(CircadianPauses(d, at(9, 0, 5))) != 0 {
		t.Fatalf("during the preview: %d, pauses %v", f.backlight, CircadianPauses(d, at(9, 0, 5)))
	}
	if err := PreviewCircadian(d, circadian.LaneKelvin, 2000, false, at(9, 0, 6)); err != nil {
		t.Fatal(err)
	}
	_, _, amber := circadian.Ramp(256, 2000, 1)
	if !near(f.blue, amber) {
		t.Fatal("the colour preview wasn't shown")
	}
	if err := PreviewCircadian(d, circadian.LaneKelvin, 0, true, at(9, 0, 7)); err != nil {
		t.Fatal(err)
	}
	e.Step(at(9, 0, 10))
	if near(f.blue, amber) || len(CircadianPauses(d, at(9, 0, 10))) != 0 {
		t.Errorf("after the preview: pauses %v", CircadianPauses(d, at(9, 0, 10)))
	}
	if f.backlight != 500 {
		t.Errorf("backlight %d after the preview, want the clip's 500", f.backlight)
	}
	// Out of range, off and paused clips aren't previewed.
	if PreviewCircadian(d, circadian.LaneKelvin, 99, false, at(9, 0, 11)) == nil {
		t.Error("99 K accepted")
	}
	f.backlight = 900
	e.Step(at(9, 0, 15))
	if PreviewCircadian(d, circadian.LaneBacklight, 30, false, at(9, 0, 16)) == nil || f.backlight != 900 {
		t.Error("a paused clip was previewed")
	}
}

// A database opened through KC9K_DB never reaches the hardware.
func TestCircadianNothingAppliedForATestDatabase(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	circadianDry = func() bool { return os.Getenv("KC9K_DB") != "" }
	t.Setenv("KC9K_DB", "/nonexistent/t.db")
	lanes := map[string][]circadian.Point{}
	for k, v := range testBacklight {
		lanes[k] = v
	}
	for k, v := range testGamma {
		lanes[k] = v
	}
	circadianWith(t, d, lanes, func(c *circadian.Config) { c.Dark.Mode = circadian.DarkDark })
	NewCircadian(d).Step(at(21, 0, 0))
	RunCircadian(d) // returns at once
	if err := PreviewCircadian(d, circadian.LaneBacklight, 20, false, at(21, 0, 1)); err != nil {
		t.Fatal(err)
	}
	if f.backlight != 500 || f.gammaSets != 0 || len(f.darks) != 0 {
		t.Errorf("touched %+v", f)
	}
}

func TestCircadianDarkRule(t *testing.T) {
	d := testDB(t)
	if CircadianDarkRule(d, at(23, 0, 0)) != nil {
		t.Error("a rule although the circadian rhythm is off")
	}
	circadianWith(t, d, nil, func(c *circadian.Config) {
		c.DarkPages, c.DarkExempt = true, []string{"example.com"}
		c.Dark = circadian.Dark{Mode: circadian.DarkScheduled, LightFrom: 7 * 60, LightTo: 20 * 60}
	})
	if r := CircadianDarkRule(d, at(23, 0, 0)); r == nil || !r.On || r.FollowSystem || len(r.Exempt) != 1 {
		t.Errorf("at night: %+v", r)
	}
	if r := CircadianDarkRule(d, at(12, 0, 0)); r == nil || r.On || r.FollowSystem {
		t.Errorf("at noon: %+v", r)
	}
	c := CircadianConfig(d)
	c.Dark.Mode = circadian.DarkSystem
	_ = SetCircadianConfig(d, c)
	if r := CircadianDarkRule(d, at(12, 0, 0)); r == nil || r.On || !r.FollowSystem {
		t.Errorf("system: %+v", r)
	}
}
