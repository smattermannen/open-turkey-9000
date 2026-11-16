package enforce

import (
	"fmt"
	"log"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
)

// computerInUse reports whether someone is using the computer (§3.7: input
// in the last 3 minutes, screen not locked). Replaced in tests.
var computerInUse = func() bool { return blocker.SomeoneAtDesktop(idleLimit) }

// lastEarned is the trusted time break allowances were last topped up.
var lastEarned int64

// earnBreaks adds, to each enforced block with an allowance, the reward
// earned since the last cycle. Only time the computer is in use earns
// (R-BREAK-2); time while the daemon wasn't running, or a gap longer than a
// minute (a suspend), earns nothing.
func earnBreaks(d *db.DB, enforced []db.ActiveBlockDetail, trusted int64) {
	elapsed := trusted - lastEarned
	first := lastEarned == 0
	lastEarned = trusted
	if first || elapsed <= 0 || elapsed > 60 {
		return
	}
	asked, inUse := false, false
	for _, b := range enforced {
		k, err := d.GetBreaks(b.BlockName)
		if err != nil {
			log.Printf("breaks: %v", err)
			continue
		}
		expired := k.Expire(RewardPeriod(d, b.BlockName, k.Keep, time.Unix(trusted, 0)), trusted)
		if expired != k {
			if err := d.SetBreaks(b.BlockName, expired); err != nil {
				log.Printf("breaks: %v", err)
			}
			k = expired
		}
		earned := k.Earn(elapsed, trusted)
		if earned == k {
			continue
		}
		if !asked {
			asked, inUse = true, computerInUse()
		}
		if !inUse {
			return
		}
		if earned != k {
			if err := d.SetBreaks(b.BlockName, earned); err != nil {
				log.Printf("breaks: %v", err)
			}
		}
	}
}

// RewardPeriod names the period a reward kept for keep belongs to at t:
// the day, ISO week or month, or the block window it falls in (the day,
// for a block without windows). "" for a reward kept up to one break.
func RewardPeriod(d *db.DB, block, keep string, t time.Time) string {
	switch keep {
	case "day":
		return t.Format("2006-01-02")
	case "week":
		y, w := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", y, w)
	case "month":
		return t.Format("2006-01")
	case "window":
		schedules, err := d.ListSchedules(block)
		if err == nil {
			for _, s := range schedules {
				w := s.Window()
				if !w.Allow && w.ActiveAt(t) {
					return fmt.Sprintf("%s %d", t.Format("2006-01-02"), w.Start)
				}
			}
		}
		return t.Format("2006-01-02")
	}
	return ""
}
