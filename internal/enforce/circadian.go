// circadian.go — the circadian rhythm's engine (§4.10): every few seconds it
// brings the backlight, the gamma ramps (software brightness and colour
// temperature), the wallpaper and dark mode to what the clips and times in
// the settings say for now.
//
// Nothing here is locked, and KC9K never puts a level back: when a level
// isn't what KC9K last wrote, the user changed it, and that clip pauses
// until its next point (or `kc9k circadian resume`).
//
// It runs in its own goroutine beside the daemon cycle (RunCircadian), never
// inside Tick. Hardware sits behind replaceable vars for tests, and nothing
// is applied for a database opened through KC9K_DB.
package enforce

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/circadian"
	"github.com/smattermannen/kc9k/internal/db"
)

// The settings the circadian rhythm keeps, all JSON.
const (
	settingCircadian        = "circadian"         // circadian.Config
	settingCircadianPaused  = "circadian.paused"  // pause key -> unix seconds it ends
	settingCircadianPreview = "circadian.preview" // + "." + clip: CircadianPreview
	settingCircadianApplied = "circadian.applied" // circadianApplied
	settingCircadianResume  = "circadian.resume"  // set by `circadian resume`, cleared by the engine
	settingCircadianStatus  = "circadian.status"  // CircadianStatus, written only when it changes
)

// What a manual change pauses. Software brightness and colour temperature
// are one gamma ramp, so they pause together; the built-in backlight and
// external screens are told apart, since each is changed on its own.
const (
	PauseBacklight = "backlight"
	PauseExternal  = "external"
	PauseGamma     = "gamma"
)

// CircadianInterval is how often the engine looks.
const CircadianInterval = 5 * time.Second

// circadianSlowInterval is how often it does the slow things: external
// screens (DDC/CI takes a moment) and asking for the desktop's night light.
const circadianSlowInterval = 30 * time.Second

// circadianRetry is how long it waits before trying a wallpaper or dark
// mode again that reached no desktop user.
const circadianRetry = time.Minute

// PreviewHold is how long a preview keeps the engine away from its clip.
const PreviewHold = 10 * time.Second

// circadianLevel is one brightness the engine drives: a backlight device or
// an external screen.
type circadianLevel struct {
	Name string
	ID   string // an external screen's ddcutil display number
	Max  int
	Get  func() (int, error)
	Set  func(int) error
}

// Hardware, replaceable in tests.
var (
	circadianDry = func() bool { return os.Getenv("KC9K_DB") != "" }

	circadianBacklights = func() []circadianLevel {
		var out []circadianLevel
		for _, b := range blocker.Backlights() {
			out = append(out, circadianLevel{Name: b.Name, Max: b.Max, Get: b.Raw, Set: b.SetRaw})
		}
		return out
	}
	circadianExternal = func() []circadianLevel {
		var out []circadianLevel
		for _, s := range blocker.ExternalScreens() {
			out = append(out, externalLevel(s))
		}
		return out
	}
	// circadianExternalKnown is the external screens the engine last found
	// (its status), without looking for them again: looking takes seconds,
	// and a preview can't wait.
	circadianExternalKnown = func(d *db.DB) []circadianLevel {
		var out []circadianLevel
		status, _ := ReadCircadianStatus(d)
		for i, display := range status.Displays {
			s := blocker.ExternalScreen{Display: display, Name: "Display " + display}
			if i < len(status.External) {
				s.Name = status.External[i]
			}
			out = append(out, externalLevel(s))
		}
		return out
	}
	circadianDDC          = blocker.DDCUtilInstalled
	circadianReadGamma    = blocker.ReadGamma
	circadianSetGamma     = blocker.SetGamma
	circadianSetWallpaper = blocker.SetWallpaper
	circadianSetDark      = blocker.SetDarkMode
	circadianSignedIn     = blocker.SignedInUserIDs
	circadianNightLight   = blocker.NightLightOn
	circadianReadLook     = blocker.ReadDesktopLook
	circadianRestoreLook  = blocker.RestoreDesktopLook
	circadianSleep        = time.Sleep
	circadianSleepHook    = blocker.SyncSleepHook
)

// wallpaperPath is where a stored wallpaper is.
func wallpaperPath(image string) string { return filepath.Join(WallpapersDir(), filepath.Base(image)) }

func externalLevel(s blocker.ExternalScreen) circadianLevel {
	return circadianLevel{Name: s.Name, ID: s.Display, Max: 100,
		Get: func() (int, error) {
			percent, err := s.Brightness()
			return int(math.Round(percent)), err
		},
		Set: func(level int) error { return s.SetBrightness(float64(level)) }}
}

