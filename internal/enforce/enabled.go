package enforce

import (
	"log"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// enabledBlocks returns the active blocks that enforce now (§3.3: enabled):
// not lifted by a break, pause or Pomodoro rest, an open allowance session,
// nor by their windows or place rules. Only the user switches a block on or off; the schedule only
// lifts it (R-SCHED-1).
func enabledBlocks(d *db.DB, active []db.ActiveBlockDetail, trusted int64) []db.ActiveBlockDetail {
	lifted := scheduleLifted(d, time.Now())
	// An open allowance session lifts its block too.
	for _, b := range openQuotaBlocks(d) {
		lifted[b.Name] = true
	}
	out := make([]db.ActiveBlockDetail, 0, len(active))
	for _, b := range active {
		if !lifted[b.BlockName] && pausedFor(d, b.BlockName, trusted) <= 0 {
			out = append(out, b)
		}
	}
	return out
}

// scheduleLifted returns the blocks their windows or place rules lift now:
// a scheduled block (block windows or a place rule) outside its windows
// and away from its places, a continuous block in an allow window. Blocks
// without windows or place rules are never in it. On a read error nothing
// is lifted.
func scheduleLifted(d *db.DB, now time.Time) map[string]bool {
	lifted := map[string]bool{}
	all, err := d.AllSchedules()
	if err != nil {
		log.Printf("schedule: %v", err)
		return lifted
	}
	windows := map[string][]schedule.Window{}
	for _, s := range all {
		windows[s.BlockName] = append(windows[s.BlockName], s.Window())
	}
	ruled, err := PlaceRuled(d)
	if err != nil {
		log.Printf("places: %v", err)
		return lifted
	}
	atPlace := map[string]bool{}
	if len(ruled) > 0 {
		if atPlace, err = PlaceBlocks(d, now); err != nil {
			log.Printf("places: %v", err)
			return map[string]bool{}
		}
	}
	for name := range ruled {
		if _, ok := windows[name]; !ok {
			windows[name] = nil
		}
	}
	for name, ws := range windows {
		if !schedule.Enforced(ws, now) && !atPlace[name] {
			lifted[name] = true
		}
	}
	return lifted
}

// ScheduleLifted returns every block its windows or place rules lift now.
func ScheduleLifted(d *db.DB, now time.Time) map[string]bool { return scheduleLifted(d, now) }

// LiftedBySchedule reports whether a block's windows or place rules lift
// it now (see scheduleLifted).
func LiftedBySchedule(d *db.DB, name string, now time.Time) bool {
	return scheduleLifted(d, now)[name]
}

// HasScheduleOrPlace reports whether a block has windows or a place rule,
// so that it is enabled only as they say.
func HasScheduleOrPlace(d *db.DB, name string) bool {
	if s, err := d.ListSchedules(name); err == nil && len(s) > 0 {
		return true
	}
	ruled, err := PlaceRuled(d)
	return err == nil && ruled[name]
}

// EnabledNames returns the active blocks that are enabled now (§3.3), the
// rest being lifted.
func EnabledNames(d *db.DB, trusted int64) map[string]bool {
	out := map[string]bool{}
	active, err := d.GetActiveBlocks()
	if err != nil {
		return out
	}
	for _, b := range enabledBlocks(d, active, trusted) {
		out[b.BlockName] = true
	}
	return out
}
