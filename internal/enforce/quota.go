package enforce

import (
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

// noteQuotaApps adds seconds to the open sessions of blocks whose apps are
// in the foreground.
func noteQuotaApps(d *db.DB, foreground []string, seconds int64) {
	if len(foreground) == 0 {
		return
	}
	for _, b := range openQuotaBlocks(d) {
		for _, app := range b.Apps {
			if slices.Contains(foreground, app) {
				addQuotaUse(b.Name, seconds)
				break
			}
		}
	}
}

// reconcileQuotas spends open sessions and keeps each quota block on, except
// during a session. A lock or an enforcing schedule always beats a session.
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
		s := state.Spend(spent).Step(now.Unix(), quota.DayOf(now))
		extras, err := d.GetLockExtras(name)
		if err != nil {
			log.Printf("daemon: allowance: %v", err)
			continue
		}
		boot, _ := db.BootID()
		locked := BlockLocked(detail, extras, trusted, boot)

		if s.Limit == 0 {
			if err := d.DeleteQuota(name); err != nil {
				log.Printf("daemon: allowance: %v", err)
			}
			if detail.Active && detail.Scheduled && !locked {
				if err := d.DeactivateBlock(name); err != nil {
					log.Printf("daemon: allowance: %v", err)
				}
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
			continue
		}

		switch {
		case s.Open && detail.Active:
			err = d.DeactivateBlock(name)
		case !s.Open && !detail.Active:
			err = d.ActivateBlock(name, false, 0, 0, true)
		default:
			err = nil
		}
		if err != nil {
			log.Printf("daemon: allowance: %v", err)
		}
	}
	return nil
}
