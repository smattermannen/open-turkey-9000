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
	cmd.Flags().String("relock", "", "Re-lock: this long after the block is unlocked, all its locks come on again in their order (e.g. 30m)")
	cmd.Flags().String("for", "", "Timer lock: for this long (e.g. 90m, 2h, 3d, 1w)")
	cmd.Flags().String("until", "", "Timer lock: until this time (HH:MM, or a date: YYYY-MM-DD HH:MM)")
	cmd.Flags().String("range", "", `Range lock: locked at these times, e.g. "mon-fri 09:00-17:00" (22:00-06:00 runs past midnight)`)
	cmd.Flags().Bool("range-invert", false, "Range lock: locked outside those times instead")
	trackLockFlagOrder(cmd) // after every lock flag is defined
}

// lockRequest is what the lock flags ask for. The password itself and the
// boot a restart lock belongs to are read by the caller.
type lockRequest struct {
	now time.Time
	// specs: every lock asked for, in the order given, one per flag (a flag
	// given twice is two locks of its kind, §3.4). A timer lock until a
	// time has Until in wall-clock time; password locks have no hash yet.
	specs      []db.QueuedLock
	kinds      []string // lock kinds in the order given (R-LOCK-6), each once
	text       bool
	style      string
	chars      int
	password   bool
	restart    bool
	extras     db.LockExtras // delay, text lock style, re-lock, range
	until      int64         // timer lock end, wall clock (0: none)
	forSeconds int64         // timer lock given as a duration
	untilSpec  string        // the first --until as given
	given      []string      // each spec's flag value as given
}

// parseLockFlags reads and checks the lock flags given to cmd.
func parseLockFlags(cmd *cobra.Command, now time.Time) (lockRequest, error) {
	r := lockRequest{now: now}
	r.password, _ = cmd.Flags().GetBool("password-stdin")
	r.restart, _ = cmd.Flags().GetBool("restart")
	var err error
	if _, _, _, err = textLockRequested(cmd); err != nil {
		return r, err
	}
	if spec, _ := cmd.Flags().GetString("relock"); spec != "" {
		d, err := parseUserDuration(spec)
		if err != nil || d < time.Minute || d > 30*24*time.Hour {
			return r, fmt.Errorf("invalid --relock %q (1m to 30d)", spec)
		}
		r.extras.Relock = int64(d / time.Second)
	}
	if r.specs, r.given, err = lockSpecs(cmd, now); err != nil {
		return r, err
	}
	// The single values: each kind's first lock (what a block's place of
	// that kind takes).
	firstOf := map[string]bool{}
	for _, q := range r.specs {
		if firstOf[q.Kind] {
			continue
		}
		firstOf[q.Kind] = true
		r.kinds = append(r.kinds, q.Kind)
		switch q.Kind {
		case db.LayerTimer:
			if q.For > 0 {
				r.forSeconds, r.until = q.For, now.Unix()+q.For
			} else {
				r.until = q.Until
			}
		case db.LayerText:
			r.text, r.style, r.chars = true, q.Style, q.Chars
			r.extras.Style = q.Style
		case db.LayerDelay:
			r.extras.Delay = q.Delay
		case db.LayerRange:
			r.extras.RangeDays, r.extras.RangeFrom, r.extras.RangeTo, r.extras.RangeInvert = q.RangeDays, q.RangeFrom, q.RangeTo, q.RangeInvert
		}
	}
	r.untilSpec = untilSpecOf(cmd)
	return r, nil
}

// untilSpecOf is the first --until as given ("" without one).
func untilSpecOf(cmd *cobra.Command) string {
	for _, o := range lockOccurrences(cmd) {
		if o.flag == "until" {
			return o.value
		}
	}
	return ""
}

