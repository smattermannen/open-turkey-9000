package blocker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// W50: no program in System32 is a browser (System32\Microsoft-Edge-WebView
// holds msedge.dll; the former glob made every System32 program one).
// Real folder listing on windows-latest.
func TestWinGuardSystem32NotBrowsers(t *testing.T) {
	sys := filepath.Join(systemRoot(), "System32")
	if fileExists(filepath.Join(sys, "Microsoft-Edge-WebView", "msedge.dll")) {
		t.Log("System32 has Microsoft-Edge-WebView\\msedge.dll: the case that froze the laptop")
	}
	entries, err := os.ReadDir(sys)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".exe") {
			continue
		}
		exe := filepath.Join(sys, e.Name())
		n++
		if isBrowserProgram(exe) {
			t.Errorf("%s counts as a browser program", exe)
		}
		if isBrowserProc(&winProc{PID: 99999, Name: exeBase(exe), Exe: exe, UID: 1001}) {
			t.Errorf("%s counts as a browser process", exe)
		}
	}
	if n < 100 {
		t.Fatalf("only %d programs in %s", n, sys)
	}
}

// Browsers are still recognised: by name, by xul.dll beside them, by
// chrome.dll or msedge.dll beside them or in a version folder; a folder
// that isn't a version doesn't count.
func TestWinGuardBrowserFixtures(t *testing.T) {
	dir := t.TempDir()
	mk := func(parts ...string) string {
		p := filepath.Join(append([]string{dir}, parts...)...)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	ff := mk("ff", "renamed.exe")
	mk("ff", "xul.dll")
	chrome := mk("chrome", "Application", "chrome.exe")
	mk("chrome", "Application", "120.0.6099.71", "chrome.dll")
	edge := mk("edge", "Application", "msedge.exe")
	mk("edge", "Application", "131.0.2903.70", "msedge.dll")
	tool := mk("tool", "tool.exe")
	mk("tool", "Microsoft-Edge-WebView", "msedge.dll")
	for _, exe := range []string{ff, chrome, edge} {
		if !isBrowserProgram(exe) {
			t.Errorf("%s is a browser", exe)
		}
	}
	if isBrowserProgram(tool) {
		t.Errorf("%s (msedge.dll in a non-version folder) counts as a browser", tool)
	}
	for _, name := range []string{"msedge", "chrome", "firefox", "librewolf"} {
		exe := mk(name, name+".exe")
		if !isBrowserProc(&winProc{PID: 99998, Name: name, Exe: exe, UID: 1001}) {
			t.Errorf("%s is a browser", name)
		}
	}
	for name, want := range map[string]bool{"120.0.6099.71": true, "1.2": true, "Locales": false, "1.": false, ".1": false, "12": false, "1..2": false} {
		if versionFolder(name) != want {
			t.Errorf("versionFolder(%q) = %v", name, !want)
		}
	}
}

// fakeTerminate records what would be ended.
func fakeTerminate(t *testing.T) *[]uint32 {
	old := terminateProcess
	t.Cleanup(func() { terminateProcess = old })
	var ended []uint32
	terminateProcess = func(pid uint32) error { ended = append(ended, pid); return nil }
	return &ended
}

// The hard deny-list: no kill path ends a session-critical process; only
// a rule-named one may end a program in Windows' folder.
func TestWinKillDenyList(t *testing.T) {
	ended := fakeTerminate(t)
	sys := filepath.Join(systemRoot(), "System32")
	self, _ := os.Executable()
	critical := []*winProc{{PID: 4, Name: "system"}, {PID: 0, Name: "idle"}}
	for i, name := range []string{"dwm", "csrss", "winlogon", "wininit", "smss", "lsass", "services", "svchost",
		"fontdrvhost", "sihost", "ctfmon", "explorer", "shellhost", "shellexperiencehost", "startmenuexperiencehost",
		"runtimebroker", "dllhost", "applicationframehost", "textinputhost", "logonui"} {
		exe := filepath.Join(sys, name+".exe")
		if name == "explorer" {
			exe = filepath.Join(systemRoot(), "explorer.exe")
		}
		critical = append(critical, &winProc{PID: uint32(5000 + i), Name: name, Exe: exe, UID: 1001},
			&winProc{PID: uint32(6000 + i), Name: name, UID: 1001}) // unreadable path
	}
	critical = append(critical, &winProc{PID: 7000, Name: exeBase(self), Exe: self, UID: 1001})
	for _, p := range critical {
		if p.Name == exeBase(self) {
			p.Name = "kc9k"
		}
		if killProcess(p) == nil || killNamedProcess(p) == nil {
			t.Errorf("%s (%s) was ended", p.Name, p.Exe)
		}
	}
	if len(*ended) != 0 {
		t.Fatalf("ended %v", *ended)
	}
	notepad := &winProc{PID: 8000, Name: "notepad", Exe: filepath.Join(sys, "notepad.exe"), UID: 1001}
	if killProcess(notepad) == nil {
		t.Error("an unnamed kill ended a program of Windows' folder")
	}
	taskmgr := &winProc{PID: 8001, Name: "taskmgr", Exe: filepath.Join(sys, "Taskmgr.exe"), UID: 1001}
	if killNamedProcess(taskmgr) != nil {
		t.Error("a blocked task manager was not ended")
	}
	fake := &winProc{PID: 8002, Name: "svchost", Exe: `C:\Users\me\Downloads\svchost.exe`, UID: 1001}
	if killProcess(fake) != nil {
		t.Error("svchost.exe in Downloads was spared")
	}
	if len(*ended) != 2 || (*ended)[0] != 8001 || (*ended)[1] != 8002 {
		t.Errorf("ended %v, want [8001 8002]", *ended)
	}
}

// What froze the laptop, repeated: the extension guard on, past its grace,
// unsupported browsers blocked, no extension check-in; Windows' own
// programs (dwm in System32) run beside a browser. Only the browser is
// closed. Through the blocked-app path too: a dwm under a blocked app is
// spared, and Frozen Chicken's close-all spares it.
func TestWinGuardNeverClosesWindows(t *testing.T) {
	ended := fakeTerminate(t)
	oldSnap := processSnapshot
	defer func() { processSnapshot = oldSnap }()
	sys := filepath.Join(systemRoot(), "System32")
	dir := t.TempDir()
	lw := filepath.Join(dir, "LibreWolf", "librewolf.exe")
	_ = os.MkdirAll(filepath.Dir(lw), 0o755)
	_ = os.WriteFile(lw, []byte("x"), 0o644)
	long := time.Now().Add(-time.Hour)
	procs := map[uint32]*winProc{}
	add := func(pid, ppid uint32, name, exe string) {
		procs[pid] = &winProc{PID: pid, PPID: ppid, Name: name, Exe: exe, UID: 1001, Session: 1, Started: long}
	}
	for i, name := range []string{"dwm", "fontdrvhost", "sihost", "ctfmon", "svchost", "runtimebroker", "dllhost", "notepad", "cmd"} {
		add(uint32(91000+i), 1, name, filepath.Join(sys, name+".exe"))
	}
	add(91100, 1, "shellhost", filepath.Join(systemRoot(), "SystemApps", "ShellHost.exe"))
	add(91200, 1, "librewolf", lw)
	add(91300, 1, "game", filepath.Join(dir, "game.exe"))
	add(91301, 91300, "dwm", filepath.Join(sys, "dwm.exe"))
	processSnapshot = func() map[uint32]*winProc { return procs }

	g := NewExtensionGuard(1)
	g.started = time.Now().Add(-time.Hour)
	for i := 0; i < 3; i++ {
		g.EnforceFor(time.Now(), time.Time{}, true, nil)
	}
	for _, pid := range *ended {
		if pid != 91200 {
			t.Errorf("the extension guard ended %s (pid %d)", procs[pid].Name, pid)
		}
	}
	if len(*ended) == 0 {
		t.Error("the browser without an extension was not closed")
	}
	*ended = nil
	if _, err := KillBlocked([]string{"game", "dwm", "explorer", "svchost"}); err != nil {
		t.Fatal(err)
	}
	if len(*ended) != 1 || (*ended)[0] != 91300 {
		t.Errorf("blocking apps ended %v, want only the game", *ended)
	}
	*ended = nil
	CloseAllExcept(nil)
	for _, pid := range *ended {
		if p := procs[pid]; underSystemRoot(p.Exe) {
			t.Errorf("Frozen Chicken ended %s", p.Name)
		}
	}
}
