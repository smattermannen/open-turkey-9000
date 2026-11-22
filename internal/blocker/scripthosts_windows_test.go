package blocker

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// startIn starts a program in dir and ends it when the test does.
func startIn(t *testing.T, dir string, name string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		t.Skipf("%s: %v", name, err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	return cmd
}

func gone(cmd *exec.Cmd) bool {
	done := make(chan struct{})
	go func() { _, _ = cmd.Process.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}

// W12: app entries naming scripts close the script hosts running them, for
// real: a batch file, a PowerShell script, a Python script.
func TestWinProcScriptEntries(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("kc9kloop.bat", "@ping -n 60 127.0.0.1 >nul\r\n")
	write("kc9kwait.ps1", "Start-Sleep 60\r\n")
	write("kc9kpy.py", "import time\ntime.sleep(60)\n")
	bat := startIn(t, dir, "cmd.exe", "/c", "kc9kloop.bat")
	ps := startIn(t, dir, "powershell.exe", "-NoProfile", "-File", "kc9kwait.ps1")
	var py *exec.Cmd
	if _, err := exec.LookPath("python.exe"); err == nil {
		py = startIn(t, dir, "python.exe", "kc9kpy.py")
	}
	other := startIn(t, dir, "cmd.exe", "/c", "ping -n 60 127.0.0.1 >nul")
	time.Sleep(2 * time.Second)
	if got := RunningApps([]string{"kc9kloop.bat", "kc9kwait", filepath.Join(dir, "kc9kpy.py")}); len(got) < 2 {
		t.Errorf("running scripts seen: %v", got)
	}
	n, err := KillBlocked([]string{"kc9kloop.bat", "kc9kwait", filepath.Join(dir, "kc9kpy.py")})
	if err != nil || n < 2 {
		t.Errorf("killed %d, %v", n, err)
	}
	if !gone(bat) {
		t.Error("the batch file's cmd still runs")
	}
	if !gone(ps) {
		t.Error("the PowerShell script still runs")
	}
	if py != nil && !gone(py) {
		t.Error("the Python script still runs")
	}
	if procs := processSnapshot(); procs[uint32(other.Process.Pid)] == nil {
		t.Error("a cmd running no blocked script was closed")
	}
}

// W12: a fetch tool run as a Python module, given a blocked site.
func TestWinProcFetchToolModule(t *testing.T) {
	if _, err := exec.LookPath("python.exe"); err != nil {
		t.Skip("no Python")
	}
	dir := t.TempDir()
	pkg := filepath.Join(dir, "yt_dlp")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "__main__.py"), []byte("import time\ntime.sleep(60)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(pkg, "__init__.py"), nil, 0o644)
	cmd := startIn(t, dir, "python.exe", "-m", "yt_dlp", "https://www.kc9k-blocked.example/watch")
	time.Sleep(2 * time.Second)
	tools := KillNamingFor([]string{"kc9k-blocked.example"}, nil)
	if !slices.Contains(tools, "yt-dlp") {
		t.Errorf("tools closed: %v", tools)
	}
	if !gone(cmd) {
		t.Error("python -m yt_dlp still runs")
	}
}
