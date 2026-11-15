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

// CloseAllExcept closes every regular user's process that allowedAppKeep
// doesn't keep, matching allowed entries as app blocks match theirs
// (names, paths, Flatpak IDs, scripts). It returns how many it closed.
func CloseAllExcept(allowed []string) int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	flatpakIDs := flatpakAppIDs(allowed)
	isAllowed := map[int]bool{}
	parent := map[int]int{}
	var users []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		ppid, ok := readPPID(pid)
		if !ok {
			continue
		}
		parent[pid] = ppid
		if len(allowed) > 0 && (matchesBlockedProcess(pid, allowed) || matchesProcessScope(pid, allowed, flatpakIDs)) {
			isAllowed[pid] = true
		}
		if ownedByUser(pid) {
			users = append(users, pid)
		}
	}
	underAllowed := func(pid int) bool {
		for p, n := parent[pid], 0; p > 1 && n < 64; p, n = parent[p], n+1 {
			if isAllowed[p] {
				return true
			}
		}
		return false
	}
	closed := 0
	for _, pid := range users {
		exe, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe")
		if err != nil {
			continue
		}
		exe = strings.TrimSuffix(exe, " (deleted)")
		if allowedAppKeep(exe, isAllowed[pid], underAllowed(pid)) {
			continue
		}
		if err := syscall.Kill(pid, syscall.SIGKILL); err == nil {
			log.Printf("freeze: closed %s (pid %d): not an allowed app", baseName(exe), pid)
			closed++
		}
	}
	return closed
}
