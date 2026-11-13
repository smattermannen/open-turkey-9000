package enforce

import (
	"log"
	"strconv"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/freeze"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// Settings that control strict mode.
const (
	SettingStrict     = "strict"
	SettingStrictUser = "strict_user"
	SettingNoEscape   = "no_escape"

	// StrictLockName is the setting lock on strict mode itself (R-STRICT-1).
	StrictLockName = "strict"
)

// BlockLocked reports whether an active block has a lock in force. Quotas and
// schedules are not locks: they can't take away administrator rights.
func BlockLocked(detail *db.BlockDetail, extras db.LockExtras, trusted int64, bootID string) bool {
	if !detail.Active {
		return false
	}
	return detail.Locked || detail.LockedUntil > trusted || extras.PasswordHash != "" ||
		extras.Delay > 0 || (extras.RestartBoot != "" && extras.RestartBoot == bootID)
}

// AnyLock reports whether any block or browser setting is locked, or a freeze
// is running. Strict mode and no-escape hold while it is true.
func AnyLock(d *db.DB, now time.Time, trusted int64) (bool, error) {
	if locked, err := BlocksLocked(d, now, trusted); err != nil || locked {
		return locked, err
	}
	return AnySettingLocked(d, trusted)
}

// AnySettingLocked reports whether a lock holds any browser setting, the
// global bypass prevention or strict mode.
func AnySettingLocked(d *db.DB, trusted int64) (bool, error) {
	return settingLocked(d, trusted, "")
}

// settingLocked is AnySettingLocked, leaving out the setting lock skip.
func settingLocked(d *db.DB, trusted int64, skip string) (bool, error) {
	bootID, err := db.BootID()
	if err != nil {
		return false, err
	}
	locks, err := d.SettingLocks()
	if err != nil {
		return false, err
	}
	for _, l := range locks {
		// A lock on a retired option (see RetiredBrowserSettings) can no
		// longer be unlocked, so it no longer counts.
		if l.Name == skip {
			continue
		}
		if (IsBrowserSetting(l.Name) || l.Name == db.NepsisLockName || l.Name == StrictLockName) && l.InForce(trusted, bootID) {
			return true, nil
		}
	}
	return false, nil
}

// BlocksLocked reports whether any block is locked or a freeze is running.
func BlocksLocked(d *db.DB, now time.Time, trusted int64) (bool, error) {
	until, err := d.FreezeUntil()
	if err != nil {
		return false, err
	}
	if now.Unix() < until {
		return true, nil
	}
	bootID, err := db.BootID()
	if err != nil {
		return false, err
	}
	blocks, err := d.ListBlocks()
	if err != nil {
		return false, err
	}
	for _, b := range blocks {
		detail, err := d.GetBlock(b.Name)
		if err != nil {
			return false, err
		}
		extras, err := d.GetLockExtras(b.Name)
		if err != nil {
			return false, err
		}
		if BlockLocked(detail, extras, trusted, bootID) {
			return true, nil
		}
	}
	return false, nil
}

// reconcileFreezeSchedules starts or extends Frozen Chicken while a freeze
// schedule's window is open, until the window ends. It never shortens a
// freeze, so a longer manual freeze carries on.
func reconcileFreezeSchedules(d *db.DB, now time.Time) error {
	schedules, err := d.ListFreezeSchedules()
	if err != nil || len(schedules) == 0 {
		return err
	}
	windows := make([]schedule.Window, len(schedules))
	modes := make([]string, len(schedules))
	for i, s := range schedules {
		windows[i], modes[i] = s.Window, s.Mode
	}
	until, mode, ok := freeze.Scheduled(windows, modes, now)
	if !ok {
		return nil
	}
	current, err := d.FreezeUntil()
	if err != nil {
		return err
	}
	if until.Unix() <= current {
		return nil
	}
	log.Printf("daemon: scheduled freeze (%s) until %s", mode, until.Format("2006-01-02 15:04"))
	return d.SetFreeze(until.Unix(), mode)
}

// FreezeScheduleHeld reports whether a freeze schedule's window is open now
// or opens within schedule.EditFreeze: then it can't be removed, since that
// would cancel a freeze that is running or about to start.
func FreezeScheduleHeld(w schedule.Window, now time.Time) bool {
	return w.ActiveWithin(now, schedule.EditFreeze)
}

// escapeHonoured reports whether the kernel parameter may lift enforcement.
func escapeHonoured(d *db.DB) bool {
	noEscape, err := d.GetSetting(SettingNoEscape)
	if err != nil {
		return false
	}
	return noEscape != "on" && freeze.Bypassed(blocker.KernelCmdline())
}

// LockFreezeEngaged reports whether a lock-mode freeze is being enforced now.
func LockFreezeEngaged(d *db.DB, now time.Time) bool {
	until, err := d.FreezeUntil()
	if err != nil || now.Unix() >= until || escapeHonoured(d) {
		return false
	}
	mode, err := d.FreezeMode()
	return err != nil || mode == freeze.ModeLock
}

func enforceFreeze(d *db.DB, now time.Time) {
	until, err := d.FreezeUntil()
	if err != nil {
		log.Printf("daemon: freeze state error: %v", err)
		return
	}
	if now.Unix() >= until || escapeHonoured(d) {
		return
	}
	mode, err := d.FreezeMode()
	if err != nil {
		log.Printf("daemon: freeze state error: %v", err)
		mode = freeze.ModeLock
	}
	switch mode {
	case freeze.ModeLogout:
		err = blocker.LogoutUsers()
	case freeze.ModeShutdown:
		err = blocker.PowerOff()
	default:
		err = blocker.LockSessions()
	}
	if err != nil {
		log.Printf("daemon: freeze: %v", err)
	}
}

func enforceLockdown(d *db.DB, now time.Time, trusted int64) {
	strict, err := d.GetSetting(SettingStrict)
	if err != nil {
		log.Printf("daemon: lockdown: %v", err)
		return
	}
	want := false
	if strict == "on" && !escapeHonoured(d) {
		// Strict mode's own lock only keeps it switched on; it acts while
		// something else is locked.
		if want, err = BlocksLocked(d, now, trusted); err == nil && !want {
			want, err = settingLocked(d, trusted, StrictLockName)
		}
		if err != nil {
			log.Printf("daemon: lockdown: %v", err)
			return
		}
	}
	if !want {
		if err := blocker.RemoveLockdown(); err != nil {
			log.Printf("daemon: lockdown: %v", err)
		}
		if err := blocker.RemoveNoexec(); err != nil {
			log.Printf("daemon: noexec: %v", err)
		}
		return
	}
	user, err := d.GetSetting(SettingStrictUser)
	if err != nil {
		log.Printf("daemon: lockdown: %v", err)
		return
	}
	if err := blocker.ApplyLockdown(user); err != nil {
		log.Printf("daemon: lockdown: %v", err)
	}
	if err := blocker.ApplyNoexec(); err != nil {
		log.Printf("daemon: noexec: %v", err)
	}
}

// finishDelayedStops ends blocks whose delay lock has run out.
func finishDelayedStops(d *db.DB, trusted int64) {
	blocks, err := d.ListBlocks()
	if err != nil {
		log.Printf("daemon: delay: %v", err)
		return
	}
	for _, b := range blocks {
		extras, err := d.GetLockExtras(b.Name)
		if err != nil {
			log.Printf("daemon: delay: %v", err)
			continue
		}
		if extras.DelayUntil != 0 && trusted >= extras.DelayUntil {
			// The delay lock's countdown is over: it is unlocked, and the
			// block stays on (§3.4).
			extras.Delay, extras.DelayUntil = 0, 0
			if err := d.SetLockExtras(b.Name, extras); err != nil {
				log.Printf("daemon: delay: %v", err)
			} else {
				log.Printf("daemon: the delay lock of %s is unlocked", b.Name)
			}
		}
		if extras.StopAt != 0 && trusted >= extras.StopAt {
			if err := d.DeactivateBlock(b.Name); err != nil {
				log.Printf("daemon: delay: %v", err)
			} else {
				log.Printf("daemon: delay lock ended %s (%s)", b.Name, strconv.FormatInt(extras.Delay, 10))
			}
		}
	}
}
