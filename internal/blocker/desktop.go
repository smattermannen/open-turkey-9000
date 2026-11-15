// desktop.go — which desktop the user is on (R-DESKTOP-1): window titles
// and app time need X11, and are built for Cinnamon.
package blocker

import (
	"os/exec"
	"strings"
)

// sessionInfo returns the active graphical session's type (x11, wayland)
// and desktop (cinnamon, gnome…), lower case, from loginctl; empty when
// there is none. A variable for tests.
var sessionInfo = func() (kind, desktop string) {
	out, err := exec.Command("loginctl", "list-sessions", "--no-legend").Output()
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		props, err := exec.Command("loginctl", "show-session", fields[0], "-p", "Type", "-p", "Desktop", "-p", "Active", "-p", "Class").Output()
		if err != nil {
			continue
		}
		p := map[string]string{}
		for _, kv := range strings.Split(string(props), "\n") {
			if k, v, ok := strings.Cut(kv, "="); ok {
				p[k] = strings.ToLower(strings.TrimSpace(v))
			}
		}
		if p["Active"] == "yes" && p["Class"] == "user" && (p["Type"] == "x11" || p["Type"] == "wayland") {
			return p["Type"], p["Desktop"]
		}
	}
	return "", ""
}

// DesktopWarning says why window titles and app time may not work on the
// desktop in use (R-DESKTOP-1), or "" on Cinnamon on X11 or with no
// graphical session.
func DesktopWarning() string {
	kind, desktop := sessionInfo()
	switch {
	case kind == "":
		return ""
	case kind == "wayland":
		return "This desktop runs on Wayland: window titles and app time only work for apps that use XWayland."
	case !strings.Contains(desktop, "cinnamon"):
		name := desktop
		if name == "" {
			name = "unknown"
		}
		return "This desktop (" + name + ") isn't Cinnamon: window titles and app time may not work."
	}
	return ""
}

// GraphicalSessions returns the IDs of the user desktop sessions now (for
// autostart at sign-in). A variable for tests.
var GraphicalSessions = func() map[string]bool {
	out := map[string]bool{}
	list, err := exec.Command("loginctl", "list-sessions", "--no-legend").Output()
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(list), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		props, err := exec.Command("loginctl", "show-session", fields[0], "-p", "Type", "-p", "Class").Output()
		if err != nil {
			continue
		}
		text := string(props)
		if strings.Contains(text, "Class=user") && (strings.Contains(text, "Type=x11") || strings.Contains(text, "Type=wayland")) {
			out[fields[0]] = true
		}
	}
	return out
}
