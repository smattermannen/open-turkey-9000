package winagent

// agent_windows.go — `kc9k agent`, started by the service in a user's
// session: connects to the service (and only to the service's own
// process), says which session it is, and answers the service's requests
// until the service goes away.

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/smattermannen/kc9k/internal/platform"
	"github.com/smattermannen/kc9k/internal/winapi"
	"github.com/smattermannen/kc9k/internal/winsvc"
	"golang.org/x/sys/windows"
)

func init() {
	Register(OpStatus, func(json.RawMessage) (any, error) {
		idle, _ := winapi.IdleMillis()
		return Status{IdleMillis: idle, Locked: sessionLocked()}, nil
	})
}

// emit sends an event to the service; set while connected.
var emit func(event string, data any)

// sessionLocked is set by the agent's session watch (W3); false until then.
var sessionLocked = func() bool { return false }

// Emit sends an event to the service, if connected.
func Emit(event string, data any) {
	if emit != nil {
		emit(event, data)
	}
}

// RunAgent is `kc9k agent`. It returns when the service goes away. It
// writes why it ends to its output (the service reads it: an agent that
// ended by itself is not tamper, §3.10).
func RunAgent() error {
	err := runAgent()
	resetGamma()
	if err != nil {
		fmt.Fprintln(os.Stderr, markerGaveUp, err)
	} else {
		fmt.Fprintln(os.Stderr, markerDone)
	}
	return err
}

func runAgent() error {
	var session uint32
	if err := windows.ProcessIdToSessionId(uint32(os.Getpid()), &session); err != nil || session == 0 {
		return errors.New("the agent runs in a user's session, started by the service")
	}
	// No mutex for "one per session": any program of the user's could hold
	// it first and keep the agent out. The service starts one per session
	// and keeps the newest connection.
	conn, err := winapi.DialPipe(AgentPipe, 10*time.Second)
	if err != nil {
		return fmt.Errorf("the service can't be reached: %w", err)
	}
	defer conn.Close()
	if err := winsvc.VerifyServer(conn); err != nil {
		return err
	}
	var wmu sync.Mutex
	send := func(v any) error {
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		wmu.Lock()
		defer wmu.Unlock()
		_, err = conn.Write(append(data, '\n'))
		return err
	}
	if err := send(Hello{Session: session, User: platform.CallerName(), PID: uint32(os.Getpid())}); err != nil {
		return err
	}
	emit = func(event string, data any) {
		raw, _ := json.Marshal(data)
		_ = send(Reply{Event: event, Result: raw})
	}
	startDesktopWork()
	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			return nil // the service stopped; it starts a new agent
		}
		var req Request
		if json.Unmarshal(line, &req) != nil || req.ID == 0 {
			continue
		}
		// Each request in its own goroutine (a challenge waits for hours);
		// screen work one at a time, every handler under recover()
		// (dispatch.go).
		go func() { _ = send(agentDispatch.answer(req)) }()
	}
}

// startDesktopWork starts what the agent watches on its own (W3: hotkeys,
// session lock). Nothing yet.
var startDesktopWork = func() {}
