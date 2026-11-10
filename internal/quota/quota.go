// Package quota holds the pure rules of daily quotas.
//
// A block with a quota stays on all the time. Its owner opens it with a quota
// session. The daemon spends only the time the block's sites or apps are in
// use (see enforce/quota.go); without LibreWolf, which runs the extension that
// sees tabs, a session with websites spends all of its time.
package quota

import (
	"fmt"
	"time"
)

const (
	MinLimit = time.Minute
	MaxLimit = 24 * time.Hour

	// MaxStep caps what one daemon cycle can spend, so suspend or a clock jump
	// doesn't burn the quota in one go.
	MaxStep = 30

	// NoPending means no limit change is waiting for midnight.
	NoPending = -1
)

// State is one block's quota. Times are unix seconds; Limit, Used and
// PendingLimit are seconds. A PendingLimit of 0 removes the quota at midnight.
type State struct {
	Limit        int64
	Used         int64
	Day          string
	Open         bool
	SessionEnd   int64 // 0 means the session runs until the quota is spent
	LastTick     int64
	PendingLimit int64
}

// New returns a fresh quota.
func New(limit, now int64, day string) State {
	return State{Limit: limit, Day: day, LastTick: now, PendingLimit: NoPending}
}

// Validate checks a requested limit.
func Validate(d time.Duration) error {
	if d < MinLimit || d > MaxLimit {
		return fmt.Errorf("daily quota must be between %s and %s", MinLimit, MaxLimit)
	}
	return nil
}

// DayOf returns the local calendar day that a quota belongs to.
func DayOf(t time.Time) string { return t.Format("2006-01-02") }

// Remaining returns the unspent seconds.
func (s State) Remaining() int64 {
	if s.Used >= s.Limit {
		return 0
	}
	return s.Limit - s.Used
}

// SetLimit applies a new limit: a lower one now, a higher one at midnight, so
// a quota can't be raised on impulse.
func (s State) SetLimit(limit int64) State {
	if limit <= s.Limit {
		s.Limit, s.PendingLimit = limit, NoPending
	} else {
		s.PendingLimit = limit
	}
	return s
}

// Step advances a quota to now: it starts a new day when the date changes
// and closes a session that reached its end. It spends nothing: see Spend.
func (s State) Step(now int64, day string) State {
	if day != s.Day {
		s.Day, s.Used, s.Open, s.SessionEnd = day, 0, false, 0
		if s.PendingLimit != NoPending {
			s.Limit, s.PendingLimit = s.PendingLimit, NoPending
		}
	}
	if s.Open && s.SessionEnd != 0 && now >= s.SessionEnd {
		s.Open, s.SessionEnd = false, 0
	}
	s.LastTick = now
	return s
}

// Elapsed is the wall time since the last step, capped at MaxStep so suspend
// or a clock jump doesn't burn the quota in one go.
func (s State) Elapsed(now int64) int64 {
	return min(max(now-s.LastTick, 0), MaxStep)
}

// Spend spends seconds of an open session (at most 2*MaxStep at once, so a
// forged or stale report can't burn much), closing it when the quota runs out.
func (s State) Spend(seconds int64) State {
	if !s.Open || seconds <= 0 {
		return s
	}
	s.Used += min(seconds, 2*MaxStep)
	if s.Used >= s.Limit {
		s.Open, s.SessionEnd = false, 0
	}
	return s
}
