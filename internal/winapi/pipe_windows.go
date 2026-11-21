package winapi

// pipe_windows.go — a named pipe handle opened for overlapped I/O, so
// one goroutine can read while another writes (synchronous pipe handles
// serialise them, which would hang a command waiting for typed keys while
// it prints).

import (
	"errors"
	"io"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// PipeConn is a pipe handle opened for overlapped I/O.
type PipeConn struct {
	h         windows.Handle
	closeOnce sync.Once
}

// NewPipeConn wraps h, opened with FILE_FLAG_OVERLAPPED.
func NewPipeConn(h windows.Handle) *PipeConn { return &PipeConn{h: h} }

// Handle is the pipe handle.
func (p *PipeConn) Handle() windows.Handle { return p.h }

// overlapped runs one overlapped operation and waits for it.
func (p *PipeConn) overlapped(op func(*windows.Overlapped) error) (uint32, error) {
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(ev)
	ov := windows.Overlapped{HEvent: ev}
	err = op(&ov)
	if err != nil && err != windows.ERROR_IO_PENDING {
		return 0, err
	}
	var n uint32
	if err := windows.GetOverlappedResult(p.h, &ov, &n, true); err != nil {
		return n, err
	}
	return n, nil
}

func (p *PipeConn) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	n, err := p.overlapped(func(ov *windows.Overlapped) error {
		return windows.ReadFile(p.h, b, nil, ov)
	})
	if err == windows.ERROR_BROKEN_PIPE || err == windows.ERROR_PIPE_NOT_CONNECTED || err == windows.ERROR_NO_DATA {
		return int(n), io.EOF
	}
	if err == nil && n == 0 {
		return 0, io.EOF
	}
	return int(n), err
}

func (p *PipeConn) Write(b []byte) (int, error) {
	written := 0
	for written < len(b) {
		n, err := p.overlapped(func(ov *windows.Overlapped) error {
			return windows.WriteFile(p.h, b[written:], nil, ov)
		})
		written += int(n)
		if err != nil {
			return written, err
		}
		if n == 0 {
			return written, errors.New("pipe write made no progress")
		}
	}
	return written, nil
}

// Connect waits for a client on a server end.
func (p *PipeConn) Connect() error {
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(ev)
	ov := windows.Overlapped{HEvent: ev}
	switch err := windows.ConnectNamedPipe(p.h, &ov); err {
	case nil, windows.ERROR_PIPE_CONNECTED:
		return nil
	case windows.ERROR_IO_PENDING:
		var n uint32
		return windows.GetOverlappedResult(p.h, &ov, &n, true)
	default:
		return err
	}
}

func (p *PipeConn) Close() error {
	var err error
	p.closeOnce.Do(func() { err = windows.CloseHandle(p.h) })
	return err
}

// Listener makes server ends of one pipe name.
type Listener struct {
	name  *uint16
	sa    *windows.SecurityAttributes
	first bool
}

// Listen prepares server ends of name with the security descriptor sddl.
// The first instance fails if anyone created the pipe before: nobody may
// answer in KC9K's place.
func Listen(name, sddl string) (*Listener, error) {
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, err
	}
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	return &Listener{name: n, first: true,
		sa: &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}}, nil
}

// Accept makes a new instance and waits for a client on it.
func (l *Listener) Accept() (*PipeConn, error) {
	flags := uint32(windows.PIPE_ACCESS_DUPLEX | windows.FILE_FLAG_OVERLAPPED)
	if l.first {
		flags |= windows.FILE_FLAG_FIRST_PIPE_INSTANCE
	}
	h, err := windows.CreateNamedPipe(l.name, flags,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|windows.PIPE_REJECT_REMOTE_CLIENTS,
		windows.PIPE_UNLIMITED_INSTANCES, 64<<10, 64<<10, 0, l.sa)
	if err != nil {
		return nil, err
	}
	l.first = false
	c := NewPipeConn(h)
	if err := c.Connect(); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// DialPipe opens the client end of name, waiting up to wait while every
// instance is busy. A missing pipe is windows.ERROR_FILE_NOT_FOUND.
func DialPipe(name string, wait time.Duration) (*PipeConn, error) {
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		h, err := windows.CreateFile(n, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
			windows.FILE_FLAG_OVERLAPPED|windows.SECURITY_SQOS_PRESENT|windows.SECURITY_IMPERSONATION, 0)
		if err == nil {
			return NewPipeConn(h), nil
		}
		if err != windows.ERROR_PIPE_BUSY || time.Now().After(deadline) {
			return nil, err
		}
		_ = WaitNamedPipe(name, 2000)
	}
}

// ServerPID is the process at the server end of a client's pipe.
func (p *PipeConn) ServerPID() (uint32, error) {
	var pid uint32
	err := windows.GetNamedPipeServerProcessId(p.h, &pid)
	return pid, err
}

// ClientPID is the process at the client end of a server's pipe.
func (p *PipeConn) ClientPID() (uint32, error) {
	var pid uint32
	err := windows.GetNamedPipeClientProcessId(p.h, &pid)
	return pid, err
}

// ProcessImage is the full path of process pid's executable.
func ProcessImage(pid uint32) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf[:n]), nil
}
