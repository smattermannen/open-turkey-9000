// allowedapps.go — Frozen Chicken's "allowed apps only" mode (§3.8,
// R-FREEZE-1): while it runs, a regular user's programs are closed unless
// they are on the freeze's app list (or run under one that is), or are
// system and desktop processes.
package blocker

import (
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// sessionHelpers keep the desktop working: services every session runs.
var sessionHelpers = []string{"gvfs", "dconf-service", "evolution-", "ibus", "fcitx", "polkit", "gpg-agent",
	"ssh-agent", "goa-", "tracker", "obexd", "nm-applet", "blueman", "mintupdate", "xapp", "nemo-desktop",
	"systemd", "at-spi", "pipewire", "wireplumber", "pulseaudio", "dbus", "xdg-", "gnome-keyring"}

// systemExe reports whether a program is part of the system or the desktop
// rather than something the user runs: an essential app, a session helper,
// or a program in the system's helper folders.
func systemExe(path string) bool {
	name := strings.ToLower(baseName(path))
	if isEssentialApp(name) {
		return true
	}
	for _, h := range sessionHelpers {
		if strings.HasPrefix(name, h) {
			return true
		}
	}
	// Helper folders only: whole programs (browsers, office suites) also
	// live under /usr/lib and must not count as system.
	for _, dir := range []string{"/usr/libexec/", "/usr/lib/xorg/", "/usr/lib/systemd/", "/usr/lib/policykit-1/",
		"/usr/lib/gvfs/", "/usr/lib/cinnamon", "/usr/lib/x86_64-linux-gnu/", "/usr/lib/aarch64-linux-gnu/",
		"/usr/local/bin/kc9k"} {
		if strings.HasPrefix(path, dir) {
			return true
		}
	}
	return false
}

// allowedAppKeep decides one process: keep it if it is a system program,
// allowed, or started by an allowed program (ancestorAllowed).
func allowedAppKeep(exe string, allowed, ancestorAllowed bool) bool {
	return allowed || ancestorAllowed || systemExe(exe)
}

// appProcess is one process as "allowed apps only" judges it.
type appProcess struct {
	pid, ppid int
	exe       string // the program file ("" when it can't be read)
	user      bool   // a regular user's process
	allowed   bool   // matches an entry of the app list
}

// readAppProcesses lists every process with what allowedAppKeep needs.
// Replaced in tests.
var readAppProcesses = func(allowed []string) []appProcess {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	flatpakIDs := flatpakAppIDs(allowed)
	var out []appProcess
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		ppid, ok := readPPID(pid)
		if !ok {
			continue
		}
		p := appProcess{pid: pid, ppid: ppid}
		p.allowed = len(allowed) > 0 && (matchesBlockedProcess(pid, allowed) || matchesProcessScope(pid, allowed, flatpakIDs))
		if p.user = ownedByUser(pid); p.user {
			if exe, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe"); err == nil {
				p.exe = strings.TrimSuffix(exe, " (deleted)")
			}
		}
		out = append(out, p)
	}
	return out
}

// killApp ends one process. Replaced in tests.
var killApp = func(pid int) error { return syscall.Kill(pid, syscall.SIGKILL) }

// appsToClose picks the regular users' processes allowedAppKeep doesn't
// keep: not system or desktop programs, not allowed, and not started (at
// any depth, through at most 64 parents) by an allowed program. A process
// whose program can't be read is left alone.
func appsToClose(procs []appProcess) []appProcess {
	byPID := make(map[int]appProcess, len(procs))
	for _, p := range procs {
		byPID[p.pid] = p
	}
	underAllowed := func(p appProcess) bool {
		for pid, n := p.ppid, 0; pid > 1 && n < 64; n++ {
			parent, ok := byPID[pid]
			if !ok {
				return false
			}
			if parent.allowed {
				return true
			}
			pid = parent.ppid
		}
		return false
	}
	var out []appProcess
	for _, p := range procs {
		if !p.user || p.exe == "" || allowedAppKeep(p.exe, p.allowed, underAllowed(p)) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// CloseAllExcept closes every regular user's process that allowedAppKeep
// doesn't keep, matching allowed entries as app blocks match theirs
// (names, paths, Flatpak IDs, scripts). It returns how many it closed.
func CloseAllExcept(allowed []string) int {
	closed := 0
	for _, p := range appsToClose(readAppProcesses(allowed)) {
		if err := killApp(p.pid); err == nil {
			log.Printf("freeze: closed %s (pid %d): not an allowed app", baseName(p.exe), p.pid)
			closed++
		}
	}
	return closed
}
