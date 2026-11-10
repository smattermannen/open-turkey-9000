// Package freeze holds the pure rules of Frozen Chicken: the daemon keeps every
// session locked until a set time.
//
// Frozen Chicken uses the wall clock, not db.TrustedNow. Trusted time ignores
// time powered off, which would turn "frozen for 2 hours" into an open-ended
// lockout for someone who shuts the computer down. Two limits keep a mistake
// recoverable: MaxDuration, and the BypassParam kernel parameter.
package freeze

import (
	"fmt"
	"github.com/smattermannen/kc9k/internal/schedule"
	"strings"
	"time"
)

const (
	MinDuration = time.Minute
	MaxDuration = 24 * time.Hour

	// BypassParam on the kernel command line (edit the boot entry in GRUB)
	// stops the freeze from being enforced during that boot.
	BypassParam = "kc9k.nofreeze"
)

// The ways a freeze keeps the computer unusable.
const (
	ModeLock     = "lock"
	ModeLogout   = "logout"
	ModeShutdown = "shutdown"
)

// ValidMode reports whether mode is known.
func ValidMode(mode string) bool {
	return mode == ModeLock || mode == ModeLogout || mode == ModeShutdown
}

// Validate checks a requested freeze length.
func Validate(d time.Duration) error {
	if d < MinDuration || d > MaxDuration {
		return fmt.Errorf("freeze duration must be between %s and %s", MinDuration, MaxDuration)
	}
	return nil
}

// Bypassed reports whether a kernel command line carries BypassParam.
func Bypassed(cmdline string) bool {
	for _, f := range strings.Fields(cmdline) {
		if f == BypassParam {
			return true
		}
	}
	return false
}

// Engaged reports whether sessions must be locked right now.
func Engaged(now, until int64, cmdline string) bool {
	return now < until && !Bypassed(cmdline)
}

// WindowEnd returns when the window w, open at t, closes: the same day at
// w.End on the clock, where 24:00 is the next midnight. It is built from the
// clock time, not counted from midnight: on a day the clocks change, the two
// are an hour apart.
func WindowEnd(w schedule.Window, t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), w.End/60, w.End%60, 0, 0, t.Location())
}

// Scheduled returns how long a scheduled freeze must run at t: until the
// latest end among the windows open at t, with that window's mode. ok is
// false when no window is open. Back-to-back windows (Monday 23:00-24:00,
// Tuesday 00:00-06:00) chain, because the next one is open when the first
// ends.
func Scheduled(windows []schedule.Window, modes []string, t time.Time) (until time.Time, mode string, ok bool) {
	for i, w := range windows {
		if !w.ActiveAt(t) {
			continue
		}
		if end := WindowEnd(w, t); !ok || end.After(until) {
			until, mode, ok = end, modes[i], true
		}
	}
	return until, mode, ok
}
