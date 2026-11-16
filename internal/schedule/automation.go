package schedule

import (
	"slices"
	"time"
)

// Automation is what switches an active block by itself (§3.6, owner
// 2026-11-16): its windows and its places, from its default. A block
// disabled by default is enabled while a window or a place switches it; one
// enabled by default is lifted then. With windows and places, switching
// needs both or either.
type Automation struct {
	Windows   []Window
	Places    []int // the ids of its places
	DefaultOn bool  // enabled by default: a window or a place turns it off
	NeedBoth  bool  // with windows and places: switching needs both
}

// Continuous reports whether nothing switches the block.
func (a Automation) Continuous() bool { return len(a.Windows) == 0 && len(a.Places) == 0 }

// Switched reports whether the windows and places switch the block from
// its default: inWindow (one of its windows is open), atPlace (the computer
// is at one of its places).
func (a Automation) Switched(inWindow, atPlace bool) bool {
	hasWindows, hasPlaces := len(a.Windows) > 0, len(a.Places) > 0
	switch {
	case hasWindows && hasPlaces && a.NeedBoth:
		return inWindow && atPlace
	case hasWindows && hasPlaces:
		return inWindow || atPlace
	case hasWindows:
		return inWindow
	}
	return hasPlaces && atPlace
}

// Enabled reports whether the block is enabled (while it is on).
func (a Automation) Enabled(inWindow, atPlace bool) bool {
	if a.Continuous() {
		return true
	}
	return a.Switched(inWindow, atPlace) != a.DefaultOn
}

// UnknownPlace is what "at one of its places" is taken to be while no
// network can be seen: whatever keeps the block enabled (fail closed:
// switching Wi-Fi off lifts nothing).
func (a Automation) UnknownPlace() bool { return !a.DefaultOn }

// InAny reports whether one of the windows is open at t.
func InAny(ws []Window, t time.Time) bool {
	for _, w := range ws {
		if w.ActiveAt(t) {
			return true
		}
	}
	return false
}

// inAnyAt is InAny on the weekly grid (day 0 is Sunday).
func inAnyAt(ws []Window, day, minute int) bool {
	for _, w := range ws {
		if w.Days&(1<<uint(day)) != 0 && minute >= w.Start && minute < w.End {
			return true
		}
	}
	return false
}

// AutomationWeakens reports whether changing a block's automation from a to
// b would lift it at some time of the week, somewhere, where a has it
// enabled: at none of the places, at each one, or with no network seen.
// Windows are in 5-minute steps, so the grid is looked at every 5 minutes.
// Such a change weakens the block (R-LOCK-1: refused while it is locked);
// any other only adds enabled time.
func AutomationWeakens(a, b Automation) bool {
	type at struct{ a, b bool }
	locations := []at{{false, false}, {a.UnknownPlace(), b.UnknownPlace()}}
	seen := map[int]bool{}
	for _, id := range append(slices.Clone(a.Places), b.Places...) {
		if !seen[id] {
			seen[id] = true
			locations = append(locations, at{slices.Contains(a.Places, id), slices.Contains(b.Places, id)})
		}
	}
	for day := 0; day < 7; day++ {
		for minute := 0; minute < 1440; minute += 5 {
			inA, inB := inAnyAt(a.Windows, day, minute), inAnyAt(b.Windows, day, minute)
			for _, l := range locations {
				if a.Enabled(inA, l.a) && !b.Enabled(inB, l.b) {
					return true
				}
			}
		}
	}
	return false
}
