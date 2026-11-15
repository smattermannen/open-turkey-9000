package quota

import (
	"testing"
	"time"
)

// §3.6: an allowance per day, week or month, refilled at a set time.
func TestPeriods(t *testing.T) {
	at := func(d, h int) time.Time { return time.Date(2026, 11, d, h, 0, 0, 0, time.Local) }
	day := State{}
	if day.PeriodOf(at(16, 1)) != "2026-11-16" {
		t.Error("day")
	}
	early := State{Refill: 4 * 60}
	if early.PeriodOf(at(16, 3)) != "2026-11-15" || early.PeriodOf(at(16, 5)) != "2026-11-16" {
		t.Error("refill at 04:00")
	}
	week := State{Period: "week"}
	if week.PeriodOf(at(16, 9)) != week.PeriodOf(at(22, 9)) || week.PeriodOf(at(16, 9)) == week.PeriodOf(at(23, 9)) {
		t.Error("ISO week: Monday 16th to Sunday 22nd")
	}
	month := State{Period: "month"}
	if month.PeriodOf(at(30, 9)) != "2026-11" {
		t.Error("month")
	}
	s := State{Limit: 3600, Period: "week", Day: week.PeriodOf(at(16, 9)), Used: 1800, PendingLimit: NoPending}
	if got := s.Step(at(20, 9).Unix(), s.PeriodOf(at(20, 9))); got.Used != 1800 {
		t.Error("used reset inside the week")
	}
	if got := s.Step(at(23, 9).Unix(), s.PeriodOf(at(23, 9))); got.Used != 0 {
		t.Error("used kept into the next week")
	}
	if ValidateFor(30*time.Hour, "") == nil || ValidateFor(30*time.Hour, "week") != nil {
		t.Error("limits per period")
	}
}
