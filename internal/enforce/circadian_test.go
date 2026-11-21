package enforce

import (
	"math"
	"os"
	"slices"
	"strconv"
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
	gammaReads int
	noX        bool
	wallpapers []string
	darks      []bool
	users      int      // signed in, as user IDs 1000, 1001…
	signedOut  []string // not signed in, with desktop settings
	reached    [][]string
	nightLight bool
	hooks      []bool // the sleep hook written (true) or removed (false)
	slept      int64  // seconds asleep since boot
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

func (f *fakeScreen) signedIn() []string {
	var out []string
	for i := 0; i < f.users; i++ {
		out = append(out, strconv.Itoa(1000+i))
	}
	return out
}

// reach is who a desktop change reaches, noted in f.reached.
func (f *fakeScreen) reach(r blocker.Reach) []string {
	var out []string
	for _, uid := range f.signedIn() {
		if !slices.Contains(r.Skip, uid) {
			out = append(out, uid)
		}
	}
	if r.SignedOut {
		for _, uid := range f.signedOut {
			if !slices.Contains(r.Skip, uid) {
				out = append(out, uid)
			}
		}
	}
	f.reached = append(f.reached, out)
	return out
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
	known := circadianExternalKnown
	readGamma, setGamma := circadianReadGamma, circadianSetGamma
	setWallpaper, setDark, nightLight, signedIn := circadianSetWallpaper, circadianSetDark, circadianNightLight, circadianSignedIn
	hook, slept := circadianSleepHook, sleptSeconds
	t.Cleanup(func() {
		circadianSleepHook, sleptSeconds = hook, slept
		circadianSignedIn = signedIn
		circadianDry, circadianBacklights, circadianExternal, circadianDDC = dry, backlights, external, ddc
		circadianExternalKnown = known
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
		return []circadianLevel{{Name: "monitor", ID: "1", Max: 100,
			Get: func() (int, error) { return f.external, nil },
			Set: func(v int) error { f.external = v; return nil }}}
	}
	circadianExternalKnown = func(*db.DB) []circadianLevel { return circadianExternal() }
	circadianDDC = func() bool { return true }
	circadianReadGamma = func(want blocker.RampFunc) blocker.GammaReading {
		f.gammaReads++
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
	circadianSignedIn = f.signedIn
	circadianSetWallpaper = func(path string, r blocker.Reach) ([]string, error) {
		f.wallpapers = append(f.wallpapers, path)
		return f.reach(r), nil
	}
	circadianSetDark = func(dark bool, r blocker.Reach) ([]string, error) {
		f.darks = append(f.darks, dark)
		return f.reach(r), nil
	}
	circadianNightLight = func() bool { return f.nightLight }
	circadianSleepHook = func(on bool) error { f.hooks = append(f.hooks, on); return nil }
	sleptSeconds = func() int64 { return f.slept }
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

// §4.10: off, it touches nothing, and the desktop's night light keeps
// working (owner, 2026-11-15: it turned itself off, also with circadian
// off). After a restart KC9K knows the ramp it last set: with circadian
// off it gives plain ramps back only if that one is still on the screens,
// never over someone else's; on, someone else's pauses the clips instead
// of being overridden. Off with nothing to give back, nothing is read.
func TestCircadianGammaAfterRestart(t *testing.T) {
	off := func(d *db.DB) {
		c := CircadianConfig(d)
		c.On = false
		if err := SetCircadianConfig(d, c); err != nil {
			t.Fatal(err)
		}
	}
	// Off after a restart, the night light's ramp on the screens: left.
	d, f := testDB(t), fakeCircadian(t)
	circadianWith(t, d, testGamma, nil)
	NewCircadian(d).Step(at(20, 0, 0))
	f.someoneSetsGamma()
	f.nightLight = true
	theirs := append([]uint16{}, f.blue...)
	off(d)
	sets := f.gammaSets
	NewCircadian(d).Step(at(20, 0, 5))
	if f.gammaSets != sets || !near(f.blue, theirs) {
		t.Fatal("off after a restart, KC9K reset the night light's ramp")
	}
	// Nothing left to give back: nothing is read any more.
	reads := f.gammaReads
	e := NewCircadian(d)
	e.Step(at(20, 0, 10))
	e.Step(at(20, 0, 15))
	if f.gammaReads != reads || f.gammaSets != sets {
		t.Errorf("off: %d reads, %d sets", f.gammaReads-reads, f.gammaSets-sets)
	}

	// Off after a restart, KC9K's own ramp still on the screens: given back.
	d2, f2 := testDB(t), fakeCircadian(t)
	circadianWith(t, d2, testGamma, nil)
	NewCircadian(d2).Step(at(20, 0, 0))
	off(d2)
	NewCircadian(d2).Step(at(20, 0, 5))
	if !near(f2.blue, plain256()) {
		t.Error("off after a restart, KC9K's own ramp stayed")
	}

	// On after a restart, someone else's ramp: the clips pause.
	d3, f3 := testDB(t), fakeCircadian(t)
	circadianWith(t, d3, testGamma, nil)
	NewCircadian(d3).Step(at(20, 0, 0))
	f3.someoneSetsGamma()
	theirs = append([]uint16{}, f3.blue...)
	NewCircadian(d3).Step(at(20, 0, 5))
	if !near(f3.blue, theirs) || LanePausedUntil(CircadianPauses(d3, at(20, 0, 5)), circadian.LaneKelvin) == 0 {
		t.Error("on after a restart, someone else's ramp was overridden")
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

// Owner 2026-11-17: after a boot the desktop showed yesterday's wallpaper
// until a minute after sign-in. The wallpaper due is written into the
// settings of people who aren't signed in, so their desktop starts with
// it; someone signed in later who wasn't given it gets it at once, not a
// minute later; a hand-picked one stays until the next change.
func TestCircadianWallpaperBeforeSignIn(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	circadianWith(t, d, nil, func(c *circadian.Config) {
		c.Wallpapers = []circadian.Wallpaper{{Image: "day.jpg", From: 8 * 60, To: 18 * 60}}
		c.Dark.Mode = circadian.DarkLight
	})
	// At boot: nobody signed in; anna (1000) has desktop settings.
	f.users, f.signedOut = 0, []string{"1000"}
	e := NewCircadian(d)
	e.Step(at(9, 0, 0))
	if len(f.wallpapers) != 1 || !slices.Equal(f.reached[0], []string{"1000"}) {
		t.Fatalf("wallpaper before sign-in: %v to %v", f.wallpapers, f.reached)
	}
	// anna signs in: she has it already, nothing is given again.
	f.users, f.signedOut = 1, nil
	e.Step(at(9, 0, 20))
	if len(f.wallpapers) != 1 || len(f.darks) != 1 {
		t.Errorf("given again at sign-in: %v, %v", f.wallpapers, f.darks)
	}

	// A boot where her settings couldn't be written: given at sign-in,
	// at once although the try at boot reached nobody.
	d2, f2 := testDB(t), fakeCircadian(t)
	circadianWith(t, d2, nil, func(c *circadian.Config) {
		c.Wallpapers = []circadian.Wallpaper{{Image: "day.jpg", From: 8 * 60, To: 18 * 60}}
	})
	f2.users = 0
	e2 := NewCircadian(d2)
	e2.Step(at(9, 0, 0))
	f2.users = 1
	e2.Step(at(9, 0, 10))
	if len(f2.wallpapers) != 2 || !slices.Equal(f2.reached[1], []string{"1000"}) {
		t.Fatalf("not given at sign-in: %v to %v", f2.wallpapers, f2.reached)
	}
	// Ben signs in too: only he is given it, anna's (maybe hand-picked)
	// wallpaper stays.
	f2.users = 2
	e2.Step(at(9, 5, 0))
	if len(f2.wallpapers) != 3 || !slices.Equal(f2.reached[2], []string{"1001"}) {
		t.Errorf("second sign-in: %v to %v", f2.wallpapers, f2.reached)
	}
	e2.Step(at(9, 10, 0))
	if len(f2.wallpapers) != 3 {
		t.Errorf("given again: %v", f2.wallpapers)
	}
}

// State from before 2026-11-17 has no list of users: it counts as given
// to everyone signed in, so nobody's wallpaper is changed by the upgrade.
func TestCircadianWallpaperOldState(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	circadianWith(t, d, nil, func(c *circadian.Config) {
		c.Wallpapers = []circadian.Wallpaper{{Image: "day.jpg", From: 8 * 60, To: 18 * 60}}
	})
	key, _ := WallpaperKey(CircadianConfig(d), 9*60)
	if err := d.SetSetting(settingCircadianApplied, `{"wallpaper":"`+key+`","dark":""}`); err != nil {
		t.Fatal(err)
	}
	NewCircadian(d).Step(at(9, 0, 0))
	if len(f.wallpapers) != 0 {
		t.Errorf("an upgrade gave the wallpaper again: %v", f.wallpapers)
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
	if f.backlight != 200 || f.external != 20 {
		t.Fatalf("preview wrote %d and %d", f.backlight, f.external)
	}
	// While the backlight is previewed, another clip is previewed too:
	// neither note hides the other.
	if err := PreviewCircadian(d, circadian.LaneKelvin, 2000, false, at(9, 0, 2)); err != nil {
		t.Fatal(err)
	}
	_, _, amber := circadian.Ramp(256, 2000, 1)
	if !near(f.blue, amber) {
		t.Fatal("the colour preview wasn't shown")
	}
	e.Step(at(9, 0, 5))
	if f.backlight != 200 || !near(f.blue, amber) || len(CircadianPauses(d, at(9, 0, 5))) != 0 {
		t.Fatalf("during the previews: %d, pauses %v", f.backlight, CircadianPauses(d, at(9, 0, 5)))
	}
	if err := PreviewCircadian(d, circadian.LaneKelvin, 0, true, at(9, 0, 7)); err != nil {
		t.Fatal(err)
	}
	e.Step(at(9, 0, 10))
	if near(f.blue, amber) || f.backlight != 200 {
		t.Errorf("after the colour preview: backlight %d", f.backlight)
	}
	// The backlight preview runs out by itself; a slow cycle later both
	// the panel and the monitor follow the clip again, unpaused.
	e.Step(at(9, 0, 40))
	if f.backlight != 501 || f.external != 50 || len(CircadianPauses(d, at(9, 0, 40))) != 0 {
		t.Errorf("after the previews: %d and %d, pauses %v", f.backlight, f.external, CircadianPauses(d, at(9, 0, 40)))
	}
	// An old note left in the settings means nothing to a new daemon.
	NewCircadian(d).Step(at(9, 5, 0))
	if len(CircadianPauses(d, at(9, 5, 0))) != 0 {
		t.Errorf("a restarted daemon paused: %v", CircadianPauses(d, at(9, 5, 0)))
	}
	// Out of range, off and paused clips aren't previewed.
	if PreviewCircadian(d, circadian.LaneKelvin, 99, false, at(9, 5, 1)) == nil {
		t.Error("99 K accepted")
	}
	f.backlight = 900
	e.Step(at(9, 5, 5))
	if PreviewCircadian(d, circadian.LaneBacklight, 30, false, at(9, 5, 6)) == nil || f.backlight != 900 {
		t.Error("a paused clip was previewed")
	}
}

// §4.10 (never overrides the user, off touches nothing): a colour preview
// ends and is taken over, then a brightness preview runs and ends (the
// desktop, 2026-11-20: kelvin 3000, then software 50 and --end). The engine
// takes over the ramp the last preview wrote, not one mixed with the
// earlier preview's colour: on, the clip carries on unpaused; switched off
// right after, the plain ramps come back (the screen stayed at 50 %).
func TestCircadianPreviewAfterAnEndedPreview(t *testing.T) {
	for _, switchOff := range []bool{false, true} {
		d, f := testDB(t), fakeCircadian(t)
		circadianWith(t, d, testGamma, nil)
		e := NewCircadian(d)
		e.Step(at(9, 0, 0))
		if err := PreviewCircadian(d, circadian.LaneKelvin, 3000, false, at(9, 0, 1)); err != nil {
			t.Fatal(err)
		}
		e.Step(at(9, 0, 5))
		e.Step(at(9, 0, 13)) // the colour preview ran out: taken over
		if err := PreviewCircadian(d, circadian.LaneSoftware, 50, false, at(9, 0, 14)); err != nil {
			t.Fatal(err)
		}
		if err := PreviewCircadian(d, circadian.LaneSoftware, 0, true, at(9, 0, 16)); err != nil {
			t.Fatal(err)
		}
		wantRed, wantBlue := plain256(), plain256()
		if switchOff {
			c := CircadianConfig(d)
			c.On = false
			if err := SetCircadianConfig(d, c); err != nil {
				t.Fatal(err)
			}
		} else {
			kelvin, brightness, _ := gammaTarget(CircadianConfig(d), circadian.MinuteOfDay(at(9, 0, 18)))
			wantRed, _, wantBlue = circadian.Ramp(256, kelvin, brightness)
		}
		e.Step(at(9, 0, 18))
		if !near(f.red, wantRed) || !near(f.blue, wantBlue) {
			t.Errorf("off %v: the preview's ramp stayed on the screen", switchOff)
		}
		if p := CircadianPauses(d, at(9, 0, 18)); len(p) != 0 {
			t.Errorf("off %v: paused %v", switchOff, p)
		}
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

// §4.10 preview (owner, 2026-11-16: "Play the day"): the next 24 hours
// shown in a few seconds, the clips as previews (the engine stays away),
// the wallpaper and dark mode at their times; afterwards the previews end,
// the engine carries on from now and the desktop's look is put back.
func TestPlayCircadian(t *testing.T) {
	d, f := testDB(t), fakeCircadian(t)
	lanes := map[string][]circadian.Point{}
	for k, v := range testBacklight {
		lanes[k] = v
	}
	for k, v := range testGamma {
		lanes[k] = v
	}
	circadianWith(t, d, lanes, func(c *circadian.Config) {
		c.Wallpapers = []circadian.Wallpaper{{Image: "day.jpg", From: 8 * 60, To: 18 * 60},
			{Image: "night.jpg", From: 22 * 60, To: 6 * 60}}
		c.Dark = circadian.Dark{Mode: circadian.DarkScheduled, LightFrom: 7 * 60, LightTo: 20 * 60}
	})
	oldSleep, oldRead, oldRestore := circadianSleep, circadianReadLook, circadianRestoreLook
	t.Cleanup(func() { circadianSleep, circadianReadLook, circadianRestoreLook = oldSleep, oldRead, oldRestore })
	slept := time.Duration(0)
	circadianSleep = func(d time.Duration) { slept += d }
	restored := false
	circadianReadLook = func() blocker.DesktopLook { return blocker.DesktopLook{"me": {"k": "'before'"}} }
	circadianRestoreLook = func(l blocker.DesktopLook) error { restored = l["me"]["k"] == "'before'"; return nil }

	var minutes []float64
	var backlights []int
	start := at(9, 0, 0)
	if err := PlayCircadian(d, 15*time.Second, start, func(m float64) {
		minutes = append(minutes, m)
		backlights = append(backlights, f.backlight)
	}); err != nil {
		t.Fatal(err)
	}
	if len(minutes) != 151 || minutes[0] != 9*60 || slept < 14*time.Second || slept > 16*time.Second {
		t.Errorf("frames %d from %v, slept %v", len(minutes), minutes[0], slept)
	}
	// The whole day went by: noon's 80 % and the evening's 40 %.
	high, low := 0, 1000
	for _, b := range backlights {
		high, low = max(high, b), min(low, b)
	}
	if high < 790 || low > 410 {
		t.Errorf("backlight went from %d to %d", low, high)
	}
	if len(f.wallpapers) < 2 || !strings.HasSuffix(f.wallpapers[0], "day.jpg") || !strings.HasSuffix(f.wallpapers[1], "night.jpg") {
		t.Errorf("wallpapers %v", f.wallpapers)
	}
	if len(f.darks) < 2 {
		t.Errorf("dark mode %v", f.darks)
	}
	if !restored {
		t.Error("the desktop's look wasn't put back")
	}
	running, _ := previews(d, time.Now())
	if len(running) != 0 {
		t.Errorf("previews still running: %v", running)
	}
	// Off, or nothing to play: said.
	c := CircadianConfig(d)
	c.On = false
	_ = SetCircadianConfig(d, c)
	if err := PlayCircadian(d, time.Second, start, nil); err == nil || !strings.Contains(err.Error(), "off") {
		t.Errorf("off: %v", err)
	}
}
