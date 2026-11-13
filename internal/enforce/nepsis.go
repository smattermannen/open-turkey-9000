// nepsis.go — bypass detection in the daemon: count recent events of the
// chosen detectors (db/events.go) and climb the ladder of steps when there
// are too many. The global setting counts every block's events; a block's
// own counts that block's while it is on. Each scope keeps its level, when
// it last fired (only newer events count after that), and a quiet clock:
// after the quiet time without firing, it falls back a step.
//
// It runs every daemon cycle, and at once when the extension reports a
// blocked page (see hits.go), so a step's close-tab closes that page's tab.
package enforce

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
)

// nepsisMu keeps the cycle and the rules server from judging a scope at once.
var nepsisMu sync.Mutex

// enforceNepsis checks the global and each active block's bypass detection.
func enforceNepsis(d *db.DB, now time.Time, active []db.ActiveBlockDetail) {
	if err := d.PruneEvents(now); err != nil {
		log.Printf("nepsis: %v", err)
	}
	scopes := []string{""}
	for _, b := range active {
		scopes = append(scopes, b.BlockName)
	}
	checkScopes(d, now, scopes)
}

// checkScopes judges each scope ("" is the global one), and reports whether
// a step that fired asks to close the tab of the page that tripped it.
func checkScopes(d *db.DB, now time.Time, scopes []string) (closeTab bool) {
	nepsisMu.Lock()
	defer nepsisMu.Unlock()
	var warnings []string
	fired := false
	for _, scope := range scopes {
		n, err := loadNepsis(d, scope)
		if err != nil {
			log.Printf("nepsis: %v", err)
			continue
		}
		if !n.On() {
			continue
		}
		r := checkNepsis(d, now, scope, n)
		closeTab = closeTab || r.closeTab
		fired = fired || r.fired
		if r.warning != "" {
			warnings = append(warnings, r.warning)
		}
	}
	// R-WARN-2: every new event is announced, once even when several
	// ladders count it; a step that fired has said more already.
	if !fired && len(warnings) > 0 {
		notifyDesktop("Event triggered, be careful", strings.Join(warnings, "\n"))
	}
	return closeTab
}

// judgement is what checking one scope found.
type judgement struct {
	closeTab bool   // a step that fired closes the tab
	fired    bool   // a step fired (and was announced)
	warning  string // new events counted without a step firing
}

func loadNepsis(d *db.DB, scope string) (db.Nepsis, error) {
	if scope == "" {
		return d.GlobalNepsis()
	}
	return d.BlockNepsis(scope)
}

// ladder is one scope's place on its ladder; times are unix seconds.
type ladder struct {
	Level int   // the step that fires next (0-based)
	Fired int64 // when a step last fired
	Calm  int64 // when the quiet clock last started: firing or falling back
	Seen  int64 // the newest event announced (an event id)
}

func loadLadder(d *db.DB, scope string) ladder {
	var l ladder
	value, _ := d.GetSetting(db.NepsisLadderKey(scope))
	parts := strings.Fields(value)
	if len(parts) >= 3 {
		l.Level, _ = strconv.Atoi(parts[0])
		l.Fired, _ = strconv.ParseInt(parts[1], 10, 64)
		l.Calm, _ = strconv.ParseInt(parts[2], 10, 64)
	}
	if len(parts) >= 4 {
		l.Seen, _ = strconv.ParseInt(parts[3], 10, 64)
	}
	return l
}

func saveLadder(d *db.DB, scope string, l ladder) {
	if err := d.SetSetting(db.NepsisLadderKey(scope), fmt.Sprintf("%d %d %d %d", l.Level, l.Fired, l.Calm, l.Seen)); err != nil {
		log.Printf("nepsis: %v", err)
	}
}

// settle falls back a step for every quiet time that passed since Calm.
func (l ladder) settle(n db.Nepsis, now int64) ladder {
	l.Level = min(l.Level, len(n.Steps)-1)
	if n.QuietMinutes <= 0 {
		return l
	}
	quiet := int64(n.QuietMinutes) * 60
	for l.Level > 0 && now-l.Calm >= quiet {
		l.Level--
		l.Calm += quiet
	}
	return l
}

// NepsisState returns a scope's current step (0-based) and how many events
// count towards it now.
func NepsisState(d *db.DB, now time.Time, scope string, n db.Nepsis) (level, count int, err error) {
	l := loadLadder(d, scope).settle(n, now.Unix())
	count, err = d.CountEventsOf(scope, since(l, n.Step(l.Level), now), n.Detectors)
	return l.Level, count, err
}

// since is when events start to count for a step: within its window, and
// after the last time a step fired.
func since(l ladder, step db.NepsisStep, now time.Time) int64 {
	from := now.Add(-step.Window()).Unix()
	if l.Fired >= from {
		from = l.Fired + 1
	}
	return from
}

// checkNepsis fires the scope's current step if enough events happened, or
// else says so when new events were counted.
func checkNepsis(d *db.DB, now time.Time, scope string, n db.Nepsis) (r judgement) {
	before := loadLadder(d, scope)
	l := before.settle(n, now.Unix())
	step := n.Step(l.Level)
	count, err := d.CountEventsOf(scope, since(l, step, now), n.Detectors)
	if err != nil {
		log.Printf("nepsis: %v", err)
		return r
	}
	last, err := d.LastEventIDOf(scope, n.Detectors)
	if err != nil {
		log.Printf("nepsis: %v", err)
		return r
	}
	fresh := last > l.Seen
	l.Seen = max(l.Seen, last)
	who := "Your blocks"
	if scope != "" {
		who = fmt.Sprintf("Block '%s'", scope)
	}
	if count < step.Count {
		if fresh && count > 0 {
			r.warning = fmt.Sprintf("%s: %d of %d within %s before step %d of %d (%s).",
				who, count, step.Count, fmtMinutes(step.Minutes), l.Level+1, len(n.Steps), describeActions(step))
		}
		if l != before {
			saveLadder(d, scope, l)
		}
		return r
	}
	fired := l.Level
	l.Fired, l.Calm = now.Unix(), now.Unix()
	l.Level = min(l.Level+1, len(n.Steps)-1)
	saveLadder(d, scope, l)
	r.fired = true

	why := fmt.Sprintf("%s stopped %d things within %s (step %d of %d): %s.",
		who, count, fmtMinutes(step.Minutes), fired+1, len(n.Steps), describeActions(step))
	log.Printf("nepsis: %s", why)
	notifyDesktop("Killer Chicken 9000: bypass prevention", why)
	for _, a := range step.Actions {
		if a.Kind == db.ConsequenceCloseTab {
			r.closeTab = true
			continue
		}
		if err := Run(d, Consequence{Kind: a.Kind, Minutes: a.Minutes}, ""); err != nil {
			log.Printf("nepsis: %v", err)
		}
	}
	return r
}

func describeActions(step db.NepsisStep) string {
	var did []string
	for _, a := range step.Actions {
		did = append(did, Consequence{Kind: a.Kind, Minutes: a.Minutes}.String())
	}
	return strings.Join(did, ", ")
}

// fmtMinutes writes a step's window: "10 minutes", "2 hours".
func fmtMinutes(m int) string {
	switch {
	case m == 1:
		return "1 minute"
	case m%60 == 0 && m >= 120:
		return fmt.Sprintf("%d hours", m/60)
	case m == 60:
		return "1 hour"
	}
	return fmt.Sprintf("%d minutes", m)
}
