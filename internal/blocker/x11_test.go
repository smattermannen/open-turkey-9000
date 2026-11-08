package blocker

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestXauthCookie(t *testing.T) {
	entry := func(display, name string, cookie []byte) []byte {
		field := func(b []byte) []byte { return append([]byte{byte(len(b) >> 8), byte(len(b))}, b...) }
		out := []byte{1, 0} // family: local
		out = append(out, field([]byte("host"))...)
		out = append(out, field([]byte(display))...)
		out = append(out, field([]byte(name))...)
		return append(out, field(cookie)...)
	}
	data := append(entry("1", "MIT-MAGIC-COOKIE-1", []byte("one")), entry("0", "MIT-MAGIC-COOKIE-1", []byte("zero"))...)
	if got := string(xauthCookie(data, "0")); got != "zero" {
		t.Errorf("display 0: %q", got)
	}
	if got := string(xauthCookie(data, "5")); got != "one" {
		t.Errorf("unknown display falls back to the first cookie: %q", got)
	}
	if got := xauthCookie(data[:7], "0"); got != nil {
		t.Errorf("truncated file: %q", got)
	}
	if displayNumber(":0.0") != "0" || displayNumber(":12") != "12" {
		t.Error("display numbers")
	}
}

// startX starts a virtual X server needing the returned Xauthority file.
func startX(t *testing.T) (display, xauth string) {
	t.Helper()
	for _, tool := range []string{"Xvfb", "xauth", "xterm"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed", tool)
		}
	}
	display = "77"
	for i := 77; i < 99; i++ {
		if _, err := os.Stat("/tmp/.X11-unix/X" + strings.TrimPrefix(display, ":")); os.IsNotExist(err) {
			break
		}
		display = string(rune('0'+i/10)) + string(rune('0'+i%10))
	}
	xauth = filepath.Join(t.TempDir(), "Xauthority")
	key := make([]byte, 16)
	rand.Read(key)
	if out, err := exec.Command("xauth", "-f", xauth, "add", ":"+display, ".", hex.EncodeToString(key)).CombinedOutput(); err != nil {
		t.Fatalf("xauth: %v %s", err, out)
	}
	server := exec.Command("Xvfb", ":"+display, "-auth", xauth, "-screen", "0", "800x600x24", "-nolisten", "tcp")
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Process.Kill(); server.Wait() })
	for i := 0; i < 50; i++ {
		if _, err := os.Stat("/tmp/.X11-unix/X" + display); err == nil {
			return ":" + display, xauth
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("Xvfb didn't start")
	return "", ""
}

func startClient(t *testing.T, display, xauth string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), "DISPLAY="+display, "XAUTHORITY="+xauth)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	return cmd
}

func findWindow(t *testing.T, display, xauth, title string) (X11Window, bool) {
	t.Helper()
	for i := 0; i < 40; i++ {
		c, err := openUserDisplay(display, xauth)
		if err != nil {
			t.Fatal(err)
		}
		windows, err := c.windows()
		c.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range windows {
			if strings.Contains(w.Title, title) {
				return w, true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return X11Window{}, false
}

func TestX11ListsAndClosesWindows(t *testing.T) {
	display, xauth := startX(t)

	// Without a window manager: found among the root's children.
	// sleep, not a shell: a shell's prompt would retitle the window.
	term := startClient(t, display, xauth, "xterm", "-T", "Watching YouTube - Player", "-e", "sleep", "60")
	w, ok := findWindow(t, display, xauth, "YouTube")
	if !ok {
		t.Fatal("window not found without a window manager")
	}
	if w.PID != term.Process.Pid {
		t.Errorf("window PID %d, want %d", w.PID, term.Process.Pid)
	}

	// A wrong cookie is refused.
	if _, err := dialX11(strings.TrimPrefix(display, ":"), []byte("0123456789abcdef")); err == nil {
		t.Error("connected with a wrong cookie")
	}

	if _, err := exec.LookPath("openbox"); err != nil {
		t.Skip("openbox not installed: closing needs a window manager")
	}
	startClient(t, display, xauth, "openbox")
	time.Sleep(time.Second)
	w, ok = findWindow(t, display, xauth, "YouTube")
	if !ok {
		t.Fatal("window not found through the window manager's client list")
	}
	c, err := openUserDisplay(display, xauth)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.closeWindow(w.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { term.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the window didn't close when asked")
	}
}
