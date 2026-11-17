package enforce

import (
	"log"
	"maps"
	"slices"
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
	// R-MODE-4: a mode run by a Mode action enforces whatever its
	// automation says.
	for name := range modeRunning(d, trusted) {
		delete(lifted, name)
	}
	out := make([]db.ActiveBlockDetail, 0, len(active))
	for _, b := range active {
		if !lifted[b.BlockName] && pausedFor(d, b.BlockName, trusted) <= 0 {
			out = append(out, b)
		}
	}
	return out
}

// scheduleLifted returns the blocks their automation lifts now (§3.6): a
// block disabled by default that its windows and places don't switch on,
// one enabled by default that they switch off. Continuous blocks are never
// in it. On a read error nothing is lifted.
func scheduleLifted(d *db.DB, now time.Time) map[string]bool {
	lifted := map[string]bool{}
	autos, places, err := allAutomations(d)
	if err != nil {
		log.Printf("automation: %v", err)
		return lifted
	}
	var at map[int]bool
	unknown := false
	if slices.ContainsFunc(slices.Collect(maps.Values(autos)), func(a schedule.Automation) bool { return len(a.Places) > 0 }) {
		at, unknown = placesHere(now, places)
	}
	for name, a := range autos {
		if !a.Continuous() && !automationEnabled(a, now, at, unknown) {
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
