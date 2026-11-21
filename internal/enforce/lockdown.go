package enforce

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
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

// BlockLocked reports whether an active block has a lock in force, apart
// from its schedule lock (see BlocksLocked). Allowances are not locks.
func BlockLocked(detail *db.BlockDetail, extras db.LockExtras, trusted int64, bootID string) bool {
	if !detail.Active {
		return false
	}
	return len(db.LayersOn(detail, extras, trusted, bootID)) > 0
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
		if (IsLockableSetting(l.Name) || l.Name == db.NepsisLockName || l.Name == StrictLockName ||
			strings.HasPrefix(l.Name, FreezeLockPrefix)) && l.InForce(trusted, bootID) {
			return true, nil
		}
	}
	return false, nil
}

// BlocksLocked reports whether any block is locked, or a freeze or monk
// mode is running.
func BlocksLocked(d *db.DB, now time.Time, trusted int64) (bool, error) {
	until, err := d.FreezeUntil()
	if err != nil {
		return false, err
	}
	if now.Unix() < until {
		return true, nil
	}
	// Monk mode run for a set time can't be ended early, so it holds what
	// locks hold; switched on as a mode it can be switched off.
	if MonkEnd(d, trusted) > 0 {
		return true, nil
	}
	// A mode run by a Mode action is locked for its time (R-MODE-4).
	if len(modeRunning(d, trusted)) > 0 {
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
		// The schedule lock (§3.4) is a lock like the others.
		if on, lead, err := d.ScheduleLock(b.Name); err == nil && on {
			if frozen, err := ScheduleFrozen(d, b.Name, now, time.Duration(lead)*time.Minute); err == nil && frozen {
				return true, nil
			}
			if detail.Active {
				if within, err := ScheduleActiveNow(d, b.Name, now); err == nil && within {
					return true, nil
				}
			}
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

// FreezeScheduleHeld reports whether a freeze schedule's lock is on: its
// window is open or opens within the lock's lead time. Then it can't be
// removed (R-FREEZE-2). Without a lock it can always be removed; a freeze
// that is running goes on regardless.
func FreezeScheduleHeld(f db.FreezeSchedule, now time.Time) bool {
	return f.Locked(now)
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

// What a logout freeze does to logins (replaced in tests).
var (
	holdLogins    = blocker.HoldLogins
	releaseLogins = blocker.ReleaseLogins
)

func enforceFreeze(d *db.DB, now time.Time) {
	until, err := d.FreezeUntil()
	if err != nil {
		log.Printf("daemon: freeze state error: %v", err)
		return
	}
	if now.Unix() >= until || escapeHonoured(d) {
		if err := releaseLogins(); err != nil {
			log.Printf("daemon: freeze: %v", err)
		}
		return
	}
	mode, err := d.FreezeMode()
	if err != nil {
		log.Printf("daemon: freeze state error: %v", err)
		mode = freeze.ModeLock
	}
	if mode != freeze.ModeLogout {
		if err := releaseLogins(); err != nil {
			log.Printf("daemon: freeze: %v", err)
		}
	}
	switch mode {
	case freeze.ModeLogout:
		// Refuse new logins first, then end the sessions there are.
		if err := holdLogins(time.Unix(until, 0)); err != nil {
			log.Printf("daemon: freeze: %v", err)
		}
		err = logoutUsers()
	case freeze.ModeShutdown:
		err = powerOff()
	case freeze.ModeApps:
		blocker.CloseAllExcept(FreezeApps(d)) // R-FREEZE-1
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
		if err := blocker.RemoveNoexec(ExemptFolders(d)...); err != nil {
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
	exempt := ExemptFolders(d)
	if err := blocker.ApplyNoexec(exempt...); err != nil {
		log.Printf("daemon: noexec: %v", err)
	}
	if n := blocker.CloseNoexecPrograms(exempt); n > 0 {
		log.Printf("daemon: strict mode closed %d processes run from people's own files", n)
	}
}

// SettingFreezeApps is the app list of "allowed apps only" freezes (JSON).
const SettingFreezeApps = "freeze.apps"

// FreezeApps returns that list.
func FreezeApps(d *db.DB) []string {
	var out []string
	if v, err := d.GetSetting(SettingFreezeApps); err == nil && v != "" {
		_ = json.Unmarshal([]byte(v), &out)
	}
	return out
}

// FreezeLockPrefix names a scheduled freeze's setting lock: "freeze:<id>"
// (§3.8: a scheduled freeze can have locks).
const FreezeLockPrefix = "freeze:"

// SettingStrictExempt holds strict mode's exempt folders (§3.8), JSON.
const SettingStrictExempt = "strict.exempt"

// ExemptFolders returns the folders programs may still start from during
// strict mode's lockdown.
func ExemptFolders(d *db.DB) []string {
	var out []string
	if v, err := d.GetSetting(SettingStrictExempt); err == nil && v != "" {
		_ = json.Unmarshal([]byte(v), &out)
	}
	return out
}

// SetExemptFolders stores them.
func SetExemptFolders(d *db.DB, dirs []string) error {
	data, err := json.Marshal(dirs)
	if err != nil {
		return err
	}
	return d.SetSetting(SettingStrictExempt, string(data))
}

// finishDelayedStops ends blocks whose delay lock has run out, and moves
// layered locks on (R-LOCK-6): the lock after one that is gone starts.
func finishDelayedStops(d *db.DB, trusted int64) {
	blocks, err := d.ListBlocks()
	if err != nil {
		log.Printf("daemon: delay: %v", err)
		return
	}
	// Delay breaks (§3.6) whose countdown is over begin.
	for _, b := range blocks {
		k, err := d.GetBreaks(b.Name)
		if err != nil || k.DelayAt == 0 || trusted < k.DelayAt {
			continue
		}
		k.DelayAt = 0
		if err := d.SetBreaks(b.Name, k); err != nil {
			log.Printf("daemon: delay break: %v", err)
			continue
		}
		if err := d.SetPause(b.Name, trusted+k.DelayLength); err != nil {
			log.Printf("daemon: delay break: %v", err)
			continue
		}
		log.Printf("daemon: delay break of %s begins", b.Name)
		noticeOnce("delay-break "+b.Name+" "+strconv.FormatInt(trusted/60, 10), "Break for '"+b.Name+"'",
			"Its delay break begins now.")
	}
	bootID, bootErr := db.BootID()
	defer func() {
		if bootErr != nil {
			return
		}
		for _, b := range blocks {
			if err := d.AdvanceBlockLayers(b.Name, trusted, bootID); err != nil {
				log.Printf("daemon: lock layers of %s: %v", b.Name, err)
			}
		}
	}()
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
