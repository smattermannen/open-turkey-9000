// notices.go — desktop notifications a few minutes before something happens
// to you (§4.10, the user chooses which and how far ahead): a block's window
// starts or ends, a timer lock ends, a Pomodoro period changes, an allowance
// runs out, an app is closed; a scheduled Frozen Chicken is always announced.
package enforce

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/quota"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// NoticeLead is how far ahead notices warn by default.
const NoticeLead = 5 * time.Minute

// The notifications the user can choose (§4.10); all on by default.
// Frozen Chicken is always announced (R-WARN-1).
const (
	NoticeWindowStart = "window-start"
	NoticeWindowEnd   = "window-end"
	NoticeTimerEnd    = "timer-end"
	NoticePomodoro    = "pomodoro"
	NoticeAllowance   = "allowance"
	NoticeAppBlocked  = "app-blocked"
)

// NoticeKinds lists them in display order.
var NoticeKinds = []string{NoticeWindowStart, NoticeWindowEnd, NoticeTimerEnd, NoticePomodoro, NoticeAllowance, NoticeAppBlocked}

// SettingNotices stores NoticeOptions as JSON.
const SettingNotices = "notifications"

// NoticeOptions are the user's notification choices: kinds turned off and
// how many minutes ahead to warn (each advance once).
type NoticeOptions struct {
	Off     []string `json:"off,omitempty"`
	Advance []int    `json:"advance"`
}

// DefaultNoticeOptions: everything on, 5 minutes ahead.
func DefaultNoticeOptions() NoticeOptions { return NoticeOptions{Advance: []int{5}} }

// On reports whether a kind of notification is wanted.
func (o NoticeOptions) On(kind string) bool {
	for _, k := range o.Off {
		if k == kind {
			return false
		}
	}
	return true
}

// lead is the furthest advance.
func (o NoticeOptions) lead() time.Duration {
	m := 1
	for _, a := range o.Advance {
		m = max(m, a)
	}
	return time.Duration(m) * time.Minute
}

// advanceFor is the smallest advance at least m minutes, or 0 when the
// event is further away than every advance.
func (o NoticeOptions) advanceFor(m int) int {
	best := 0
	for _, a := range o.Advance {
		if a >= m && (best == 0 || a < best) {
			best = a
		}
	}
	return best
}

// LoadNoticeOptions reads the choices, or the defaults.
func LoadNoticeOptions(d *db.DB) NoticeOptions {
	o := DefaultNoticeOptions()
	if v, err := d.GetSetting(SettingNotices); err == nil && v != "" {
		var got NoticeOptions
		if json.Unmarshal([]byte(v), &got) == nil && len(got.Advance) > 0 {
			o = got
		}
	}
	return o
}

// SaveNoticeOptions stores the choices.
func SaveNoticeOptions(d *db.DB, o NoticeOptions) error {
	data, err := json.Marshal(o)
	if err != nil {
		return err
	}
	return d.SetSetting(SettingNotices, string(data))
}

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

// NoticeInput is what notices are worked out from: the windows of blocks
// that are on, scheduled freezes, open allowance sessions (seconds left),
// timer locks (when each ends, unix seconds) and Pomodoro cycles of blocks
// that are on.
type NoticeInput struct {
	BlockWindows  map[string][]schedule.Window
	FreezeWindows []schedule.Window
	FreezeModes   []string
	QuotaLeft     map[string]int64
	TimerEnds     map[string]int64
	Pomodoros     map[string]quota.Cycle
	// MonkWindows: monk mode's windows while its automation is on
	// (§3.8): announced as a scheduled freeze is, 5 and 1 minutes ahead.
	MonkWindows []schedule.Window
}

