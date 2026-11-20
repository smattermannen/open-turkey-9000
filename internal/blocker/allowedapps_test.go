package blocker

import "testing"

// R-FREEZE-1: only allowed programs and system processes keep running.
func TestAllowedAppKeep(t *testing.T) {
	for _, c := range []struct {
		exe              string
		allowed, ancestr bool
		keep             bool
	}{
		{"/usr/bin/cinnamon", false, false, true},
		{"/usr/libexec/gvfsd", false, false, true},
		{"/usr/lib/xorg/Xorg", false, false, true},
		{"/usr/bin/pipewire", false, false, true},
		{"/usr/local/bin/kc9k-bin", false, false, true},
		{"/usr/bin/steam", false, false, false},
		{"/home/me/game.AppImage", false, false, false},
		{"/usr/bin/libreoffice", true, false, true},
		{"/usr/lib/libreoffice/program/soffice.bin", false, true, true},
		{"/usr/bin/gnome-terminal-server", false, false, false},
		{"/usr/lib/firefox/firefox", false, false, false},
		{"/usr/lib/chromium/chromium", false, false, false},
		{"/usr/lib/libreoffice/program/soffice.bin", false, false, false},
	} {
		if got := allowedAppKeep(c.exe, c.allowed, c.ancestr); got != c.keep {
			t.Errorf("%s: keep = %v; want %v", c.exe, got, c.keep)
		}
	}
}

// R-FREEZE-1: which processes an "allowed apps only" freeze closes, on a
// made-up process tree: an allowed app's children and grandchildren stay,
// system and desktop programs stay, other users' and root's processes are
// never touched, a parent loop doesn't hang.
func TestAppsToClose(t *testing.T) {
	procs := []appProcess{
		{pid: 100, ppid: 1, exe: "/usr/bin/cinnamon-session", user: true},
		{pid: 200, ppid: 100, exe: "/usr/bin/libreoffice", user: true, allowed: true},
		{pid: 201, ppid: 200, exe: "/usr/lib/libreoffice/program/soffice.bin", user: true},
		{pid: 202, ppid: 201, exe: "/usr/bin/python3", user: true}, // a grandchild of the allowed app
		{pid: 300, ppid: 100, exe: "/usr/bin/steam", user: true},
		{pid: 301, ppid: 300, exe: "/home/me/.steam/game", user: true},
		{pid: 400, ppid: 1, exe: "/usr/sbin/sshd"},                 // root's
		{pid: 500, ppid: 1, exe: "/usr/libexec/gvfsd", user: true}, // desktop helper
		{pid: 600, ppid: 601, exe: "/usr/bin/loop-a", user: true},  // a parent loop
		{pid: 601, ppid: 600, exe: "/usr/bin/loop-b", user: true},
		{pid: 700, ppid: 100, exe: "", user: true},                // program unreadable
		{pid: 800, ppid: 999, exe: "/usr/bin/orphan", user: true}, // parent gone
	}
	var got []int
	for _, p := range appsToClose(procs) {
		got = append(got, p.pid)
	}
	want := []int{300, 301, 600, 601, 800}
	if len(got) != len(want) {
		t.Fatalf("closed %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("closed %v; want %v", got, want)
		}
	}
}

// CloseAllExcept closes what appsToClose picks, through killApp, and counts
// only the ones that ended.
func TestCloseAllExcept(t *testing.T) {
	oldRead, oldKill := readAppProcesses, killApp
	t.Cleanup(func() { readAppProcesses, killApp = oldRead, oldKill })
	var gotAllowed []string
	readAppProcesses = func(allowed []string) []appProcess {
		gotAllowed = allowed
		return []appProcess{
			{pid: 10, ppid: 1, exe: "/usr/bin/steam", user: true},
			{pid: 11, ppid: 1, exe: "/usr/bin/discord", user: true},
			{pid: 12, ppid: 1, exe: "/usr/bin/libreoffice", user: true, allowed: true},
		}
	}
	var killed []int
	killApp = func(pid int) error {
		killed = append(killed, pid)
		if pid == 11 {
			return syscallErr("gone")
		}
		return nil
	}
	if n := CloseAllExcept([]string{"libreoffice"}); n != 1 || len(killed) != 2 || len(gotAllowed) != 1 {
		t.Errorf("closed %d, killed %v, allowed %v", n, killed, gotAllowed)
	}
}

type syscallErr string

func (e syscallErr) Error() string { return string(e) }
