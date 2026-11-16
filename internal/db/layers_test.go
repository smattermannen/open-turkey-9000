package db

import (
	"reflect"
	"testing"
	"time"
)

// R-LOCK-6: locks unlock one at a time in the user's order; a duration
// timer lock behind another counts from its unlocking; a restart lock
// behind another needs a boot after it.
func TestLockLayers(t *testing.T) {
	const now, boot = 1000, "boot-a"
	detail := &BlockDetail{Active: true, Locked: true, LockChars: 10}
	e := LockExtras{Order: "text,timer,restart", TimerAfter: 3600, RestartBoot: RestartPending}
	if got := LayersOn(detail, e, now, boot); !reflect.DeepEqual(got, []string{LayerText, LayerTimer, LayerRestart}) {
		t.Fatalf("layers on = %v", got)
	}
	if AdvanceLayers(detail, &e, now, boot) {
		t.Error("advanced while the text lock holds")
	}
	detail.Locked, detail.LockChars = false, 0 // text unlocked
	if !AdvanceLayers(detail, &e, now+10, boot) || detail.LockedUntil != now+10+3600 || e.TimerAfter != 0 {
		t.Fatalf("timer after text: until %d, after %d", detail.LockedUntil, e.TimerAfter)
	}
	if e.RestartBoot != RestartPending {
		t.Error("restart lock started while the timer holds")
	}
	if !AdvanceLayers(detail, &e, now+5000, boot) || e.RestartBoot != boot {
		t.Fatalf("restart after timer: %q", e.RestartBoot)
	}
	if !LayerOn(LayerRestart, detail, e, now+5000, boot) || LayerOn(LayerRestart, detail, e, now+5000, "boot-b") {
		t.Error("restart lock should hold this boot only")
	}

	// Blocks locked before layers: timer, restart, password, text, delay.
	old := &BlockDetail{Active: true, Locked: true, LockedUntil: 2000}
	if got := Layers(old, LockExtras{PasswordHash: "x", Delay: 60}); !reflect.DeepEqual(got, []string{LayerTimer, LayerPassword, LayerText, LayerDelay}) {
		t.Errorf("legacy order = %v", got)
	}
	// Kinds missing from a stored order still count, at the end.
	if got := Layers(old, LockExtras{Order: "text", PasswordHash: "x"}); !reflect.DeepEqual(got, []string{LayerText, LayerTimer, LayerPassword}) {
		t.Errorf("partial order = %v", got)
	}
}

// Re-lock with layers: a lifted text lock stays lifted while the password
// after it holds; once both are unlocked they come back together.
func TestLayerRelock(t *testing.T) {
	const boot = "b"
	detail := &BlockDetail{Active: true, Locked: true, LockChars: 10}
	e := LockExtras{Order: "text,password", PasswordHash: "x", Relock: 600, LiftedKinds: "text"}
	if got := LayersOn(detail, e, 100, boot); !reflect.DeepEqual(got, []string{LayerPassword}) {
		t.Fatalf("text lifted, password on: %v", got)
	}
	e.LiftedKinds, e.LiftedUntil = "text,password", 700
	if got := LayersOn(detail, e, 100, boot); len(got) != 0 {
		t.Fatalf("both lifted: %v", got)
	}
	if !AdvanceLayers(detail, &e, 700, boot) || e.LiftedKinds != "" {
		t.Fatalf("re-lock time over: %+v", e)
	}
	if got := LayersOn(detail, e, 700, boot); !reflect.DeepEqual(got, []string{LayerText, LayerPassword}) {
		t.Errorf("back on: %v", got)
	}
	// Lifts stored before LiftedKinds cover both.
	legacy := LockExtras{PasswordHash: "x", Relock: 600, LiftedUntil: 700}
	if got := LayersOn(detail, legacy, 100, boot); len(got) != 0 {
		t.Errorf("legacy lift: %v", got)
	}
}

// §3.4 range lock: on between its times on its days; overnight belongs to
// the day it starts; inverted, on outside them.
func TestRangeLock(t *testing.T) {
	mon := time.Date(2026, 11, 16, 0, 0, 0, 0, time.Local) // a Monday
	at := func(day, h, m int) time.Time { return mon.AddDate(0, 0, day).Add(time.Duration(h*60+m) * time.Minute) }
	workdays := 1<<uint(time.Monday) | 1<<uint(time.Tuesday) | 1<<uint(time.Wednesday) | 1<<uint(time.Thursday) | 1<<uint(time.Friday)
	e := LockExtras{RangeDays: workdays, RangeFrom: 9 * 60, RangeTo: 17 * 60}
	for _, c := range []struct {
		t    time.Time
		want bool
	}{{at(0, 10, 0), true}, {at(0, 8, 59), false}, {at(0, 17, 0), false}, {at(5, 10, 0), false}} {
		if e.InRange(c.t) != c.want {
			t.Errorf("work range at %s: %v", c.t.Format("Mon 15:04"), !c.want)
		}
	}
	night := LockExtras{RangeDays: 1 << uint(time.Friday), RangeFrom: 22 * 60, RangeTo: 6 * 60}
	if !night.InRange(at(4, 23, 0)) || !night.InRange(at(5, 5, 0)) || night.InRange(at(4, 5, 0)) {
		t.Error("overnight range")
	}
	e.RangeInvert = true
	if e.InRange(at(0, 10, 0)) || !e.InRange(at(0, 20, 0)) {
		t.Error("inverted range")
	}
	detail := &BlockDetail{Active: true}
	if on := LayersOn(detail, LockExtras{RangeDays: workdays, RangeFrom: 0, RangeTo: 1}, at(0, 0, 0).Unix(), ""); len(on) != 1 || on[0] != LayerRange {
		t.Errorf("range layer: %v", on)
	}
}

