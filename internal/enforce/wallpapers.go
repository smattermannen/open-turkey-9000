// wallpapers.go — wallpapers of modes (§4.10, R-MODE-5; owner 2026-11-17):
// a mode's pictures show while it is enabled, each between its two times,
// over circadian rhythm's unless circadian rhythm overrides modes (one
// setting, shown in both places); of several modes, the one enabled last
// wins. While the place isn't known (no network seen: Wi-Fi off, nothing
// in range, the first seconds after boot or waking) a mode switched by
// places can't say, and the wallpaper default applies: the latest
// wallpaper (left as it is), circadian rhythm's, or a picture of its own.
// The default also applies when nothing else does. The circadian engine
// (circadian.go) puts the choice on the desktop, also into the settings of
// people not signed in, so a desktop that starts later starts with it; it
// looks every second while the place is unknown after boot or waking, and
// a sleep hook puts the default on before the computer sleeps.
package enforce

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/circadian"
	"github.com/smattermannen/kc9k/internal/db"
)

// settingModeWallpapers + a mode's id holds its wallpapers (JSON);
// FrozenChickenWallpapers stands for Frozen Chicken, which has no id.
const (
	settingModeWallpapers   = "mode.wallpapers."
	FrozenChickenWallpapers = 0
)

// settingWallpaper holds the wallpaper settings outside circadian rhythm;
// settingBeforeSleep the time the sleep hook changed the wallpaper.
const (
	settingWallpaper   = "wallpaper"
	settingBeforeSleep = "wallpaper.before_sleep"
)

// Wallpaper defaults: what shows when neither a mode nor circadian rhythm
// has a wallpaper, and while the place isn't known.
const (
	WallpaperLatest    = "latest"    // the desktop keeps what it shows
	WallpaperCircadian = "circadian" // circadian rhythm's wallpapers (even with it switched off)
	WallpaperCustom    = "custom"    // a picture chosen for it
)

// placeGrace: a place known a moment ago isn't left because one look saw no
// network; for this long, what shows stays.
const placeGrace = time.Minute

// WallpaperSettings are the wallpaper settings outside circadian rhythm.
type WallpaperSettings struct {
	// CircadianFirst: circadian rhythm overrides modes ("Override modes");
	// off (the default), modes override circadian rhythm ("Override
	// circadian rhythm wallpapers").
	CircadianFirst bool   `json:"circadian_first"`
	Default        string `json:"default"`               // WallpaperLatest, WallpaperCircadian or WallpaperCustom
	Custom         string `json:"custom,omitempty"`      // the stored picture, for WallpaperCustom
	CustomName     string `json:"custom_name,omitempty"` // its file's name when it was chosen
}

// LoadWallpaperSettings reads the wallpaper settings.
func LoadWallpaperSettings(d *db.DB) WallpaperSettings {
	var s WallpaperSettings
	readJSONSetting(d, settingWallpaper, &s)
	if s.Default != WallpaperCircadian && s.Default != WallpaperCustom {
		s.Default = WallpaperLatest
	}
	if s.Default == WallpaperCustom && s.Custom == "" {
		s.Default = WallpaperLatest
	}
	return s
}

