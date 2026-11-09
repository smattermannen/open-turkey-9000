//go:build roottest

// Run as root with Xvfb, xauth and xterm:
//
//	go test -tags roottest -run TitleGuard ./internal/blocker
package blocker

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// userXterm starts an xterm as a regular user (1500) with the given title.
func userXterm(t *testing.T, display, xauth, title string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("xterm", "-T", title, "-e", "sleep", "60")
	cmd.Env = []string{"DISPLAY=" + display, "XAUTHORITY=" + xauth, "HOME=/tmp", "PATH=/usr/bin:/bin"}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 1500, Gid: 1500}}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	return cmd
}

func exited(cmd *exec.Cmd, within time.Duration) bool {
	done := make(chan struct{})
	go func() { cmd.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(within):
		return false
	}
}

func readableXauth(t *testing.T, xauth string) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "kc9k-xauth-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	os.Chmod(dir, 0755)
	data, err := os.ReadFile(xauth)
	if err != nil {
		t.Fatal(err)
	}
	copy := filepath.Join(dir, "Xauthority")
	if err := os.WriteFile(copy, data, 0644); err != nil {
		t.Fatal(err)
	}
	// The user's own file, as ~/.Xauthority is: the guard reads no other.
	if err := os.Chown(copy, 1500, 1500); err != nil {
		t.Fatal(err)
	}
	return copy
}

// setWindowPID makes a window claim to belong to pid, as any X client can.
func setWindowPID(t *testing.T, display, xauth string, id uint32, pid int) {
	t.Helper()
	c, err := openUserDisplay(display, xauth, fileOwner(t, xauth))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	atom, err := c.atom("_NET_WM_PID")
	if err != nil {
		t.Fatal(err)
	}
	var req bytes.Buffer
	req.WriteByte(18) // ChangeProperty
	req.WriteByte(0)  // replace
	binary.Write(&req, binary.LittleEndian, uint16(7))
	binary.Write(&req, binary.LittleEndian, id)
	binary.Write(&req, binary.LittleEndian, atom)
	binary.Write(&req, binary.LittleEndian, uint32(6)) // CARDINAL
	req.Write([]byte{32, 0, 0, 0})
	binary.Write(&req, binary.LittleEndian, uint32(1))
	binary.Write(&req, binary.LittleEndian, uint32(pid))
	if _, _, err := c.request(req.Bytes(), false); err != nil {
		t.Fatal(err)
	}
	if got, _, err := c.property(id, "_NET_WM_PID"); err != nil || len(got) < 4 ||
		int(binary.LittleEndian.Uint32(got)) != pid {
		t.Fatalf("_NET_WM_PID not changed: %v %v", got, err)
	}
}

// A window can name any process as its own. The guard believes it only for
// a process of the desktop's own user, so no one can title a window to get
// another user's program killed.
func TestTitleGuardSparesOtherUsersProcesses(t *testing.T) {
	display, xauth := startX(t)
	xauth = readableXauth(t, xauth)
	userXterm(t, display, xauth, "Watching YouTube - Player")
	w, ok := findWindow(t, display, xauth, "YouTube")
	if !ok {
		t.Fatal("window never appeared")
	}
	victim := exec.Command("sleep", "60")
	victim.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 1501, Gid: 1501}}
	if err := victim.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { victim.Process.Kill(); victim.Wait() })
	setWindowPID(t, display, xauth, w.ID, victim.Process.Pid)

	g := NewTitleGuard()
	g.sessions = func() []titleSession { return []titleSession{{display: display, xauth: xauth, uid: 1500}} }
	now := time.Now()
	g.Enforce([]string{"youtube"}, now)
	if killed := g.Enforce([]string{"youtube"}, now.Add(TitleCloseGrace)); killed != 0 {
		t.Fatalf("killed %d processes for a window claiming another user's", killed)
	}
	if exited(victim, 500*time.Millisecond) {
		t.Fatal("another user's program was killed")
	}
}

