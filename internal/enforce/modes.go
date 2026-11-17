// modes.go — monk mode run by its automation (§3.8 Monk mode, R-MONK-3;
// owner 2026-11-17). A mode of blocks needs nothing here: its blocks hold
// copies of its automation (db.SetBlockAutomation) and are enforced as
// blocks.
//
// While monk mode is switched on and its windows and places have it on,
// the daemon keeps its end (db.SettingMonkUntil) ahead of now: to the end
// of the window when only windows decide, or half a minute at a time while
// places do (it ends soon after the computer leaves). Everything else is
// monk mode as always: the cuts, what it blocks, putting the network back.
package enforce

import (
	"log"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// monkPlaceStep is how far ahead monk mode's end is kept while places
// decide whether it runs.
const monkPlaceStep = 30 * time.Second

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

// MonkAutomationOn reports whether monk mode's automation runs it now: it
// is switched on and its windows and places have it on (R-MONK-3).
func MonkAutomationOn(d *db.DB, now time.Time) bool {
	m, a, err := MonkAutomation(d)
	if err != nil || !m.Active {
		return false
	}
	return monkAutomationAt(d, a, now)
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

// windowsEnd returns when windows-only automation a, which has monk mode
// on at now, next has it off (at most a week ahead).
func windowsEnd(a schedule.Automation, now time.Time) time.Time {
	t := now.Truncate(time.Minute)
	for i := 0; i < 7*24*60; i++ {
		t = t.Add(time.Minute)
		if !a.Enabled(schedule.InAny(a.Windows, t), false) {
			return t
		}
	}
	return t
}

// runMonkAutomation is monk mode's automation in the daemon cycle: while
// it has monk mode on, monk mode's end is kept ahead (it only ever grows).
func runMonkAutomation(d *db.DB, now time.Time, trusted int64) {
	m, a, err := MonkAutomation(d)
	if err != nil || !m.Active || trusted == 0 || !monkAutomationAt(d, a, now) {
		return
	}
	end := now.Add(monkPlaceStep)
	if len(a.Places) == 0 {
		end = windowsEnd(a, now)
	}
	until := trusted + int64(end.Sub(now).Round(time.Second)/time.Second)
	before, _ := d.MonkUntil()
	if until <= before {
		return
	}
	if err := d.ExtendMonk(until); err != nil {
		log.Printf("monk mode: %v", err)
		return
	}
	if before <= trusted {
		log.Printf("monk mode: started by its automation")
	}
}
