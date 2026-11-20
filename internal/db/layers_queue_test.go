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

// §3.4, §4.6: with re-lock, text1, password1, text2 are unlocked in turn;
// re-lock brings all three back, in that order.
func TestRelockSeveralTextLocks(t *testing.T) {
	const now, boot = 1_000_000, "boot-a"
	detail := &BlockDetail{Active: true, Locked: true, LockChars: 10}
	e := LockExtras{Order: "text,password", PasswordHash: "pw1", Relock: 600,
		Queue: []QueuedLock{{Kind: LayerText, Chars: 20}}}
	LiftTextLayer(detail, &e, LayerText, now, boot)
	LiftTextLayer(detail, &e, LayerPassword, now, boot)
	// The second text lock holds; the re-lock time hasn't started.
	if detail.LockChars != 20 || e.LiftedUntil != 0 || len(e.Relifted) != 1 || e.Relifted[0].Chars != 10 {
		t.Fatalf("after two unlocks: %+v %+v", detail, e)
	}
	if on := LayersOn(detail, e, now, boot); strings.Join(on, ",") != "text" {
		t.Fatalf("on = %v", on)
	}
	LiftTextLayer(detail, &e, LayerText, now+10, boot)
	if e.LiftedUntil != now+610 || len(LayersOn(detail, e, now+10, boot)) != 0 {
		t.Fatalf("after the last unlock: %+v", e)
	}
	AdvanceLayers(detail, &e, now+611, boot)
	on := LayersOn(detail, e, now+611, boot)
	if strings.Join(on, ",") != "text,password,text" || detail.LockChars != 10 || e.PasswordHash != "pw1" ||
		len(e.Queue) != 1 || e.Queue[0].Chars != 20 || len(e.Relifted) != 0 {
		t.Fatalf("after re-lock: on %v, %+v %+v", on, detail, e)
	}
}
