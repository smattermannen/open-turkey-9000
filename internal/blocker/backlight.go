// backlight.go — the screen's real brightness, for the circadian rhythm:
// the laptop panel through /sys/class/backlight, and external screens
// through DDC/CI with ddcutil when it is installed.
package blocker

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// backlightDir holds the kernel's backlight devices (a variable for tests).
var backlightDir = "/sys/class/backlight"

// Backlight is one backlight device.
type Backlight struct {
	Name string
	Max  int
	dir  string
}

// Backlights returns the backlight devices to drive. A panel often has
// several (the firmware's, the platform's and the graphics driver's); like
// the desktops, KC9K prefers firmware, then platform, then raw devices, and
// drives every device of the best kind there is.
func Backlights() []Backlight {
	entries, err := os.ReadDir(backlightDir)
	if err != nil {
		return nil
	}
	byKind := map[string][]Backlight{}
	for _, e := range entries {
		dir := filepath.Join(backlightDir, e.Name())
		max, err := readInt(filepath.Join(dir, "max_brightness"))
		if err != nil || max <= 0 {
			continue
		}
		kind := "raw"
		if data, err := os.ReadFile(filepath.Join(dir, "type")); err == nil {
			kind = strings.TrimSpace(string(data))
		}
		byKind[kind] = append(byKind[kind], Backlight{Name: e.Name(), Max: max, dir: dir})
	}
	for _, kind := range []string{"firmware", "platform", "raw"} {
		if len(byKind[kind]) > 0 {
			return byKind[kind]
		}
	}
	return nil
}

func readInt(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// Raw is the device's current level, from 0 to Max.
func (b Backlight) Raw() (int, error) {
	return readInt(filepath.Join(b.dir, "brightness"))
}

// SetRaw sets the device's level.
func (b Backlight) SetRaw(level int) error {
	level = max(0, min(b.Max, level))
	return os.WriteFile(filepath.Join(b.dir, "brightness"), []byte(strconv.Itoa(level)), 0o644)
}

// PercentToRaw turns a brightness in percent into a level of max, never
// fully off: a black screen at night would look broken.
func PercentToRaw(percent float64, max int) int {
	return int(math.Max(1, math.Round(percent/100*float64(max))))
}

// RawToPercent turns a level of max into percent.
func RawToPercent(level, max int) float64 {
	if max <= 0 {
		return 0
	}
	return float64(level) * 100 / float64(max)
}

// ExternalScreen is a screen whose brightness DDC/CI sets.
type ExternalScreen struct {
	Display string // ddcutil's display number
	Name    string
}

// ddcutilTimeout bounds one ddcutil run: DDC/CI is slow, and a screen that
// doesn't answer must not hold the circadian cycle up.
const ddcutilTimeout = 8 * time.Second

var ddc struct {
	sync.Mutex
	screens []ExternalScreen
	at      time.Time
	max     map[string]int // display -> its brightness maximum, once read
}

// DDCUtilInstalled reports whether ddcutil is there to reach external screens.
func DDCUtilInstalled() bool {
	_, err := exec.LookPath("ddcutil")
	return err == nil
}

func ddcutil(args ...string) (string, error) {
	path, err := exec.LookPath("ddcutil")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), ddcutilTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, args...).Output()
	return string(out), err
}

// ExternalScreens lists the screens that answer DDC/CI. Looking takes
// seconds, so the answer is kept for ten minutes (screens plugged in later
// are found then).
func ExternalScreens() []ExternalScreen {
	ddc.Lock()
	defer ddc.Unlock()
	if !ddc.at.IsZero() && time.Since(ddc.at) < 10*time.Minute {
		return ddc.screens
	}
	ddc.at = time.Now()
	ddc.screens = nil
	out, err := ddcutil("detect", "--brief")
	if err != nil {
		return nil
	}
	ddc.screens = parseDDCDetect(out)
	return ddc.screens
}

// parseDDCDetect reads `ddcutil detect --brief`: a "Display N" line per
// screen that answers, then indented details such as "Monitor: …".
func parseDDCDetect(out string) []ExternalScreen {
	var screens []ExternalScreen
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if number, ok := strings.CutPrefix(line, "Display "); ok {
			number = strings.TrimSpace(number)
			if _, err := strconv.Atoi(number); err == nil {
				screens = append(screens, ExternalScreen{Display: number, Name: "Display " + number})
			}
			continue
		}
		if monitor, ok := strings.CutPrefix(trimmed, "Monitor:"); ok && len(screens) > 0 {
			// mfg:model:serial
			parts := strings.Split(strings.TrimSpace(monitor), ":")
			if len(parts) >= 2 && parts[1] != "" {
				screens[len(screens)-1].Name = parts[1]
			}
		}
	}
	return screens
}

// Brightness reads the screen's brightness in percent of its maximum.
func (s ExternalScreen) Brightness() (float64, error) {
	out, err := ddcutil("--display", s.Display, "getvcp", "10", "--brief")
	if err != nil {
		return 0, err
	}
	cur, max, err := parseVCP(out)
	if err != nil {
		return 0, err
	}
	ddc.Lock()
	if ddc.max == nil {
		ddc.max = map[string]int{}
	}
	ddc.max[s.Display] = max
	ddc.Unlock()
	return RawToPercent(cur, max), nil
}

// parseVCP reads `getvcp 10 --brief`: "VCP 10 C <current> <max>".
func parseVCP(out string) (int, int, error) {
	fields := strings.Fields(out)
	if len(fields) >= 5 && fields[0] == "VCP" && fields[2] == "C" {
		cur, err1 := strconv.Atoi(fields[3])
		max, err2 := strconv.Atoi(fields[4])
		if err1 == nil && err2 == nil && max > 0 {
			return cur, max, nil
		}
	}
	return 0, 0, fmt.Errorf("unexpected answer from ddcutil: %q", strings.TrimSpace(out))
}

// SetBrightness sets the screen's brightness in percent. Screens nearly
// always count from 0 to 100; others are scaled by the maximum Brightness
// read.
func (s ExternalScreen) SetBrightness(percent float64) error {
	ddc.Lock()
	max := ddc.max[s.Display]
	ddc.Unlock()
	if max <= 0 {
		max = 100
	}
	value := int(math.Max(1, math.Round(percent/100*float64(max))))
	_, err := ddcutil("--noverify", "--display", s.Display, "setvcp", "10", strconv.Itoa(value))
	return err
}
