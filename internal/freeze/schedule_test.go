package freeze

import (
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/schedule"
)

func TestScheduled(t *testing.T) {
	night := schedule.Window{Days: schedule.AllDays, Start: 23 * 60, End: 24 * 60}
	early := schedule.Window{Days: schedule.AllDays, Start: 0, End: 6 * 60}
	lunch := schedule.Window{Days: schedule.Weekdays, Start: 12 * 60, End: 13 * 60}
	windows := []schedule.Window{night, early, lunch}
	modes := []string{ModeLock, ModeShutdown, ModeLogout}
	at := func(day, hour, minute int) time.Time { return time.Date(2026, 11, day, hour, minute, 0, 0, time.Local) }

	until, mode, ok := Scheduled(windows, modes, at(9, 23, 30)) // Monday night
	if !ok || !until.Equal(at(10, 0, 0)) || mode != ModeLock {
		t.Errorf("Monday 23:30: until %v %s %v", until, mode, ok)
	}
	until, mode, ok = Scheduled(windows, modes, at(10, 0, 0)) // midnight: the next window runs on
	if !ok || !until.Equal(at(10, 6, 0)) || mode != ModeShutdown {
		t.Errorf("Tuesday 00:00: until %v %s %v", until, mode, ok)
	}
	if _, _, ok := Scheduled(windows, modes, at(10, 6, 0)); ok {
		t.Error("06:00 is past the end of the early window")
	}
	if _, _, ok := Scheduled(windows, modes, at(14, 12, 30)); ok {
		t.Error("the lunch window is weekdays only, and the 14th is a Saturday")
	}
	until, _, ok = Scheduled(windows, modes, at(13, 12, 30)) // Friday lunch
	if !ok || !until.Equal(at(13, 13, 0)) {
		t.Errorf("Friday 12:30: until %v %v", until, ok)
	}
}
