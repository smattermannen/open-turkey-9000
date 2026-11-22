package blocker

// remotecontrol_shared.go — what opens a browser to remote control, on
// both systems (remotecontrol.go reads /proc; extguard_windows.go the
// process's command line and environment block).

import "strings"

// remoteControlFlags are command-line switches that open a browser to remote
// control, without their leading dashes (Firefox takes one or two).
var remoteControlFlags = map[string]bool{
	"marionette":                 true,
	"remote-allow-system-access": true,
	"remote-debugging-port":      true,
	"start-debugger-server":      true,
	"jsdebugger":                 true,
	"wait-for-jsdebugger":        true,
	// Chrome: driven through a pipe instead of a port, and started with
	// extensions switched off (the KC9K extension included).
	"remote-debugging-pipe":     true,
	"disable-extensions":        true,
	"disable-extensions-except": true,
}

// remoteControlEnv are environment variables that do the same.
var remoteControlEnv = []string{"MOZ_MARIONETTE", "MOZ_REMOTE_ALLOW_SYSTEM_ACCESS"}

// remoteControlArgs reports whether a process's arguments or environment
// ("KEY=value" each) open it to remote control. Windows' switches may
// also start with "/" for Firefox; names are compared without case, as
// both browsers and Windows' environment do.
func remoteControlArgs(args, env []string) bool {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") && !(strings.HasPrefix(a, "/") && len(a) > 1 && !strings.ContainsAny(a[1:], `/\`)) {
			continue
		}
		a = strings.ToLower(strings.TrimLeft(a, "-/"))
		if i := strings.IndexByte(a, '='); i >= 0 {
			a = a[:i]
		}
		if remoteControlFlags[a] {
			return true
		}
	}
	for _, kv := range env {
		key, value, _ := strings.Cut(kv, "=")
		for _, name := range remoteControlEnv {
			if strings.EqualFold(key, name) && value != "" {
				return true
			}
		}
	}
	return false
}
