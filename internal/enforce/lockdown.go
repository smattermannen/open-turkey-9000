package enforce

import (
	"log"
	"strconv"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/freeze"
)

// Settings that control strict mode.
const (
	SettingStrict     = "strict"
	SettingStrictUser = "strict_user"
	SettingNoEscape   = "no_escape"
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

// AnySettingLocked reports whether a lock holds any browser setting.
func AnySettingLocked(d *db.DB, trusted int64) (bool, error) {
	bootID, err := db.BootID()
	if err != nil {
		return false, err
	}
	locks, err := d.SettingLocks()
	if err != nil {
		return false, err
	}
	for _, l := range locks {
		if l.InForce(trusted, bootID) {
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
		if want, err = AnyLock(d, now, trusted); err != nil {
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
		if extras.StopAt != 0 && trusted >= extras.StopAt {
			if err := d.DeactivateBlock(b.Name); err != nil {
				log.Printf("daemon: delay: %v", err)
			} else {
				log.Printf("daemon: delay lock ended %s (%s)", b.Name, strconv.FormatInt(extras.Delay, 10))
			}
		}
	}
}
