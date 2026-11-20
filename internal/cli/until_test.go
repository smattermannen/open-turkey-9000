package cli

import (
	"testing"
	"time"
)

// --until HH:MM means that clock time, today or tomorrow, also across the
// nights the clocks change (Europe/Stockholm 2026: 29 March, 25 October).
func TestUntilTodayWhenClocksChange(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Stockholm")
	if err != nil {
		t.Skip(err)
	}
	cases := []struct {
		now   time.Time
		clock string
		want  string
		hours float64 // real hours from now
	}{
		{time.Date(2026, 6, 14, 22, 0, 0, 0, loc), "07:00", "2026-06-15 07:00", 9},
		{time.Date(2026, 6, 14, 6, 0, 0, 0, loc), "07:00", "2026-06-14 07:00", 1},
		{time.Date(2026, 3, 28, 22, 0, 0, 0, loc), "07:00", "2026-03-29 07:00", 8},   // a 23-hour day
		{time.Date(2026, 10, 24, 22, 0, 0, 0, loc), "07:00", "2026-10-25 07:00", 10}, // a 25-hour day
		{time.Date(2026, 3, 29, 1, 0, 0, 0, loc), "05:00", "2026-03-29 05:00", 3},
		{time.Date(2026, 10, 25, 1, 0, 0, 0, loc), "05:00", "2026-10-25 05:00", 5},
		// A time inside the skipped hour lands just after it.
		{time.Date(2026, 3, 29, 1, 0, 0, 0, loc), "02:30", "2026-03-29 03:30", 1.5},
	}
	for _, c := range cases {
		got, err := untilToday(c.now, c.clock)
		if err != nil {
			t.Fatal(err)
		}
		if got.Format("2006-01-02 15:04") != c.want || got.Sub(c.now).Hours() != c.hours {
			t.Errorf("at %s, --until %s = %s (%.1fh on); want %s (%.1fh)", c.now.Format("2006-01-02 15:04"),
				c.clock, got.Format("2006-01-02 15:04"), got.Sub(c.now).Hours(), c.want, c.hours)
		}
	}
}

// R-GUI-2: --until also takes a date picked on a calendar, as far away as
// wanted (SPEC §3.4: timer locks have no upper limit); a passed date is
// refused, never turned into "tomorrow".
func TestUntilDates(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Stockholm")
	if err != nil {
		t.Skip(err)
	}
	now := time.Date(2026, 11, 15, 22, 0, 0, 0, loc)
	for spec, want := range map[string]string{
		"2099-12-24 18:00":   "2099-12-24 18:00",
		"2099-12-24T18:00":   "2099-12-24 18:00",
		"2099-12-24":         "2099-12-24 00:00",
		" 2026-11-16 07:30 ": "2026-11-16 07:30",
		"2026-11-15 22:01":   "2026-11-15 22:01",
	} {
		got, err := untilToday(now, spec)
		if err != nil || got.Format("2006-01-02 15:04") != want {
			t.Errorf("--until %q = %v, %v; want %s", spec, got, err, want)
		}
	}
	for _, spec := range []string{"2026-11-15 22:00", "2020-01-01", "2026-13-01 10:00", "2026-11-31",
		"24/12/2099", "07:00x", "25:00", "7", "", "tomorrow"} {
		if got, err := untilToday(now, spec); err == nil {
			t.Errorf("--until %q accepted as %v", spec, got)
		}
	}
}

// R-GUI-1: typed durations take days and weeks, with or without spaces.
func TestParseUserDuration(t *testing.T) {
	for text, want := range map[string]time.Duration{
		"90m": 90 * time.Minute, "2h30m": 150 * time.Minute, "3d": 72 * time.Hour,
		"2d 4h": 52 * time.Hour, "1w": 168 * time.Hour, "1w 2d 3h 4m": (9*24+3)*time.Hour + 4*time.Minute,
		"100000m": 100000 * time.Minute, "1.5h": 90 * time.Minute, "0": 0, "-5m": -5 * time.Minute,
		"30s": 30 * time.Second, " 45m ": 45 * time.Minute, "638000h": 638000 * time.Hour,
	} {
		if got, err := parseUserDuration(text); err != nil || got != want {
			t.Errorf("%q = %v, %v; want %v", text, got, err, want)
		}
	}
	for _, text := range []string{"", "5", "abc", "5x", "d", "2d 4", "1e3m", "999999999w", "5m-", "--5m"} {
		if got, err := parseUserDuration(text); err == nil {
			t.Errorf("%q accepted as %v", text, got)
		}
	}
	for d, want := range map[time.Duration]string{
		100000 * time.Minute: "69 days 10 hours 40 minutes", time.Minute: "1 minute",
		25 * time.Hour: "1 day 1 hour", 0: "0 minutes", 48 * time.Hour: "2 days",
	} {
		if got := humanDuration(d); got != want {
			t.Errorf("humanDuration(%v) = %q; want %q", d, got, want)
		}
	}
}

// The owner's case: a timer lock of 100000 minutes and one until a date in
// 2099 are both set as asked, on blocks and on setting locks.
func TestLongTimerLocks(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "start", "b", "--for", "100000m", "--yes")
	detail, _ := d.GetBlock("b")
	trusted, _ := d.TrustedNow(time.Now())
	if left := detail.LockedUntil - trusted; left < 100000*60-120 || left > 100000*60+5 {
		t.Errorf("--for 100000m locked for %d s", left)
	}
	// A second timer lock until 2099, after the first (§4.6).
	mustRun(t, "start", "b", "--until", "2099-12-24 18:00", "--yes")
	extras, _ := d.GetLockExtras("b")
	if len(extras.Queue) != 1 || time.Unix(extras.Queue[0].Until, 0).Year() != 2099 {
		t.Errorf("--until 2099-12-24: queue %+v", extras.Queue)
	}

	mustRun(t, "browser", "safesearch", "on", "--until", "2099-12-24", "--yes")
	if err := runCLI(t, "browser", "safesearch", "off"); err == nil {
		t.Error("a setting locked until 2099 was turned off")
	}
	if err := runCLI(t, "browser", "web-only", "on", "--until", "2020-01-01 10:00", "--yes"); err == nil {
		t.Error("a setting lock until a passed date was accepted")
	}
}
