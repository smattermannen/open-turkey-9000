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
