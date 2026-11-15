package enforce

import (
	"log"
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

// pomodoroWatch remembers which blocks with a Pomodoro were enabled (apart
// from their own rests) last cycle; nil until the daemon's first look.
var pomodoroWatch map[string]bool

// restartPomodoros starts a block's Pomodoro rhythm afresh, work first,
// each time the block is enabled (R-SCHED-3): switched on, its window or
// place opening, a break, pause or allowance session ending.
func restartPomodoros(d *db.DB, now time.Time, trusted int64) {
	active, err := d.GetActiveBlocks()
	if err != nil {
		log.Printf("pomodoro: %v", err)
		return
	}
	lifted := scheduleLifted(d, now)
	for _, b := range openQuotaBlocks(d) {
		lifted[b.Name] = true
	}
	on := map[string]bool{}
	for _, b := range active {
		until, _ := d.PausedUntil(b.BlockName)
		on[b.BlockName] = !lifted[b.BlockName] && until <= trusted
	}
	first := pomodoroWatch == nil
	if first {
		pomodoroWatch = map[string]bool{}
	}
	for name, enabled := range on {
		if enabled && !pomodoroWatch[name] && !first {
			if p, err := d.GetPomodoro(name); err == nil && p != nil && p.Anchor != trusted {
				p.Anchor = trusted
				if err := d.SetPomodoro(name, *p); err != nil {
					log.Printf("pomodoro: %v", err)
				}
			}
		}
	}
	pomodoroWatch = on
}
