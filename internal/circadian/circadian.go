// Package circadian is the time-of-day logic of the circadian rhythm (§4.10):
// automation clips for the screen, wallpapers by time, and dark mode by
// time. It is pure: no files, no devices, no database. The enforce package
// applies what it computes.
//
// An automation clip is a 24-hour curve, like an automation clip in a music
// program: points (a time and a value), and between two points a curve
// whose tension bends it. The clip repeats every day, so the last point
// curves on into the first point of the next day.
//
// Nothing here is ever locked: the circadian rhythm is a comfort, and
// KC9K never puts a level back after the user changed it.
package circadian

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DayMinutes is the length of a clip.
const DayMinutes = 24 * 60

// The clips.
const (
	LaneBacklight = "backlight" // the screen's own brightness, in percent
	LaneSoftware  = "software"  // brightness drawn by the graphics card, in percent
	LaneKelvin    = "kelvin"    // colour temperature, in kelvin
)

// LaneInfo describes a clip: its name, range and unit.
type LaneInfo struct {
	Name    string
	Title   string
	Unit    string
	Min     float64
	Max     float64
	Neutral float64 // the value that changes nothing
	Detail  string
}

// Lanes lists the clips in display order.
var Lanes = []LaneInfo{
	{LaneBacklight, "Backlight", "%", 1, 100, 100,
		"The screen's real brightness: the laptop's backlight, and external screens through DDC/CI (ddcutil). Saves power and is easiest on the eyes."},
	{LaneSoftware, "Software brightness", "%", 10, 100, 100,
		"Darkens the picture itself through the graphics card. Works on every screen, and goes darker than the backlight can."},
	{LaneKelvin, "Colour temperature", "K", 1000, 10000, 6500,
		"6500 K is neutral. Lower is warmer (less blue light, as by a fire), higher is cooler (like a clear morning sky)."},
}

// FindLane returns the clip with that name.
func FindLane(name string) (LaneInfo, bool) {
	for _, l := range Lanes {
		if l.Name == name {
			return l, true
		}
	}
	return LaneInfo{}, false
}

// Point is one point of a clip. Minute is its time of day (0–1439). Tension
// bends the curve from this point to the next: 0 is straight, positive
// bulges it up and negative down, -1 to 1.
type Point struct {
	Minute  int     `json:"minute"`
	Value   float64 `json:"value"`
	Tension float64 `json:"tension"`
}

// Lane is one clip: whether it is on, and its points in time order.
type Lane struct {
	On     bool    `json:"on"`
	Points []Point `json:"points"`
}

// Wallpaper is shown from From to To (minutes of the day; To before From
// runs past midnight). Image is the stored file's name.
type Wallpaper struct {
	Image string `json:"image"`
	Name  string `json:"name"` // the file's name when it was added, for people
	From  int    `json:"from"`
	To    int    `json:"to"`
}

// Dark modes.
const (
	DarkSystem    = "system"    // KC9K leaves dark mode alone
	DarkLight     = "light"     // always light
	DarkDark      = "dark"      // always dark
	DarkScheduled = "scheduled" // light from LightFrom to LightTo, dark otherwise
)

// Dark is the dark mode setting, like a phone's: light between two times.
type Dark struct {
	Mode      string `json:"mode"`
	LightFrom int    `json:"light_from"`
	LightTo   int    `json:"light_to"`
}

// Config is the whole circadian rhythm.
type Config struct {
	On         bool             `json:"on"`
	Lanes      map[string]*Lane `json:"lanes"`
	Wallpapers []Wallpaper      `json:"wallpapers"`
	Dark       Dark             `json:"dark"`
	// DarkPages: the browser shows light pages dark while dark mode is on.
	DarkPages  bool     `json:"dark_pages"`
	DarkExempt []string `json:"dark_exempt"`
}

// Default is the configuration before the user changed anything: off, with
// the Natural preset's clips ready to switch on.
func Default() Config {
	c := Config{Lanes: map[string]*Lane{}, Wallpapers: []Wallpaper{},
		Dark:       Dark{Mode: DarkSystem, LightFrom: 7 * 60, LightTo: 20 * 60},
		DarkExempt: []string{}}
	p, _ := FindPreset("natural")
	for _, l := range Lanes {
		c.Lanes[l.Name] = &Lane{On: false, Points: append([]Point{}, p.Lanes[l.Name]...)}
	}
	c.Normalize()
	return c
}