// SaveWallpaperSettings stores them.
func SaveWallpaperSettings(d *db.DB, s WallpaperSettings) error {
	switch {
	case s.Default != WallpaperLatest && s.Default != WallpaperCircadian && s.Default != WallpaperCustom:
		return fmt.Errorf("the wallpaper default is latest, circadian or custom")
	case s.Default == WallpaperCustom && s.Custom == "":
		return fmt.Errorf("a custom wallpaper default needs its picture")
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return d.SetSetting(settingWallpaper, string(raw))
}

// ModeWallpapers returns a mode's wallpapers (FrozenChickenWallpapers:
// Frozen Chicken's).
func ModeWallpapers(d *db.DB, modeID int) []circadian.Wallpaper {
	out := []circadian.Wallpaper{}
	readJSONSetting(d, settingModeWallpapers+strconv.Itoa(modeID), &out)
	return out
}

// SetModeWallpapers stores a mode's wallpapers.
func SetModeWallpapers(d *db.DB, modeID int, ws []circadian.Wallpaper) error {
	if len(ws) == 0 {
		return d.SetSetting(settingModeWallpapers+strconv.Itoa(modeID), "")
	}
	raw, err := json.Marshal(ws)
	if err != nil {
		return err
	}
	return d.SetSetting(settingModeWallpapers+strconv.Itoa(modeID), string(raw))
}

// WallpaperInUse reports whether a stored picture is still a wallpaper of
// circadian rhythm or of a mode, or the default's.
func WallpaperInUse(d *db.DB, image string) bool {
	uses := func(ws []circadian.Wallpaper) bool {
		return slices.ContainsFunc(ws, func(w circadian.Wallpaper) bool { return w.Image == image })
	}
	if uses(CircadianConfig(d).Wallpapers) || LoadWallpaperSettings(d).Custom == image ||
		uses(ModeWallpapers(d, FrozenChickenWallpapers)) {
		return true
	}
	modes, _ := d.Modes()
	for _, m := range modes {
		if uses(ModeWallpapers(d, m.ID)) {
			return true
		}
	}
	return false
}

// modeLook is what the modes say about the wallpaper now.
type modeLook struct {
	key, image string // the wallpaper of the enabled mode enabled last ("": none)
	mode       string // that mode's name
	// unsure: a mode switched by places, with a wallpaper now, might be
	// enabled, but the place isn't known.
	unsure bool
	// placeMatters: a mode switched by places has wallpapers at all.
	placeMatters bool
}

// lookAtModes finds the wallpapers of the enabled modes at minute. unknown
// says whether the place is known; it is only asked when it matters.
// since: when each mode was first seen enabled, kept up to date here.
func lookAtModes(d *db.DB, minute float64, now time.Time, unknown func() bool, since map[int]time.Time) modeLook {
	var out modeLook
	modes, err := d.Modes()
	if err != nil {
		return out
	}
	type candidate struct {
		id     int
		name   string
		places bool
		on     func() bool
		ws     []circadian.Wallpaper
	}
	trusted, _ := d.TrustedNow(now)
	var enabled map[string]bool
	var all []candidate
	for _, m := range modes {
		ws := ModeWallpapers(d, m.ID)
		if len(ws) == 0 {
			continue
		}
		m := m
		on := func() bool {
			if m.Monk {
				return MonkOn(d, trusted)
			}
			if enabled == nil {
				enabled = EnabledNames(d, trusted)
			}
			return slices.ContainsFunc(m.Blocks, func(b string) bool { return enabled[b] })
		}
		all = append(all, candidate{m.ID, m.Name, len(m.Spec.Places) > 0, on, ws})
		out.placeMatters = out.placeMatters || len(m.Spec.Places) > 0
	}
	if ws := ModeWallpapers(d, FrozenChickenWallpapers); len(ws) > 0 {
		all = append(all, candidate{FrozenChickenWallpapers, db.FrozenChickenName, false, func() bool {
			until, err := d.FreezeUntil()
			return err == nil && until > now.Unix()
		}, ws})
	}
	seen := map[int]bool{}
	var bestAt time.Time
	for _, c := range all {
		if !c.on() {
			continue
		}
		seen[c.id] = true
		i := circadian.Config{Wallpapers: c.ws}.WallpaperAt(minute)
		if c.places && unknown() {
			out.unsure = out.unsure || i >= 0
			continue // enabled or not isn't known: keeps when it was enabled
		}
		if _, had := since[c.id]; !had {
			since[c.id] = now
		}
		if i < 0 {
			continue
		}
		if out.key == "" || since[c.id].After(bestAt) {
			w := c.ws[i]
			out.key, out.image, out.mode, bestAt = fmt.Sprintf("mode%d:%s@%d-%d", c.id, w.Image, w.From, w.To), w.Image, c.name, since[c.id]
		}
	}
	for id := range since {
		if !seen[id] {
			delete(since, id)
		}
	}
	return out
}

// pickWallpaper chooses between the modes' wallpaper, circadian rhythm's
// and the default. keep: leave the desktop, and what KC9K counts as given,
// as they are. hold: the place was known a moment ago, so while it is
// unknown what shows stays (one look without networks isn't leaving).
func pickWallpaper(s WallpaperSettings, cfg circadian.Config, minute float64, modes modeLook, hold bool) (key, image string, keep bool) {
	circKey, circImage := WallpaperKey(cfg, minute)
	switch {
	case s.CircadianFirst && circKey != "":
		return circKey, circImage, false
	case modes.key != "":
		return modes.key, modes.image, false
	case !modes.unsure && circKey != "":
		return circKey, circImage, false
	case modes.unsure && hold:
		return "", "", true
	}
	switch s.Default {
	case WallpaperCircadian:
		all := cfg
		all.On = true
		if key, image := WallpaperKey(all, minute); key != "" {
			return key, image, false
		}
	case WallpaperCustom:
		return "default:" + s.Custom, s.Custom, false
	}
	// The latest wallpaper: nothing changes. While the place is unknown, the
	// one given counts as still given, so it isn't given again (over one
	// picked by hand) when the place is known again.
	return "", "", modes.unsure
}

// WallpaperNow says which wallpaper KC9K shows now and why: "" when it
// leaves the desktop as it is. For `kc9k wallpaper`.
func WallpaperNow(d *db.DB, now time.Time) (image, why string) {
	cfg := CircadianConfig(d)
	minute := circadian.MinuteOfDay(now)
	places, _ := d.Places()
	unknownOnce := func() bool { _, unknown := placesHere(now, places); return unknown }
	look := lookAtModes(d, minute, now, unknownOnce, map[int]time.Time{})
	s := LoadWallpaperSettings(d)
	key, image, _ := pickWallpaper(s, cfg, minute, look, false)
	circKey, _ := WallpaperKey(cfg, minute)
	switch {
	case key == "" && look.unsure:
		return "", "the place isn't known (no network seen): the latest wallpaper stays"
	case key == "":
		return "", "no mode or circadian rhythm wallpaper now: the latest wallpaper stays"
	case key == look.key:
		return image, "the mode '" + look.mode + "'"
	case key == circKey:
		return image, "circadian rhythm"
	case look.unsure:
		return image, "the default, while the place isn't known (no network seen)"
	}
	return image, "the default (no mode or circadian rhythm wallpaper now)"
}

// lookWithGrace looks at the modes with the place as the engine knows it:
// a place seen a moment ago is kept for placeGrace (keep), so one look
// without networks doesn't change the wallpaper.
func (e *Circadian) lookWithGrace(minute float64, now time.Time) (modeLook, bool) {
	if e.modeOnSince == nil {
		e.modeOnSince = map[int]time.Time{}
	}
	asked, unknown := false, false
	ask := func() bool {
		if !asked {
			asked = true
			places, _ := e.d.Places()
			_, unknown = placesHere(now, places)
			if !unknown {
				e.placeKnownAt = now
			}
		}
		return unknown
	}
	look := lookAtModes(e.d, minute, now, ask, e.modeOnSince)
	e.unsure = look.unsure
	e.placeMatters = look.placeMatters
	graced := look.unsure && !e.placeKnownAt.IsZero() && now.Sub(e.placeKnownAt) < placeGrace
	return look, graced
}

// beforeSleepGiven reports whether the sleep hook changed the wallpaper and
// the computer has woken since (or didn't sleep after all): what KC9K
// counts as given is then what the hook gave, so it is given afresh.
func (e *Circadian) beforeSleepGiven(now time.Time, woke bool) bool {
	raw, _ := e.d.GetSetting(settingBeforeSleep)
	at, err := strconv.ParseInt(raw, 10, 64)
	if raw == "" || err != nil {
		return false
	}
	if !woke && now.Unix()-at < 60 {
		return false // the computer is about to sleep: nothing changes until it wakes
	}
	_ = e.d.SetSetting(settingBeforeSleep, "")
	return true
}

// WallpaperBeforeSleep puts the wallpaper for an unknown place on the
// desktop before the computer sleeps (the sleep hook): with the default
// Custom or Circadian rhythm, a mode switched by places doesn't show its
// wallpaper at the next place before KC9K can tell where that is.
func WallpaperBeforeSleep(d *db.DB, now time.Time) error {
	if circadianDry() {
		return nil
	}
	s := LoadWallpaperSettings(d)
	if s.Default == WallpaperLatest {
		return nil
	}
	cfg := CircadianConfig(d)
	minute := circadian.MinuteOfDay(now)
	look := lookAtModes(d, minute, now, func() bool { return true }, map[int]time.Time{})
	if !look.placeMatters {
		return nil
	}
	key, image, keep := pickWallpaper(s, cfg, minute, look, false)
	applied := circadianApplied{}
	readJSONSetting(d, settingCircadianApplied, &applied)
	if keep || key == "" || key == applied.Wallpaper {
		return nil
	}
	if _, err := circadianSetWallpaper(wallpaperPath(image), blocker.Reach{SignedOut: true}); err != nil {
		return err
	}
	return d.SetSetting(settingBeforeSleep, strconv.FormatInt(now.Unix(), 10))
}

// syncSleepHook keeps the sleep hook while it can matter: a mode switched
// by places has wallpapers and the default isn't the latest wallpaper.
func (e *Circadian) syncSleepHook() {
	want := e.placeMatters && LoadWallpaperSettings(e.d).Default != WallpaperLatest
	if e.hookSynced && e.hookWanted == want {
		return
	}
	if err := circadianSleepHook(want); err != nil {
		return // tried again next step
	}
	e.hookSynced, e.hookWanted = true, want
}