// §3.6 reward: kept for a period it accumulates past one break, and
// empties when the period ends; a longer period is looser.
func TestRewardKeep(t *testing.T) {
	one := Breaks{Allowance: 600, Per: 3600}
	if got := one.Earn(3600*3, 0); got.BalanceMs != 600*1000 {
		t.Errorf("one: balance %d", got.BalanceMs)
	}
	day := Breaks{Allowance: 600, Per: 3600, Keep: "day", Period: "2026-11-16"}
	day = day.Earn(3600*3, 0)
	if day.BalanceMs != 1800*1000 {
		t.Errorf("day: balance %d", day.BalanceMs)
	}
	if day.Expire("2026-11-16", 0).BalanceMs != 1800*1000 || day.Expire("2026-11-17", 0).BalanceMs != 0 {
		t.Error("expiry")
	}
	if !day.LooserThan(one) || (Breaks{Allowance: 600, Per: 3600, Keep: "day"}).LooserThan(Breaks{Allowance: 600, Per: 3600, Keep: "week"}) {
		t.Error("looser order")
	}
}

// §3.6 reward kept for a length (owner, 2026-11-16): each hour's part is
// kept that long after it was earned, with no upper limit on the length or
// the amount; a break takes the oldest first, and ending it early gives
// back the newest of what it took, at its own age.
func TestRewardKeptForALength(t *testing.T) {
	const hour = 3600
	h0 := int64(1000 * hour)
	k := Breaks{Allowance: 600, Per: 3600, Keep: "86400"}
	k = k.Earn(3*hour, h0)      // 30 minutes in hour 1000
	k = k.Earn(hour, h0+5*hour) // 10 minutes in hour 1005
	if k.Balance() != 2400 || k.Chunks != "[[1000,1800000],[1005,600000]]" {
		t.Fatalf("earned: %d s, %s", k.Balance(), k.Chunks)
	}
	if got := k.Expire("", h0+hour+86400-1); got.Balance() != 2400 {
		t.Errorf("kept a day after its hour: %d", got.Balance())
	}
	if got := k.Expire("", h0+hour+86400); got.Balance() != 600 || got.Chunks != "[[1005,600000]]" {
		t.Errorf("expired a day after its hour: %d, %s", got.Balance(), got.Chunks)
	}
	if got := k.Earn(100*86400, h0); got.Balance() != 2400+100*24*600 {
		t.Errorf("no cap on the amount: %d", got.Balance())
	}
	taken := k.Take(2000 * 1000)
	if taken.Balance() != 400 || taken.Chunks != "[[1005,400000]]" || taken.OnBreak != "[[1000,1800000],[1005,200000]]" {
		t.Errorf("take: %d, %s, on break %s", taken.Balance(), taken.Chunks, taken.OnBreak)
	}
	back := taken.GiveBack(1000 * 1000)
	if back.Balance() != 1400 || back.Chunks != "[[1000,800000],[1005,600000]]" || back.OnBreak != "" {
		t.Errorf("give back: %d, %s, on break %s", back.Balance(), back.Chunks, back.OnBreak)
	}
	// Switching to a length keeps what is saved, as earned now.
	one := Breaks{Allowance: 600, Per: 3600, BalanceMs: 600 * 1000}
	if got := one.WithKeep("86400", "", h0); got.Chunks != "[[1000,600000]]" || got.Balance() != 600 {
		t.Errorf("to a length: %+v", got)
	}
	if got := k.WithKeep("", "", h0); got.Balance() != 600 || got.Chunks != "" {
		t.Errorf("back to one: %+v", got)
	}
	if got := k.WithKeep("week", "2026-W47", h0); got.Balance() != 2400 || got.Period != "2026-W47" || got.Chunks != "" {
		t.Errorf("to a week: %+v", got)
	}
	for _, c := range []struct {
		keep, old string
		looser    bool
	}{
		{"86400", "3600", true}, {"3600", "86400", false}, {"86400", "week", true}, {"week", "86400", true},
		{"", "86400", false}, {"86400", "", true}, {"week", "day", true}, {"day", "week", false}, {"week", "week", false},
	} {
		if got := KeepLooser(c.keep, c.old); got != c.looser {
			t.Errorf("KeepLooser(%q, %q) = %v", c.keep, c.old, got)
		}
	}
	if ValidKeep("1800") || !ValidKeep("3600") || !ValidKeep("month") || ValidKeep("year") {
		t.Error("ValidKeep")
	}
}
