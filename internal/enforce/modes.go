// modes.go — monk mode as a mode (§3.8 Monk mode, R-MONK-3; owner
// 2026-11-17, later). A mode of blocks needs nothing here: its blocks hold
// copies of its automation (db.SetBlockAutomation) and are enforced as
// blocks.
//
// Monk mode runs while it is switched on and its automation has it on (with
// no windows or places: whenever it is on), and switching it off ends that,
// as for any mode. Only a run for a set time (kc9k monk --for, a Mode
// action: db.SettingMonkUntil) is locked: it holds whatever the switch and
// the automation say, until its time is over (MonkEnd).
package enforce

import (
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// MonkAutomation returns monk mode as a mode and its automation.
func MonkAutomation(d *db.DB) (db.Mode, schedule.Automation, error) {
	m, err := d.MonkMode()
	if err != nil {
		return db.Mode{}, schedule.Automation{}, err
	}
	all, err := d.SavedSchedules()
	if err != nil {
		return db.Mode{}, schedule.Automation{}, err
	}
	a, _ := m.Spec.Automation(all)
	a.DefaultOn = false // never enabled by default
	return m, a, nil
}

// monkAutomationAt reports whether an automation of monk mode has it on at
// now. Without windows or places it never does (unlike a block's, which a
// continuous automation keeps enabled).
func monkAutomationAt(d *db.DB, a schedule.Automation, now time.Time) bool {
	if a.Continuous() {
		return false
	}
	var at map[int]bool
	unknown := false
	if len(a.Places) > 0 {
		places, err := d.Places()
		if err != nil {
			return true // fail closed, as a block's places do
		}
		at, unknown = placesHere(now, places)
	}
	return automationEnabled(a, now, at, unknown)
}

// MonkAutomationOn reports whether monk mode runs as a mode now: it is
// switched on and its windows and places have it on, or it has none
// (R-MONK-3). A run for a set time is MonkEnd's.
func MonkAutomationOn(d *db.DB, now time.Time) bool {
	m, a, err := MonkAutomation(d)
	if err != nil || !m.Active {
		return false
	}
	return a.Continuous() || monkAutomationAt(d, a, now)
}

// MonkAutomationWith reports whether monk mode, switched on with the
// automation a, would be running now.
func MonkAutomationWith(d *db.DB, a schedule.Automation, now time.Time) bool {
	a.DefaultOn = false
	return monkAutomationAt(d, a, now)
}

// monkKeepsWifi reports whether monk mode's automation needs the Wi-Fi
// radio on to see where the computer is (R-MONK-2).
func monkKeepsWifi(d *db.DB) bool {
	m, a, err := MonkAutomation(d)
	return err == nil && m.Active && len(a.Places) > 0
}