// UpcomingNotices lists what is about to happen within NoticeLead, with
// every notification on (see Notices).
func UpcomingNotices(blockWindows map[string][]schedule.Window, freezeWindows []schedule.Window,
	freezeModes []string, quotaLeft map[string]int64, now time.Time) []Notice {
	return Notices(NoticeInput{BlockWindows: blockWindows, FreezeWindows: freezeWindows,
		FreezeModes: freezeModes, QuotaLeft: quotaLeft}, DefaultNoticeOptions(), now)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Notices lists what is about to happen, as the options ask. Times have
// minute granularity, so a key names the minute the event happens and the
// advance it is announced at.
func Notices(in NoticeInput, o NoticeOptions, now time.Time) []Notice {
	var notices []Notice
	at := func(m int) string {
		return now.Truncate(time.Minute).Add(time.Duration(m) * time.Minute).Format("2006-01-02 15:04")
	}
	lead := o.lead()
	// ahead adds a notice for an event m minutes away, at its advance.
	ahead := func(kind, key string, m int, title, body string) {
		if !o.On(kind) || m <= 0 {
			return
		}
		if a := o.advanceFor(m); a > 0 {
			notices = append(notices, Notice{Key: fmt.Sprintf("%s %s %s +%d", kind, key, at(m), a), Title: title, Body: body})
		}
	}

	for _, name := range sortedKeys(in.BlockWindows) {
		ws := in.BlockWindows[name]
		enforced := func(t time.Time) bool { return schedule.Enforced(ws, t) }
		if m := startsWithin(enforced, now, lead); m > 0 {
			ahead(NoticeWindowStart, name, m, fmt.Sprintf("'%s' starts in %s", name, minutes(m)), "Its schedule enables the block then.")
		}
		if m := startsWithin(func(t time.Time) bool { return !enforced(t) }, now, lead); m > 0 {
			ahead(NoticeWindowEnd, name, m, fmt.Sprintf("'%s' ends in %s", name, minutes(m)), "Its window closes then.")
		}
	}

	if m := startsWithin(func(t time.Time) bool {
		for _, w := range in.FreezeWindows {
			if w.ActiveAt(t) {
				return true
			}
		}
		return false
	}, now, max(lead, NoticeLead)); m > 0 && m <= 5 {
		mode := "lock"
		for i, w := range in.FreezeWindows {
			if w.ActiveAt(now.Add(time.Duration(m) * time.Minute)) {
				mode = in.FreezeModes[i]
			}
		}
		what := map[string]string{"lock": "The screen will lock.", "logout": "You will be logged out.",
			"shutdown": "The computer will shut down.", "apps": "Only the allowed apps will run."}[mode]
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

	if m := startsWithin(func(t time.Time) bool { return schedule.InAny(in.MonkWindows, t) }, now, max(lead, NoticeLead)); m > 0 && m <= 5 {
		key := "monk "
		if m == 1 {
			key = "monk-last "
		}
		notices = append(notices, Notice{
			Key:   key + at(m),
			Title: fmt.Sprintf("Monk mode starts in %s", minutes(m)),
			Body:  "Every website is blocked and the internet connection is cut until its window ends.",
		})
	}

	for _, name := range sortedKeys(in.QuotaLeft) {
		left := in.QuotaLeft[name]
		if left > 0 && o.On(NoticeAllowance) {
			m := int((left + 59) / 60)
			if a := o.advanceFor(m); a > 0 {
				notices = append(notices, Notice{
					Key:   fmt.Sprintf("allowance %s %s +%d", name, now.Format("2006-01-02"), a),
					Title: fmt.Sprintf("'%s': %s of today's allowance left", name, minutes(m)),
					Body:  "The block is enabled again when it's spent.",
				})
			}
		}
	}

	for _, name := range sortedKeys(in.TimerEnds) {
		left := in.TimerEnds[name] - now.Unix()
		if left > 0 {
			m := int((left + 59) / 60)
			ahead(NoticeTimerEnd, name, m, fmt.Sprintf("The timer lock of '%s' ends in %s", name, minutes(m)), "The block stays on; it can be switched off then.")
		}
	}

	for _, name := range sortedKeys(in.Pomodoros) {
		c := in.Pomodoros[name]
		rest := func(t time.Time) bool { return c.OnBreak(t.Unix()) }
		if m := startsWithin(rest, now, lead); m > 0 {
			ahead(NoticePomodoro, name, m, fmt.Sprintf("'%s': rest in %s", name, minutes(m)), "Pomodoro: the work period ends then.")
		}
		if m := startsWithin(func(t time.Time) bool { return !rest(t) }, now, lead); m > 0 {
			ahead(NoticePomodoro, name, m, fmt.Sprintf("'%s': work in %s", name, minutes(m)), "Pomodoro: the rest ends then.")
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
	in := NoticeInput{BlockWindows: map[string][]schedule.Window{}, QuotaLeft: map[string]int64{},
		TimerEnds: map[string]int64{}, Pomodoros: map[string]quota.Cycle{}}
	active := map[string]bool{}
	if blocks, err := d.GetActiveBlocks(); err == nil {
		trusted, _ := d.TrustedNow(now)
		for _, b := range blocks {
			active[b.BlockName] = true
			if b.LockedUntil > trusted && trusted > 0 {
				in.TimerEnds[b.BlockName] = now.Unix() + b.LockedUntil - trusted
			}
			if p, err := d.GetPomodoro(b.BlockName); err == nil && p != nil {
				in.Pomodoros[b.BlockName] = p.Cycle()
			}
		}
	}
	// R-SCHED-1: a window only enables a block that is on.
	if schedules, err := d.AllSchedules(); err == nil {
		for _, s := range schedules {
			if active[s.BlockName] {
				in.BlockWindows[s.BlockName] = append(in.BlockWindows[s.BlockName], s.Window())
			}
		}
	}
	if schedules, err := d.ListFreezeSchedules(); err == nil {
		for _, s := range schedules {
			in.FreezeWindows = append(in.FreezeWindows, s.Window)
			in.FreezeModes = append(in.FreezeModes, s.Mode)
		}
	}
	// Monk mode's windows, switched on, when they start it by themselves
	// (a place, which can't be foreseen, may be needed too).
	if m, a, err := MonkAutomation(d); err == nil && m.Active && len(a.Windows) > 0 && (len(a.Places) == 0 || !a.NeedBoth) {
		in.MonkWindows = a.Windows
	}
	if names, err := d.QuotaBlocks(); err == nil {
		for _, name := range names {
			if s, err := d.GetQuota(name); err == nil && s != nil && s.Open {
				in.QuotaLeft[name] = s.Remaining()
			}
		}
	}

	notices := Notices(in, LoadNoticeOptions(d), now)
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

// noticeOnce shows a notification once per key (at most once a minute when
// the key names the minute).
func noticeOnce(key, title, body string) {
	noticeLog.Lock()
	defer noticeLog.Unlock()
	if _, done := noticeLog.shown[key]; done {
		return
	}
	noticeLog.shown[key] = time.Now()
	blocker.NotifyDesktop(title, body)
}