// CircadianConfig reads the circadian rhythm's settings.
func CircadianConfig(d *db.DB) circadian.Config {
	value, err := d.GetSetting(settingCircadian)
	if err != nil {
		log.Printf("circadian: %v", err)
	}
	if value == "" {
		return circadian.Default()
	}
	var c circadian.Config
	if err := json.Unmarshal([]byte(value), &c); err != nil {
		log.Printf("circadian: unreadable settings: %v", err)
		return circadian.Default()
	}
	c.Normalize()
	return c
}

// SetCircadianConfig stores the circadian rhythm's settings.
func SetCircadianConfig(d *db.DB, c circadian.Config) error {
	c.Normalize()
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return d.SetSetting(settingCircadian, string(data))
}

// CircadianConfigured reports whether the user ever changed the settings.
func CircadianConfigured(d *db.DB) bool {
	value, _ := d.GetSetting(settingCircadian)
	return value != ""
}

// WallpapersDir is where the circadian rhythm's wallpapers are kept, beside
// the database, readable by every desktop user.
func WallpapersDir() string { return filepath.Join(filepath.Dir(db.ResolveDBPath()), "wallpapers") }

func readJSONSetting(d *db.DB, key string, into any) bool {
	value, err := d.GetSetting(key)
	if err != nil || value == "" {
		return false
	}
	return json.Unmarshal([]byte(value), into) == nil
}

func writeJSONSetting(d *db.DB, key string, value any) {
	data, err := json.Marshal(value)
	if err == nil {
		err = d.SetSetting(key, string(data))
	}
	if err != nil {
		log.Printf("circadian: %v", err)
	}
}

// CircadianPauses is the pauses in force at now: what is paused (PauseBacklight,
// PauseExternal, PauseGamma) and until when, in unix seconds.
func CircadianPauses(d *db.DB, now time.Time) map[string]int64 {
	pauses := map[string]int64{}
	readJSONSetting(d, settingCircadianPaused, &pauses)
	for key, until := range pauses {
		if until <= now.Unix() {
			delete(pauses, key)
		}
	}
	return pauses
}

// PauseKeys is what a pause of the clip lane is stored under.
func PauseKeys(lane string) []string {
	if lane == circadian.LaneBacklight {
		return []string{PauseBacklight, PauseExternal}
	}
	return []string{PauseGamma}
}

// LanePausedUntil is when the clip lane's pause ends (unix seconds), or 0.
func LanePausedUntil(pauses map[string]int64, lane string) int64 {
	var until int64
	for _, key := range PauseKeys(lane) {
		until = max(until, pauses[key])
	}
	return until
}

// ResumeCircadian ends the pause of a clip (every pause when lane is ""),
// and tells the engine to take the levels over as they are.
func ResumeCircadian(d *db.DB, lane string, now time.Time) error {
	pauses := CircadianPauses(d, now)
	if lane == "" {
		pauses = map[string]int64{}
	} else {
		for _, key := range PauseKeys(lane) {
			delete(pauses, key)
		}
	}
	writeJSONSetting(d, settingCircadianPaused, pauses)
	return d.SetSetting(settingCircadianResume, fmt.Sprint(now.UnixNano()))
}

// CircadianPreview is a value being tried out from the interface: the
// command that set it drives the hardware itself, and the engine stays away
// from that clip until Until (unix seconds).
type CircadianPreview struct {
	Lane  string  `json:"lane"`
	Value float64 `json:"value"`
	Until int64   `json:"until"`
	// For software brightness and colour temperature: the ramp the command
	// wrote and when (unix ns), so the engine takes over exactly the ramp
	// on the screens, not one mixed with an earlier preview's that ended.
	Kelvin     float64 `json:"kelvin,omitempty"`
	Brightness float64 `json:"brightness,omitempty"`
	At         int64   `json:"at,omitempty"`
}

// circadianApplied is what the engine last gave the desktop, so it gives
// each wallpaper and dark mode once and leaves later changes by hand alone.
type circadianApplied struct {
	Wallpaper string `json:"wallpaper"`
	Dark      string `json:"dark"`
	// The user IDs each was given to; nil (from older versions): everyone
	// signed in then.
	WallpaperUsers []string `json:"wallpaper_users"`
	DarkUsers      []string `json:"dark_users"`
	Gamma          bool     `json:"gamma"` // KC9K's ramps are on the screens
	// The ramp KC9K last set, so that after a restart it tells its own
	// from someone else's (the desktop's night light): 0 from older
	// versions, which didn't note it.
	Kelvin     float64 `json:"kelvin,omitempty"`
	Brightness float64 `json:"brightness,omitempty"`
}

// CircadianStatus is what the engine found, for `kc9k circadian show`.
type CircadianStatus struct {
	Backlights []string `json:"backlights"`
	External   []string `json:"external"`
	Displays   []string `json:"displays"` // ddcutil's numbers of External
	DDCUtil    bool     `json:"ddcutil"`
	Gamma      string   `json:"gamma"`
	NightLight bool     `json:"night_light"`
}

