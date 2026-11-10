package freeze

import (
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/schedule"
)

// A scheduled freeze ends at its window's end on the clock, also on the days
// the clocks change (Europe/Stockholm 2026: 29 March and 25 October).
func TestWindowEndWhenClocksChange(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Stockholm")
	if err != nil {
		t.Skip(err)
	}
	evening := schedule.Window{Days: schedule.Sunday, Start: 20 * 60, End: 23 * 60}
	night := schedule.Window{Days: schedule.Sunday, Start: 22 * 60, End: 24 * 60}
	for _, day := range []time.Time{
		time.Date(2026, 3, 29, 21, 0, 0, 0, loc),
		time.Date(2026, 10, 25, 21, 0, 0, 0, loc),
		time.Date(2026, 6, 14, 21, 0, 0, 0, loc),
	} {
		if got := WindowEnd(evening, day).Format("2006-01-02 15:04"); got != day.Format("2006-01-02")+" 23:00" {
			t.Errorf("%s: 20:00-23:00 ends %s", day.Format("2006-01-02"), got)
		}
		next := day.AddDate(0, 0, 1).Format("2006-01-02") + " 00:00"
		if got := WindowEnd(night, day.Add(90*time.Minute)).Format("2006-01-02 15:04"); got != next {
			t.Errorf("%s: 22:00-24:00 ends %s; want %s", day.Format("2006-01-02"), got, next)
		}
		until, _, ok := Scheduled([]schedule.Window{evening}, []string{ModeLock}, day)
		if !ok || until.Sub(day) != 2*time.Hour {
			t.Errorf("%s: at 21:00 the freeze runs %v more; want 2h", day.Format("2006-01-02"), until.Sub(day))
		}
	}
}
