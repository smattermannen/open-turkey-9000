package winagent

// manager_windows.go — the service's side: start an agent in every session
// with a signed-in user and start it again if it ends; accept the agents'
// connections (only kc9k.exe itself, from the installed path, in the
// session it says) and call them.

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/smattermannen/kc9k/internal/winapi"
	"golang.org/x/sys/windows"
)

// agentSDDL: SYSTEM, and signed-in users read and write (not create
// instances).
const agentSDDL = "D:P(D;;GA;;;NU)(A;;GA;;;SY)(A;;0x12019b;;;AU)"

// internalSDDL: SYSTEM only.
const internalSDDL = "D:P(A;;GA;;;SY)"

// agent is one connected agent.
type agent struct {
	session uint32
	user    string
	conn    *winapi.PipeConn
	wmu     sync.Mutex
	mu      sync.Mutex
	waiting map[uint64]chan Reply
	closed  chan struct{}
}

// Manager keeps the agents. The service has one (Default).
type Manager struct {
	mu      sync.Mutex
	agents  map[uint32]*agent
	procs   map[uint32]windows.Handle // session -> agent process we started
	started map[uint32]time.Time
	nextID  atomic.Uint64
	// Restarts counts agents started again after they ended (§3.10:
	// W5 counts one ended by a person).
	Restarts atomic.Int64
	// OnEvent is called with each event an agent sends.
	OnEvent func(session uint32, event string, data json.RawMessage)
}

// Default is the service's manager; nil in any other process.
var Default *Manager

// Hooks the blocker package sets (it can't be imported here):
var (
	// SessionFilter picks the sessions that get an agent (a logout
	// freeze signs the others out).
	SessionFilter = func(sessions []uint32) []uint32 { return sessions }
	// OnConnect runs when a session's agent connects (to give it the
	// hotkeys).
	OnConnect = func(session uint32) {}
	// OnHotkey runs when a person presses a hotkey.
	OnHotkey = func(session uint32, id int) {}
)

// NewManager makes an empty manager.
func NewManager() *Manager {
	return &Manager{agents: map[uint32]*agent{}, procs: map[uint32]windows.Handle{}, started: map[uint32]time.Time{}}
}

// Run accepts agents and keeps one running in each session until stop.
func (m *Manager) Run(stop <-chan struct{}) {
	go m.acceptAgents(stop)
	go m.serveInternal(stop)
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()
	for {
		m.keepAgents()
		select {
		case <-stop:
			return
		case <-tick.C:
		}
	}
}

// Sessions are the sessions with a connected agent.
func (m *Manager) Sessions() []uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]uint32, 0, len(m.agents))
	for s := range m.agents {
		out = append(out, s)
	}
	return out
}

// userSessions are the sessions with a signed-in user.
func userSessions() []uint32 {
	var info *windows.WTS_SESSION_INFO
	var n uint32
	if err := windows.WTSEnumerateSessions(0, 0, 1, &info, &n); err != nil {
		return nil
	}
	defer windows.WTSFreeMemory(uintptr(unsafe.Pointer(info)))
	list := unsafe.Slice(info, n)
	var out []uint32
	for _, s := range list {
		if s.SessionID == 0 {
			continue
		}
		var token windows.Token
		if windows.WTSQueryUserToken(s.SessionID, &token) == nil {
			token.Close()
			out = append(out, s.SessionID)
		}
	}
	return out
}