// lockSpecs reads every lock the flags ask for, one per flag given, in the
// order given (§3.4, §4.6: a flag given twice is a second lock of its
// kind). --lock-chars, --lock-words and --range-invert apply to every text
// or range lock; --lock-words alone is a text lock of words. A timer lock
// for a duration keeps For, one until a time Until (wall clock). Password
// locks come without their hash: the caller reads one password for each.
func lockSpecs(cmd *cobra.Command, now time.Time) ([]db.QueuedLock, []string, error) {
	occ := lockOccurrences(cmd)
	chars, _ := cmd.Flags().GetInt("lock-chars")
	words := false
	if cmd.Flags().Lookup("lock-words") != nil {
		words, _ = cmd.Flags().GetBool("lock-words")
	}
	invert := false
	if cmd.Flags().Lookup("range-invert") != nil {
		invert, _ = cmd.Flags().GetBool("range-invert")
	}
	plainText := false
	for _, o := range occ {
		plainText = plainText || o.flag == "lock" || o.flag == "text-lock"
	}
	var out []db.QueuedLock
	var given []string // each lock's flag value as given (an --until's "18:00")
	ranges := 0
	for _, o := range occ {
		q := db.QueuedLock{Kind: o.kind}
		switch o.flag {
		case "for":
			d, err := parseUserDuration(o.value)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid --for duration %q: %w", o.value, err)
			}
			if d <= 0 {
				return nil, nil, fmt.Errorf("--for duration must be positive")
			}
			q.For = int64(d / time.Second)
		case "until":
			t, err := untilToday(now, o.value)
			if err != nil {
				return nil, nil, err
			}
			q.Until = t.Unix()
		case "lock", "text-lock", "lock-words":
			if o.flag == "lock-words" && plainText {
				continue // the style of the --lock given with it
			}
			if err := lock.CheckLockChars(chars); err != nil {
				return nil, nil, err
			}
			q.Chars = chars
			if words {
				q.Style = lock.StyleWords
			}
		case "lock-text":
			style, err := lock.CustomStyle(o.value)
			if err != nil {
				return nil, nil, err
			}
			q.Style, q.Chars = style, len(style)-len(lock.StyleCustom)
		case "delay":
			d, err := parseUserDuration(o.value)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid --delay duration %q: %w", o.value, err)
			}
			if d < time.Minute || d > 40*24*time.Hour {
				return nil, nil, fmt.Errorf("--delay must be between 1 minute and 40 days")
			}
			q.Delay = int64(d.Seconds())
		case "range":
			var err error
			if q.RangeDays, q.RangeFrom, q.RangeTo, err = parseRange(o.value); err != nil {
				return nil, nil, err
			}
			q.RangeInvert = invert
			ranges++
		}
		out = append(out, q)
		given = append(given, o.value)
	}
	if invert && ranges == 0 {
		return nil, nil, fmt.Errorf("--range-invert goes with --range")
	}
	return out, given, nil
}

// splitSpecs splits a new block's locks into those that take their kind's
// place and those that wait in the queue: from the first second lock of a
// kind on, every lock waits, so the order stays as given (§3.4).
func splitSpecs(specs []db.QueuedLock) (placed, queued []db.QueuedLock) {
	seen := map[string]bool{}
	for i, q := range specs {
		if seen[q.Kind] {
			return specs[:i], specs[i:]
		}
		seen[q.Kind] = true
	}
	return specs, nil
}

// passwordLocks counts the password locks among specs.
func passwordLocks(specs []db.QueuedLock) int {
	n := 0
	for _, q := range specs {
		if q.Kind == db.LayerPassword {
			n++
		}
	}
	return n
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
		arg := func(value string) []string {
			if f.Value.Type() == "bool" {
				return []string{"--" + name}
			}
			return []string{"--" + name, value}
		}
		// A flag given twice is two locks (§3.4): each time it was given.
		if v, ok := f.Value.(*orderedValue); ok && len(v.sets) > 0 {
			for _, s := range v.sets {
				if (f.Value.Type() == "bool" && s.value != "true") || s.value == "" {
					continue
				}
				all = append(all, given{s.seq, arg(s.value)})
			}
			continue
		}
		if f.Value.Type() == "bool" && f.Value.String() != "true" {
			continue
		}
		// Flags without an order of their own (lock-chars, relock) go last.
		all = append(all, given{1_000_000 + i, arg(f.Value.String())})
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
	var locks []lockToSet
	for i, q := range r.specs {
		k := q.Kind
		order, after := "", ""
		if len(r.specs) > 1 {
			order = fmt.Sprintf("%d of %d: ", i+1, len(r.specs))
		}
		if i > 0 {
			after = "after the " + db.LayerName(r.specs[i-1].Kind) + " is unlocked"
		}
		switch k {
		case db.LayerTimer:
			var length string
			switch {
			case q.For > 0 && i > 0:
				length = humanDuration(time.Duration(q.For)*time.Second) + ", counting from when the " + db.LayerName(r.specs[i-1].Kind) + " is unlocked"
			case q.For > 0:
				length = humanDuration(time.Duration(q.For)*time.Second) + " from the press"
			case i < len(r.given) && !strings.Contains(r.given[i], "-"):
				length = "until the next " + r.given[i] + " after the press"
			default:
				length = "until " + time.Unix(q.Until, 0).Format("2006-01-02 15:04")
			}
			locks = append(locks, lockToSet{target, order + "timer lock", length})
		case db.LayerText:
			locks = append(locks, lockToSet{target, order + textLockKind(q.Style, q.Chars), after})
		case db.LayerPassword:
			length := "the password given when the hotkey was added"
			if after != "" {
				length += ", " + after
			}
			locks = append(locks, lockToSet{target, order + "password lock", length})
		case db.LayerRestart:
			length := "until the computer restarts"
			if after != "" {
				length = "until a restart " + after
			}
			locks = append(locks, lockToSet{target, order + "restart lock", length})
		case db.LayerRange:
			rg := db.LockExtras{RangeDays: q.RangeDays, RangeFrom: q.RangeFrom, RangeTo: q.RangeTo, RangeInvert: q.RangeInvert}
			locks = append(locks, lockToSet{target, order + "range lock", describeRange(rg) + " (can't be unlocked then)"})
		case db.LayerDelay:
			length := "unlocking takes " + humanDuration(time.Duration(q.Delay)*time.Second)
			if after != "" {
				length += ", " + after
			}
			locks = append(locks, lockToSet{target, order + "delay lock", length})
		}
	}
	return append(locks, relockToSet(target, r.extras.Relock)...)
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
