// polite.go — asking a blocked app to close before killing it.
//
// A blocked app with a window is first asked to close, as if its close
// button were pressed, so it can offer to save. It is killed on the next
// cycle if it is still there (PoliteGrace). An app without a window is
// killed at once, and so is an app started again within PoliteAgain of the
// last time it was asked: reopening it doesn't buy more time.
package blocker

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// PoliteGrace is how long an app asked to close may stay.
	PoliteGrace = 4 * time.Second
	// PoliteAgain is how long after asking an app that another start of it
	// is killed without asking.
	PoliteAgain = time.Minute
)

type politeState struct {
	mu    sync.Mutex
	asked map[int]time.Time    // process -> when it was asked
	names map[string]time.Time // program name -> when one was last asked
}

var polite = &politeState{asked: map[int]time.Time{}, names: map[string]time.Time{}}

// sort splits the matched processes into those to wait for and those newly
// asked (a subset of wait). Everything else is to be killed now.
func (p *politeState) sort(roots []int, now time.Time, name func(pid int) string) (wait map[int]bool, fresh []int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	wait = map[int]bool{}
	present := map[int]bool{}
	for _, pid := range roots {
		present[pid] = true
		if at, ok := p.asked[pid]; ok {
			if now.Sub(at) < PoliteGrace {
				wait[pid] = true
			}
			continue
		}
		if at, ok := p.names[name(pid)]; ok && now.Sub(at) < PoliteAgain {
			continue // reopened: no second chance
		}
		p.asked[pid] = now
		wait[pid] = true
		fresh = append(fresh, pid)
	}
	for _, pid := range fresh {
		p.names[name(pid)] = now
	}
	for pid := range p.asked {
		if !present[pid] {
			delete(p.asked, pid)
		}
	}
	for n, at := range p.names {
		if now.Sub(at) >= PoliteAgain {
			delete(p.names, n)
		}
	}
	return wait, fresh
}

// expire makes the processes killable on the next cycle whatever the time.
func (p *politeState) expire(pids []int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, pid := range pids {
		if _, ok := p.asked[pid]; ok {
			p.asked[pid] = time.Time{}
		}
	}
}

var politeBusy atomic.Bool

// askToClose asks the windows of the processes (or of anything they
// started) to close. A process with no window on a desktop that could be
// read is killed at once with everything it started. It runs beside the
// daemon cycle: talking to the X server must never hold the cycle up.
func askToClose(pids []int) {
	if len(pids) == 0 || !politeBusy.CompareAndSwap(false, true) {
		polite.expire(pids)
		return
	}
	go func() {
		defer politeBusy.Store(false)
		parent, _ := processTree()
		rootOf := func(pid int) int {
			for i := 0; pid > 1 && i < 64; i, pid = i+1, parent[pid] {
				for _, root := range pids {
					if pid == root {
						return root
					}
				}
			}
			return 0
		}
		hasWindow := map[int]bool{}
		read := false
		for _, u := range desktopUsers() {
			s := sessionFor(u)
			c, err := openUserDisplay(s.display, s.xauth, s.uid)
			if err != nil {
				continue
			}
			windows, err := c.windows()
			if err == nil {
				read = true
				for _, w := range windows {
					if root := rootOf(w.PID); root != 0 && ownedBy(w.PID, s.uid) {
						hasWindow[root] = true
						log.Printf("apps: asking %q (pid %d) to close", w.Title, w.PID)
						_ = c.closeWindow(w.ID)
					}
				}
			}
			c.Close()
		}
		if !read {
			polite.expire(pids) // no desktop to ask on: nothing to wait for
			return
		}
		kill := map[int]uint64{}
		for _, pid := range pids {
			if hasWindow[pid] {
				continue
			}
			if real, effective, ok := processUIDs(pid); ok && real == effective {
				kill[pid] = real
			}
		}
		killTrees(kill)
	}()
}
