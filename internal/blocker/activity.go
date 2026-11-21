package blocker

import (
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
)

// ForegroundApps returns the app in the focused window of every desktop user
// who is actually there: not behind a locked screen and not idle for longer
// than idleLimit seconds. Statistics count this time as app use.
//
// It reads the focused window through xprop on display :0 (X11). An app is
// named by its Flatpak app ID when it runs in a Flatpak, otherwise by its
// executable, the same names a block's apps use.
func ForegroundApps(idleLimit uint64) []string {
	var apps []string
	for _, a := range ForegroundAppsByUser(idleLimit) {
		apps = append(apps, a.App)
	}
	return apps
}

// UserApp is an app in the foreground of a desktop user's session.
type UserApp struct{ User, App string }

// ForegroundAppsByUser is ForegroundApps with the user each app is in the
// foreground for (§4.9 statistics per user).
func ForegroundAppsByUser(idleLimit uint64) []UserApp {
	xprop, err := exec.LookPath("xprop")
	if err != nil {
		return nil
	}
	gdbus, _ := exec.LookPath("gdbus")
	var apps []UserApp
	for _, u := range desktopUsers() {
		if awayFromDesktop(u, gdbus, idleLimit) {
			continue
		}
		out, err := asUser(u, xprop, "-root", "_NET_ACTIVE_WINDOW").Output()
		if err != nil {
			continue
		}
		window := lastField(string(out))
		if !strings.HasPrefix(window, "0x") || window == "0x0" {
			continue
		}
		out, err = asUser(u, xprop, "-id", window, "_NET_WM_PID").Output()
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(lastField(string(out)))
		if err != nil || pid <= 0 {
			continue
		}
		if name := appName(pid); name != "" {
			apps = append(apps, UserApp{u.Username, name})
		}
	}
	return apps
}

// SomeoneAtDesktop reports whether a desktop user is at the computer: signed
// in, the screen not locked and input within idleLimit seconds. When it
// can't tell (no logind, no X session and no Cinnamon screensaver) it says
// yes for a signed-in user.
func SomeoneAtDesktop(idleLimit uint64) bool {
	gdbus, _ := exec.LookPath("gdbus")
	for _, u := range desktopUsers() {
		if !awayFromDesktop(u, gdbus, idleLimit) {
			return true
		}
	}
	return false
}

// awayFromDesktop reports whether u's screen is locked (screenLocked) or u
// has been idle for longer than idleLimit seconds. The idle time comes from
// the X server (x11IdleSeconds), else from Cinnamon's screensaver (6 no
// longer answers it). When neither can tell, it reports false.
func awayFromDesktop(u *user.User, gdbus string, idleLimit uint64) bool {
	call := func(method string) string {
		if gdbus == "" {
			return ""
		}
		out, err := asUser(u, gdbus, "call", "--session", "--timeout", "1",
			"--dest", "org.cinnamon.ScreenSaver", "--object-path", "/org/cinnamon/ScreenSaver",
			"--method", "org.cinnamon.ScreenSaver."+method).Output()
		if err != nil {
			return ""
		}
		return string(out)
	}
	// Locked: asked of logind, not the session bus, where any program the
	// user runs can answer for org.cinnamon.ScreenSaver (B8, 2026-11-21).
	if screenLocked(u.Uid) {
		return true
	}
	// The X server's idle time first: the session bus is the user's own,
	// and an answer there saying "idle" would stop use being counted.
	idle, ok := x11IdleSeconds(u)
	if !ok {
		var err error
		if idle, err = strconv.ParseUint(lastField(call("GetSessionIdleTime")), 10, 64); err != nil {
			return false
		}
	}
	return idle > idleLimit
}

// screenLocked reports whether the user uid's screen is locked: every
// desktop (X11 or Wayland) session logind keeps for them has its
// LockedHint set, which the desktop's screen locker sets (Cinnamon, GNOME,
// KDE). No desktop session, or no logind: not locked. Replaceable in tests.
var screenLocked = func(uid string) bool { return lockedByLogind(uid, runLoginctl) }

// runLoginctl runs loginctl with args and returns what it printed.
func runLoginctl(args ...string) (string, error) {
	path, err := exec.LookPath("loginctl")
	if err != nil {
		return "", err
	}
	out, err := exec.Command(path, args...).Output()
	return string(out), err
}

// lockedByLogind is screenLocked, asking logind through loginctl.
func lockedByLogind(uid string, loginctl func(args ...string) (string, error)) bool {
	if _, err := strconv.ParseUint(uid, 10, 32); err != nil {
		return false
	}
	out, err := loginctl("show-user", uid, "--property=Sessions", "--value")
	if err != nil {
		return false
	}
	desktops, locked := 0, 0
	for _, id := range strings.Fields(out) {
		if !sessionID(id) {
			continue
		}
		props, err := loginctl("show-session", id, "--property=Type", "--property=LockedHint")
		if err != nil {
			continue
		}
		p := keyValues(props)
		if p["Type"] != "x11" && p["Type"] != "wayland" {
			continue // a text console or an ssh login has no screen to lock
		}
		desktops++
		if p["LockedHint"] == "yes" {
			locked++
		}
	}
	return desktops > 0 && locked == desktops
}

// sessionID reports whether s looks like a logind session ID ("3", "c2").
func sessionID(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	return true
}

// keyValues reads "Key=value" lines.
func keyValues(s string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			out[k] = v
		}
	}
	return out
}

func lastField(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[len(fields)-1], ",()")
}

// appName names the app a process belongs to.
func appName(pid int) string {
	proc := "/proc/" + strconv.Itoa(pid)
	if data, err := os.ReadFile(proc + "/cgroup"); err == nil {
		if id := flatpakAppFromCgroup(string(data)); id != "" {
			return id
		}
	}
	if exe, err := os.Readlink(proc + "/exe"); err == nil {
		return baseName(strings.TrimSuffix(exe, " (deleted)"))
	}
	return readComm(pid)
}
