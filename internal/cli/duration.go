package cli

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// durationPart is one number and unit of a typed duration: "2d", "4h", "1.5h".
var durationPart = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(w|d|h|m|s|ms|us|µs|ns)`)

// durationUnits are the units a typed duration may use: Go's, plus days and
// weeks (R-GUI-1: never minutes only).
var durationUnits = map[string]time.Duration{
	"w": 7 * 24 * time.Hour, "d": 24 * time.Hour, "h": time.Hour, "m": time.Minute,
	"s": time.Second, "ms": time.Millisecond, "us": time.Microsecond, "µs": time.Microsecond,
	"ns": time.Nanosecond,
}

// parseUserDuration reads a duration typed by the user: Go's form ("90m",
// "1h30m") plus days and weeks ("2d 4h", "1w"), parts optionally separated
// by spaces. A leading "-" makes it negative, so callers' range checks
// still see it; "0" alone is zero.
func parseUserDuration(text string) (time.Duration, error) {
	s := strings.TrimSpace(text)
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(s, "-"), "+"))
	if s == "0" {
		return 0, nil
	}
	if s == "" {
		return 0, fmt.Errorf("time: invalid duration %q", text)
	}
	total := 0.0
	for s != "" {
		m := durationPart.FindStringSubmatch(s)
		if m == nil {
			return 0, fmt.Errorf("time: invalid duration %q (use e.g. 90m, 2h30m, 3d, 1w)", text)
		}
		n, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return 0, fmt.Errorf("time: invalid duration %q", text)
		}
		total += n * float64(durationUnits[m[2]])
		s = strings.TrimSpace(s[len(m[0]):])
	}
	if total >= math.MaxInt64 {
		return 0, fmt.Errorf("time: duration %q is too long", text)
	}
	d := time.Duration(total)
	if negative {
		d = -d
	}
	return d, nil
}

// humanDuration writes d for people, in days, hours and minutes:
// "69 days 10 hours 40 minutes", "5 minutes", "1 day".
func humanDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	minutes := int64(d.Round(time.Minute) / time.Minute)
	if minutes == 0 {
		if d > 0 {
			return d.Round(time.Second).String()
		}
		return "0 minutes"
	}
	days, rest := minutes/(24*60), minutes%(24*60)
	hours, mins := rest/60, rest%60
	var parts []string
	for _, p := range []struct {
		n    int64
		unit string
	}{{days, "day"}, {hours, "hour"}, {mins, "minute"}} {
		if p.n == 1 {
			parts = append(parts, "1 "+p.unit)
		} else if p.n > 1 {
			parts = append(parts, fmt.Sprintf("%d %ss", p.n, p.unit))
		}
	}
	return strings.Join(parts, " ")
}

// untilToday parses an --until value into the time it means:
//   - "HH:MM": that clock time next (today, or tomorrow once passed);
//   - "YYYY-MM-DD HH:MM" (or with a "T"): that date and time;
//   - "YYYY-MM-DD": the start of that day.
//
// A date already passed is refused (R-GUI-2: a timer lock until a date
// picked on a calendar).
func untilToday(now time.Time, clock string) (time.Time, error) {
	text := strings.TrimSpace(clock)
	if strings.Contains(text, "-") {
		for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04", "2006-01-02"} {
			t, err := time.ParseInLocation(layout, text, now.Location())
			if err != nil {
				continue
			}
			if !t.After(now) {
				return time.Time{}, fmt.Errorf("--until %s has already passed", t.Format("2006-01-02 15:04"))
			}
			return t, nil
		}
		return time.Time{}, fmt.Errorf("invalid --until %q (want HH:MM, YYYY-MM-DD or YYYY-MM-DD HH:MM)", clock)
	}
	var h, m int
	var rest string
	if n, _ := fmt.Sscanf(text, "%d:%d%s", &h, &m, &rest); n < 2 || rest != "" {
		return time.Time{}, fmt.Errorf("invalid --until time %q (want HH:MM, YYYY-MM-DD or YYYY-MM-DD HH:MM)", clock)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return time.Time{}, fmt.Errorf("invalid --until time %q", clock)
	}
	t := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	if !t.After(now) {
		t = time.Date(now.Year(), now.Month(), now.Day()+1, h, m, 0, 0, now.Location())
	}
	return t, nil
}
