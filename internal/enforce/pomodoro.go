package enforce

import (
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
