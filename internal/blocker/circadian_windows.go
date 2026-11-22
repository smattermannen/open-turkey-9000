package blocker

// circadian_windows.go — the circadian rhythm's devices on Windows
// (ROADMAP W6): gamma ramps, wallpapers, dark mode and external screens
// through each session's agent (they belong to the person's desktop);
// a laptop panel's backlight through WMI; the sleep hook through the
// service's power events. Night light is read, not switched: Windows has
// no interface for it, and its settings blob is not KC9K's to rewrite.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/smattermannen/kc9k/internal/platform"
	"github.com/smattermannen/kc9k/internal/winagent"
	"golang.org/x/sys/windows/registry"
)

// enableGammaRange lets ramps go as far as a warm night needs: Windows
// otherwise refuses ramps far from plain (the setting f.lux and others
// use).
var enableGammaRange sync.Once

func gammaRange() {
	enableGammaRange.Do(func() {
		k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\ICM`, registry.QUERY_VALUE|registry.SET_VALUE)
		if err != nil {
			return
		}
		defer k.Close()
		if v, _, err := k.GetIntegerValue("GdiIcmGammaRange"); err != nil || v != 256 {
			_ = k.SetDWordValue("GdiIcmGammaRange", 256)
		}
	})
}

// ReadGamma reads the ramps of every screen of every session (Other
// outranks Identity, which outranks Ours).
func ReadGamma(want RampFunc) GammaReading {
	result := GammaUnknown
	for _, s := range deskSessions() {
		if !s.Active {
			continue
		}
		var ramps []winagent.Ramp
		if winagent.Call(s.ID, winagent.OpGetGamma, nil, &ramps) != nil {
			continue
		}
		for _, r := range ramps {
			var this GammaReading
			switch {
			case want != nil && rampsMatch(want, r.Red, r.Green, r.Blue):
				this = GammaOurs
			case rampsMatch(identityRamp, r.Red, r.Green, r.Blue) || allZero(r.Red, r.Green, r.Blue):
				this = GammaIdentity
			default:
				this = GammaOther
			}
			if result == GammaUnknown || this > result {
				result = this
			}
		}
	}
	return result
}

// SetGamma sets ramp on every screen of every session at the screen and
// returns how many screens took it.
func SetGamma(ramp RampFunc) (int, error) {
	if ramp == nil {
		ramp = identityRamp
	}
	gammaRange()
	red, green, blue := ramp(256)
	total := 0
	var firstErr error
	for _, s := range deskSessions() {
		if !s.Active {
			continue
		}
		var n int
		if err := winagent.Call(s.ID, winagent.OpSetGamma, winagent.Ramp{Red: red, Green: green, Blue: blue}, &n); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		total += n
	}
	return total, firstErr
}

// reachSessions are the sessions r reaches (signed-out people can't be
// reached on Windows: their settings aren't loaded).
func reachSessions(r Reach) []deskSession {
	skip := map[string]bool{}
	for _, uid := range r.Skip {
		skip[uid] = true
	}
	var out []deskSession
	for _, s := range deskSessions() {
		if !skip[strconv.FormatInt(s.UID, 10)] {
			out = append(out, s)
		}
	}
	return out
}

// wallpaperShare is where wallpapers are put for people's desktops to
// read (the data folder is SYSTEM's alone).
var wallpaperShare = filepath.Join(platform.ShareDir, "wallpapers")

// shareWallpaper copies path where desktops can read it.
func shareWallpaper(path string) (string, error) {
	if err := os.MkdirAll(wallpaperShare, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(wallpaperShare, filepath.Base(path))
	src, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return "", err
	}
	if t, err := os.Stat(target); err == nil && t.Size() == info.Size() && !t.ModTime().Before(info.ModTime()) {
		return target, nil
	}
	tmp := target + ".tmp"
	dst, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return "", err
	}
	dst.Close()
	return target, os.Rename(tmp, target)
}

// SetWallpaper puts the image at path on the desktops r reaches and
// returns their account IDs.
func SetWallpaper(path string, r Reach) ([]string, error) {
	shared, err := shareWallpaper(path)
	if err != nil {
		return nil, err
	}
	var reached []string
	var firstErr error
	for _, s := range reachSessions(r) {
		if err := winagent.Call(s.ID, winagent.OpSetWallpaper, shared, nil); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		reached = append(reached, strconv.FormatInt(s.UID, 10))
	}
	return reached, firstErr
}

// SetDarkMode switches the desktops r reaches to dark or light.
func SetDarkMode(dark bool, r Reach) ([]string, error) {
	var reached []string
	var firstErr error
	for _, s := range reachSessions(r) {
		if err := winagent.Call(s.ID, winagent.OpSetDark, dark, nil); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		reached = append(reached, strconv.FormatInt(s.UID, 10))
	}
	return reached, firstErr
}

// Keys of a DesktopLook on Windows.
const (
	lookWallpaper   = "wallpaper"
	lookAppsLight   = "apps-light"
	lookSystemLight = "system-light"
)

// ReadDesktopLook is each signed-in person's wallpaper and theme.
func ReadDesktopLook() DesktopLook {
	look := DesktopLook{}
	for _, s := range deskSessions() {
		var l winagent.Look
		if winagent.Call(s.ID, winagent.OpGetLook, nil, &l) != nil {
			continue
		}
		look[s.User] = map[string]string{lookWallpaper: l.Wallpaper,
			lookAppsLight: strconv.Itoa(l.AppsLight), lookSystemLight: strconv.Itoa(l.SystemLight)}
	}
	return look
}

// RestoreDesktopLook puts back what ReadDesktopLook read.
func RestoreDesktopLook(look DesktopLook) error {
	var firstErr error
	for _, s := range deskSessions() {
		values, ok := look[s.User]
		if !ok {
			continue
		}
		if w := values[lookWallpaper]; w != "" {
			if err := winagent.Call(s.ID, winagent.OpSetWallpaper, w, nil); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if v := values[lookAppsLight]; v == "0" || v == "1" {
			if err := winagent.Call(s.ID, winagent.OpSetDark, v == "0", nil); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// NightLightOn reports whether Windows' Night light is on for someone at
// the screen.
func NightLightOn() bool {
	for _, s := range deskSessions() {
		var on bool
		if s.Active && winagent.Call(s.ID, winagent.OpNightLightOn, nil, &on) == nil && on {
			return true
		}
	}
	return false
}

// TurnOffNightLight: Windows offers no way to; the person does it.
func TurnOffNightLight() error {
	return fmt.Errorf("turn Night light off in Settings > System > Display: Windows lets no program switch it")
}

// Backlight is a laptop panel's backlight (WMI's WmiMonitorBrightness).
type Backlight struct {
	Name string
	Max  int
}

// The panel's brightness goes through PowerShell (WMI's
// WmiMonitorBrightness): x/sys has no WMI over COM, and \\.\LCD's
// brightness IOCTLs can't be proven on a runner without a panel. So each
// PowerShell is rare (W50; before, one ran every 5 s, every 1 s after a
// start or wake, every 100 ms while playing the day): the panel is looked
// for once, again after ForgetBacklights (a wake) or a failed call; its
// level is read at most once a minute (in between, the last read or set
// level); a set at most once a second, the newest level winning, sent by
// a timer when it came sooner; never two at once.
var panel = struct {
	sync.Mutex             // the cache
	run        sync.Mutex  // one PowerShell at a time
	looked     bool        // looked for since the last forget
	found      bool        // a panel is there
	level      int         // the last level read or set
	readAt     time.Time   // when it was last read
	setAt      time.Time   // when it was last set
	want       int         // a set waiting for its second, or -1
	timer      *time.Timer // sends want
}{want: -1}

// Seams for tests.
var (
	panelNow  = time.Now
	panelRead = func() (int, error) {
		out, err := powershell(`(Get-CimInstance -Namespace root/wmi -ClassName WmiMonitorBrightness -ErrorAction Stop | Select-Object -First 1).CurrentBrightness`)
		if err != nil {
			return 0, err
		}
		s := strings.TrimSpace(string(out))
		if s == "" {
			return 0, fmt.Errorf("no panel")
		}
		return strconv.Atoi(s)
	}
	panelWrite = func(level int) error {
		_, err := powershell(fmt.Sprintf(`Get-CimInstance -Namespace root/wmi -ClassName WmiMonitorBrightnessMethods -ErrorAction Stop | Invoke-CimMethod -MethodName WmiSetBrightness -Arguments @{Timeout=1; Brightness=%d} -ErrorAction Stop | Out-Null`, level))
		return err
	}
)

const (
	panelReadEvery = time.Minute
	panelSetEvery  = time.Second
)

// readPanel reads the panel's level through PowerShell, one at a time.
func readPanel() (int, error) {
	panel.run.Lock()
	defer panel.run.Unlock()
	return panelRead()
}

// writePanel sets it, one at a time.
func writePanel(level int) error {
	panel.run.Lock()
	defer panel.run.Unlock()
	return panelWrite(level)
}

// ForgetBacklights makes the next Backlights look for the panel again (a
// wake; the screens may have changed).
func ForgetBacklights() {
	panel.Lock()
	defer panel.Unlock()
	panel.looked = false
}

// Backlights are the laptop panels whose brightness Windows sets.
func Backlights() []Backlight {
	panel.Lock()
	looked := panel.looked
	panel.Unlock()
	if !looked {
		level, err := readPanel()
		now := panelNow()
		panel.Lock()
		panel.looked, panel.found = true, err == nil
		if err == nil {
			panel.level, panel.readAt = level, now
		}
		panel.Unlock()
	}
	panel.Lock()
	defer panel.Unlock()
	if !panel.found {
		return nil
	}
	return []Backlight{{Name: "panel", Max: 100}}
}

// Raw reads the panel's brightness (0-100): through PowerShell at most
// once a minute.
func (b Backlight) Raw() (int, error) {
	panel.Lock()
	if panelNow().Sub(panel.readAt) < panelReadEvery || panel.want >= 0 {
		level := panel.level
		if panel.want >= 0 {
			level = panel.want
		}
		panel.Unlock()
		return level, nil
	}
	panel.Unlock()
	level, err := readPanel()
	panel.Lock()
	defer panel.Unlock()
	if err != nil {
		panel.looked = false // gone (a display change): looked for again
		return 0, err
	}
	panel.readAt = panelNow()
	if panel.want < 0 {
		panel.level = level
	}
	return panel.level, nil
}

// SetRaw sets the panel's brightness (0-100): at once when the last set
// was a second ago or more, else the newest level a second after it.
func (b Backlight) SetRaw(level int) error {
	level = max(0, min(100, level))
	panel.Lock()
	wait := panelSetEvery - panelNow().Sub(panel.setAt)
	if wait > 0 {
		panel.want = level
		if panel.timer == nil {
			panel.timer = time.AfterFunc(wait, flushPanel)
		}
		panel.Unlock()
		return nil
	}
	panel.setAt, panel.want = panelNow(), -1
	panel.Unlock()
	return setPanel(level)
}

// flushPanel sends the level that waited for its second.
func flushPanel() {
	panel.Lock()
	level := panel.want
	panel.timer, panel.want = nil, -1
	if level >= 0 {
		panel.setAt = panelNow()
	}
	panel.Unlock()
	if level >= 0 {
		_ = setPanel(level)
	}
}

func setPanel(level int) error {
	err := writePanel(level)
	panel.Lock()
	defer panel.Unlock()
	if err != nil {
		panel.looked = false
		return err
	}
	panel.level = level
	return nil
}

// External screens are found through the session at the screen; their
// Display is "<session>:<index>".
var externalCache struct {
	sync.Mutex
	screens []ExternalScreen
	at      time.Time
	max     map[string]winagent.Monitor
}

// DDCUtilInstalled: DDC/CI needs nothing installed on Windows (dxva2).
func DDCUtilInstalled() bool { return true }

// ExternalScreens are the screens with DDC/CI brightness, found at most
// once a minute (DDC/CI is slow).
func ExternalScreens() []ExternalScreen {
	externalCache.Lock()
	defer externalCache.Unlock()
	if time.Since(externalCache.at) < time.Minute {
		return externalCache.screens
	}
	externalCache.at = time.Now()
	externalCache.screens = nil
	externalCache.max = map[string]winagent.Monitor{}
	for _, s := range deskSessions() {
		if !s.Active {
			continue
		}
		var list []winagent.Monitor
		if winagent.Call(s.ID, winagent.OpMonitors, nil, &list) != nil {
			continue
		}
		for _, m := range list {
			id := fmt.Sprintf("%d:%d", s.ID, m.Index)
			externalCache.screens = append(externalCache.screens, ExternalScreen{Display: id, Name: m.Name})
			externalCache.max[id] = m
		}
		break
	}
	return externalCache.screens
}

func parseDisplay(display string) (session uint32, index int, err error) {
	a, b, ok := strings.Cut(display, ":")
	if !ok {
		return 0, 0, fmt.Errorf("bad screen %q", display)
	}
	s, err1 := strconv.ParseUint(a, 10, 32)
	i, err2 := strconv.Atoi(b)
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("bad screen %q", display)
	}
	return uint32(s), i, nil
}

// Brightness reads the screen's brightness in percent.
func (s ExternalScreen) Brightness() (float64, error) {
	session, index, err := parseDisplay(s.Display)
	if err != nil {
		return 0, err
	}
	var list []winagent.Monitor
	if err := winagent.Call(session, winagent.OpMonitors, nil, &list); err != nil {
		return 0, err
	}
	for _, m := range list {
		if m.Index == index && m.Max > m.Min {
			return float64(m.Level-m.Min) * 100 / float64(m.Max-m.Min), nil
		}
	}
	return 0, fmt.Errorf("screen %s is gone", s.Name)
}

// SetBrightness sets the screen's brightness in percent.
func (s ExternalScreen) SetBrightness(percent float64) error {
	session, index, err := parseDisplay(s.Display)
	if err != nil {
		return err
	}
	externalCache.Lock()
	m, ok := externalCache.max[s.Display]
	externalCache.Unlock()
	lo, hi := uint32(0), uint32(100)
	if ok && m.Max > m.Min {
		lo, hi = m.Min, m.Max
	}
	level := lo + uint32(percent/100*float64(hi-lo)+0.5)
	return winagent.Call(session, winagent.OpSetMonitor, winagent.Monitor{Index: index, Level: level}, nil)
}

// sleepHookPath, while it exists, makes the service run `kc9k wallpaper
// before-sleep` when the computer goes to sleep (its power events).
var sleepHookPath = filepath.Join(platform.DataDir, "sleep-hook")

// SyncSleepHook turns the sleep hook on or off.
func SyncSleepHook(on bool) error {
	if !on {
		if err := os.Remove(sleepHookPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if _, err := os.Stat(sleepHookPath); err == nil {
		return nil
	}
	return os.WriteFile(sleepHookPath, []byte("wallpaper before-sleep\n"), 0o600)
}

// SleepHookOn reports whether the service should run the hook before
// sleep.
func SleepHookOn() bool {
	_, err := os.Stat(sleepHookPath)
	return err == nil
}
