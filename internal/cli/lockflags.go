// lockflags.go — the lock flags 'start' takes (§3.4). 'hotkey add' takes the
// same ones and keeps them, to start its block with them at each press
// (§3.13 Hotkey: "optionally with a lock", any kind).
package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/lock"
	"github.com/spf13/cobra"
)

// lockFlagNames are start's lock flags.
var lockFlagNames = []string{"lock", "lock-chars", "text-lock", "lock-words", "lock-text", "password-stdin",
	"restart", "delay", "relock", "for", "until", "range", "range-invert"}

// addStartLockFlags adds start's lock flags to cmd.
func addStartLockFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("lock", false, "Text lock: a challenge to type before unlocking")
	cmd.Flags().Int("lock-chars", lock.DefaultLockChars, "Text lock: challenge length in characters (1 to 5000)")
	addTextLockFlags(cmd)
	cmd.Flags().Bool("password-stdin", false, "Password lock: read the password from standard input")
	cmd.Flags().Bool("restart", false, "Restart lock: on until the computer restarts")
	cmd.Flags().String("delay", "", "Delay lock: unlocking takes this long (1m to 40d)")
	cmd.Flags().String("relock", "", "Re-lock: after unlocking, the text and password locks come on again after this long (e.g. 30m)")
	cmd.Flags().String("for", "", "Timer lock: for this long (e.g. 90m, 2h, 3d, 1w)")
	cmd.Flags().String("until", "", "Timer lock: until this time (HH:MM, or a date: YYYY-MM-DD HH:MM)")
	cmd.Flags().String("range", "", `Range lock: locked at these times, e.g. "mon-fri 09:00-17:00" (22:00-06:00 runs past midnight)`)
	cmd.Flags().Bool("range-invert", false, "Range lock: locked outside those times instead")
	trackLockFlagOrder(cmd) // after every lock flag is defined
}

// lockRequest is what the lock flags ask for. The password itself and the
// boot a restart lock belongs to are read by the caller.
type lockRequest struct {
	now        time.Time
	kinds      []string // lock kinds in the order given (R-LOCK-6)
	text       bool
	style      string
	chars      int
	password   bool
	restart    bool
	extras     db.LockExtras // delay, text lock style, re-lock, range
	until      int64         // timer lock end, wall clock (0: none)
	forSeconds int64         // timer lock given as a duration
	untilSpec  string        // --until as given
}

// parseLockFlags reads and checks the lock flags given to cmd.
func parseLockFlags(cmd *cobra.Command, now time.Time) (lockRequest, error) {
	r := lockRequest{now: now}
	r.password, _ = cmd.Flags().GetBool("password-stdin")
	r.restart, _ = cmd.Flags().GetBool("restart")
	if spec, _ := cmd.Flags().GetString("delay"); spec != "" {
		d, err := parseUserDuration(spec)
		if err != nil {
			return r, fmt.Errorf("invalid --delay duration %q: %w", spec, err)
		}
		if d < time.Minute || d > 40*24*time.Hour {
			return r, fmt.Errorf("--delay must be between 1 minute and 40 days")
		}
		r.extras.Delay = int64(d.Seconds())
	}
	var err error
	if r.text, r.style, r.chars, err = textLockRequested(cmd); err != nil {
		return r, err
	}
	r.extras.Style = r.style
	if spec, _ := cmd.Flags().GetString("relock"); spec != "" {
		d, err := parseUserDuration(spec)
		if err != nil || d < time.Minute || d > 30*24*time.Hour {
			return r, fmt.Errorf("invalid --relock %q (1m to 30d)", spec)
		}
		r.extras.Relock = int64(d / time.Second)
	}
	if spec, _ := cmd.Flags().GetString("range"); spec != "" {
		if r.extras.RangeDays, r.extras.RangeFrom, r.extras.RangeTo, err = parseRange(spec); err != nil {
			return r, err
		}
		r.extras.RangeInvert, _ = cmd.Flags().GetBool("range-invert")
	} else if cmd.Flags().Changed("range-invert") {
		return r, fmt.Errorf("--range-invert goes with --range")
	}
	forDur, _ := cmd.Flags().GetString("for")
	r.untilSpec, _ = cmd.Flags().GetString("until")
	if forDur != "" && r.untilSpec != "" {
		return r, fmt.Errorf("use only one of --for or --until")
	}
	switch {
	case forDur != "":
		d, err := parseUserDuration(forDur)
		if err != nil {
			return r, fmt.Errorf("invalid --for duration %q: %w", forDur, err)
		}
		if d <= 0 {
			return r, fmt.Errorf("--for duration must be positive")
		}
		r.until = now.Add(d).Unix()
		r.forSeconds = int64(d / time.Second)
	case r.untilSpec != "":
		t, err := untilToday(now, r.untilSpec)
		if err != nil {
			return r, err
		}
		r.until = t.Unix()
	}
	r.kinds = lockKindsInOrder(cmd)
	return r, nil
}

