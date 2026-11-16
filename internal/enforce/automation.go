// automation.go — what enables and lifts an active block by itself (§3.6,
// owner 2026-11-16): its windows and places, from its default (enabled or
// disabled), needing both or either (schedule.Automation).
package enforce

import (
	"slices"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// AutomationOf returns a block's automation.
func AutomationOf(d *db.DB, name string) (schedule.Automation, error) {
	schedules, err := d.ListSchedules(name)
	if err != nil {
		return schedule.Automation{}, err
	}
	settings, err := d.Automation(name)
	if err != nil {
		return schedule.Automation{}, err
	}
	places, err := d.Places()
	if err != nil {
		return schedule.Automation{}, err
	}
	a := schedule.Automation{Windows: db.Windows(schedules), DefaultOn: settings.DefaultOn, NeedBoth: settings.NeedBoth}
	if len(a.Windows) > 0 {
		a.DefaultOn = a.Windows[0].Allow // windows follow the default (SetAutomation); theirs decides
	}
	for _, p := range places {
		if slices.Contains(p.Blocks, name) {
			a.Places = append(a.Places, p.ID)
		}
	}
	return a, nil
}

// allAutomations returns the automation of every block with windows or
// places, and the places.
func allAutomations(d *db.DB) (map[string]schedule.Automation, []db.Place, error) {
	all, err := d.AllSchedules()
	if err != nil {
		return nil, nil, err
	}
	settings, err := d.Automations()
	if err != nil {
		return nil, nil, err
	}
	places, err := d.Places()
	if err != nil {
		return nil, nil, err
	}
	out := map[string]schedule.Automation{}
	get := func(name string) schedule.Automation {
		a := out[name]
		a.DefaultOn, a.NeedBoth = settings[name].DefaultOn, settings[name].NeedBoth
		return a
	}
	for _, s := range all {
		a := get(s.BlockName)
		a.Windows = append(a.Windows, s.Window())
		out[s.BlockName] = a
	}
	for _, p := range places {
		for _, b := range p.Blocks {
			a := get(b)
			a.Places = append(a.Places, p.ID)
			out[b] = a
		}
	}
	for name, a := range out {
		if len(a.Windows) > 0 {
			a.DefaultOn = a.Windows[0].Allow // windows follow the default (SetAutomation); theirs decides
			out[name] = a
		}
	}
	return out, places, nil
}

// placesHere returns the places the computer is at, by id; unknown when no
// network is seen (Wi-Fi off or none in range).
func placesHere(now time.Time, places []db.Place) (at map[int]bool, unknown bool) {
	seen := seenNetworks(now)
	if len(seen) == 0 {
		return nil, true
	}
	at = map[int]bool{}
	for _, p := range places {
		if AtPlace(p, seen) {
			at[p.ID] = true
		}
	}
	return at, false
}

// automationEnabled reports whether a block's automation has it enabled now.
// With no network seen its places count as whatever keeps it enabled.
func automationEnabled(a schedule.Automation, now time.Time, at map[int]bool, unknown bool) bool {
	atPlace := false
	if len(a.Places) > 0 {
		if unknown {
			atPlace = a.UnknownPlace()
		} else {
			atPlace = slices.ContainsFunc(a.Places, func(id int) bool { return at[id] })
		}
	}
	return a.Enabled(schedule.InAny(a.Windows, now), atPlace)
}