// keepAgents starts an agent where a session has none running.
func (m *Manager) keepAgents() {
	sessions := SessionFilter(userSessions())
	live := map[uint32]bool{}
	for _, s := range sessions {
		live[s] = true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for s, h := range m.procs {
		if !live[s] {
			windows.CloseHandle(h)
			delete(m.procs, s)
			continue
		}
		if ev, _ := windows.WaitForSingleObject(h, 0); ev == windows.WAIT_OBJECT_0 {
			windows.CloseHandle(h)
			delete(m.procs, s)
			m.Restarts.Add(1)
			log.Printf("agent: the agent in session %d ended; starting it again", s)
		}
	}
	for _, s := range sessions {
		if _, ok := m.procs[s]; ok {
			continue
		}
		if t, ok := m.started[s]; ok && time.Since(t) < 2*time.Second {
			continue // one that ends at once is not started in a loop
		}
		m.started[s] = time.Now()
		h, err := startAgent(s)
		if err != nil {
			log.Printf("agent: can't start in session %d: %v", s, err)
			continue
		}
		m.procs[s] = h
	}
}

// startAgent starts `kc9k agent` as session's user, on its desktop.
func startAgent(session uint32) (windows.Handle, error) {
	var token windows.Token
	if err := windows.WTSQueryUserToken(session, &token); err != nil {
		return 0, err
	}
	defer token.Close()
	self, err := os.Executable()
	if err != nil {
		return 0, err
	}
	var env *uint16
	if err := windows.CreateEnvironmentBlock(&env, token, false); err != nil {
		return 0, err
	}
	defer windows.DestroyEnvironmentBlock(env)
	cmdline, _ := windows.UTF16PtrFromString(syscall.EscapeArg(self) + " agent")
	app, _ := windows.UTF16PtrFromString(self)
	desktop, _ := windows.UTF16PtrFromString(`winsta0\default`)
	dir, _ := windows.UTF16PtrFromString(os.Getenv("SystemRoot"))
	si := &windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{})), Desktop: desktop}
	var pi windows.ProcessInformation
	err = windows.CreateProcessAsUser(token, app, cmdline, nil, nil, false,
		windows.CREATE_UNICODE_ENVIRONMENT|windows.CREATE_NO_WINDOW, env, dir, si, &pi)
	if err != nil {
		return 0, err
	}
	windows.CloseHandle(pi.Thread)
	return pi.Process, nil
}

// acceptAgents takes agents' connections.
func (m *Manager) acceptAgents(stop <-chan struct{}) {
	l, err := winapi.Listen(AgentPipe, agentSDDL)
	if err != nil {
		log.Printf("agent: pipe: %v", err)
		return
	}
	for {
		select {
		case <-stop:
			return
		default:
		}
		conn, err := l.Accept()
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		go m.register(conn)
	}
}

// register checks who connected and keeps the agent.
func (m *Manager) register(conn *winapi.PipeConn) {
	pid, err := conn.ClientPID()
	if err != nil {
		conn.Close()
		return
	}
	image, err := winapi.ProcessImage(pid)
	self, _ := os.Executable()
	if err != nil || !strings.EqualFold(image, self) {
		log.Printf("agent: refused a connection from %s (process %d): not kc9k.exe", image, pid)
		conn.Close()
		return
	}
	var session uint32
	if err := windows.ProcessIdToSessionId(pid, &session); err != nil || session == 0 {
		conn.Close()
		return
	}
	r := bufio.NewReader(conn)
	line, err := r.ReadBytes('\n')
	var hello Hello
	if err != nil || json.Unmarshal(line, &hello) != nil || hello.Session != session || hello.PID != pid {
		log.Printf("agent: refused process %d: bad hello", pid)
		conn.Close()
		return
	}
	a := &agent{session: session, user: hello.User, conn: conn, waiting: map[uint64]chan Reply{}, closed: make(chan struct{})}
	m.mu.Lock()
	if old := m.agents[session]; old != nil {
		old.conn.Close()
	}
	m.agents[session] = a
	m.mu.Unlock()
	log.Printf("agent: session %d (%s) connected", session, hello.User)
	go OnConnect(session)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			break
		}
		var rep Reply
		if json.Unmarshal(line, &rep) != nil {
			continue
		}
		if rep.ID == 0 {
			if rep.Event == EventHotkey {
				var id int
				if json.Unmarshal(rep.Result, &id) == nil {
					go OnHotkey(session, id)
				}
				continue
			}
			if m.OnEvent != nil && rep.Event != "" {
				m.OnEvent(session, rep.Event, rep.Result)
			}
			continue
		}
		a.mu.Lock()
		ch := a.waiting[rep.ID]
		delete(a.waiting, rep.ID)
		a.mu.Unlock()
		if ch != nil {
			ch <- rep
		}
	}
	close(a.closed)
	conn.Close()
	m.mu.Lock()
	if m.agents[session] == a {
		delete(m.agents, session)
	}
	m.mu.Unlock()
}

// ErrNoAgent is returned for a session without an agent.
var ErrNoAgent = errors.New("no agent in that session")

