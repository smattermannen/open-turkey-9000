package enforce

import (
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/quota"
)

// quotaUse collects, per block, the seconds its sites or apps were in use
// since the daemon last spent them: the extension reports websites (tabs in
// front, or playing sound or video), the foreground sampler reports apps.
var quotaUse = struct {
	sync.Mutex
	seconds map[string]int64
}{seconds: map[string]int64{}}

func addQuotaUse(block string, seconds int64) {
	quotaUse.Lock()
	defer quotaUse.Unlock()
	quotaUse.seconds[block] += seconds
}

func takeQuotaUse(block string) int64 {
	quotaUse.Lock()
	defer quotaUse.Unlock()
	seconds := quotaUse.seconds[block]
	delete(quotaUse.seconds, block)
	return seconds
}

// openQuotaBlocks returns the blocks with a quota session open now.
func openQuotaBlocks(d *db.DB) []*db.BlockDetail {
	names, err := d.QuotaBlocks()
	if err != nil {
		log.Printf("allowance: %v", err)
		return nil
	}
	var out []*db.BlockDetail
	for _, name := range names {
		state, err := d.GetQuota(name)
		if err != nil || state == nil || !state.Open {
			continue
		}
		if detail, err := d.GetBlock(name); err == nil {
			out = append(out, detail)
		}
	}
	return out
}

// countsByUse reports whether a block's session can be spent by use: its
// websites need the extension to be seen, which needs a browser that runs
// it. Without one, a session with websites spends all of its time.
func countsByUse(detail *db.BlockDetail) bool {
	return !detail.Lists.Enforces() || blocker.ExtensionBrowserInstalled()
}

// SettingStrictUse is strict use counting (§3.7): "on" counts background
// tabs and apps that aren't in front (minimised) as use.
const SettingStrictUse = "use.strict"

// StrictUseOn reports whether strict use counting is on.
func StrictUseOn(d *db.DB) bool {
	v, err := d.GetSetting(SettingStrictUse)
	return err == nil && v == "on"
}

// noteQuotaApps adds seconds to the open sessions of blocks whose apps are
// in the foreground, or with strict use counting running at all.
func noteQuotaApps(d *db.DB, foreground []string, seconds int64) {
	open := openQuotaBlocks(d)
	if len(open) == 0 {
		return
	}
	if StrictUseOn(d) {
		var apps []string
		for _, b := range open {
			apps = append(apps, b.Apps...)
		}
		foreground = append(foreground, blocker.RunningApps(apps)...)
	}
	if len(foreground) == 0 {
		return
	}
	for _, b := range open {
		for _, app := range b.Apps {
			if slices.Contains(foreground, app) {
				addQuotaUse(b.Name, seconds)
				break
			}
		}
	}
}

// reconcileQuotas spends open sessions and closes those a lock, an open
// block window or a Pomodoro work period beats. It never switches a block
// on or off (§3.3): a session only lifts its block.
func reconcileQuotas(d *db.DB, now time.Time, trusted int64) error {
	names, err := d.QuotaBlocks()
	if err != nil {
		return err
	}
	for _, name := range names {
		state, err := d.GetQuota(name)
		if err != nil {
			log.Printf("daemon: allowance: %v", err)
			continue
		}
		if state == nil {
			continue
		}
		detail, err := d.GetBlock(name)
		if err != nil {
			log.Printf("daemon: allowance: %v", err)
			continue
		}
		spent := takeQuotaUse(name)
		if !countsByUse(detail) {
			spent = state.Elapsed(now.Unix())
		}
		s := state.Spend(spent).Step(now.Unix(), QuotaPeriod(d, name, *state, now))
		if spent > 0 && countsByUse(detail) {
			s.LastUse = now.Unix()
		}
		s = s.Idle(now.Unix()) // §3.6: a limited session ends 3 min after switching away
		extras, err := d.GetLockExtras(name)
		if err != nil {
			log.Printf("daemon: allowance: %v", err)
			continue
		}
		boot, _ := db.BootID()
		locked := BlockLocked(detail, extras, trusted, boot)

		if s.Limit == 0 { // a removal older versions queued for the refill
			if err := d.DeleteQuota(name); err != nil {
				log.Printf("daemon: allowance: %v", err)
			}
			continue
		}

		scheduled, err := ScheduleActiveNow(d, name, now)
		if err != nil {
			log.Printf("daemon: allowance: %v", err)
		}
		if locked || scheduled || PomodoroWorking(d, name) {
			s.Open = false
		}

		saved, err := d.UpdateQuotaIfUnchanged(name, *state, s)
		if err != nil {
			log.Printf("daemon: allowance: %v", err)
			continue
		}
		if !saved {
			addQuotaUse(name, spent) // the CLI changed it; spend this next cycle
		}
	}
	return nil
}

// QuotaPeriod is the period a block's allowance is in at now (§3.6). For
// an allowance per window it is the opening of the block window open now
// (its date and start); outside windows the period stays, so nothing
// refills until the next window opens.
func QuotaPeriod(d *db.DB, name string, q quota.State, now time.Time) string {
	if q.Period != "window" {
		return q.PeriodOf(now)
	}
	if schedules, err := d.ListSchedules(name); err == nil {
		for _, s := range schedules {
			if w := s.Window(); !w.Allow && w.ActiveAt(now) {
				return fmt.Sprintf("%s %d", now.Format("2006-01-02"), w.Start)
			}
		}
	}
	if q.Day == "" {
		return "outside"
	}
	return q.Day
}
