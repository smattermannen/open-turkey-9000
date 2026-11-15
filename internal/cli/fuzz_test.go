package cli

import (
	"strings"
	"testing"
	"time"
)

// Typed durations never panic, and a duration written back in Go's form
// reads the same.
func FuzzParseUserDuration(f *testing.F) {
	for _, seed := range []string{"90m", "2d 4h", "1w", "0", "-5m", "1.5h", "100000m", "", "5", "1e3m", "9999999999w"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		d, err := parseUserDuration(text)
		if err != nil {
			return
		}
		if d >= 0 && d%time.Second == 0 {
			back, err := parseUserDuration(d.String())
			if err != nil || back != d {
				t.Errorf("%q = %v; written %q reads %v, %v", text, d, d.String(), back, err)
			}
		}
		_ = humanDuration(d)
	})
}

// --until never panics, and what it accepts is in the future.
func FuzzUntil(f *testing.F) {
	for _, seed := range []string{"07:00", "2099-12-24 18:00", "2099-12-24", "25:00", "2026-02-30", "7:5", "-1:00"} {
		f.Add(seed)
	}
	now := time.Date(2026, 11, 15, 22, 0, 0, 0, time.UTC)
	f.Fuzz(func(t *testing.T, text string) {
		got, err := untilToday(now, text)
		if err == nil && !got.After(now) {
			t.Errorf("%q accepted as %v, not after %v", text, got, now)
		}
	})
}

// Range locks never panic; what is accepted has two different times.
func FuzzParseRange(f *testing.F) {
	for _, seed := range []string{"mon-fri 09:00-17:00", "daily 22:00-06:00", "sat,sun 00:00-24:00", "x", "mon 1-2"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		days, from, to, err := parseRange(text)
		if err != nil {
			return
		}
		if days == 0 || from == to || from < 0 || to < 0 || from >= 24*60 || to >= 24*60 {
			t.Errorf("%q accepted as days %b %d-%d", text, days, from, to)
		}
		if strings.TrimSpace(text) == "" {
			t.Errorf("empty range accepted")
		}
	})
}
