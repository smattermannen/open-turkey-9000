package winagent

import (
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/circadian"
)

func withHandlers(t *testing.T, add map[string]Handler) {
	handlersMu.Lock()
	old := handlers
	handlers = map[string]Handler{}
	for k, v := range add {
		handlers[k] = v
	}
	handlersMu.Unlock()
	t.Cleanup(func() { handlersMu.Lock(); handlers = old; handlersMu.Unlock() })
}

// W51: a handler's panic is an error reply; the agent goes on.
func TestDispatchRecovers(t *testing.T) {
	withHandlers(t, map[string]Handler{
		"boom": func(json.RawMessage) (any, error) { var m map[string]int; m["x"] = 1; return nil, nil },
		"ok":   func(json.RawMessage) (any, error) { return 7, nil },
	})
	var d dispatcher
	rep := d.answer(Request{ID: 3, Op: "boom"})
	if rep.ID != 3 || !strings.Contains(rep.Error, "internal error") {
		t.Fatalf("reply %+v", rep)
	}
	if rep := d.answer(Request{ID: 4, Op: "ok"}); rep.Error != "" || string(rep.Result) != "7" {
		t.Fatalf("after a panic: %+v", rep)
	}
	if rep := d.answer(Request{ID: 5, Op: "nope"}); !strings.Contains(rep.Error, "unknown op") {
		t.Fatalf("unknown op: %+v", rep)
	}
}

// W51: screen work never runs twice at once, and a newer gamma set
// replaces one still waiting.
func TestDispatchDisplayOneAtATime(t *testing.T) {
	var running, most atomic.Int32
	release := make(chan struct{})
	var applied []string
	var mu sync.Mutex
	work := func(name string, wait bool) Handler {
		return func(raw json.RawMessage) (any, error) {
			n := running.Add(1)
			defer running.Add(-1)
			for {
				m := most.Load()
				if n <= m || most.CompareAndSwap(m, n) {
					break
				}
			}
			if wait {
				<-release
			}
			mu.Lock()
			applied = append(applied, name+string(raw))
			mu.Unlock()
			return 1, nil
		}
	}
	first := true
	withHandlers(t, map[string]Handler{
		OpSetGamma: func(raw json.RawMessage) (any, error) {
			w := first
			first = false
			return work("gamma", w)(raw)
		},
		OpSetDark:  work("dark", false),
		OpMonitors: work("monitors", false),
	})
	var d dispatcher
	var wg sync.WaitGroup
	replies := make([]Reply, 4)
	go1 := func(i int, req Request) {
		wg.Add(1)
		go func() { defer wg.Done(); replies[i] = d.answer(req) }()
	}
	go1(0, Request{ID: 1, Op: OpSetGamma, Args: json.RawMessage("1")})
	time.Sleep(100 * time.Millisecond) // the first set holds the screens
	go1(1, Request{ID: 2, Op: OpSetGamma, Args: json.RawMessage("2")})
	time.Sleep(50 * time.Millisecond)
	go1(2, Request{ID: 3, Op: OpSetGamma, Args: json.RawMessage("3")})
	go1(3, Request{ID: 4, Op: OpSetDark})
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if most.Load() != 1 {
		t.Errorf("%d screen calls ran at once", most.Load())
	}
	for _, a := range applied {
		if a == "gamma2" {
			t.Errorf("the replaced ramp was set: %v", applied)
		}
	}
	if string(replies[1].Result) != "0" || replies[1].Error != "" {
		t.Errorf("replaced set's reply %+v", replies[1])
	}
	found := false
	for _, a := range applied {
		found = found || a == "gamma3"
	}
	if !found {
		t.Errorf("the newest ramp was not set: %v", applied)
	}
}

// W51: ramps that could harm the picture are refused; every ramp the
// clips can ask for passes.
func TestCheckRamp(t *testing.T) {
	ramp := func(k, b float64) Ramp {
		r, g, bl := circadian.Ramp(256, k, b)
		return Ramp{r, g, bl}
	}
	for _, k := range []float64{1000, 1900, 3400, 6500, 10000} {
		for _, b := range []float64{0.10, 0.4, 1} {
			if err := CheckRamp(ramp(k, b)); err != nil {
				t.Errorf("%v K at %v: %v", k, b, err)
			}
		}
	}
	if err := CheckRamp(identityRampForTest()); err != nil {
		t.Errorf("plain ramp: %v", err)
	}
	bad := map[string]Ramp{
		"short":   {Red: make([]uint16, 255), Green: make([]uint16, 256), Blue: make([]uint16, 256)},
		"black":   {Red: make([]uint16, 256), Green: make([]uint16, 256), Blue: make([]uint16, 256)},
		"too dim": ramp(6500, 0.02),
	}
	falling := identityRampForTest()
	falling.Green[100] = 0
	bad["falling"] = falling
	for name, r := range bad {
		if CheckRamp(r) == nil {
			t.Errorf("%s ramp passed", name)
		}
	}
}

func identityRampForTest() Ramp {
	r := Ramp{Red: make([]uint16, 256), Green: make([]uint16, 256), Blue: make([]uint16, 256)}
	for i := 0; i < 256; i++ {
		r.Red[i], r.Green[i], r.Blue[i] = uint16(i*257), uint16(i*257), uint16(i*257)
	}
	return r
}

// W50 / §3.10: a crash that said so and an agent that ended by itself are
// not tamper; anything else is.
func TestClassifyEnd(t *testing.T) {
	crash := "goroutine 1 [running]:\npanic: runtime error: index out of range\n\ngoroutine 7..."
	fatal := "fatal error: too many callback functions\r\n"
	for _, c := range []struct {
		code uint32
		out  string
		want agentEnd
	}{
		{2, crash, endCrash},
		{2, fatal, endCrash},
		{2, "", endOutside},         // exit code 2 alone: from outside
		{1, crash, endOutside},      // a report but killed: from outside
		{1, "", endOutside},         // taskkill, Task Manager
		{0, "", endOutside},         // TerminateProcess(h, 0)
		{0, markerDone + "\n", endSelf},
		{1, markerGaveUp + " the service can't be reached\n", endSelf},
	} {
		if got := classifyEnd(c.code, c.out); got != c.want {
			t.Errorf("classifyEnd(%d, %q) = %v, want %v", c.code, c.out, got, c.want)
		}
	}
}
