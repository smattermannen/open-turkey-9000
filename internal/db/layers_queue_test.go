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
