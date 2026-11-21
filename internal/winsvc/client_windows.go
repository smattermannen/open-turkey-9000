package winsvc

// client_windows.go — `kc9k <command>` typed by a person: hands the
// arguments to the service over the pipe and relays input and output, as
// the sudo wrapper does on Linux, so no UAC prompt per command. The pipe
// must belong to the KC9K service's own process, or nothing is sent.

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/smattermannen/kc9k/internal/winapi"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

// ServiceName is the Windows service's name.
const ServiceName = "kc9k"

// ErrNoService is returned when the service isn't there to forward to.
var ErrNoService = errors.New("the Killer Chicken 9000 service is not running")

// ServicePID is the process ID of the running KC9K service, or 0.
func ServicePID() uint32 {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return 0
	}
	defer windows.CloseServiceHandle(scm)
	name, _ := windows.UTF16PtrFromString(ServiceName)
	h, err := windows.OpenService(scm, name, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return 0
	}
	s := &mgr.Service{Name: ServiceName, Handle: h}
	defer s.Close()
	status, err := s.Query()
	if err != nil {
		return 0
	}
	return status.ProcessId
}

// VerifyServer refuses a pipe that another program created in the
// service's place.
func VerifyServer(conn *winapi.PipeConn) error {
	pid, err := conn.ServerPID()
	if err != nil {
		return err
	}
	if want := ServicePID(); want == 0 || pid != want {
		return fmt.Errorf("the pipe %s is not the Killer Chicken 9000 service's (process %d); nothing was sent", PipeName, pid)
	}
	return nil
}

// console is the person's console, if input and output are one.
type console struct {
	in, out         windows.Handle
	inMode, outMode uint32
	cols, rows      int
	mu              sync.Mutex
}

func openConsole() *console {
	in, out := windows.Handle(os.Stdin.Fd()), windows.Handle(os.Stdout.Fd())
	c := &console{in: in, out: out}
	if windows.GetConsoleMode(in, &c.inMode) != nil || windows.GetConsoleMode(out, &c.outMode) != nil {
		return nil
	}
	var info windows.ConsoleScreenBufferInfo
	if windows.GetConsoleScreenBufferInfo(out, &info) == nil {
		c.cols = int(info.Window.Right-info.Window.Left) + 1
		c.rows = int(info.Window.Bottom-info.Window.Top) + 1
	}
	_ = windows.SetConsoleMode(out, c.outMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	return c
}

// apply switches the console's input for a mode the command asked for.
func (c *console) apply(mode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var cur uint32
	if windows.GetConsoleMode(c.in, &cur) != nil {
		return
	}
	switch mode {
	case CtlRaw:
		cur &^= windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT | windows.ENABLE_PROCESSED_INPUT
		cur |= windows.ENABLE_VIRTUAL_TERMINAL_INPUT
	case CtlCooked:
		cur = c.inMode
	case CtlEchoOff:
		cur &^= windows.ENABLE_ECHO_INPUT
	case CtlEchoOn:
		cur |= windows.ENABLE_ECHO_INPUT
	default:
		return
	}
	_ = windows.SetConsoleMode(c.in, cur)
}

func (c *console) restore() {
	_ = windows.SetConsoleMode(c.in, c.inMode)
	_ = windows.SetConsoleMode(c.out, c.outMode)
}

// Forward runs args in the service and returns the exit code.
func Forward(args []string) (int, error) {
	conn, err := winapi.DialPipe(PipeName, 10*time.Second)
	if err == windows.ERROR_FILE_NOT_FOUND {
		return 1, ErrNoService
	}
	if err != nil {
		return 1, err
	}
	defer conn.Close()
	if err := VerifyServer(conn); err != nil {
		return 1, err
	}
	req := Request{Args: args}
	if wd, err := os.Getwd(); err == nil {
		req.Dir = wd
	}
	con := openConsole()
	var filter *ControlFilter
	if con != nil {
		defer con.restore()
		nonce := make([]byte, 16)
		if _, err := rand.Read(nonce); err != nil {
			return 1, err
		}
		req.Terminal = &Terminal{Cols: con.cols, Rows: con.rows, Nonce: hex.EncodeToString(nonce)}
		filter = &ControlFilter{Nonce: req.Terminal.Nonce}
	}
	var wmu sync.Mutex
	send := func(kind byte, p []byte) error {
		wmu.Lock()
		defer wmu.Unlock()
		return WriteFrame(conn, kind, p)
	}
	if err := WriteRequest(conn, req); err != nil {
		return 1, err
	}

	// Ctrl-C in a cooked console stops the command, as on Linux.
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)
	go func() {
		for range interrupts {
			_ = send(FrameInterrupt, nil)
		}
	}()

	go pumpInput(con, send)

	for {
		kind, payload, err := ReadFrame(conn)
		if err != nil {
			if err == io.EOF {
				return 1, errors.New("the service closed the connection")
			}
			return 1, err
		}
		switch kind {
		case FrameStdout:
			if filter != nil {
				out, modes := filter.Filter(payload)
				_, _ = os.Stdout.Write(out)
				for _, m := range modes {
					con.apply(m)
				}
			} else {
				_, _ = os.Stdout.Write(payload)
			}
		case FrameStderr:
			_, _ = os.Stderr.Write(payload)
		case FrameExit:
			if filter != nil {
				_, _ = os.Stdout.Write(filter.Flush())
			}
			return ExitCode(payload), nil
		}
	}
}

// pumpInput sends what is typed or piped. On a console it waits for input
// before reading, so a switch to raw input applies to the next key.
func pumpInput(con *console, send func(byte, []byte) error) {
	buf := make([]byte, 4096)
	for {
		if con != nil {
			if ev, _ := windows.WaitForSingleObject(con.in, 100); ev == uint32(windows.WAIT_TIMEOUT) {
				continue
			}
		}
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			if send(FrameStdin, append([]byte(nil), buf[:n]...)) != nil {
				return
			}
		}
		if err != nil {
			_ = send(FrameStdinEOF, nil)
			return
		}
	}
}
