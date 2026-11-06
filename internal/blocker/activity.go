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
	xprop, err := exec.LookPath("xprop")
	if err != nil {
		return nil
	}
	gdbus, _ := exec.LookPath("gdbus")
	var apps []string
	for _, u := range desktopUsers() {
		if gdbus != "" && awayFromDesktop(u, gdbus, idleLimit) {
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
			apps = append(apps, name)
		}
	}
	return apps
}

// awayFromDesktop reports whether u's Cinnamon screen is locked or u has been
// idle for longer than idleLimit seconds. Without Cinnamon's screensaver it
// can't tell, and reports false.
func awayFromDesktop(u *user.User, gdbus string, idleLimit uint64) bool {
	call := func(method string) string {
		out, err := asUser(u, gdbus, "call", "--session", "--timeout", "1",
			"--dest", "org.cinnamon.ScreenSaver", "--object-path", "/org/cinnamon/ScreenSaver",
			"--method", "org.cinnamon.ScreenSaver."+method).Output()
		if err != nil {
			return ""
		}
		return string(out)
	}
	if strings.Contains(call("GetActive"), "true") {
		return true
	}
	idle, err := strconv.ParseUint(lastField(call("GetSessionIdleTime")), 10, 64)
	return err == nil && idle > idleLimit
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