// Normalize fills in what is missing and sorts the points, so a config read
// from anywhere is safe to use.
func (c *Config) Normalize() {
	if c.Lanes == nil {
		c.Lanes = map[string]*Lane{}
	}
	for _, info := range Lanes {
		l := c.Lanes[info.Name]
		if l == nil {
			l = &Lane{}
			c.Lanes[info.Name] = l
		}
		l.Points = CleanPoints(info, l.Points)
	}
	for name := range c.Lanes {
		if _, ok := FindLane(name); !ok {
			delete(c.Lanes, name)
		}
	}
	if c.Wallpapers == nil {
		c.Wallpapers = []Wallpaper{}
	}
	for i := range c.Wallpapers {
		c.Wallpapers[i].From = wrapMinute(c.Wallpapers[i].From)
		c.Wallpapers[i].To = wrapMinute(c.Wallpapers[i].To)
	}
	switch c.Dark.Mode {
	case DarkSystem, DarkLight, DarkDark, DarkScheduled:
	default:
		c.Dark.Mode = DarkSystem
	}
	c.Dark.LightFrom = wrapMinute(c.Dark.LightFrom)
	c.Dark.LightTo = wrapMinute(c.Dark.LightTo)
	if c.DarkExempt == nil {
		c.DarkExempt = []string{}
	}
}

// CleanPoints keeps points inside the day and the lane's range, sorted by
// time, one per minute (the last given wins).
func CleanPoints(info LaneInfo, points []Point) []Point {
	byMinute := map[int]Point{}
	for _, p := range points {
		if math.IsNaN(p.Value) || math.IsNaN(p.Tension) {
			continue
		}
		p.Minute = wrapMinute(p.Minute)
		p.Value = clamp(p.Value, info.Min, info.Max)
		p.Tension = clamp(p.Tension, -1, 1)
		byMinute[p.Minute] = p
	}
	out := make([]Point, 0, len(byMinute))
	for _, p := range byMinute {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Minute < out[j].Minute })
	return out
}

func wrapMinute(m int) int {
	m %= DayMinutes
	if m < 0 {
		m += DayMinutes
	}
	return m
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

// MinuteOfDay is t's time of day in minutes, with seconds as a fraction.
func MinuteOfDay(t time.Time) float64 {
	return float64(t.Hour()*60+t.Minute()) + float64(t.Second())/60 + float64(t.Nanosecond())/6e10
}

// Shape is how far (0–1) a curve with tension has gone from one point's
// value to the next at x (0–1) of the way between them. Rising is whether
// the next value is higher: positive tension always bulges the curve up, so
// it hurries up a rise and lingers before a fall.
//
// The curve is x^p, with p chosen so that the middle of the curve is where
// the tension handle sits: at x = 0.5 it has gone m = 0.5 + 0.49·tension of
// the way (for a rise). The interface draws it with the same formula.
func Shape(x, tension float64, rising bool) float64 {
	x = clamp(x, 0, 1)
	if !rising {
		tension = -tension
	}
	m := 0.5 + 0.49*clamp(tension, -1, 1)
	if math.Abs(m-0.5) < 1e-9 {
		return x
	}
	p := math.Log(m) / math.Log(0.5)
	return math.Pow(x, p)
}

// ValueAt is the clip's value at minute of the day (0–1440). A clip without
// points has none (ok false).
func (l Lane) ValueAt(minute float64) (float64, bool) {
	n := len(l.Points)
	if n == 0 {
		return 0, false
	}
	if n == 1 {
		return l.Points[0].Value, true
	}
	minute = math.Mod(minute, DayMinutes)
	if minute < 0 {
		minute += DayMinutes
	}
	// The segment that holds minute: from the last point at or before it,
	// wrapping round to yesterday's last point.
	i := sort.Search(n, func(i int) bool { return float64(l.Points[i].Minute) > minute }) - 1
	var from, to Point
	var start, end float64
	if i < 0 {
		from, to = l.Points[n-1], l.Points[0]
		start, end = float64(from.Minute-DayMinutes), float64(to.Minute)
	} else {
		from = l.Points[i]
		start = float64(from.Minute)
		if i+1 < n {
			to = l.Points[i+1]
			end = float64(to.Minute)
		} else {
			to = l.Points[0]
			end = float64(to.Minute + DayMinutes)
		}
	}
	if end <= start {
		return from.Value, true
	}
	x := (minute - start) / (end - start)
	f := Shape(x, from.Tension, to.Value >= from.Value)
	return from.Value + (to.Value-from.Value)*f, true
}

// NextPointAfter is the first moment after t at which the clip reaches one
// of its points.
func (l Lane) NextPointAfter(t time.Time) (time.Time, bool) {
	if len(l.Points) == 0 {
		return time.Time{}, false
	}
	now := MinuteOfDay(t)
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	for _, p := range l.Points {
		if float64(p.Minute) > now {
			return atMinute(day, p.Minute), true
		}
	}
	return atMinute(day.AddDate(0, 0, 1), l.Points[0].Minute), true
}

func atMinute(day time.Time, minute int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), minute/60, minute%60, 0, 0, day.Location())
}

