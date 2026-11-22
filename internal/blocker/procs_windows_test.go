package blocker

import (
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestWinProcSnapshot(t *testing.T) {
	procs := processSnapshot()
	self := procs[uint32(os.Getpid())]
	if self == nil {
		t.Fatal("this process is not in the snapshot")
	}
	exe, _ := os.Executable()
	if !strings.EqualFold(self.Exe, exe) {
		t.Errorf("path = %q, want %q", self.Exe, exe)
	}
	if self.UID < 0 || self.Started.IsZero() || time.Since(self.Started) > time.Hour {
		t.Errorf("owner %d, started %v", self.UID, self.Started)
	}
	if parentOf(procs, self) == nil {
		t.Error("no parent found for this process")
	}
	if isBrowserProc(self) {
		t.Error("the test program is taken for a browser")
	}
}

func TestWinProcKillTree(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "ping -n 60 127.0.0.1 >nul")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	procs := processSnapshot()
	tree := treeOf(uint32(cmd.Process.Pid), childrenOf(procs))
	if len(tree) < 2 {
		t.Fatalf("tree of cmd.exe = %v, want it and ping", tree)
	}
	for i := len(tree) - 1; i >= 0; i-- {
		if err := killProcess(tree[i]); err != nil {
			t.Errorf("kill %d: %v", tree[i], err)
		}
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cmd.exe still runs")
	}
	time.Sleep(500 * time.Millisecond)
	if p := processSnapshot()[tree[1]]; p != nil && p.Name == "ping" {
		t.Error("ping still runs")
	}
}

func TestWinProcTCPOwner(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		c, err := l.Accept()
		if err == nil {
			time.Sleep(2 * time.Second)
			c.Close()
		}
	}()
	c, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	server := l.Addr().(*net.TCPAddr).Port
	if pid := tcpOwner(localPort(c.LocalAddr().String()), server); pid != uint32(os.Getpid()) {
		t.Errorf("tcpOwner = %d, want this process %d", pid, os.Getpid())
	}
	if name := ClientUser(c.LocalAddr().String(), server); name == "" {
		t.Error("ClientUser found no account")
	}
}

func TestWinProcBrowserRecognition(t *testing.T) {
	dir := t.TempDir()
	// A renamed program next to a browser engine is a browser.
	exe := dir + `\totally-not-a-browser.exe`
	os.WriteFile(exe, []byte("MZ"), 0o755)
	if isBrowserProgram(exe) {
		t.Error("a program alone is taken for a browser")
	}
	os.WriteFile(dir+`\xul.dll`, []byte("MZ"), 0o644)
	if !isBrowserProgram(exe) {
		t.Error("a program next to xul.dll is not taken for a browser")
	}
	dir2 := t.TempDir()
	os.MkdirAll(dir2+`\131.0.1`, 0o755)
	os.WriteFile(dir2+`\131.0.1\chrome.dll`, []byte("MZ"), 0o644)
	if !isBrowserProgram(dir2 + `\work.exe`) {
		t.Error("a program with chrome.dll in a version folder is not taken for a browser")
	}
}

// W11: another process's environment, read as the guard reads a browser's.
func TestWinProcRemoteControlEnvironment(t *testing.T) {
	start := func(env ...string) *exec.Cmd {
		cmd := exec.Command("cmd.exe", "/c", "ping -n 30 127.0.0.1 >nul")
		cmd.Env = append(os.Environ(), env...)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
		return cmd
	}
	plain := start()
	driven := start("MOZ_MARIONETTE=1")
	time.Sleep(500 * time.Millisecond)
	env := processEnvironment(uint32(driven.Process.Pid))
	found := false
	for _, kv := range env {
		found = found || kv == "MOZ_MARIONETTE=1"
	}
	if !found {
		t.Fatalf("MOZ_MARIONETTE not in the environment read (%d entries)", len(env))
	}
	if !remoteControlledWin(uint32(driven.Process.Pid)) {
		t.Error("a process with MOZ_MARIONETTE is not taken as remote controlled")
	}
	if remoteControlledWin(uint32(plain.Process.Pid)) {
		t.Error("a plain process is taken as remote controlled")
	}
}
