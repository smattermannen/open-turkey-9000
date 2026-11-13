package enforce

import (
	"log"

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
		earned := k.Earn(elapsed)
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