// Call asks session's agent to do op and decodes its result into result.
func (m *Manager) Call(session uint32, op string, args, result any) error {
	m.mu.Lock()
	a := m.agents[session]
	m.mu.Unlock()
	if a == nil {
		return ErrNoAgent
	}
	req := Request{ID: m.nextID.Add(1), Op: op}
	if args != nil {
		data, err := json.Marshal(args)
		if err != nil {
			return err
		}
		req.Args = data
	}
	ch := make(chan Reply, 1)
	a.mu.Lock()
	a.waiting[req.ID] = ch
	a.mu.Unlock()
	data, _ := json.Marshal(req)
	a.wmu.Lock()
	_, err := a.conn.Write(append(data, '\n'))
	a.wmu.Unlock()
	if err != nil {
		return err
	}
	select {
	case rep := <-ch:
		if rep.Error != "" {
			return errors.New(rep.Error)
		}
		if result != nil && len(rep.Result) > 0 {
			return json.Unmarshal(rep.Result, result)
		}
		return nil
	case <-a.closed:
		return fmt.Errorf("the agent in session %d went away", session)
	case <-time.After(timeoutFor(op)):
		a.mu.Lock()
		delete(a.waiting, req.ID)
		a.mu.Unlock()
		return fmt.Errorf("the agent in session %d didn't answer", session)
	}
}

// internalCall is a call from a command the service runs.
type internalCall struct {
	Session uint32          `json:"session"`
	Op      string          `json:"op"`
	Args    json.RawMessage `json:"args,omitempty"`
	List    bool            `json:"list,omitempty"` // only list the sessions
}

type internalReply struct {
	Error    string          `json:"error,omitempty"`
	Result   json.RawMessage `json:"result,omitempty"`
	Sessions []uint32        `json:"sessions,omitempty"`
}

// serveInternal lets commands the service runs (SYSTEM) call agents.
func (m *Manager) serveInternal(stop <-chan struct{}) {
	l, err := winapi.Listen(ServicePipe, internalSDDL)
	if err != nil {
		log.Printf("agent: internal pipe: %v", err)
		return
	}
	for {
		select {
		case <-stop:
			return
		default:
		}
		conn, err := l.Accept()
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		go func() {
			defer conn.Close()
			line, err := bufio.NewReader(conn).ReadBytes('\n')
			if err != nil {
				return
			}
			var c internalCall
			var rep internalReply
			if err := json.Unmarshal(line, &c); err != nil {
				rep.Error = err.Error()
			} else if c.List {
				rep.Sessions = m.Sessions()
			} else {
				var raw json.RawMessage
				var args any
				if len(c.Args) > 0 {
					args = c.Args
				}
				if err := m.Call(c.Session, c.Op, args, &raw); err != nil {
					rep.Error = err.Error()
				}
				rep.Result = raw
			}
			data, _ := json.Marshal(rep)
			_, _ = conn.Write(append(data, '\n'))
		}()
	}
}

// viaService makes one internal call from a command the service runs.
func viaService(c internalCall) (internalReply, error) {
	conn, err := winapi.DialPipe(ServicePipe, 5*time.Second)
	if err != nil {
		return internalReply{}, fmt.Errorf("the service's agents can't be reached: %w", err)
	}
	defer conn.Close()
	data, _ := json.Marshal(c)
	if _, err := conn.Write(append(data, '\n')); err != nil {
		return internalReply{}, err
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return internalReply{}, err
	}
	var rep internalReply
	if err := json.Unmarshal(line, &rep); err != nil {
		return internalReply{}, err
	}
	return rep, nil
}

// Call asks session's agent to do op, from the service or from a command
// it runs.
func Call(session uint32, op string, args, result any) error {
	if Default != nil {
		return Default.Call(session, op, args, result)
	}
	c := internalCall{Session: session, Op: op}
	if args != nil {
		data, err := json.Marshal(args)
		if err != nil {
			return err
		}
		c.Args = data
	}
	rep, err := viaService(c)
	if err != nil {
		return err
	}
	if rep.Error != "" {
		return errors.New(rep.Error)
	}
	if result != nil && len(rep.Result) > 0 && string(rep.Result) != "null" {
		return json.Unmarshal(rep.Result, result)
	}
	return nil
}

// Sessions are the sessions with an agent, from the service or from a
// command it runs.
func Sessions() []uint32 {
	if Default != nil {
		return Default.Sessions()
	}
	rep, err := viaService(internalCall{List: true})
	if err != nil {
		return nil
	}
	return rep.Sessions
}

// timeoutFor is how long an op may take: a question or the challenge
// waits for a person.
func timeoutFor(op string) time.Duration {
	switch op {
	case OpConfirm:
		return 10 * time.Minute
	case OpChallenge:
		return 4 * time.Hour
	}
	return CallTimeout
}
