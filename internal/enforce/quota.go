package enforce

import (
	"log"
	"time"

	"github.com/brunodcdo/open-turkey/internal/db"
	"github.com/brunodcdo/open-turkey/internal/quota"
)

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
			log.Printf("daemon: quota: %v", err)
			continue
		}
		if state == nil {
			continue
		}
		s := state.Step(now.Unix(), quota.DayOf(now))

		detail, err := d.GetBlock(name)
		if err != nil {
			log.Printf("daemon: quota: %v", err)
			continue
		}
		locked := detail.Active && (detail.Locked || detail.LockedUntil > trusted)

		if s.Limit == 0 {
			if err := d.DeleteQuota(name); err != nil {
				log.Printf("daemon: quota: %v", err)
			}
			if detail.Active && detail.Scheduled && !locked {
				if err := d.DeactivateBlock(name); err != nil {
					log.Printf("daemon: quota: %v", err)
				}
			}
			continue
		}

		scheduled, err := ScheduleActiveNow(d, name, now)
		if err != nil {
			log.Printf("daemon: quota: %v", err)
		}
		if locked || scheduled {
			s.Open = false
		}

		saved, err := d.UpdateQuotaIfUnchanged(name, *state, s)
		if err != nil {
			log.Printf("daemon: quota: %v", err)
			continue
		}
		if !saved {
			continue // the CLI changed it; the next cycle sees the new state
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
			log.Printf("daemon: quota: %v", err)
		}
	}
	return nil
}
