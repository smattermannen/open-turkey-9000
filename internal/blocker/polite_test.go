package blocker

import (
	"reflect"
	"testing"
	"time"
)

func TestPoliteSort(t *testing.T) {
	p := &politeState{asked: map[int]time.Time{}, names: map[string]time.Time{}}
	names := map[int]string{10: "game", 11: "game", 20: "chat", 30: "game"}
	name := func(pid int) string { return names[pid] }
	t0 := time.Unix(1000, 0)

	// First sight: asked, spared this cycle. Two processes of one app both wait.
	wait, fresh := p.sort([]int{10, 11, 20}, t0, name)
	if len(wait) != 3 || !reflect.DeepEqual(fresh, []int{10, 11, 20}) {
		t.Fatalf("first cycle: wait %v fresh %v", wait, fresh)
	}
	// Inside the grace period: still spared, not asked again.
	if wait, fresh = p.sort([]int{10, 11, 20}, t0.Add(2*time.Second), name); len(wait) != 3 || len(fresh) != 0 {
		t.Errorf("inside the grace: wait %v fresh %v", wait, fresh)
	}
	// After it: killed (nothing waits). The chat app closed by itself.
	if wait, fresh = p.sort([]int{10, 11}, t0.Add(5*time.Second), name); len(wait) != 0 || len(fresh) != 0 {
		t.Errorf("after the grace: wait %v fresh %v", wait, fresh)
	}
	// Started again within a minute: no second chance.
	if wait, fresh = p.sort([]int{30}, t0.Add(20*time.Second), name); len(wait) != 0 || len(fresh) != 0 {
		t.Errorf("reopened: wait %v fresh %v", wait, fresh)
	}
	// Much later it is asked again.
	if wait, fresh = p.sort([]int{30}, t0.Add(3*time.Minute), name); !wait[30] || len(fresh) != 1 {
		t.Errorf("later: wait %v fresh %v", wait, fresh)
	}
	// With no desktop to ask on, it is killable at the next cycle.
	p.expire([]int{30})
	if wait, _ = p.sort([]int{30}, t0.Add(3*time.Minute+time.Second), name); len(wait) != 0 {
		t.Errorf("expired: still waiting %v", wait)
	}
}
