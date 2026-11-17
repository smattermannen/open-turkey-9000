// moderun.go — the Mode action (§3.9, R-MODE-4; owner 2026-11-17): a mode
// runs now for a set time, locked, whatever its automation says; then it is
// as it was. Monk mode runs for the time (StartMonk), Frozen Chicken
// freezes (lock screen); a mode of blocks switches its blocks on, keeps
// them enabled (enabledBlocks) and held (ModeRunUntil) until the time is
// over, and the daemon then switches off again those that were off
// (endModeRuns).
package enforce

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/freeze"
)

// StartModeRun is the Mode action: mode runs now for length, locked. A run
// already going is only ever extended.
func StartModeRun(d *db.DB, mode string, length time.Duration) error {
	if length < time.Minute || length > time.Duration(db.MaxMonkMinutes)*time.Minute {
		return fmt.Errorf("a mode runs from 1 minute to %d days", db.MaxMonkMinutes/(24*60))
	}
	switch {
	case strings.EqualFold(mode, db.MonkModeName) || strings.EqualFold(mode, "monk"):
		return StartMonk(d, length)
	case strings.EqualFold(mode, db.FrozenChickenName):
		if length > freeze.MaxDuration {
			length = freeze.MaxDuration
		}
		until := time.Now().Add(length).Unix()
		if current, err := d.FreezeUntil(); err != nil {
			return err
		} else if until > current {
			if err := d.SetFreeze(until, freeze.ModeLock); err != nil {
				return err
			}
		}
		applyFreeze(d, time.Now())
		return nil
	}
	m, err := d.GetMode(mode)
	if err != nil {
		return err
	}
	trusted, err := d.TrustedNow(time.Now())
	if err != nil {
		return err
	}
	until := trusted + int64(length/time.Second)
	runs := d.ModeRuns()
	i, had := db.ModeRunOf(runs, m.Name)
	if !had {
		runs = append(runs, db.ModeRun{Mode: m.Name})
		i = len(runs) - 1
	}
	if until > runs[i].Until {
		runs[i].Until = until
	}
	for _, b := range m.Blocks {
		on, err := d.IsBlockActive(b)
		if err != nil {
			return err
		}
		if on {
			continue
		}
		if err := d.ActivateBlock(b, false, 0, 0, false); err != nil {
			log.Printf("mode %s: %v", m.Name, err)
			continue
		}
		runs[i].OffBefore = append(runs[i].OffBefore, b)
	}
	if err := d.SaveModeRuns(runs); err != nil {
		return err
	}
	log.Printf("mode %s: runs until %s (Mode action)", m.Name, time.Now().Add(time.Duration(runs[i].Until-trusted)*time.Second).Format("15:04"))
	return nil // the daemon's next cycle enforces it
}

// ModeRunUntil returns until when a block is held by a run of its mode
// (trusted time), 0 when it isn't.
func ModeRunUntil(d *db.DB, block string, trusted int64) int64 {
	runs := d.ModeRuns()
	if len(runs) == 0 {
		return 0
	}
	m, in, err := d.ModeOf(block)
	if err != nil || !in {
		return 0
	}
	if i, ok := db.ModeRunOf(runs, m.Name); ok && (trusted == 0 || runs[i].Until > trusted) {
		return runs[i].Until
	}
	return 0
}

// ModeRunLeft returns how many seconds a mode's run has left, 0 when none.
func ModeRunLeft(d *db.DB, mode string, trusted int64) int64 {
	if i, ok := db.ModeRunOf(d.ModeRuns(), mode); ok && d.ModeRuns()[i].Until > trusted {
		return d.ModeRuns()[i].Until - trusted
	}
	return 0
}

// modeRunning returns the blocks a run of their mode keeps enabled now.
func modeRunning(d *db.DB, trusted int64) map[string]bool {
	out := map[string]bool{}
	for _, r := range d.ModeRuns() {
		if trusted != 0 && r.Until <= trusted {
			continue
		}
		if m, err := d.GetMode(r.Mode); err == nil {
			for _, b := range m.Blocks {
				out[b] = true
			}
		}
	}
	return out
}

// endModeRuns ends the runs whose time is over (daemon cycle): the blocks
// that were off before are switched off again, unless a lock of their own
// holds them.
func endModeRuns(d *db.DB, trusted int64) {
	runs := d.ModeRuns()
	if len(runs) == 0 || trusted == 0 {
		return
	}
	bootID, _ := db.BootID()
	var keep []db.ModeRun
	for _, r := range runs {
		if r.Until > trusted {
			keep = append(keep, r)
			continue
		}
		for _, b := range r.OffBefore {
			detail, err := d.GetBlock(b)
			if err != nil || !detail.Active {
				continue
			}
			extras, _ := d.GetLockExtras(b)
			if BlockLocked(detail, extras, trusted, bootID) {
				continue
			}
			if err := d.DeactivateBlock(b); err != nil {
				log.Printf("mode %s: %v", r.Mode, err)
			}
		}
		log.Printf("mode %s: its run is over", r.Mode)
	}
	if len(keep) != len(runs) {
		if err := d.SaveModeRuns(keep); err != nil {
			log.Printf("mode runs: %v", err)
		}
	}
}

// runModeCustoms runs what a mode opens or runs when it becomes enabled
// (§3.9 Custom action, R-CUSTOM-2): once each time it does; a computer
// started while it is enabled runs it once, when someone is signed in (a
// note per boot, so the daemon restarting doesn't run it again). A mode of
// blocks is enabled while one of its blocks is; monk mode while it runs.
func runModeCustoms(d *db.DB, enabled map[string]bool, trusted int64) {
	modes, err := d.Modes()
	if err != nil {
		return
	}
	boot, _ := db.BootID()
	for _, m := range modes {
		param, err := d.ModeCustom(m.Name)
		if err != nil || param == "" {
			continue
		}
		on := false
		if m.Monk {
			on = MonkOn(d, trusted)
		} else {
			for _, b := range m.Blocks {
				on = on || enabled[b]
			}
		}
		key := fmt.Sprintf("mode.custom.ran.%d", m.ID)
		ran, _ := d.GetSetting(key)
		if !on {
			if ran != "" {
				_ = d.SetSetting(key, "")
			}
			continue
		}
		if ran == boot || !someoneSignedIn() {
			continue
		}
		log.Printf("mode %s: enabled, running its custom action", m.Name)
		if err := runCustom(param); err != nil {
			log.Printf("mode %s: %v", m.Name, err)
		}
		_ = d.SetSetting(key, boot)
	}
}

// someoneSignedIn reports whether a desktop session exists to run custom
// actions in; tests replace it.
var someoneSignedIn = func() bool { return len(blocker.SessionUsers()) > 0 }
