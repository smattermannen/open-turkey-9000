// linuxonly.go — what in a block names a Linux-only thing (§4.10 Project,
// "Linux-only"; W56). Shared: the Windows build keeps such an entry as it
// is and leaves it unused (never turned into a Windows name), and a project
// import on Windows lists it as "not used on Windows".
package blocker

import (
	"strings"
)

// linuxPrograms are Linux desktop and system programs with no Windows
// program of the same name. A name in the list, or one starting with a
// prefix in linuxProgramPrefixes, is Linux-only.
var linuxPrograms = map[string]bool{
	"gnome-terminal-server": true, "gnome-terminal": true, "gnome-shell": true,
	"gnome-control-center": true, "gnome-software": true, "nautilus": true,
	"konsole": true, "dolphin": true, "systemsettings": true, "kgx": true,
	"xterm": true, "tilix": true, "xdg-open": true, "notify-send": true,
	"gsettings": true, "dconf": true, "systemctl": true, "loginctl": true,
	"journalctl": true, "pkexec": true, "pam-auth-update": true,
	"librewolf-bin": true, "plasmashell": true, "kwin_x11": true,
	"kwin_wayland": true, "xfwm4": true, "apt": true, "apt-get": true,
	"dpkg": true, "flatpak": true, "snap": true,
}

var linuxProgramPrefixes = []string{"gnome-", "xfce4-", "systemd", "pam_"}

// programExtensions end a script or program entry; a dotted name ending in
// one is a file name, not a Linux app ID.
var programExtensions = map[string]bool{
	"exe": true, "bat": true, "cmd": true, "com": true, "ps1": true, "py": true,
	"pyw": true, "js": true, "jar": true, "vbs": true, "ahk": true, "sh": true,
	"lnk": true, "msi": true, "appimage": true,
}

// LinuxPath reports whether s is a Linux path: it starts with "/" (not
// "//", a Windows network path) or "~/".
func LinuxPath(s string) bool {
	return (strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//")) || strings.HasPrefix(s, "~/")
}

// linuxAppID reports whether s is a Linux desktop app ID, Flatpak ID or
// X11 window class in reverse-DNS form (org.gnome.Nautilus): three or more
// dotted parts, each a letter then letters, digits, '-' or '_', the last
// not a program extension.
func linuxAppID(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) < 3 || programExtensions[strings.ToLower(parts[len(parts)-1])] {
		return false
	}
	for _, p := range parts {
		if p == "" || !(p[0] >= 'a' && p[0] <= 'z' || p[0] >= 'A' && p[0] <= 'Z') {
			return false
		}
		for _, r := range p {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
				return false
			}
		}
	}
	return true
}

// linuxProgram reports whether name is a Linux-only program name.
func linuxProgram(name string) bool {
	name = strings.ToLower(name)
	if linuxPrograms[name] {
		return true
	}
	for _, p := range linuxProgramPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// LinuxOnlyApp says why an app entry is Linux-only ("" when it isn't): a
// Linux path, a Linux app ID or X11 window class, or a Linux program.
func LinuxOnlyApp(entry string) string {
	switch e := strings.TrimSpace(entry); {
	case LinuxPath(e):
		return "a Linux path"
	case linuxAppID(e):
		return "a Linux app ID or X11 window class"
	case linuxProgram(e):
		return "a Linux program"
	}
	return ""
}

// LinuxOnlyCommand says why a custom action's "run" command is Linux-only
// ("" when it isn't): its program is a Linux path or a Linux program
// (systemd, PAM and desktop tools among them).
func LinuxOnlyCommand(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return ""
	}
	program := strings.Trim(fields[0], `"'`)
	if program == "sudo" && len(fields) > 1 { // sudo systemctl …
		program = strings.Trim(fields[1], `"'`)
	}
	switch {
	case LinuxPath(program):
		return "a Linux path"
	case linuxProgram(program):
		return "a Linux program"
	}
	return ""
}
