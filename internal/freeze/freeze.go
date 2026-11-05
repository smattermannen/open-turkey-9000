// Package freeze holds the pure rules of Frozen Turkey: the daemon keeps every
// session locked until a set time.
//
// Frozen Turkey uses the wall clock, not db.TrustedNow. Trusted time ignores
// time powered off, which would turn "frozen for 2 hours" into an open-ended
// lockout for someone who shuts the computer down. Two limits keep a mistake
// recoverable: MaxDuration, and the BypassParam kernel parameter.
package freeze

import (
	"fmt"
	"strings"
	"time"
)

const (
	MinDuration = time.Minute
	MaxDuration = 24 * time.Hour

	// BypassParam on the kernel command line (edit the boot entry in GRUB)
	// stops the freeze from being enforced during that boot.
	BypassParam = "openturkey.nofreeze"
)

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
