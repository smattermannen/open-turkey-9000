package enforce

import (
	"time"

	"github.com/smattermannen/kc9k/internal/db"
)

// withoutBreaks drops blocks that are lifted for now: in a Pomodoro break, or
// paused (a break taken, an unlocked block waiting to relock; see
// pausedFor). They stay active, with their locks, so blocking resumes by
// itself when the time is up.
func withoutBreaks(d *db.DB, active []db.ActiveBlockDetail, trusted int64) []db.ActiveBlockDetail {
	out := make([]db.ActiveBlockDetail, 0, len(active))
	for _, b := range active {
		if pausedFor(d, b.BlockName, trusted) <= 0 {
			out = append(out, b)
		}
	}
	return out
}

// PomodoroWorking reports whether a block has a Pomodoro cycle that is in a
// work period now. Its quota session can't be open then: otherwise the
// quota would be a way out of the work period.
func PomodoroWorking(d *db.DB, name string) bool {
	p, err := d.GetPomodoro(name)
	if err != nil || p == nil {
		return false
	}
	trusted, err := d.TrustedNow(time.Now())
	return err == nil && !p.Cycle().OnBreak(trusted)
}
