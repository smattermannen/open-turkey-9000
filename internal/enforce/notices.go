// notices.go — desktop notifications a few minutes before something happens
// to you: a scheduled block starts, a scheduled Frozen Chicken starts, or a
// daily quota runs out.
package enforce

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// NoticeLead is how far ahead notices warn.
const NoticeLead = 5 * time.Minute

// Notice is one notification. Key identifies the event it announces, so it
// is shown once.
type Notice struct {
	Key   string
	Title string
	Body  string
}

// startsWithin returns in how many minutes, within lead, a schedule that is
// off at now comes on, or 0 if it doesn't.
func startsWithin(on func(time.Time) bool, now time.Time, lead time.Duration) int {
	if on(now) {
		return 0
	}
	for m := 1; time.Duration(m)*time.Minute <= lead; m++ {
		if on(now.Add(time.Duration(m) * time.Minute)) {
			return m
		}
	}
	return 0
}

func minutes(m int) string {
	if m == 1 {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", m)
}

// UpcomingNotices lists what is about to happen within NoticeLead. Times
// have minute granularity, so a key names the minute the event happens.
func UpcomingNotices(blockWindows map[string][]schedule.Window, freezeWindows []schedule.Window,
	freezeModes []string, quotaLeft map[string]int64, now time.Time) []Notice {
	var notices []Notice
	at := func(m int) string {
		return now.Truncate(time.Minute).Add(time.Duration(m) * time.Minute).Format("2006-01-02 15:04")
	}

	names := make([]string, 0, len(blockWindows))
	for name := range blockWindows {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		ws := blockWindows[name]
		if m := startsWithin(func(t time.Time) bool { return schedule.Enforced(ws, t) }, now, NoticeLead); m > 0 {
			notices = append(notices, Notice{
				Key:   "block " + name + " " + at(m),
				Title: fmt.Sprintf("'%s' starts in %s", name, minutes(m)),
				Body:  "Its schedule turns the block on then.",
			})
		}
	}

	if m := startsWithin(func(t time.Time) bool {
		for _, w := range freezeWindows {
			if w.ActiveAt(t) {
				return true
			}
		}
		return false
	}, now, NoticeLead); m > 0 {
		mode := "lock"
		for i, w := range freezeWindows {
			if w.ActiveAt(now.Add(time.Duration(m) * time.Minute)) {
				mode = freezeModes[i]
			}
		}
		what := map[string]string{"lock": "The screen will lock.", "logout": "You will be logged out.",
			"shutdown": "The computer will shut down."}[mode]
		key := "freeze "
		if m == 1 { // R-WARN-1: announced again 60 seconds before
			key = "freeze-last "
		}
		notices = append(notices, Notice{
			Key:   key + at(m),
			Title: fmt.Sprintf("Frozen Chicken starts in %s", minutes(m)),
			Body:  what + " Save your work.",
		})
	}

	quotaNames := make([]string, 0, len(quotaLeft))
	for name := range quotaLeft {
		quotaNames = append(quotaNames, name)
	}
	sort.Strings(quotaNames)
	for _, name := range quotaNames {
		left := quotaLeft[name]
		if left > 0 && left <= int64(NoticeLead/time.Second) {
			notices = append(notices, Notice{
				Key:   "allowance " + name + " " + now.Format("2006-01-02"),
				Title: fmt.Sprintf("'%s': %s of today's allowance left", name, minutes(int((left+59)/60))),
				Body:  "The block turns back on when it's spent.",
			})
		}
	}
	return notices
}

// noticeLog remembers which notices were shown, so each shows once.
var noticeLog = struct {
	sync.Mutex
	shown map[string]time.Time
}{shown: map[string]time.Time{}}

// sendNotices works out what is coming up and shows each notice once.
func sendNotices(d *db.DB, now time.Time) {
	blockWindows := map[string][]schedule.Window{}
	if schedules, err := d.AllSchedules(); err == nil {
		for _, s := range schedules {
			blockWindows[s.BlockName] = append(blockWindows[s.BlockName], s.Window())
		}
	}
	var freezeWindows []schedule.Window
	var freezeModes []string
	if schedules, err := d.ListFreezeSchedules(); err == nil {
		for _, s := range schedules {
			freezeWindows = append(freezeWindows, s.Window)
			freezeModes = append(freezeModes, s.Mode)
		}
	}
	quotaLeft := map[string]int64{}
	if names, err := d.QuotaBlocks(); err == nil {
		for _, name := range names {
			if s, err := d.GetQuota(name); err == nil && s != nil && s.Open {
				quotaLeft[name] = s.Remaining()
			}
		}
	}

	notices := UpcomingNotices(blockWindows, freezeWindows, freezeModes, quotaLeft, now)
	noticeLog.Lock()
	defer noticeLog.Unlock()
	for key, when := range noticeLog.shown {
		if now.Sub(when) > 24*time.Hour {
			delete(noticeLog.shown, key)
		}
	}
	for _, n := range notices {
		if _, done := noticeLog.shown[n.Key]; done {
			continue
		}
		noticeLog.shown[n.Key] = now
		blocker.NotifyDesktop(n.Title, n.Body)
	}
}
