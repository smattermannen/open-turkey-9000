//go:build roottest

// Run as root with Xvfb, xauth, xterm (and openbox for the polite close):
//
//	go test -tags roottest -run TitleGuard ./internal/blocker
package blocker

import (
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
	return copy
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
	g.sessions = func() []titleSession { return []titleSession{{display: display, xauth: xauth}} }
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

func TestTitleGuardAsksPolitelyFirst(t *testing.T) {
	if _, err := exec.LookPath("openbox"); err != nil {
		t.Skip("openbox not installed")
	}
	display, xauth := startX(t)
	startClient(t, display, xauth, "openbox")
	xauth = readableXauth(t, xauth)
	time.Sleep(time.Second)
	blocked := userXterm(t, display, xauth, "Watching YouTube - Player")
	if _, ok := findWindow(t, display, xauth, "YouTube"); !ok {
		t.Fatal("window never appeared")
	}
	g := NewTitleGuard()
	g.sessions = func() []titleSession { return []titleSession{{display: display, xauth: xauth}} }
	g.Enforce([]string{"YOUTUBE"}, time.Now())
	if !exited(blocked, 5*time.Second) {
		t.Fatal("the window didn't close when asked")
	}
}
