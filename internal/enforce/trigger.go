// trigger.go — what a block's apps and its window titles do when they fire
// (owner, 2026-11-16; §1 "any trigger to any action"): their own action
// (close the app or the window) and any consequences the user ticked,
// announced as a list's are (R-WARN-1). A place rule has no actions: it
// only enables or lifts its block (§3.6).
package enforce

import (
	"log"
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
		if a.Name != db.TriggerClose {
			out = append(out, Consequence{Kind: a.Name, Minutes: a.Minutes, Param: a.Param})
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

// customRan holds when a custom action last ran for a block and its
// trigger (a list, apps, window titles): at most once a minute (R-CUSTOM-2).
var customRan = struct {
	sync.Mutex
	at map[string]time.Time
}{at: map[string]time.Time{}}

// customDue reports whether a custom action of key may run now, and notes
// that it does.
func customDue(key string, now time.Time) bool {
	customRan.Lock()
	defer customRan.Unlock()
	if last, ok := customRan.at[key]; ok && now.Sub(last) < time.Minute {
		return false
	}
	customRan.at[key] = now
	return true
}