// ReadCircadianStatus returns what the engine last reported; ok is false
// before it reported anything.
func ReadCircadianStatus(d *db.DB) (CircadianStatus, bool) {
	var s CircadianStatus
	ok := readJSONSetting(d, settingCircadianStatus, &s)
	return s, ok
}

// gammaTarget is the colour temperature and brightness (0–1) the clips ask
// for at minute; on is false when neither clip is on.
func gammaTarget(c circadian.Config, minute float64) (kelvin, brightness float64, on bool) {
	kelvin, brightness = 6500, 1
	if !c.On {
		return
	}
	if l := c.Lanes[circadian.LaneKelvin]; l != nil && l.On {
		if v, ok := l.ValueAt(minute); ok {
			kelvin = v
		}
		on = true
	}
	if l := c.Lanes[circadian.LaneSoftware]; l != nil && l.On {
		if v, ok := l.ValueAt(minute); ok {
			brightness = v / 100
		}
		on = true
	}
	return
}

// withPreviews puts previewed values in place of their clips'.
func withPreviews(previews map[string]CircadianPreview, kelvin, brightness float64) (float64, float64) {
	if p, ok := previews[circadian.LaneKelvin]; ok {
		kelvin = p.Value
	}
	if p, ok := previews[circadian.LaneSoftware]; ok {
		brightness = p.Value / 100
	}
	return kelvin, brightness
}

func rampFor(kelvin, brightness float64) blocker.RampFunc {
	return func(size int) ([]uint16, []uint16, []uint16) { return circadian.Ramp(size, kelvin, brightness) }
}

// nextPoint is the earliest next point of the given clips after now, or a
// day from now when they have none.
func nextPoint(c circadian.Config, now time.Time, lanes ...string) time.Time {
	best := now.Add(24 * time.Hour)
	for _, name := range lanes {
		l := c.Lanes[name]
		if l == nil || !l.On {
			continue
		}
		if at, ok := l.NextPointAfter(now); ok && at.Before(best) {
			best = at
		}
	}
	return best
}

// Circadian is the engine's memory between steps: what it last wrote, so a
// level that is no longer that was changed by someone else.
type Circadian struct {
	d  *db.DB
	mu sync.Mutex

	last     map[string]int    // pause key + "/" + device -> the level last written
	pausedBy map[string]string // pause key -> the device whose change paused it

	gammaSet        bool // gammaKelvin and gammaBrightness are what the screens were given
	gammaKelvin     float64
	gammaBrightness float64
	gammaStarted    bool // the first look at the ramps since the engine started is done

	previewSeen map[string]CircadianPreview // clip -> the preview already taken over

	slowAt       time.Time // the last look at external screens and the night light
	desktopAt    time.Time // the last wallpaper or dark mode that reached nobody
	desktopUsers []string  // who was signed in then
	desktopKeys  string    // what should have shown then

	// Modes' wallpapers (R-MODE-5, wallpapers.go).
	modeOnSince  map[int]time.Time // when each mode with wallpapers was first seen enabled
	placeKnownAt time.Time         // the last look that knew the place
	unsure       bool              // a mode switched by places might show a wallpaper, place unknown
	placeMatters bool              // a mode switched by places has wallpapers
	hookSynced   bool              // the sleep hook is as hookWanted says
	hookWanted   bool
	slept        int64     // time spent asleep since boot, at the last step
	freshAt      time.Time // the engine's start or the last wake: the place is looked for at once

	external   []string
	displays   []string
	nightLight bool
	status     string
}

// NewCircadian makes an engine for d.
func NewCircadian(d *db.DB) *Circadian {
	return &Circadian{d: d, last: map[string]int{}, pausedBy: map[string]string{}, previewSeen: map[string]CircadianPreview{},
		freshAt: time.Now(), slept: sleptSeconds()}
}

// placeLookInterval is how often the engine steps while it looks for the
// place after starting or waking (R-MODE-5: the right wallpaper as early as
// possible), for at most placeLookFor.
const (
	placeLookInterval = time.Second
	placeLookFor      = 3 * time.Minute
)

// lookingForPlace reports whether the engine should step again soon: the
// place decides the wallpaper and isn't known yet, shortly after it started
// or the computer woke.
func (e *Circadian) lookingForPlace(now time.Time) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.unsure && now.Sub(e.freshAt) < placeLookFor
}

// RunCircadian steps the engine every CircadianInterval, forever. It returns
// at once for a database opened through KC9K_DB.
func RunCircadian(d *db.DB) {
	if circadianDry() {
		return
	}
	e := NewCircadian(d)
	e.Step(time.Now())
	last := time.Now()
	ticker := time.NewTicker(placeLookInterval)
	defer ticker.Stop()
	for now := range ticker.C {
		if now.Sub(last) >= CircadianInterval || e.lookingForPlace(now) {
			e.Step(now)
			last = now
		}
	}
}

