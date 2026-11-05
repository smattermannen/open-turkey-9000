package enforce

import (
	"log"

	"github.com/brunodcdo/open-turkey/internal/db"
	"github.com/brunodcdo/open-turkey/internal/quota"
)

// withoutBreaks drops blocks in a Pomodoro break. They stay active, with their
// locks, so blocking resumes by itself when the break ends.
func withoutBreaks(d *db.DB, active []db.ActiveBlockDetail, trusted int64) []db.ActiveBlockDetail {
	out := make([]db.ActiveBlockDetail, 0, len(active))
	for _, b := range active {
		p, err := d.GetPomodoro(b.BlockName)
		if err != nil {
			log.Printf("pomodoro: %v", err)
		}
		if p == nil || !quota.OnBreak(p.On, p.Off, p.Anchor, trusted) {
			out = append(out, b)
		}
	}
	return out
}
