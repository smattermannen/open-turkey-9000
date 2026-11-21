//go:build roottest

// Run as root on a disposable machine with Playwright's Chromium:
//
//	go test -tags roottest -run Guard ./internal/blocker
package blocker

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	g.started = time.Now().Add(-2 * ExtensionGrace()) // the daemon has been up a while

	normal := startChrome(t)
	remote := startChrome(t, "--remote-debugging-port=9333")

	g.Enforce(time.Now(), time.Time{}, false)
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

// Browsers read their settings when they start, so one started before KC9K
// changed them is closed once the grace period after the change has passed;
// one started after the change is left alone.
func TestGuardRestartsBrowsersOlderThanSettings(t *testing.T) {
	g := NewExtensionGuard(47613)
	g.started = time.Now().Add(-2 * ExtensionGrace())

	before := startChrome(t)
	changed := time.Now()
	time.Sleep(1100 * time.Millisecond) // process start times count in 1/100 s
	after := startChrome(t)

	// Within the grace period nothing happens yet.
	g.Enforce(changed.Add(ExtensionGrace()/2), changed, false)
	if !alive(before) || !alive(after) {
		t.Fatal("a browser was closed inside the grace period after the change")
	}

	g.Enforce(changed.Add(2*ExtensionGrace()), changed, false)
	done := make(chan struct{})
	go func() { _, _ = before.Process.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the browser started before the change is still running")
	}
	if !alive(after) {
		t.Fatal("the browser started after the change was closed")
	}
}

// Block unsupported browsers and the extension guard (§4.10): a browser run
// from an AppImage (files on FUSE, shown as root's, closed to root) is the
// user's, so it is judged by its engine whatever its name, and is never a
// supported browser. KC9K_TEST_APPIMAGE names a browser AppImage (e.g. the
// official LibreWolf one); it runs as SUDO_UID.
func TestGuardJudgesAppImageBrowser(t *testing.T) {
	image := os.Getenv("KC9K_TEST_APPIMAGE")
	uid, _ := strconv.Atoi(os.Getenv("SUDO_UID"))
	if image == "" || uid < 1000 {
		t.Skip("set KC9K_TEST_APPIMAGE and run through sudo")
	}
	home := t.TempDir()
	if err := os.Chown(home, uid, uid); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(image, "--headless", "--no-remote", "--profile", home, "about:blank")
	cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin"}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(uid)}}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); _, _ = cmd.Process.Wait() })
	var pid int
	for deadline := time.Now().Add(20 * time.Second); pid == 0 && time.Now().Before(deadline); time.Sleep(500 * time.Millisecond) {
		parent, _ := processTree()
		for p := range parent {
			exe, _ := os.Readlink("/proc/" + strconv.Itoa(p) + "/exe")
			if pgid, _ := syscall.Getpgid(p); pgid == cmd.Process.Pid && strings.HasPrefix(exe, "/tmp/.mount_") &&
				exeName(parent[p]) != exeName(p) {
				if maps, err := os.ReadFile("/proc/" + strconv.Itoa(p) + "/maps"); err == nil && mapsBrowserEngine(string(maps)) {
					pid = p
				}
			}
		}
	}
	if pid == 0 {
		t.Fatal("no browser process from the AppImage")
	}
	if !exeOwnedByUser(pid) {
		t.Error("the AppImage's browser counts as installed by root")
	}
	if !engineBrowser(pid) {
		t.Error("the AppImage's browser isn't recognised by its engine")
	}
	if id := supportedExe(pid); id != "" {
		t.Errorf("the AppImage's browser counts as supported %s", id)
	}
}
