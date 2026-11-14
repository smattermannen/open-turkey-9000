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
	settingCircadianPreview = "circadian.preview" // CircadianPreview
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
			out = append(out, circadianLevel{Name: s.Name, Max: 100,
				Get: func() (int, error) {
					percent, err := s.Brightness()
					return int(math.Round(percent)), err
				},
				Set: func(level int) error { return s.SetBrightness(float64(level)) }})
		}
		return out
	}
	circadianDDC          = blocker.DDCUtilInstalled
	circadianReadGamma    = blocker.ReadGamma
	circadianSetGamma     = blocker.SetGamma
	circadianSetWallpaper = blocker.SetWallpaper
	circadianSetDark      = blocker.SetDarkMode
	circadianNightLight   = blocker.NightLightOn
)

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
}

// circadianApplied is what the engine last gave the desktop, so it gives
// each wallpaper and dark mode once and leaves later changes by hand alone.
type circadianApplied struct {
	Wallpaper string `json:"wallpaper"`
	Dark      string `json:"dark"`
	Gamma     bool   `json:"gamma"` // KC9K's ramps are on the screens
}

// CircadianStatus is what the engine found, for `kc9k circadian show`.
type CircadianStatus struct {
	Backlights []string `json:"backlights"`
	External   []string `json:"external"`
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

// withPreview puts a previewed value in place of its clip's.
func withPreview(p *CircadianPreview, kelvin, brightness float64) (float64, float64) {
	if p != nil && p.Lane == circadian.LaneKelvin {
		kelvin = p.Value
	}
	if p != nil && p.Lane == circadian.LaneSoftware {
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

	slowAt     time.Time // the last look at external screens and the night light
	desktopAt  time.Time // the last wallpaper or dark mode that reached nobody
	external   []string
	nightLight bool
	status     string
}

// NewCircadian makes an engine for d.
func NewCircadian(d *db.DB) *Circadian {
	return &Circadian{d: d, last: map[string]int{}, pausedBy: map[string]string{}}
}

// RunCircadian steps the engine every CircadianInterval, forever. It returns
// at once for a database opened through KC9K_DB.
func RunCircadian(d *db.DB) {
	if circadianDry() {
		return
	}
	e := NewCircadian(d)
	e.Step(time.Now())
	ticker := time.NewTicker(CircadianInterval)
	defer ticker.Stop()
	for now := range ticker.C {
		e.Step(now)
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

// previewNow reads the preview, if one is running at now.
func previewNow(d *db.DB, now time.Time) (*CircadianPreview, bool) {
	var p CircadianPreview
	if !readJSONSetting(d, settingCircadianPreview, &p) || p.Lane == "" {
		return nil, false
	}
	return &p, p.Until > now.Unix()
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

	// A preview: the command drives the hardware; what it wrote is ours.
	preview, running := previewNow(e.d, now)
	if preview != nil {
		e.adoptPreview(cfg, preview, minute)
		if !running {
			_ = e.d.SetSetting(settingCircadianPreview, "")
			preview = nil
		}
	}

	pause := func(key string, lanes ...string) bool {
		// A preview that began while this step was reading the hardware
		// explains the change.
		if p, on := previewNow(e.d, time.Now()); p != nil && on {
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
	previewed := preview != nil && preview.Lane == circadian.LaneBacklight

	backlights := circadianBacklights()
	e.levels(PauseBacklight, backlights, backlightOn, previewed, lane, minute, pauses, pause, resume)
	if slow {
		e.slowAt = now
		e.external = nil
		if backlightOn {
			screens := circadianExternal()
			for _, s := range screens {
				e.external = append(e.external, s.Name)
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
	reading := e.gamma(cfg, preview, minute, now, pauses, pause, resume, &applied)
	e.desktop(cfg, minute, now, &applied)
	if applied != before {
		writeJSONSetting(e.d, settingCircadianApplied, applied)
	}
	if pausesChanged {
		writeJSONSetting(e.d, settingCircadianPaused, pauses)
	}

	status := CircadianStatus{Backlights: []string{}, External: append([]string{}, e.external...),
		DDCUtil: circadianDDC(), Gamma: reading.String(), NightLight: e.nightLight}
	for _, b := range backlights {
		status.Backlights = append(status.Backlights, b.Name)
	}
	if data, err := json.Marshal(status); err == nil && string(data) != e.status {
		if e.d.SetSetting(settingCircadianStatus, string(data)) == nil {
			e.status = string(data)
		}
	}
}

// adoptPreview notes what a preview wrote as the engine's own, so it isn't
// taken for a change by hand once the preview ends.
func (e *Circadian) adoptPreview(cfg circadian.Config, p *CircadianPreview, minute float64) {
	if p.Lane == circadian.LaneBacklight {
		for _, b := range circadianBacklights() {
			e.last[PauseBacklight+"/"+b.Name] = blocker.PercentToRaw(p.Value, b.Max)
		}
		for id := range e.last {
			if strings.HasPrefix(id, PauseExternal+"/") {
				e.last[id] = blocker.PercentToRaw(p.Value, 100)
			}
		}
		return
	}
	kelvin, brightness, _ := gammaTarget(cfg, minute)
	e.gammaKelvin, e.gammaBrightness = withPreview(p, kelvin, brightness)
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
func (e *Circadian) gamma(cfg circadian.Config, preview *CircadianPreview, minute float64, now time.Time,
	pauses map[string]int64, pause func(string, ...string) bool, resume func(string), applied *circadianApplied) blocker.GammaReading {
	kelvin, brightness, on := gammaTarget(cfg, minute)
	if preview != nil && preview.Lane != circadian.LaneBacklight {
		return circadianReadGamma(rampFor(e.gammaKelvin, e.gammaBrightness))
	}
	if !on {
		// Both clips off: give the screens their plain ramps back, once,
		// unless someone else has set ramps since.
		reading := circadianReadGamma(nil)
		if applied.Gamma {
			if e.gammaSet {
				reading = circadianReadGamma(rampFor(e.gammaKelvin, e.gammaBrightness))
			}
			if reading == blocker.GammaUnknown {
				return reading // no screen to restore yet
			}
			if reading == blocker.GammaOurs || !e.gammaSet {
				if _, err := circadianSetGamma(nil); err != nil {
					log.Printf("circadian: restoring the colours: %v", err)
				}
				reading = blocker.GammaIdentity
			}
			applied.Gamma = false
		}
		e.gammaSet = false
		if _, paused := pauses[PauseGamma]; paused {
			resume(PauseGamma)
		}
		return reading
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
	applied.Gamma = true
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

// desktop gives the wallpaper and dark mode, each once per change of what
// should show, so what the user changes by hand afterwards stays.
func (e *Circadian) desktop(cfg circadian.Config, minute float64, now time.Time, applied *circadianApplied) {
	canTry := e.desktopAt.IsZero() || now.Sub(e.desktopAt) >= circadianRetry
	if key, image := WallpaperKey(cfg, minute); key != applied.Wallpaper {
		if key == "" {
			applied.Wallpaper = ""
		} else if canTry {
			if n, err := circadianSetWallpaper(filepath.Join(WallpapersDir(), image)); n > 0 {
				applied.Wallpaper = key
			} else {
				e.desktopAt = now
				if err != nil {
					log.Printf("circadian: wallpaper: %v", err)
				}
			}
		}
	}
	if key := DarkKey(cfg, minute); key != applied.Dark {
		if key == "" {
			applied.Dark = ""
		} else if canTry {
			if n, err := circadianSetDark(key == "dark"); n > 0 {
				applied.Dark = key
			} else {
				e.desktopAt = now
				if err != nil {
					log.Printf("circadian: dark mode: %v", err)
				}
			}
		}
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
	if end {
		var p CircadianPreview
		if readJSONSetting(d, settingCircadianPreview, &p) && p.Lane != "" {
			p.Until = now.Unix()
			writeJSONSetting(d, settingCircadianPreview, p)
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
	// The note goes first, so the engine never sees the new level without it.
	writeJSONSetting(d, settingCircadianPreview, p)
	if circadianDry() {
		return nil
	}
	if lane == circadian.LaneBacklight {
		for _, b := range circadianBacklights() {
			if err := b.Set(blocker.PercentToRaw(value, b.Max)); err != nil {
				return err
			}
		}
		return nil
	}
	kelvin, brightness, _ := gammaTarget(cfg, circadian.MinuteOfDay(now))
	kelvin, brightness = withPreview(&p, kelvin, brightness)
	_, err := circadianSetGamma(rampFor(kelvin, brightness))
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
