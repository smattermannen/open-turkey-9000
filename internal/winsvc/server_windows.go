package winsvc

// server_windows.go — the service's end of the pipe. Each connection is one
// forwarded command: who sent it comes from the client's token (as the
// socket's uid on Linux, never from what the client says), and the command
// runs as a child of the service (LocalSystem), with the caller named in
// its environment, its input and output relayed in frames. As on Linux,
// where only the sudo group may run kc9k, only members of Administrators
// may run commands (SPEC §6 2026-11-21, By Opus).

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/smattermannen/kc9k/internal/platform"
	"github.com/smattermannen/kc9k/internal/winapi"
	"golang.org/x/sys/windows"
)

// pipeSDDL lets SYSTEM do anything and signed-in users read and write
// (not FILE_CREATE_PIPE_INSTANCE: nobody else may add instances of the
// pipe and answer in KC9K's place): 0x12019b is FILE_GENERIC_READ |
// FILE_GENERIC_WRITE without FILE_APPEND_DATA, which for a pipe is
// FILE_CREATE_PIPE_INSTANCE. Network logons are refused.
const pipeSDDL = "D:P(D;;GA;;;NU)(A;;GA;;;SY)(A;;0x12019b;;;AU)"

// maxCommands bounds the commands running at once.
const maxCommands = 32

// refusedForwards are commands that never run through the pipe: they are
// the service, the agent, or install the service.
var refusedForwards = map[string]bool{"service": true, "agent": true, "install": true, "watchdog": true, "finish-uninstall": true}

// Caller is who sent a command, from the pipe client's token.
type Caller struct {
	SID     string
	UID     int64
	Name    string
	Session uint32
	Admin   bool
	token   windows.Token // impersonation token, for reading files as the caller
}

// Serve accepts forwarded commands until stop is closed.
func Serve(stop <-chan struct{}) error {
	l, err := winapi.Listen(PipeName, pipeSDDL)
	if err != nil {
		return fmt.Errorf("pipe %s: %w", PipeName, err)
	}
	slots := make(chan struct{}, maxCommands)
	for {
		select {
		case <-stop:
			return nil
		case slots <- struct{}{}:
		}
		conn, err := l.Accept()
		if err != nil {
			<-slots
			if err == windows.ERROR_ACCESS_DENIED {
				return fmt.Errorf("pipe %s exists already: another program created it", PipeName)
			}
			continue
		}
		go func() {
			defer func() { <-slots }()
			defer conn.Close()
			defer windows.FlushFileBuffers(conn.Handle())
			handle(conn, conn.Handle())
		}()
	}
}

const sizeofSA = unsafe.Sizeof(windows.SecurityAttributes{})

// clientCaller reads who is on the other end of the pipe from its token.
func clientCaller(h windows.Handle) (Caller, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := winapi.ImpersonateNamedPipeClient(h); err != nil {
		return Caller{}, fmt.Errorf("impersonate: %w", err)
	}
	var token windows.Token
	err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE|windows.TOKEN_IMPERSONATE, true, &token)
	if rerr := windows.RevertToSelf(); rerr != nil {
		// A thread that can't stop impersonating must not run anything else.
		panic(rerr)
	}
	if err != nil {
		return Caller{}, fmt.Errorf("client token: %w", err)
	}
	u, err := token.GetTokenUser()
	if err != nil {
		token.Close()
		return Caller{}, err
	}
	c := Caller{SID: u.User.Sid.String(), UID: platform.RID(u.User.Sid), Name: platform.AccountName(u.User.Sid), token: token}
	var session uint32
	var n uint32
	if err := windows.GetTokenInformation(token, windows.TokenSessionId, (*byte)(unsafe.Pointer(&session)), 4, &n); err == nil {
		c.Session = session
	}
	c.Admin = isAdminToken(token)
	return c, nil
}

// isAdminToken reports whether token's user is in Administrators, elevated
// or not (UAC's filtered token holds the group for deny only).
func isAdminToken(token windows.Token) bool {
	admins, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return false
	}
	groups, err := token.GetTokenGroups()
	if err != nil {
		return false
	}
	for _, g := range groups.AllGroups() {
		if g.Sid.Equals(admins) {
			return true
		}
	}
	return false
}