// Within reports whether minute lies in [from, to), running past midnight
// when to is before from. from == to is the whole day.
func Within(minute float64, from, to int) bool {
	f, e := float64(from), float64(to)
	switch {
	case from == to:
		return true
	case from < to:
		return minute >= f && minute < e
	default:
		return minute >= f || minute < e
	}
}

// WallpaperAt is the index of the wallpaper to show at minute, or -1. Where
// times overlap, the one that started most recently wins, so a short
// wallpaper inside a long one shows during its own time.
func (c Config) WallpaperAt(minute float64) int {
	best, bestAgo := -1, math.Inf(1)
	for i, w := range c.Wallpapers {
		if !Within(minute, w.From, w.To) {
			continue
		}
		ago := math.Mod(minute-float64(w.From)+DayMinutes, DayMinutes)
		if ago < bestAgo {
			best, bestAgo = i, ago
		}
	}
	return best
}

// DarkAt says whether dark mode should be on at minute; decided is false in
// the system mode, where KC9K leaves it alone.
func (c Config) DarkAt(minute float64) (dark, decided bool) {
	switch c.Dark.Mode {
	case DarkLight:
		return false, true
	case DarkDark:
		return true, true
	case DarkScheduled:
		return !Within(minute, c.Dark.LightFrom, c.Dark.LightTo), true
	}
	return false, false
}

// ParseClock reads "HH:MM" (or "24:00", the end of the day) as minutes.
func ParseClock(text string) (int, error) {
	h, m, ok := strings.Cut(strings.TrimSpace(text), ":")
	hours, err1 := strconv.Atoi(h)
	mins, err2 := strconv.Atoi(m)
	if !ok || err1 != nil || err2 != nil || hours < 0 || hours > 24 || mins < 0 || mins > 59 ||
		(hours == 24 && mins != 0) {
		return 0, fmt.Errorf("%q isn't a time of day (use HH:MM)", text)
	}
	return hours*60 + mins, nil
}

// Clock writes minutes as "HH:MM".
func Clock(minute int) string {
	minute = wrapMinute(minute)
	return fmt.Sprintf("%02d:%02d", minute/60, minute%60)
}

// ParsePoints reads points written as "HH:MM=VALUE" or "HH:MM=VALUE~TENSION",
// separated by spaces or commas.
func ParsePoints(info LaneInfo, text string) ([]Point, error) {
	var points []Point
	for _, field := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' }) {
		at, rest, ok := strings.Cut(field, "=")
		if !ok {
			return nil, fmt.Errorf("%q: write a point as HH:MM=VALUE", field)
		}
		minute, err := ParseClock(at)
		if err != nil {
			return nil, err
		}
		value, tension, _ := strings.Cut(rest, "~")
		v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(value, info.Unit), "%"), 64)
		if err != nil || math.IsNaN(v) {
			return nil, fmt.Errorf("%q: %q isn't a number", field, value)
		}
		if v < info.Min || v > info.Max {
			return nil, fmt.Errorf("%q: %s goes from %g to %g %s", field, info.Title, info.Min, info.Max, info.Unit)
		}
		p := Point{Minute: wrapMinute(minute), Value: v}
		if tension != "" {
			t, err := strconv.ParseFloat(tension, 64)
			if err != nil || t < -1 || t > 1 {
				return nil, fmt.Errorf("%q: tension is a number from -1 to 1", field)
			}
			p.Tension = t
		}
		points = append(points, p)
	}
	return CleanPoints(info, points), nil
}

// FormatPoints writes points the way ParsePoints reads them.
func FormatPoints(points []Point) string {
	parts := make([]string, 0, len(points))
	for _, p := range points {
		s := Clock(p.Minute) + "=" + strconv.FormatFloat(math.Round(p.Value*10)/10, 'f', -1, 64)
		if p.Tension != 0 {
			s += "~" + strconv.FormatFloat(math.Round(p.Tension*100)/100, 'f', -1, 64)
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " ")
}
