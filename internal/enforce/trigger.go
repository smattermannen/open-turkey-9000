// trigger.go — what a block's apps, its window titles and its place rules
// do when they fire (owner, 2026-11-16; §1 "any trigger to any action"):
// their own action (close the app or the window; enable the block at the
// place), and any consequences the user ticked, announced as a list's are
// (R-WARN-1).
package enforce

import (
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
)

// triggerEvery is how often one block's trigger group may fire its
// consequences: an app that keeps running doesn't close the browsers every
// cycle.
const triggerEvery = time.Minute

var triggerFired = struct {
	sync.Mutex
	at map[string]time.Time
}{at: map[string]time.Time{}}

// runningApps finds which of the apps run now (tests replace it).
var runningApps = blocker.RunningApps

// triggerActions returns what a block's trigger group does.
func triggerActions(d *db.DB, block, group string) []db.TriggerAction {
	actions, err := d.TriggerActions(block, group)
	if err != nil {
		log.Printf("trigger: %v", err)
		return db.DefaultTriggerActions(group)
	}
	return actions
}

func hasTriggerAction(actions []db.TriggerAction, name string) bool {
	_, ok := db.FindTriggerAction(actions, name)
	return ok
}

// triggerConsequences are a group's actions besides its own.
func triggerConsequences(actions []db.TriggerAction) []Consequence {
	var out []Consequence
	for _, a := range actions {
		if a.Name != db.TriggerClose && a.Name != db.TriggerEnable {
			out = append(out, Consequence{Kind: a.Name, Minutes: a.Minutes})
		}
	}
	return out
}

// fireTrigger carries out a group's consequences, at most once per
// triggerEvery for one block and group. It reports whether it fired.
func fireTrigger(d *db.DB, block, group string, actions []db.TriggerAction, why string, now time.Time) bool {
	consequences := triggerConsequences(actions)
	if len(consequences) == 0 {
		return false
	}
	key := block + "\x00" + group
	triggerFired.Lock()
	if last, ok := triggerFired.at[key]; ok && now.Sub(last) < triggerEvery {
		triggerFired.Unlock()
		return false
	}
	triggerFired.at[key] = now
	triggerFired.Unlock()
	for _, c := range consequences {
		runAnnounced(d, c, why)
	}
	return true
}

// appsTrigger closes a block's apps (unless its apps' actions leave them
// open) and fires the apps' other actions when one ran.
func appsTrigger(d *db.DB, b db.ActiveBlockDetail, now time.Time) {
	if len(b.Apps) == 0 {
		return
	}
	actions := triggerActions(d, b.BlockName, db.TriggerApps)
	why := "An app of block '" + b.BlockName + "' was running."
	if hasTriggerAction(actions, db.TriggerClose) {
		if killAppsFor(d, b.Apps, blockUIDs(b), db.EventApp, b.BlockName) > 0 {
			fireTrigger(d, b.BlockName, db.TriggerApps, actions, why, now)
		}
		return
	}
	if len(triggerConsequences(actions)) > 0 && len(runningApps(b.Apps)) > 0 {
		if fireTrigger(d, b.BlockName, db.TriggerApps, actions, why, now) {
			logEvent(d, db.EventApp, b.BlockName, "")
		}
	}
}

// placeEnables reports whether a block's rule at a place enables it there.
func placeEnables(d *db.DB, block string, placeID int) bool {
	return hasTriggerAction(triggerActions(d, block, db.PlaceTrigger(placeID)), db.TriggerEnable)
}

// placeArrivals fires a place rule's other actions when the computer
// arrives at the place (networks seen: Wi-Fi off is no arrival), for the
// blocks that are on. The first look after a start only records where the
// computer is.
func placeArrivals(d *db.DB, now time.Time) {
	places, err := d.Places()
	if err != nil {
		return
	}
	ruled := false
	for _, p := range places {
		ruled = ruled || len(p.Blocks) > 0
	}
	if !ruled {
		return
	}
	seen := seenNetworks(now)
	for _, p := range places {
		if len(p.Blocks) == 0 {
			continue
		}
		here := len(seen) > 0 && AtPlace(p, seen)
		key := "place.here." + strconv.Itoa(p.ID)
		was, _ := d.GetSetting(key)
		if here == (was == "1") && was != "" {
			continue
		}
		state := "0"
		if here {
			state = "1"
		}
		if err := d.SetSetting(key, state); err != nil {
			log.Printf("places: %v", err)
		}
		if !here || was == "" {
			continue
		}
		for _, block := range p.Blocks {
			detail, err := d.GetBlock(block)
			if err != nil || !detail.Active {
				continue
			}
			group := db.PlaceTrigger(p.ID)
			fireTrigger(d, block, group, triggerActions(d, block, group), "The computer arrived at '"+p.Name+"'.", now)
		}
	}
}