// handle runs one forwarded command.
func handle(conn *winapi.PipeConn, h windows.Handle) {
	var mu sync.Mutex
	stdout := FrameWriter{W: conn, Kind: FrameStdout, Lock: &mu}
	stderr := FrameWriter{W: conn, Kind: FrameStderr, Lock: &mu}
	exit := func(code int) {
		mu.Lock()
		defer mu.Unlock()
		_ = WriteFrame(conn, FrameExit, ExitPayload(code))
	}
	caller, err := clientCaller(h)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		exit(1)
		return
	}
	defer caller.token.Close()
	req, err := ReadRequest(conn)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		exit(1)
		return
	}
	if !caller.Admin {
		fmt.Fprintf(stderr, "Error: Killer Chicken 9000 is run by the computer's administrators; %s is not one\n", caller.Name)
		exit(1)
		return
	}
	if len(req.Args) > 0 && refusedForwards[req.Args[0]] {
		fmt.Fprintf(stderr, "Error: 'kc9k %s' is not run through the service\n", req.Args[0])
		exit(1)
		return
	}
	code := runForwarded(conn, caller, req, stdout, stderr)
	exit(code)
}

// runForwarded runs the command as a child of the service and relays its
// input and output.
func runForwarded(conn io.Reader, caller Caller, req Request, stdout, stderr io.Writer) int {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	cmd := exec.Command(self, req.Args...)
	cmd.Dir = platform.DataDir
	if req.Dir != "" && filepath.IsAbs(req.Dir) {
		if info, err := os.Stat(req.Dir); err == nil && info.IsDir() {
			cmd.Dir = req.Dir
		}
	}
	// The caller's token, inheritable, so the command can read files the
	// caller named with the caller's rights (readAsCaller).
	var dup windows.Token
	if err := windows.DuplicateTokenEx(caller.token, windows.TOKEN_QUERY|windows.TOKEN_IMPERSONATE|windows.TOKEN_DUPLICATE,
		&windows.SecurityAttributes{Length: uint32(sizeofSA), InheritHandle: 1}, windows.SecurityImpersonation, windows.TokenImpersonation, &dup); err != nil {
		fmt.Fprintf(stderr, "Error: caller token: %v\n", err)
		return 1
	}
	defer dup.Close()
	cmd.Env = childEnv(caller, req, uintptr(dup))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:                 true,
		CreationFlags:              windows.CREATE_NO_WINDOW,
		AdditionalInheritedHandles: []syscall.Handle{syscall.Handle(dup)},
	}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	go func() {
		var closeIn sync.Once
		defer closeIn.Do(func() { in.Close() })
		// Read frames until the client goes away, also after its input
		// ended: a Ctrl-C or the client's end must still stop the command
		// (an unlock with its input from a file kept its challenge window
		// open after the client was killed).
		for {
			kind, payload, err := ReadFrame(conn)
			if err != nil {
				// The client went away: stop the command, as closing a
				// terminal does on Linux.
				_ = cmd.Process.Kill()
				return
			}
			switch kind {
			case FrameStdin:
				_, _ = in.Write(payload) // a command that stopped reading: dropped
			case FrameStdinEOF:
				closeIn.Do(func() { in.Close() })
			case FrameInterrupt:
				_ = cmd.Process.Kill()
				return
			}
		}
	}()
	err = cmd.Wait()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	if err != nil {
		log.Printf("service: command %q: %v", strings.Join(req.Args, " "), err)
		return 1
	}
	return 0
}

// childEnv is the service's environment for a command it runs, without
// anything a caller could have steered, plus who called.
func childEnv(caller Caller, req Request, token uintptr) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(strings.ToUpper(kv), "KC9K_") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		platform.EnvCallerSID+"="+caller.SID,
		platform.EnvCallerUID+"="+strconv.FormatInt(caller.UID, 10),
		platform.EnvCallerUser+"="+caller.Name,
		platform.EnvCallerSession+"="+strconv.FormatUint(uint64(caller.Session), 10),
		platform.EnvCallerToken+"="+strconv.FormatUint(uint64(token), 10),
	)
	if t := req.Terminal; t != nil && t.Nonce != "" && !strings.ContainsAny(t.Nonce, ",;\x1b\x07") {
		env = append(env, fmt.Sprintf("%s=%d,%d,%s", platform.EnvTerminal, t.Cols, t.Rows, t.Nonce))
	}
	return env
}
