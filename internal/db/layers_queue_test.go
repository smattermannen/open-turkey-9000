package db

import (
	"strings"
	"testing"
)

// §3.4: queued locks take their kind's place once nothing before them is
// on; a dated timer lock already past lets the next one through at once.
func TestQueuePromotion(t *testing.T) {
	const now, boot = 1_000_000, "boot-a"
	detail := &BlockDetail{Active: true, Locked: true, LockChars: 10}
	e := LockExtras{Order: "text", Queue: []QueuedLock{
		{Kind: LayerTimer, Until: now - 5},
		{Kind: LayerText, Chars: 40, Style: "words"},
		{Kind: LayerTimer, For: 600},
	}}
	if got := LayersOn(detail, e, now, boot); len(got) != 4 || got[0] != LayerText {
		t.Fatalf("on = %v", got)
	}
	if AdvanceLayers(detail, &e, now, boot) {
		t.Error("promoted while the first text lock is on")
	}
	LiftTextLayer(detail, &e, LayerText, now, boot)
	// The past timer lock is gone at once; the second text lock holds.
	if !detail.Locked || detail.LockChars != 40 || e.Style != "words" || len(e.Queue) != 1 {
		t.Fatalf("after the first text lock: %+v %+v", detail, e)
	}
	LiftTextLayer(detail, &e, LayerText, now+100, boot)
	if detail.LockedUntil != now+100+600 || len(e.Queue) != 0 || !strings.HasSuffix(e.Order, "timer") {
		t.Fatalf("the last timer lock: until %d, %+v", detail.LockedUntil, e)
	}
	if on := LayersOn(detail, e, now+200, boot); len(on) != 1 || on[0] != LayerTimer {
		t.Errorf("on = %v", on)
	}
	if on := LayersOn(detail, e, now+800, boot); len(on) != 0 {
		t.Errorf("on after it = %v", on)
	}
}

// §3.4, §4.6: with re-lock the block's whole configuration comes back:
// text1, password1, text2 and a timer lock are unlocked in turn; once none
// is on the re-lock time runs, then all four are on again, in their order.
func TestRelockWholeConfiguration(t *testing.T) {
	const now, boot = 1_000_000, "boot-a"
	config := []QueuedLock{{Kind: LayerText, Chars: 10}, {Kind: LayerPassword, PasswordHash: "pw1"},
		{Kind: LayerText, Chars: 20}, {Kind: LayerTimer, For: 300}}
	detail := &BlockDetail{Active: true, Locked: true, LockChars: 10}
	e := LockExtras{Order: "text,password", PasswordHash: "pw1", Relock: 600,
		Queue: []QueuedLock{config[2], config[3]}, Config: config}
	LiftTextLayer(detail, &e, LayerText, now, boot)
	LiftTextLayer(detail, &e, LayerPassword, now, boot)
	if detail.LockChars != 20 || e.PasswordHash != "" || e.LiftedUntil != 0 {
		t.Fatalf("after two unlocks: %+v %+v", detail, e)
	}
	LiftTextLayer(detail, &e, LayerText, now+10, boot)
	if detail.LockedUntil != now+310 || e.LiftedUntil != 0 {
		t.Fatalf("the timer lock didn't start: %+v %+v", detail, e)
	}
	// The timer lock is over: nothing is on, the re-lock time runs.
	if !AdvanceLayers(detail, &e, now+400, boot) || e.LiftedUntil != now+1000 || len(LayersOn(detail, e, now+400, boot)) != 0 {
		t.Fatalf("re-lock time: %+v", e)
	}
	AdvanceLayers(detail, &e, now+1001, boot)
	on := LayersOn(detail, e, now+1001, boot)
	if strings.Join(on, ",") != "text,password,text,timer" || detail.LockChars != 10 || e.LiftedUntil != 0 ||
		len(e.Queue) != 3 || e.Queue[0].PasswordHash != "pw1" || e.Queue[2].For != 300 || len(e.Config) != 4 {
		t.Fatalf("after re-lock: on %v, %+v %+v", on, detail, e)
	}
	// A lock added during the re-lock time ends it: the time starts again
	// only once nothing is on.
	detail2 := &BlockDetail{Active: true}
	e2 := LockExtras{Relock: 600, Config: []QueuedLock{{Kind: LayerDelay, Delay: 60}}, LiftedUntil: now + 300}
	e2.Delay = 60
	AdvanceLayers(detail2, &e2, now, boot)
	if e2.LiftedUntil != 0 {
		t.Errorf("re-lock time kept running with a lock on: %+v", e2)
	}
}
