package blocker

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// W50: the panel's PowerShell is rare: looked for once, read at most once
// a minute, set at most once a second (the newest level winning), never
// two at once.
func TestWinPanelRateLimits(t *testing.T) {
	oldRead, oldWrite, oldNow := panelRead, panelWrite, panelNow
	panel.Lock()
	panel.looked, panel.found, panel.level, panel.readAt, panel.setAt, panel.want = false, false, 0, time.Time{}, time.Time{}, -1
	panel.Unlock()
	t.Cleanup(func() {
		panelRead, panelWrite, panelNow = oldRead, oldWrite, oldNow
		panel.Lock()
		panel.looked = false
		panel.Unlock()
	})
	var mu sync.Mutex
	clock := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	panelNow = func() time.Time { mu.Lock(); defer mu.Unlock(); return clock }
	advance := func(d time.Duration) { mu.Lock(); clock = clock.Add(d); mu.Unlock() }
	var reads, writes, running, most atomic.Int32
	busy := func() func() {
		n := running.Add(1)
		if n > most.Load() {
			most.Store(n)
		}
		time.Sleep(5 * time.Millisecond)
		return func() { running.Add(-1) }
	}
	var written []int
	panelRead = func() (int, error) { defer busy()(); reads.Add(1); return 50, nil }
	panelWrite = func(l int) error {
		defer busy()()
		writes.Add(1)
		mu.Lock()
		written = append(written, l)
		mu.Unlock()
		return nil
	}

	for i := 0; i < 20; i++ {
		if len(Backlights()) != 1 {
			t.Fatal("no panel")
		}
	}
	if reads.Load() != 1 {
		t.Fatalf("looked for the panel %d times, want 1", reads.Load())
	}
	b := Backlight{Name: "panel", Max: 100}
	for i := 0; i < 50; i++ { // a step each second for 50 s
		if v, err := b.Raw(); err != nil || v != 50 {
			t.Fatalf("raw %d %v", v, err)
		}
		advance(time.Second)
	}
	if reads.Load() != 1 {
		t.Errorf("read %d times in 50 s, want 1 (the look)", reads.Load())
	}
	advance(15 * time.Second)
	_, _ = b.Raw()
	if reads.Load() != 2 {
		t.Errorf("read %d times after a minute, want 2", reads.Load())
	}

	// Play the day: a set every 100 ms for 3 s of fake time.
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(l int) { defer wg.Done(); _ = b.SetRaw(l) }(i)
		wg.Wait()
		advance(100 * time.Millisecond)
	}
	time.Sleep(1500 * time.Millisecond) // the waiting level's timer (real time)
	if w := writes.Load(); w < 3 || w > 5 {
		t.Errorf("%d sets in 3 s, want about 3: %v", w, written)
	}
	if most.Load() > 1 {
		t.Errorf("%d PowerShells at once", most.Load())
	}
	mu.Lock()
	last := written[len(written)-1]
	mu.Unlock()
	if last != 29 {
		t.Errorf("the newest level wasn't set last: %v", written)
	}
	if v, _ := b.Raw(); v != 29 {
		t.Errorf("raw after sets %d, want 29", v)
	}
	ForgetBacklights()
	Backlights()
	if reads.Load() != 3 {
		t.Errorf("after a wake: %d reads, want 3", reads.Load())
	}
}
