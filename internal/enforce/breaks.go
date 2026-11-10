package enforce

import (
	"log"

	"github.com/smattermannen/kc9k/internal/db"
)

// lastEarned is the trusted time break allowances were last topped up.
var lastEarned int64

// earnBreaks adds, to each enforced block with an allowance, the break time
// earned since the last cycle. Time while the daemon wasn't running, or a
// gap longer than a minute (a suspend), earns nothing.
func earnBreaks(d *db.DB, enforced []db.ActiveBlockDetail, trusted int64) {
	elapsed := trusted - lastEarned
	first := lastEarned == 0
	lastEarned = trusted
	if first || elapsed <= 0 || elapsed > 60 {
		return
	}
	for _, b := range enforced {
		k, err := d.GetBreaks(b.BlockName)
		if err != nil {
			log.Printf("breaks: %v", err)
			continue
		}
		if earned := k.Earn(elapsed); earned != k {
			if err := d.SetBreaks(b.BlockName, earned); err != nil {
				log.Printf("breaks: %v", err)
			}
		}
	}
}
