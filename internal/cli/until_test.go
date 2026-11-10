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
