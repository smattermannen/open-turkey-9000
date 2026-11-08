//go:build roottest

// Run as root on a disposable machine with Playwright's Chromium:
//
//	go test -tags roottest -run Guard ./internal/blocker
package blocker

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func startChrome(t *testing.T, extra ...string) *exec.Cmd {
	t.Helper()
	matches, _ := filepath.Glob("/opt/pw-browsers/chromium-*/chrome-linux*/chrome")
	if len(matches) == 0 {
		t.Skip("no Chromium to test with")
	}
	home := t.TempDir()
	if err := os.Chown(home, 1500, 1500); err != nil {
		t.Fatal(err)
	}
	args := append([]string{"--headless", "--no-sandbox", "--disable-gpu", "--user-data-dir=" + home + "/profile"}, extra...)
	cmd := exec.Command(matches[0], append(args, "about:blank")...)
	cmd.Env = []string{"HOME=" + home}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 1500, Gid: 1500}}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	time.Sleep(2 * time.Second)
	return cmd
}

func alive(cmd *exec.Cmd) bool { return cmd.Process.Signal(syscall.Signal(0)) == nil }

// A browser started for remote control is closed at once; a normal one gets
// its grace period to check in.
func TestGuardClosesRemoteControlledBrowser(t *testing.T) {
	g := NewExtensionGuard(47613)
	g.started = time.Now().Add(-2 * ExtensionGrace) // the daemon has been up a while

	normal := startChrome(t)
	remote := startChrome(t, "--remote-debugging-port=9333")

	g.Enforce(time.Now())
	done := make(chan struct{})
	go func() { _, _ = remote.Process.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the remote-controlled browser is still running")
	}
	if !alive(normal) {
		t.Fatal("a freshly started normal browser was closed inside its grace period")
	}
}
