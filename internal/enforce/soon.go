// soon.go — what the extension needs to show time left: the blocks that
// aren't stopping pages now but will, with how long until they do, and when
// the active ones end.
package enforce

import (
	"log"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/quota"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// SoonRule is a block that will take its pages away in Seconds: an open
// quota session running out (Kind "quota": Seconds only pass while its
// sites are in use), a break ending ("break"), or a schedule about to start
// ("schedule").
type SoonRule struct {
	Block   string     `json:"block"`
	Kind    string     `json:"kind"`
	Seconds int64      `json:"seconds"`
	Lists   []ListRule `json:"lists"`
}

// SoonLead is how far ahead a schedule's start is announced.
const SoonLead = 2 * time.Hour

// pause is an active block that is lifted for now.
type pause struct {
	block db.ActiveBlockDetail
	left  int64 // seconds until it is back
}

// pausedBlocks returns the active blocks that are lifted now (a Pomodoro
// break, a break taken, an unlocked block waiting to relock).
func pausedBlocks(d *db.DB, active []db.ActiveBlockDetail, trusted int64) []pause {
	var out []pause
	for _, b := range active {
		if left := pausedFor(d, b.BlockName, trusted); left > 0 {
			out = append(out, pause{b, left})
		}
	}
	return out
}

// pausedFor returns how many seconds a block stays lifted, 0 when it isn't.
func pausedFor(d *db.DB, name string, trusted int64) int64 {
	var left int64
	if p, err := d.GetPomodoro(name); err != nil {
		log.Printf("pomodoro: %v", err)
	} else if p != nil {
		left = quota.BreakLeft(p.On, p.Off, p.Anchor, trusted)
	}
	if until, err := d.PausedUntil(name); err != nil {
		log.Printf("pause: %v", err)
	} else if until-trusted > left {
		left = until - trusted
	}
	return left
}

// soonRules collects the SoonRules at now.
func soonRules(d *db.DB, active []db.ActiveBlockDetail, now time.Time, trusted int64) []SoonRule {
	out := []SoonRule{}
	on := map[string]bool{}
	for _, b := range active {
		on[b.BlockName] = true
	}
	for _, p := range pausedBlocks(d, active, trusted) {
		if lists := listRules(p.block); len(lists) > 0 {
			out = append(out, SoonRule{Block: p.block.BlockName, Kind: "break", Seconds: p.left, Lists: lists})
		}
	}
	for _, b := range openQuotaBlocks(d) {
		q, err := d.GetQuota(b.Name)
		if err != nil || q == nil {
			continue
		}
		s := q.Step(now.Unix(), quota.DayOf(now))
		lists := listRules(db.ActiveBlockDetail{BlockName: b.Name, Lists: b.Lists})
		if s.Open && len(lists) > 0 {
			out = append(out, SoonRule{Block: b.Name, Kind: "quota", Seconds: s.Remaining(), Lists: lists})
		}
	}
	all, err := d.AllSchedules()
	if err != nil {
		log.Printf("soon: %v", err)
		return out
	}
	windows := map[string][]schedule.Window{}
	for _, s := range all {
		windows[s.BlockName] = append(windows[s.BlockName], s.Window())
	}
	for name, ws := range windows {
		if on[name] {
			continue
		}
		for m := time.Duration(0); m <= SoonLead; m += time.Minute {
			at := now.Truncate(time.Minute).Add(m)
			if !schedule.Enforced(ws, at) {
				continue
			}
			detail, err := d.GetBlock(name)
			if err != nil {
				break
			}
			lists := listRules(db.ActiveBlockDetail{BlockName: name, Lists: detail.Lists})
			if len(lists) > 0 {
				out = append(out, SoonRule{Block: name, Kind: "schedule", Seconds: max(int64(at.Sub(now).Seconds()), 0), Lists: lists})
			}
			break
		}
	}
	return out
}

// blockEnds returns when each active block ends, for those that have an end:
// a stop that was asked for and is waiting out its delay, or the close of
// the schedule window that turned the block on.
func blockEnds(d *db.DB, active []db.ActiveBlockDetail, now time.Time, trusted int64) map[string]int64 {
	out := map[string]int64{}
	for _, b := range active {
		if extras, err := d.GetLockExtras(b.BlockName); err == nil && extras.StopAt > trusted {
			out[b.BlockName] = now.Unix() + extras.StopAt - trusted
			continue
		}
		if !b.Scheduled || b.Locked || b.LockedUntil > trusted {
			continue
		}
		schedules, err := d.ListSchedules(b.BlockName)
		if err != nil || len(schedules) == 0 {
			continue
		}
		var ws []schedule.Window
		for _, s := range schedules {
			ws = append(ws, s.Window())
		}
		for m := time.Duration(0); m <= 24*time.Hour; m += time.Minute {
			at := now.Truncate(time.Minute).Add(m)
			if !schedule.Enforced(ws, at) {
				if m > 0 {
					out[b.BlockName] = at.Unix()
				}
				break
			}
		}
	}
	return out
}
