package enforce

import (
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/quota"
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
	if titles(quota) != "'social': 4 minutes of today's allowance left" {
		t.Errorf("quota notices: %s", titles(quota))
	}

	// The key names the event, so the same event a minute later is the same notice.
	a := UpcomingNotices(blocks, nil, nil, nil, at(8, 56))
	b := UpcomingNotices(blocks, nil, nil, nil, at(8, 58))
	if len(a) != 1 || len(b) != 1 || a[0].Key != b[0].Key {
		t.Errorf("keys differ for one event: %+v %+v", a, b)
	}
}

// §4.10: the user's choices: kinds off, several advances (each once), and
// the new events: window end, timer lock end, Pomodoro change.
func TestNoticeChoices(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 11, 16, h, m, 0, 0, time.Local) }
	w := []schedule.Window{{Days: schedule.AllDays, Start: 9 * 60, End: 10 * 60}}
	in := NoticeInput{BlockWindows: map[string][]schedule.Window{"b": w},
		TimerEnds: map[string]int64{"t": at(12, 3).Unix()},
		Pomodoros: map[string]quota.Cycle{"p": {On: 25 * 60, Off: 5 * 60, Anchor: at(13, 0).Unix()}}}
	o := NoticeOptions{Advance: []int{2, 5}}
	keys := func(ns []Notice) string {
		var out []string
		for _, n := range ns {
			out = append(out, n.Title)
		}
		return strings.Join(out, " | ")
	}
	if got := keys(Notices(in, o, at(8, 56))); got != "'b' starts in 4 minutes" {
		t.Errorf("8:56: %q", got)
	}
	a, b := Notices(in, o, at(8, 56)), Notices(in, o, at(8, 58))
	if a[0].Key == b[0].Key {
		t.Error("the 2-minute warning has the same key as the 5-minute one")
	}
	if got := keys(Notices(in, o, at(9, 57))); got != "'b' ends in 3 minutes" {
		t.Errorf("9:57: %q", got)
	}
	if got := keys(Notices(in, o, at(12, 1))); got != "The timer lock of 't' ends in 2 minutes" {
		t.Errorf("12:01: %q", got)
	}
	if got := keys(Notices(in, o, at(13, 22))); got != "'p': rest in 3 minutes" {
		t.Errorf("13:22: %q", got)
	}
	o.Off = []string{NoticeWindowStart}
	if got := Notices(in, o, at(8, 56)); len(got) != 0 {
		t.Errorf("window-start off: %q", keys(got))
	}
}

// §3.8: monk mode run by its windows is announced as a scheduled freeze
// is: 5 minutes and 1 minute ahead (owner 2026-11-17: it cuts the network).
func TestMonkNotices(t *testing.T) {
	now := time.Date(2026, 11, 17, 17, 55, 0, 0, time.Local)
	day := 1 << uint(now.Weekday())
	in := NoticeInput{MonkWindows: []schedule.Window{{Days: day, Start: 18 * 60, End: 20 * 60}}}
	got := Notices(in, DefaultNoticeOptions(), now)
	if len(got) != 1 || got[0].Title != "Monk mode starts in 5 minutes" {
		t.Fatalf("5 minutes ahead: %+v", got)
	}
	got = Notices(in, DefaultNoticeOptions(), now.Add(4*time.Minute))
	if len(got) != 1 || got[0].Title != "Monk mode starts in 1 minute" || got[0].Key == "" {
		t.Fatalf("1 minute ahead: %+v", got)
	}
	if got := Notices(in, DefaultNoticeOptions(), now.Add(-30*time.Minute)); len(got) != 0 {
		t.Errorf("half an hour ahead: %+v", got)
	}
}
