// nepsis.go — bypass prevention in the daemon cycle: count recent events
// (db/events.go) and take the chosen consequence when there are too many.
// The global setting counts every event; a block's own counts that block's
// events while it is on. After firing, only newer events count.
package enforce

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
)

// enforceNepsis checks the global and each active block's bypass prevention.
func enforceNepsis(d *db.DB, now time.Time, active []db.ActiveBlockDetail) {
	if err := d.PruneEvents(now); err != nil {
		log.Printf("nepsis: %v", err)
	}
	if n, err := d.GlobalNepsis(); err != nil {
		log.Printf("nepsis: %v", err)
	} else if n.On() {
		checkNepsis(d, now, "", n)
	}
	for _, b := range active {
		n, err := d.BlockNepsis(b.BlockName)
		if err != nil {
			log.Printf("nepsis: %v", err)
			continue
		}
		if n.On() {
			checkNepsis(d, now, b.BlockName, n)
		}
	}
}

func nepsisFiredKey(block string) string {
	if block == "" {
		return "nepsis.fired"
	}
	return "nepsis.fired." + block
}

// NepsisCount returns how many events count towards a bypass prevention now
// (block "" for the global one).
func NepsisCount(d *db.DB, now time.Time, block string, n db.Nepsis) (int, error) {
	since := now.Add(-n.Window()).Unix()
	if value, _ := d.GetSetting(nepsisFiredKey(block)); value != "" {
		if fired, err := strconv.ParseInt(value, 10, 64); err == nil && fired >= since {
			since = fired + 1
		}
	}
	return d.CountEvents(block, since)
}

func checkNepsis(d *db.DB, now time.Time, block string, n db.Nepsis) {
	count, err := NepsisCount(d, now, block, n)
	if err != nil {
		log.Printf("nepsis: %v", err)
		return
	}
	if count < n.Count {
		return
	}
	if err := d.SetSetting(nepsisFiredKey(block), strconv.FormatInt(now.Unix(), 10)); err != nil {
		log.Printf("nepsis: %v", err)
		return
	}
	who := "Your blocks"
	if block != "" {
		who = fmt.Sprintf("Block '%s'", block)
	}
	why := fmt.Sprintf("%s stopped %d things within %d minutes. Time to step away.", who, count, n.Minutes)
	if err := Run(d, Consequence{Kind: n.Consequence, Minutes: n.ActionMinutes}, why, true); err != nil {
		log.Printf("nepsis: %v", err)
	}
}
