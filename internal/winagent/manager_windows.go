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
	outs    map[uint32]*agentOutput   // session -> that agent's output
	started map[uint32]time.Time
	nextID  atomic.Uint64
	// Restarts counts agents started again after they were ended from
	// outside (§3.10: W5 counts one ended by a person as tamper). An agent
	// that crashed and said so (Crashes) or ended by itself is not one.
	Restarts atomic.Int64
	Crashes  atomic.Int64
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
	return &Manager{agents: map[uint32]*agent{}, procs: map[uint32]windows.Handle{}, outs: map[uint32]*agentOutput{},
		started: map[uint32]time.Time{}}
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
			delete(m.outs, s)
			continue
		}
		if ev, _ := windows.WaitForSingleObject(h, 0); ev == windows.WAIT_OBJECT_0 {
			end, why := agentEnded(h, m.outs[s])
			windows.CloseHandle(h)
			delete(m.procs, s)
			delete(m.outs, s)
			switch end {
			case endOutside:
				m.Restarts.Add(1)
			case endCrash:
				m.Crashes.Add(1)
			}
			log.Printf("agent: the agent in session %d %s%s; starting it again", s, end, why)
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
		h, out, err := startAgent(s)
		if err != nil {
			log.Printf("agent: can't start in session %d: %v", s, err)
			continue
		}
		m.procs[s], m.outs[s] = h, out
	}
}

// startAgent starts `kc9k agent` as session's user, on its desktop.
func startAgent(session uint32) (windows.Handle, *agentOutput, error) {
	var token windows.Token
	if err := windows.WTSQueryUserToken(session, &token); err != nil {
		return 0, nil, err
	}
	defer token.Close()
	self, err := os.Executable()
	if err != nil {
		return 0, nil, err
	}
	var env *uint16
	if err := windows.CreateEnvironmentBlock(&env, token, false); err != nil {
		return 0, nil, err
	}
	defer windows.DestroyEnvironmentBlock(env)
	return spawnWithOutput(token, self, syscall.EscapeArg(self)+" agent", env, `winsta0\default`, session)
}

// spawnWithOutput starts a program (as token's user; with token 0 as this
// process's) with its standard output and error on a pipe this process
// reads (agentOutput): a Go crash report lands there, so a crash is told
// from a kill. Only that pipe's end is inherited.
func spawnWithOutput(token windows.Token, exe, cmdline string, env *uint16, desktop string, session uint32) (windows.Handle, *agentOutput, error) {
	sa := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), InheritHandle: 1}
	var r, w windows.Handle
	syscall.ForkLock.Lock()
	defer syscall.ForkLock.Unlock()
	if err := windows.CreatePipe(&r, &w, &sa, 0); err != nil {
		return 0, nil, err
	}
	_ = windows.SetHandleInformation(r, windows.HANDLE_FLAG_INHERIT, 0)
	defer windows.CloseHandle(w)
	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		windows.CloseHandle(r)
		return 0, nil, err
	}
	defer attrs.Delete()
	if err := attrs.Update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&w), unsafe.Sizeof(w)); err != nil {
		windows.CloseHandle(r)
		return 0, nil, err
	}
	si := &windows.StartupInfoEx{ProcThreadAttributeList: attrs.List()}
	si.Cb = uint32(unsafe.Sizeof(*si))
	si.Flags = windows.STARTF_USESTDHANDLES
	si.StdOutput, si.StdErr = w, w
	if desktop != "" {
		si.Desktop, _ = windows.UTF16PtrFromString(desktop)
	}
	cmd, _ := windows.UTF16PtrFromString(cmdline)
	app, _ := windows.UTF16PtrFromString(exe)
	dir, _ := windows.UTF16PtrFromString(os.Getenv("SystemRoot"))
	flags := uint32(windows.CREATE_NO_WINDOW | windows.EXTENDED_STARTUPINFO_PRESENT)
	if env != nil {
		flags |= windows.CREATE_UNICODE_ENVIRONMENT
	}
	var pi windows.ProcessInformation
	if token != 0 {
		err = windows.CreateProcessAsUser(token, app, cmd, nil, nil, true, flags, env, dir, &si.StartupInfo, &pi)
	} else {
		err = windows.CreateProcess(app, cmd, nil, nil, true, flags, env, dir, &si.StartupInfo, &pi)
	}
	if err != nil {
		windows.CloseHandle(r)
		return 0, nil, err
	}
	windows.CloseHandle(pi.Thread)
	return pi.Process, readAgentOutput(session, r), nil
}

// agentOutput is what an agent wrote: its first lines go to the service's
// log, the last 16 KB are kept to tell how it ended.
type agentOutput struct {
	mu   sync.Mutex
	tail []byte
	done chan struct{}
}

func readAgentOutput(session uint32, r windows.Handle) *agentOutput {
	o := &agentOutput{done: make(chan struct{})}
	go func() {
		defer close(o.done)
		f := os.NewFile(uintptr(r), "agent output")
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 4096), 64*1024)
		logged := 0
		for sc.Scan() {
			line := sc.Text()
			if logged < 200 {
				log.Printf("agent %d: %s", session, line)
				logged++
			}
			o.mu.Lock()
			o.tail = append(o.tail, line...)
			o.tail = append(o.tail, '\n')
			if len(o.tail) > 16<<10 {
				o.tail = append([]byte(nil), o.tail[len(o.tail)-16<<10:]...)
			}
			o.mu.Unlock()
		}
	}()
	return o
}

// text is the tail, once the agent's output closed (or after a second).
func (o *agentOutput) text() string {
	if o == nil {
		return ""
	}
	select {
	case <-o.done:
	case <-time.After(time.Second):
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return string(o.tail)
}

// agentEnded tells how the ended agent h ended (§3.10: only from outside
// is tamper), and why, for the log.
func agentEnded(h windows.Handle, out *agentOutput) (agentEnd, string) {
	var code uint32
	_ = windows.GetExitCodeProcess(h, &code)
	text := out.text()
	end := classifyEnd(code, text)
	why := fmt.Sprintf(" (exit code %d)", code)
	if end == endCrash {
		why = fmt.Sprintf(" (exit code %d: %s)", code, crashReport(text))
	}
	return end, why
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
