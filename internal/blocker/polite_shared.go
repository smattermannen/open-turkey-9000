package blocker

import (
	"sync"
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
