package winagent

// dispatch.go — how the agent answers requests (shared, so it is tested on
// Linux too; W51): every handler under recover() (a bug in one is an
// error reply, never the agent's end); screen work one at a time, a newer
// gamma ramp replacing one still waiting; ramps checked before they reach
// a screen (principle 1: never harm the device); and how the service
// tells an agent that crashed or gave up from one ended from outside
// (§3.10: only that is tamper).

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/smattermannen/kc9k/internal/circadian"
)

// Ops of W6 (the screens).
const (
	OpGetGamma     = "get-gamma"      // → []Ramp (one per screen)
	OpSetGamma     = "set-gamma"      // Ramp → screens set
	OpGetLook      = "get-look"       // → Look
	OpSetWallpaper = "set-wallpaper"  // string path
	OpSetDark      = "set-dark"       // bool
	OpMonitors     = "monitors"       // → []Monitor
	OpSetMonitor   = "set-monitor"    // Monitor (Index, Level)
	OpNightLightOn = "night-light-on" // → bool
)

// Ramp is a gamma ramp: 256 values per colour.
type Ramp struct {
	Red   []uint16 `json:"r"`
	Green []uint16 `json:"g"`
	Blue  []uint16 `json:"b"`
}

// Handler answers one op in the agent.
type Handler func(args json.RawMessage) (any, error)

var (
	handlersMu sync.Mutex
	handlers   = map[string]Handler{}
)

// Register adds a handler for op (the desktop work of W3 and W6).
func Register(op string, h Handler) {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	handlers[op] = h
}

func handlerFor(op string) Handler {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	return handlers[op]
}

// displayOps read or change the screens: never two at once (gamma sets,
// DDC/CI, wallpaper and dark-mode broadcasts overlapping was a W51
// suspect).
var displayOps = map[string]bool{
	OpGetGamma: true, OpSetGamma: true, OpMonitors: true, OpSetMonitor: true,
	OpSetWallpaper: true, OpSetDark: true, OpNightLightOn: true, OpGetLook: true,
}

// dispatcher answers requests; the agent has one.
type dispatcher struct {
	displayMu sync.Mutex
	gammaGen  atomic.Uint64 // the newest gamma set's number
}

var agentDispatch dispatcher

// answer runs the request's handler and makes its reply.
func (d *dispatcher) answer(req Request) Reply {
	rep := Reply{ID: req.ID}
	h := handlerFor(req.Op)
	if h == nil {
		rep.Error = "unknown op " + req.Op
		return rep
	}
	if displayOps[req.Op] {
		var gen uint64
		if req.Op == OpSetGamma {
			gen = d.gammaGen.Add(1)
		}
		d.displayMu.Lock()
		defer d.displayMu.Unlock()
		if req.Op == OpSetGamma && gen != d.gammaGen.Load() {
			// A newer ramp came while this one waited: it replaces this.
			rep.Result = json.RawMessage("0")
			return rep
		}
	}
	result, err := safeCall(req.Op, h, req.Args)
	if err != nil {
		rep.Error = err.Error()
	} else if result != nil {
		rep.Result, _ = json.Marshal(result)
	}
	return rep
}

// safeCall runs h; a panic becomes an error (logged with its stack to the
// agent's output, which the service logs).
func safeCall(op string, h Handler, args json.RawMessage) (result any, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("agent: %s failed: %v\n%s", op, r, debug.Stack())
			result, err = nil, fmt.Errorf("%s: internal error: %v", op, r)
		}
	}()
	return h(args)
}

// minRampTop is, per colour, the lowest top a ramp may have: the lowest
// any colour temperature the clips allow (1000–10000 K) gives at the
// darkest software brightness (10 %), less a tenth for rounding.
var minRampTop = func() [3]uint16 {
	out := [3]uint16{65535, 65535, 65535}
	for k := 1000.0; k <= 10000; k += 50 {
		r, g, b := circadian.Ramp(256, k, 0.10)
		for i, c := range [][]uint16{r, g, b} {
			out[i] = min(out[i], c[255]*9/10)
		}
	}
	return out
}()

// CheckRamp refuses a ramp that could harm the picture: not 256 values per
// colour, falling anywhere, or darker than the clips' floors allow
// (principle 1; W51).
func CheckRamp(r Ramp) error {
	for i, c := range [][]uint16{r.Red, r.Green, r.Blue} {
		if len(c) != 256 {
			return errors.New("a ramp has 256 values per colour")
		}
		for j := 1; j < len(c); j++ {
			if c[j] < c[j-1] {
				return fmt.Errorf("the ramp falls at %d", j)
			}
		}
		if c[255] < minRampTop[i] {
			return fmt.Errorf("the ramp is darker than the circadian rhythm allows (%d < %d)", c[255], minRampTop[i])
		}
	}
	return nil
}

// Markers the agent writes to its output before it ends by itself.
const (
	markerDone   = "kc9k agent: the service closed the connection"
	markerGaveUp = "kc9k agent: gave up:"
)

// agentEnd is how an agent ended.
type agentEnd int

const (
	endOutside agentEnd = iota // ended from outside: tamper (§3.10)
	endCrash                   // crashed, and said so: logged, not tamper
	endSelf                    // ended by itself (service gone, gave up)
)

func (e agentEnd) String() string {
	return [...]string{"ended from outside", "crashed", "ended by itself"}[e]
}

// classifyEnd tells how an agent ended from its exit code and the tail of
// its output. A Go crash exits with 2 after a "panic: " or "fatal error: "
// report; an agent that ends by itself writes its marker first. Anything
// else (TerminateProcess, Task Manager, taskkill) is from outside.
func classifyEnd(code uint32, output string) agentEnd {
	switch {
	case code == 2 && crashReport(output) != "":
		return endCrash
	case code == 0 && strings.Contains(output, markerDone):
		return endSelf
	case code != 0 && strings.Contains(output, markerGaveUp):
		return endSelf
	}
	return endOutside
}

// crashReport is the first line of a Go crash report in output, or "".
func crashReport(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "panic: ") || strings.HasPrefix(line, "fatal error: ") {
			return line
		}
	}
	return ""
}