// lockArgs returns the lock flags given to cmd as arguments, in the order
// they were given (e.g. ["--lock", "--for", "1h"]): what a hotkey keeps.
func lockArgs(cmd *cobra.Command) []string {
	type given struct {
		seq  int
		args []string
	}
	var all []given
	for i, name := range lockFlagNames {
		f := cmd.Flags().Lookup(name)
		if f == nil || !f.Changed {
			continue
		}
		seq := 1_000_000 + i // flags without an order of their own (lock-chars, relock) go last
		if v, ok := f.Value.(*orderedValue); ok {
			seq = v.seq
		}
		args := []string{"--" + name}
		if f.Value.Type() == "bool" {
			if f.Value.String() != "true" {
				continue
			}
		} else {
			args = append(args, f.Value.String())
		}
		all = append(all, given{seq, args})
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].seq < all[j].seq })
	var out []string
	for _, g := range all {
		out = append(out, g.args...)
	}
	return out
}

// parseLockArgs reads stored lock arguments (a hotkey's) as start would.
func parseLockArgs(args []string, now time.Time) (lockRequest, error) {
	c := &cobra.Command{Use: "locks"}
	addStartLockFlags(c)
	if err := c.ParseFlags(args); err != nil {
		return lockRequest{}, err
	}
	if extra := c.Flags().Args(); len(extra) > 0 {
		return lockRequest{}, fmt.Errorf("unexpected %q among lock flags", strings.Join(extra, " "))
	}
	return parseLockFlags(c, now)
}

// pressLocks describes the locks a press of a hotkey sets, in unlock order,
// for confirmLocks (R-LOCK-12) and the dialog shown at each press.
func pressLocks(block string, r lockRequest) []lockToSet {
	target := "block '" + block + "'"
	relock := ""
	if r.extras.Relock > 0 {
		relock = fmt.Sprintf(", re-lock %s after unlocking", humanDuration(time.Duration(r.extras.Relock)*time.Second))
	}
	var locks []lockToSet
	for i, k := range r.kinds {
		order, after := "", ""
		if len(r.kinds) > 1 {
			order = fmt.Sprintf("%d of %d: ", i+1, len(r.kinds))
		}
		if i > 0 {
			after = "after the " + db.LayerName(r.kinds[i-1]) + " is unlocked"
		}
		switch k {
		case db.LayerTimer:
			var length string
			switch {
			case r.forSeconds > 0 && i > 0:
				length = humanDuration(time.Duration(r.forSeconds)*time.Second) + ", counting from when the " + db.LayerName(r.kinds[i-1]) + " is unlocked"
			case r.forSeconds > 0:
				length = humanDuration(time.Duration(r.forSeconds)*time.Second) + " from the press"
			case strings.Contains(r.untilSpec, "-"):
				length = "until " + time.Unix(r.until, 0).Format("2006-01-02 15:04")
			default:
				length = "until the next " + r.untilSpec + " after the press"
			}
			locks = append(locks, lockToSet{target, order + "timer lock", length})
		case db.LayerText:
			locks = append(locks, lockToSet{target, order + textLockKind(r.style, r.chars) + relock, after})
		case db.LayerPassword:
			length := "the password given when the hotkey was added"
			if after != "" {
				length += ", " + after
			}
			locks = append(locks, lockToSet{target, order + "password lock" + relock, length})
		case db.LayerRestart:
			length := "until the computer restarts"
			if after != "" {
				length = "until a restart " + after
			}
			locks = append(locks, lockToSet{target, order + "restart lock", length})
		case db.LayerRange:
			locks = append(locks, lockToSet{target, order + "range lock", describeRange(r.extras) + " (can't be unlocked then)"})
		case db.LayerDelay:
			length := "unlocking takes " + humanDuration(time.Duration(r.extras.Delay)*time.Second)
			if after != "" {
				length += ", " + after
			}
			locks = append(locks, lockToSet{target, order + "delay lock", length})
		}
	}
	return locks
}

// describeLockList joins locks into one line: "timer lock (1 hour from the
// press), then text lock (…)".
func describeLockList(locks []lockToSet) string {
	parts := make([]string, len(locks))
	for i, l := range locks {
		kind := l.Kind
		if _, rest, ok := strings.Cut(kind, ": "); ok && len(locks) > 1 {
			kind = rest // the order is told by "then"
		}
		parts[i] = kind
		if l.Length != "" {
			parts[i] += " (" + l.Length + ")"
		}
	}
	return strings.Join(parts, ", then ")
}