func TestTitleGuardKillsWhenCloseIsIgnored(t *testing.T) {
	display, xauth := startX(t)
	xauth = readableXauth(t, xauth)
	blocked := userXterm(t, display, xauth, "Watching YouTube - Player")
	other := userXterm(t, display, xauth, "Notes")
	if _, ok := findWindow(t, display, xauth, "YouTube"); !ok {
		t.Fatal("window never appeared")
	}
	findWindow(t, display, xauth, "Notes")

	g := NewTitleGuard()
	g.sessions = func() []titleSession { return []titleSession{{display: display, xauth: xauth, uid: 1500}} }
	now := time.Now()
	if killed := g.Enforce([]string{"youtube"}, now); killed != 0 {
		t.Fatalf("killed %d on the first cycle; it should only ask", killed)
	}
	// No window manager here, so the request is ignored: the next cycle
	// after the grace period kills the program.
	if killed := g.Enforce([]string{"youtube"}, now.Add(TitleCloseGrace)); killed == 0 {
		t.Fatal("nothing killed after the grace period")
	}
	if !exited(blocked, 3*time.Second) {
		t.Fatal("the blocked window's program is still running")
	}
	if exited(other, 500*time.Millisecond) {
		t.Fatal("an unrelated window was closed")
	}
}

// The first cycle only asks: the guard sends the window manager a
// _NET_CLOSE_WINDOW for the window, as a pager's close button would, and
// kills nothing. (TestX11ListsAndClosesWindows shows a window manager acting
// on it.) Here a test client listens on the root window, as a window manager
// does, so the check doesn't depend on how fast xterm starts.
func TestTitleGuardAsksPolitelyFirst(t *testing.T) {
	display, xauth := startX(t)
	xauth = readableXauth(t, xauth)
	blocked := userXterm(t, display, xauth, "Watching YouTube - Player")
	w, ok := findWindow(t, display, xauth, "YouTube")
	if !ok {
		t.Fatal("window never appeared")
	}
	wm := listenRoot(t, display, xauth)
	closeAtom, err := wm.atom("_NET_CLOSE_WINDOW")
	if err != nil {
		t.Fatal(err)
	}

	g := NewTitleGuard()
	g.sessions = func() []titleSession { return []titleSession{{display: display, xauth: xauth, uid: 1500}} }
	if killed := g.Enforce([]string{"YOUTUBE"}, time.Now()); killed != 0 {
		t.Fatalf("killed %d on the first cycle; it should only ask", killed)
	}
	_ = wm.conn.SetDeadline(time.Now().Add(3 * time.Second))
	for {
		event := make([]byte, 32)
		if _, err := io.ReadFull(wm.r, event); err != nil {
			t.Fatalf("the window manager was never asked to close window %d: %v", w.ID, err)
		}
		// 33 is ClientMessage; a sent event has the top bit set.
		if event[0]&0x7f != 33 || binary.LittleEndian.Uint32(event[4:]) != w.ID ||
			binary.LittleEndian.Uint32(event[8:]) != closeAtom {
			continue
		}
		if event[1] != 32 || binary.LittleEndian.Uint32(event[16:]) != 2 {
			t.Fatalf("malformed _NET_CLOSE_WINDOW: % x", event)
		}
		break
	}
	if exited(blocked, 300*time.Millisecond) {
		t.Fatal("the program was killed instead of asked")
	}
}

// listenRoot returns a connection that receives what is sent to the root
// window for the window manager (SubstructureNotify, which any number of
// clients may select).
func listenRoot(t *testing.T, display, xauth string) *x11Conn {
	t.Helper()
	c, err := openUserDisplay(display, xauth, fileOwner(t, xauth))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	var req bytes.Buffer
	req.WriteByte(2) // ChangeWindowAttributes
	req.WriteByte(0)
	binary.Write(&req, binary.LittleEndian, uint16(4))
	binary.Write(&req, binary.LittleEndian, c.root)
	binary.Write(&req, binary.LittleEndian, uint32(1<<11)) // CWEventMask
	binary.Write(&req, binary.LittleEndian, uint32(1<<19)) // SubstructureNotify
	if _, _, err := c.request(req.Bytes(), false); err != nil {
		t.Fatal(err)
	}
	return c
}
