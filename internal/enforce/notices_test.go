package enforce

import (
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/schedule"
)

func TestUpcomingNotices(t *testing.T) {
	work := schedule.Window{Days: schedule.AllDays, Start: 9 * 60, End: 17 * 60}
	gamesAllowed := schedule.Window{Days: schedule.AllDays, Start: 18 * 60, End: 20 * 60, Allow: true}
	night := schedule.Window{Days: schedule.AllDays, Start: 23 * 60, End: 24 * 60}
	blocks := map[string][]schedule.Window{"work": {work}, "games": {gamesAllowed}}
	at := func(h, m int) time.Time { return time.Date(2026, 11, 9, h, m, 0, 0, time.Local) }

	titles := func(ns []Notice) string {
		var out []string
		for _, n := range ns {
			out = append(out, n.Title)
		}
		return strings.Join(out, " | ")
	}

	got := UpcomingNotices(blocks, nil, nil, nil, at(8, 57))
	if titles(got) != "'work' starts in 3 minutes" {
		t.Errorf("08:57: %s", titles(got))
	}
	if got := UpcomingNotices(blocks, nil, nil, nil, at(8, 50)); len(got) != 0 {
		t.Errorf("08:50 is more than 5 minutes ahead: %s", titles(got))
	}
	if got := UpcomingNotices(blocks, nil, nil, nil, at(10, 0)); len(got) != 0 {
		t.Errorf("an open window isn't news: %s", titles(got))
	}
	// An allow window ending turns the block back on.
	if got := UpcomingNotices(blocks, nil, nil, nil, at(19, 59)); titles(got) != "'games' starts in 1 minute" {
		t.Errorf("19:59: %s", titles(got))
	}

	freeze := UpcomingNotices(nil, []schedule.Window{night}, []string{"shutdown"}, nil, at(22, 55))
	if len(freeze) != 1 || freeze[0].Title != "Frozen Chicken starts in 5 minutes" || !strings.Contains(freeze[0].Body, "shut down") {
		t.Errorf("22:55 freeze notice: %+v", freeze)
	}

	quota := UpcomingNotices(nil, nil, nil, map[string]int64{"social": 240, "news": 900}, at(12, 0))
	if titles(quota) != "'social': 4 minutes of today's quota left" {
		t.Errorf("quota notices: %s", titles(quota))
	}

	// The key names the event, so the same event a minute later is the same notice.
	a := UpcomingNotices(blocks, nil, nil, nil, at(8, 56))
	b := UpcomingNotices(blocks, nil, nil, nil, at(8, 58))
	if len(a) != 1 || len(b) != 1 || a[0].Key != b[0].Key {
		t.Errorf("keys differ for one event: %+v %+v", a, b)
	}
}
