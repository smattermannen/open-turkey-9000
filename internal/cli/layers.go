// layers.go — layered sequential locks in the CLI (§3.4, R-LOCK-6/7).
//
// The user's order is the order of the lock flags on the command line
// (`start B --lock --for 1h`: text lock first, then a timer lock of an hour
// counted from unlocking it). Locks added to an active block go last.
package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// lockFlagKind maps each lock flag to the lock kind it sets.
var lockFlagKind = map[string]string{
	"for": db.LayerTimer, "until": db.LayerTimer,
	"lock": db.LayerText, "text-lock": db.LayerText, "lock-words": db.LayerText, "lock-text": db.LayerText,
	"password-stdin": db.LayerPassword,
	"restart":        db.LayerRestart,
	"delay":          db.LayerDelay,
	"range":          db.LayerRange,
	"range-invert":   db.LayerRange,
}

// flagSeq numbers flag assignments, so the order they were given in can be
// read back after parsing.
var flagSeq int

// orderedValue wraps a flag's value and remembers when it was last set.
type orderedValue struct {
	pflag.Value
	seq int
}

func (v *orderedValue) Set(s string) error {
	flagSeq++
	v.seq = flagSeq
	return v.Value.Set(s)
}

// trackLockFlagOrder makes the command's lock flags remember their order.
// Call it after the flags are defined.
func trackLockFlagOrder(cmd *cobra.Command) {
	for name := range lockFlagKind {
		if f := cmd.Flags().Lookup(name); f != nil {
			if _, ok := f.Value.(*orderedValue); !ok {
				f.Value = &orderedValue{Value: f.Value}
			}
		}
	}
}

// lockKindsInOrder returns the lock kinds the given flags ask for, in the
// order they were given.
func lockKindsInOrder(cmd *cobra.Command) []string {
	type given struct {
		kind string
		seq  int
	}
	var all []given
	for name, kind := range lockFlagKind {
		f := cmd.Flags().Lookup(name)
		if f == nil || !f.Changed {
			continue
		}
		seq := 0
		if v, ok := f.Value.(*orderedValue); ok {
			seq = v.seq
		}
		all = append(all, given{kind, seq})
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].seq != all[j].seq {
			return all[i].seq < all[j].seq
		}
		return all[i].kind < all[j].kind
	})
	var kinds []string
	for _, g := range all {
		kinds = db.AppendLayers(kinds, g.kind)
	}
	return kinds
}

// describeLayers shows a block's locks in unlock order, e.g.
// "text lock → timer lock (1h after the text lock)".
func describeLayers(layers []string, detail *db.BlockDetail, e db.LockExtras) string {
	parts := make([]string, len(layers))
	for i, k := range layers {
		parts[i] = db.LayerName(k)
		if k == db.LayerTimer && e.TimerAfter > 0 && i > 0 {
			parts[i] += fmt.Sprintf(" (%s after the %s)", time.Duration(e.TimerAfter)*time.Second, db.LayerName(layers[i-1]))
		}
		if k == db.LayerRestart && e.RestartBoot == db.RestartPending && i > 0 {
			parts[i] += " (a restart after the " + db.LayerName(layers[i-1]) + ")"
		}
	}
	return strings.Join(parts, " → ")
}

// liftTextLayer unlocks the text or password lock (kind) of an active
// block (db.LiftTextLayer) and saves it.
func liftTextLayer(database *db.DB, detail *db.BlockDetail, e *db.LockExtras, kind string, trusted int64, bootID string) error {
	db.LiftTextLayer(detail, e, kind, trusted, bootID)
	return database.SaveLayers(detail, *e)
}

// parseRange reads a range lock: "mon-fri 09:00-17:00" (days as for
// schedules; "22:00-06:00" runs past midnight).
func parseRange(spec string) (days, from, to int, err error) {
	fields := strings.Fields(spec)
	if len(fields) < 2 {
		return 0, 0, 0, fmt.Errorf(`invalid --range %q (want days and times, e.g. "mon-fri 09:00-17:00")`, spec)
	}
	times := fields[len(fields)-1]
	start, end, ok := strings.Cut(times, "-")
	if !ok {
		return 0, 0, 0, fmt.Errorf("invalid --range times %q (want HH:MM-HH:MM)", times)
	}
	if days, err = schedule.ParseDays(strings.Join(fields[:len(fields)-1], " ")); err != nil {
		return 0, 0, 0, err
	}
	if from, err = schedule.ParseTimeOfDay(start); err != nil {
		return 0, 0, 0, err
	}
	if to, err = schedule.ParseEndTime(end); err != nil {
		return 0, 0, 0, err
	}
	if to == 24*60 {
		to = 0
	}
	if from == to {
		return 0, 0, 0, fmt.Errorf("a range lock needs two different times")
	}
	return days, from, to, nil
}

// describeRange shows a range lock, e.g. "mon,tue 09:00-17:00" or
// "outside daily 22:00-06:00".
func describeRange(e db.LockExtras) string {
	text := fmt.Sprintf("%s %02d:%02d-%02d:%02d", schedule.FormatDays(e.RangeDays), e.RangeFrom/60, e.RangeFrom%60, e.RangeTo/60, e.RangeTo%60)
	if e.RangeInvert {
		return "outside " + text
	}
	return text
}
