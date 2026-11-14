package enforce

import (
	"time"

	"github.com/smattermannen/kc9k/internal/db"
)


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