func (e *Circadian) forget(pauseKey string) {
	for id := range e.last {
		if strings.HasPrefix(id, pauseKey+"/") {
			delete(e.last, id)
		}
	}
	delete(e.pausedBy, pauseKey)
	if pauseKey == PauseGamma {
		e.gammaSet = false
	}
}

// previews reads the previews noted: those running at now, and those that
// ended in the last half minute (what they wrote is still on the screen).
func previews(d *db.DB, now time.Time) (running, ended map[string]CircadianPreview) {
	running, ended = map[string]CircadianPreview{}, map[string]CircadianPreview{}
	for _, info := range circadian.Lanes {
		var p CircadianPreview
		if !readJSONSetting(d, settingCircadianPreview+"."+info.Name, &p) || p.Lane != info.Name {
			continue
		}
		switch {
		case p.Until > now.Unix():
			running[info.Name] = p
		case now.Unix()-p.Until < 30:
			ended[info.Name] = p
		}
	}
	return
}

// Step brings everything to what the settings say for now.
func (e *Circadian) Step(now time.Time) {
	if circadianDry() {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	cfg := CircadianConfig(e.d)
	minute := circadian.MinuteOfDay(now)

	// A wake (time spent asleep grew): the place is looked for afresh, as at
	// boot (R-MODE-5).
	woke := false
	if slept := sleptSeconds(); slept-e.slept > 5 {
		woke, e.freshAt, e.placeKnownAt = true, now, time.Time{}
		ForgetSeenNetworks()
		e.slept = slept
	}

	// Pauses that ran out: that clip starts afresh from whatever the level
	// is now.
	stored := map[string]int64{}
	readJSONSetting(e.d, settingCircadianPaused, &stored)
	pauses := CircadianPauses(e.d, now)
	pausesChanged := len(pauses) != len(stored)
	for key := range stored {
		if _, still := pauses[key]; !still {
			e.forget(key)
		}
	}
	if marker, _ := e.d.GetSetting(settingCircadianResume); marker != "" {
		for _, key := range []string{PauseBacklight, PauseExternal, PauseGamma} {
			if _, paused := pauses[key]; !paused {
				e.forget(key)
			}
		}
		_ = e.d.SetSetting(settingCircadianResume, "")
	}

	// Previews: the command drives the hardware; what it wrote is ours.
	// The notes are never cleared here (a new preview could be lost that
	// way); one that ended is taken over once.
	running, ended := previews(e.d, now)
	written := lastGammaPreview(running, ended)
	for lane, p := range ended {
		if e.previewSeen[lane] != p {
			e.adoptPreview(cfg, ended, written, lane, minute)
		}
		e.previewSeen[lane] = p
	}
	for lane, p := range running {
		e.adoptPreview(cfg, running, written, lane, minute)
		e.previewSeen[lane] = p
	}
	gammaPreviewed := false
	for lane := range running {
		gammaPreviewed = gammaPreviewed || lane != circadian.LaneBacklight
	}

	pause := func(key string, lanes ...string) bool {
		// A preview that began while this step was reading the hardware
		// explains the change.
		if now, _ := previews(e.d, time.Now()); len(now) > 0 {
			return false
		}
		pauses[key] = nextPoint(cfg, now, lanes...).Unix()
		pausesChanged = true
		return true
	}
	resume := func(key string) {
		delete(pauses, key)
		pausesChanged = true
	}

	slow := e.slowAt.IsZero() || now.Sub(e.slowAt) >= circadianSlowInterval
	lane := cfg.Lanes[circadian.LaneBacklight]
	backlightOn := cfg.On && lane != nil && lane.On
	_, previewed := running[circadian.LaneBacklight]

	backlights := circadianBacklights()
	e.levels(PauseBacklight, backlights, backlightOn, previewed, lane, minute, pauses, pause, resume)
	if slow {
		e.slowAt = now
		e.external, e.displays = nil, nil
		if backlightOn {
			screens := circadianExternal()
			for _, s := range screens {
				e.external = append(e.external, s.Name)
				e.displays = append(e.displays, s.ID)
			}
			e.levels(PauseExternal, screens, true, previewed, lane, minute, pauses, pause, resume)
		} else {
			e.forget(PauseExternal)
		}
		e.nightLight = cfg.On && circadianNightLight()
	}

	applied := circadianApplied{}
	readJSONSetting(e.d, settingCircadianApplied, &applied)
	before := applied
	if e.beforeSleepGiven(now, woke) {
		// The sleep hook gave the default: what should show is given afresh.
		applied.Wallpaper, applied.WallpaperUsers = "", nil
	}
	reading := e.gamma(cfg, gammaPreviewed, minute, now, pauses, pause, resume, &applied)
	e.desktop(cfg, minute, now, &applied)
	e.syncSleepHook()
	if !sameApplied(applied, before) {
		writeJSONSetting(e.d, settingCircadianApplied, applied)
	}
	if pausesChanged {
		writeJSONSetting(e.d, settingCircadianPaused, pauses)
	}

	status := CircadianStatus{Backlights: []string{}, External: append([]string{}, e.external...),
		Displays: append([]string{}, e.displays...),
		DDCUtil:  circadianDDC(), Gamma: reading.String(), NightLight: e.nightLight}
	for _, b := range backlights {
		status.Backlights = append(status.Backlights, b.Name)
	}
	if data, err := json.Marshal(status); err == nil && string(data) != e.status {
		if e.d.SetSetting(settingCircadianStatus, string(data)) == nil {
			e.status = string(data)
		}
	}
}

// lastGammaPreview is the colour or software brightness preview written
// last (its ramp is the one on the screens), or none when no note says.
func lastGammaPreview(groups ...map[string]CircadianPreview) CircadianPreview {
	var last CircadianPreview
	for _, group := range groups {
		for lane, p := range group {
			if lane != circadian.LaneBacklight && p.At > last.At {
				last = p
			}
		}
	}
	return last
}

// adoptPreview notes what the preview of lane wrote as the engine's own, so
// it isn't taken for a change by hand once the preview ends. written is the
// gamma preview written last (lastGammaPreview).
func (e *Circadian) adoptPreview(cfg circadian.Config, all map[string]CircadianPreview, written CircadianPreview,
	lane string, minute float64) {
	if lane == circadian.LaneBacklight {
		value := all[lane].Value
		for _, b := range circadianBacklights() {
			e.last[PauseBacklight+"/"+b.Name] = blocker.PercentToRaw(value, b.Max)
		}
		for id := range e.last {
			if strings.HasPrefix(id, PauseExternal+"/") {
				e.last[id] = blocker.PercentToRaw(value, 100)
			}
		}
		return
	}
	if written.At > 0 {
		e.gammaKelvin, e.gammaBrightness = written.Kelvin, written.Brightness
	} else { // a note from an older version
		kelvin, brightness, _ := gammaTarget(cfg, minute)
		e.gammaKelvin, e.gammaBrightness = withPreviews(all, kelvin, brightness)
	}
	e.gammaSet = true
}

// levels drives the backlight devices (or external screens) under one pause.
// A clip that is off is forgotten, so switching it on again starts afresh;
// a previewed one is left to the command that previews it.
func (e *Circadian) levels(key string, devices []circadianLevel, on, previewed bool, lane *circadian.Lane,
	minute float64, pauses map[string]int64, pause func(string, ...string) bool, resume func(string)) {
	if !on || lane == nil {
		e.forget(key)
		return
	}
	target, ok := lane.ValueAt(minute)
	if previewed || !ok {
		return
	}
	for _, dev := range devices {
		id := key + "/" + dev.Name
		raw, err := dev.Get()
		if err != nil {
			continue
		}
		last, known := e.last[id]
		tolerance := max(1, dev.Max/100)
		moved := known && abs(raw-last) > tolerance
		if _, paused := pauses[key]; paused {
			// Back at what KC9K last wrote (the desktop dimmed an idle
			// screen and lit it again): carry on.
			if e.pausedBy[key] == id && known && !moved {
				resume(key)
				delete(e.pausedBy, key)
			} else {
				continue
			}
		} else if moved {
			if pause(key, circadian.LaneBacklight) {
				e.pausedBy[key] = id
				log.Printf("circadian: %s was changed by hand (%d, KC9K wrote %d): the backlight clip is paused until its next point", dev.Name, raw, last)
				continue
			}
		}
		want := blocker.PercentToRaw(target, dev.Max)
		if want != raw {
			if err := dev.Set(want); err != nil {
				log.Printf("circadian: %s: %v", dev.Name, err)
				continue
			}
		}
		e.last[id] = want
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// gamma drives the gamma ramps: software brightness and colour temperature
// as one ramp.
func (e *Circadian) gamma(cfg circadian.Config, previewed bool, minute float64, now time.Time,
	pauses map[string]int64, pause func(string, ...string) bool, resume func(string), applied *circadianApplied) blocker.GammaReading {
	kelvin, brightness, on := gammaTarget(cfg, minute)
	if previewed {
		return circadianReadGamma(rampFor(e.gammaKelvin, e.gammaBrightness))
	}
	if !on {
		// Both clips off (§4.10: off, it touches nothing): KC9K's own ramp,
		// if it is still on the screens, gives way to plain ones, once.
		// Someone else's (the desktop's night light) is left alone, also
		// right after a restart; nothing is read once there is nothing to
		// give back.
		reading := blocker.GammaUnknown
		if applied.Gamma {
			kelvin, brightness, known := applied.Kelvin, applied.Brightness, applied.Kelvin > 0
			if e.gammaSet {
				kelvin, brightness, known = e.gammaKelvin, e.gammaBrightness, true
			}
			if known {
				reading = circadianReadGamma(rampFor(kelvin, brightness))
			} else {
				reading = circadianReadGamma(nil)
			}
			if reading == blocker.GammaUnknown {
				return reading // no screen to restore yet
			}
			// An older version's ramp can't be told from someone else's: it
			// is given back unless the desktop's night light is on.
			if reading == blocker.GammaOurs || (!known && reading == blocker.GammaOther && !circadianNightLight()) {
				if _, err := circadianSetGamma(nil); err != nil {
					log.Printf("circadian: restoring the colours: %v", err)
				}
				reading = blocker.GammaIdentity
			}
			applied.Gamma, applied.Kelvin, applied.Brightness = false, 0, 0
		}
		e.gammaSet = false
		if _, paused := pauses[PauseGamma]; paused {
			resume(PauseGamma)
		}
		return reading
	}
	if !e.gammaStarted {
		// Right after a restart: what KC9K last set, to tell it from
		// someone else's ramp (the desktop's night light), which then
		// pauses the clips instead of being overridden.
		e.gammaStarted = true
		if !e.gammaSet && applied.Gamma && applied.Kelvin > 0 {
			e.gammaSet, e.gammaKelvin, e.gammaBrightness = true, applied.Kelvin, applied.Brightness
		}
	}

	var reading blocker.GammaReading
	if e.gammaSet {
		reading = circadianReadGamma(rampFor(e.gammaKelvin, e.gammaBrightness))
	} else {
		reading = circadianReadGamma(rampFor(kelvin, brightness))
	}
	_, paused := pauses[PauseGamma]
	switch reading {
	case blocker.GammaUnknown:
		return reading
	case blocker.GammaOther:
		if paused {
			return reading
		}
		// Right after the daemon starts (or a pause ends) the ramps found
		// are taken over: they are most likely KC9K's own from before.
		if e.gammaSet {
			if pause(PauseGamma, circadian.LaneSoftware, circadian.LaneKelvin) {
				log.Printf("circadian: the screen's colours were changed by someone else: software brightness and colour temperature are paused until their next point")
			}
			return reading
		}
	default: // ours or plain
		if paused {
			resume(PauseGamma)
		}
	}
	same := e.gammaSet && math.Abs(kelvin-e.gammaKelvin) < 1 && math.Abs(brightness-e.gammaBrightness) < 0.001
	if reading == blocker.GammaOurs && same {
		return reading
	}
	if n, err := circadianSetGamma(rampFor(kelvin, brightness)); n == 0 {
		if err != nil {
			log.Printf("circadian: setting the colours: %v", err)
		}
		return reading
	}
	e.gammaSet, e.gammaKelvin, e.gammaBrightness = true, kelvin, brightness
	applied.Gamma, applied.Kelvin, applied.Brightness = true, kelvin, brightness
	return blocker.GammaOurs
}

// WallpaperKey names the wallpaper to show at minute ("" for none), so a
// change of picture or of its times is noticed.
func WallpaperKey(c circadian.Config, minute float64) (key, image string) {
	if !c.On {
		return "", ""
	}
	i := c.WallpaperAt(minute)
	if i < 0 {
		return "", ""
	}
	w := c.Wallpapers[i]
	return fmt.Sprintf("%s@%d-%d", w.Image, w.From, w.To), w.Image
}

// DarkKey is "dark" or "light" when KC9K decides dark mode at minute, ""
// when it leaves it alone.
func DarkKey(c circadian.Config, minute float64) string {
	if !c.On {
		return ""
	}
	dark, decided := c.DarkAt(minute)
	switch {
	case !decided:
		return ""
	case dark:
		return "dark"
	}
	return "light"
}

// sameApplied reports whether a and b say the same.
func sameApplied(a, b circadianApplied) bool {
	return a.Wallpaper == b.Wallpaper && a.Dark == b.Dark && a.Gamma == b.Gamma && a.Kelvin == b.Kelvin &&
		a.Brightness == b.Brightness && slices.Equal(a.WallpaperUsers, b.WallpaperUsers) &&
		(a.WallpaperUsers == nil) == (b.WallpaperUsers == nil) && slices.Equal(a.DarkUsers, b.DarkUsers) &&
		(a.DarkUsers == nil) == (b.DarkUsers == nil)
}

// desktop gives the wallpaper and dark mode, each once per change of what
// should show, so what the user changes by hand afterwards stays. A change
// goes to everyone signed in and, into their settings, to the people who
// aren't, so a desktop that starts later (after a boot, at sign-in) starts
// with the right one; someone who signs in without having been given it
// (their settings couldn't be written) gets it at once.
func (e *Circadian) desktop(cfg circadian.Config, minute float64, now time.Time, applied *circadianApplied) {
	signedIn := circadianSignedIn()
	slices.Sort(signedIn)
	// R-MODE-5: a mode's wallpaper, circadian rhythm's or the default.
	look, hold := e.lookWithGrace(minute, now)
	key, image, keep := pickWallpaper(LoadWallpaperSettings(e.d), cfg, minute, look, hold)
	dark := DarkKey(cfg, minute)
	// A failure waits circadianRetry, unless someone signed in or out since,
	// or something else should show now.
	canTry := e.desktopAt.IsZero() || now.Sub(e.desktopAt) >= circadianRetry || !slices.Equal(signedIn, e.desktopUsers) ||
		key+"|"+dark != e.desktopKeys
	failed := false
	give := func(key string, done *string, users *[]string, set func(blocker.Reach) ([]string, error), what string) {
		if key == "" {
			*done, *users = "", nil
			return
		}
		if *done != "" && *users == nil {
			*users = append([]string{}, signedIn...) // from an older version: given to everyone signed in
		}
		fresh := key != *done
		if !fresh && !slices.ContainsFunc(signedIn, func(uid string) bool { return !slices.Contains(*users, uid) }) {
			return
		}
		if !canTry {
			return
		}
		reach := blocker.Reach{SignedOut: true}
		if !fresh {
			reach = blocker.Reach{Skip: *users} // only those who signed in since
		}
		reached, err := set(reach)
		if len(reached) == 0 {
			failed = true
			if err != nil {
				log.Printf("circadian: %s: %v", what, err)
			}
			return
		}
		if fresh {
			*done, *users = key, nil
		}
		merged := append(append([]string{}, *users...), reached...)
		slices.Sort(merged)
		*users = slices.Compact(merged)
	}
	if !keep {
		give(key, &applied.Wallpaper, &applied.WallpaperUsers, func(r blocker.Reach) ([]string, error) {
			return circadianSetWallpaper(wallpaperPath(image), r)
		}, "wallpaper")
	}
	give(dark, &applied.Dark, &applied.DarkUsers, func(r blocker.Reach) ([]string, error) {
		return circadianSetDark(dark == "dark", r)
	}, "dark mode")
	if failed {
		e.desktopAt, e.desktopUsers, e.desktopKeys = now, signedIn, key+"|"+dark
	}
}

// PreviewCircadian shows value on the clip lane at once, for the interface
// while a point is dragged, and keeps the engine away from that clip for
// PreviewHold. With end, the preview is over and the engine carries on from
// the clip. A clip that is off or paused isn't previewed: KC9K only moves
// levels it is driving.
func PreviewCircadian(d *db.DB, lane string, value float64, end bool, now time.Time) error {
	info, ok := circadian.FindLane(lane)
	if !ok {
		return fmt.Errorf("unknown clip %q", lane)
	}
	key := settingCircadianPreview + "." + lane
	if end {
		var p CircadianPreview
		if readJSONSetting(d, key, &p) && p.Until > now.Unix() {
			p.Until = now.Unix()
			writeJSONSetting(d, key, p)
		}
		return nil
	}
	if value < info.Min || value > info.Max {
		return fmt.Errorf("%s goes from %g to %g %s", info.Title, info.Min, info.Max, info.Unit)
	}
	cfg := CircadianConfig(d)
	if l := cfg.Lanes[lane]; !cfg.On || l == nil || !l.On {
		return fmt.Errorf("%s is off", info.Title)
	}
	if LanePausedUntil(CircadianPauses(d, now), lane) > 0 {
		return fmt.Errorf("%s is paused", info.Title)
	}
	p := CircadianPreview{Lane: lane, Value: value, Until: now.Add(PreviewHold).Unix()}
	if lane != circadian.LaneBacklight {
		// The ramp: this value and any other preview still running.
		running, _ := previews(d, now)
		running[lane] = p
		kelvin, brightness, _ := gammaTarget(cfg, circadian.MinuteOfDay(now))
		p.Kelvin, p.Brightness = withPreviews(running, kelvin, brightness)
		p.At = now.UnixNano()
	}
	// The note goes first, so the engine never sees the new level without it.
	writeJSONSetting(d, key, p)
	if circadianDry() {
		return nil
	}
	if lane == circadian.LaneBacklight {
		var firstErr error
		for _, b := range append(circadianBacklights(), circadianExternalKnown(d)...) {
			if err := b.Set(blocker.PercentToRaw(value, b.Max)); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}
	_, err := circadianSetGamma(rampFor(p.Kelvin, p.Brightness))
	return err
}

// DarkRule is what the extension needs for dark pages (§4.10).
type DarkRule struct {
	// On: show light pages dark now.
	On bool `json:"on"`
	// FollowSystem: KC9K leaves dark mode alone, so the page follows what
	// the browser says the system prefers.
	FollowSystem bool     `json:"follow_system"`
	Exempt       []string `json:"exempt"`
}

// CircadianDarkRule is the dark pages rule at now, or nil when dark pages
// are off.
func CircadianDarkRule(d *db.DB, now time.Time) *DarkRule {
	cfg := CircadianConfig(d)
	if !cfg.On || !cfg.DarkPages {
		return nil
	}
	rule := &DarkRule{Exempt: append([]string{}, cfg.DarkExempt...)}
	dark, decided := cfg.DarkAt(circadian.MinuteOfDay(now))
	rule.On = decided && dark
	rule.FollowSystem = !decided
	return rule
}

// PlayFrame is how long one step of PlayCircadian shows.
const PlayFrame = 100 * time.Millisecond

// PlayCircadian plays the next 24 hours on the screens in length (§4.10
// preview; owner, 2026-11-16: "Play the day"): the clips that are on and
// not paused (as previews, so the engine stays away), the wallpaper and
// dark mode at their times. Afterwards the clips carry on from now and the
// wallpaper, dark mode and themes are put back exactly as they were.
// frame, when given, is told each minute of the day shown.
func PlayCircadian(d *db.DB, length time.Duration, now time.Time, frame func(minute float64)) error {
	cfg := CircadianConfig(d)
	if !cfg.On {
		return fmt.Errorf("circadian rhythm is off: switch it on to play the day")
	}
	pauses := CircadianPauses(d, now)
	var lanes []string
	for _, info := range circadian.Lanes {
		if l := cfg.Lanes[info.Name]; l != nil && l.On && LanePausedUntil(pauses, info.Name) == 0 {
			lanes = append(lanes, info.Name)
		}
	}
	desktop := len(cfg.Wallpapers) > 0 || cfg.Dark.Mode != circadian.DarkSystem
	if len(lanes) == 0 && !desktop {
		return fmt.Errorf("nothing to play: every clip is off or paused, and no wallpaper or dark mode is set")
	}
	var look blocker.DesktopLook
	if desktop && !circadianDry() {
		look = circadianReadLook()
	}
	defer func() {
		for _, lane := range lanes {
			_ = PreviewCircadian(d, lane, 0, true, time.Now())
		}
		if look != nil {
			if err := circadianRestoreLook(look); err != nil {
				log.Printf("circadian: putting the desktop back after playing the day: %v", err)
			}
		}
	}()
	frames := max(int(length/PlayFrame), 1)
	start := circadian.MinuteOfDay(now)
	wallpaper, dark := "\x00", "\x00"
	external := circadianExternalKnown(d)
	for i := 0; i <= frames; i++ {
		minute := math.Mod(start+1440*float64(i)/float64(frames), 1440)
		if frame != nil {
			frame(minute)
		}
		values := map[string]float64{}
		at := time.Now()
		until := at.Add(PreviewHold).Unix()
		kelvin, brightness, _ := gammaTarget(cfg, minute)
		for _, lane := range lanes {
			if v, ok := cfg.Lanes[lane].ValueAt(minute); ok {
				values[lane] = v
				p := CircadianPreview{Lane: lane, Value: v, Until: until}
				if lane != circadian.LaneBacklight {
					p.Kelvin, p.Brightness, p.At = kelvin, brightness, at.UnixNano()
				}
				writeJSONSetting(d, settingCircadianPreview+"."+lane, p)
			}
		}
		if !circadianDry() {
			if v, ok := values[circadian.LaneBacklight]; ok {
				for _, b := range circadianBacklights() {
					_ = b.Set(blocker.PercentToRaw(v, b.Max))
				}
				if i%10 == 0 { // ddcutil is slow: about once a second
					for _, b := range external {
						_ = b.Set(blocker.PercentToRaw(v, b.Max))
					}
				}
			}
			_, kOK := values[circadian.LaneKelvin]
			_, sOK := values[circadian.LaneSoftware]
			if kOK || sOK {
				if _, err := circadianSetGamma(rampFor(kelvin, brightness)); err != nil && i == 0 {
					log.Printf("circadian: playing the day: %v", err)
				}
			}
			if key, image := WallpaperKey(cfg, minute); key != wallpaper {
				wallpaper = key
				if image != "" {
					_, _ = circadianSetWallpaper(filepath.Join(WallpapersDir(), image), blocker.Reach{})
				}
			}
			if key := DarkKey(cfg, minute); key != dark {
				dark = key
				if key != "" {
					_, _ = circadianSetDark(key == "dark", blocker.Reach{})
				}
			}
		}
		if i < frames {
			circadianSleep(length / time.Duration(frames))
		}
	}
	return nil
}
