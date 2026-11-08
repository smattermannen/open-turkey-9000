// remotecontrol.go — browsers started for remote control.
//
// A Firefox-family browser started with --marionette and
// --remote-allow-system-access can be driven by a small script with full
// browser privileges: enough to switch the KC9K extension's blocking off
// while it keeps checking in. The same goes for the other remote-debugging
// switches. They need no root, only a command line, so the extension guard
// closes any browser started that way. (ci/browser_test.py drives LibreWolf
// exactly like this; testing builds let it, see remoteControlAllowed.)
package blocker

import (
	"bytes"
	"os"
	"strconv"
	"strings"
)

// remoteControlFlags are command-line switches that open a browser to remote
// control, without their leading dashes (Firefox takes one or two).
var remoteControlFlags = map[string]bool{
	"marionette":                 true,
	"remote-allow-system-access": true,
	"remote-debugging-port":      true,
	"start-debugger-server":      true,
	"jsdebugger":                 true,
	"wait-for-jsdebugger":        true,
}

// remoteControlEnv are environment variables that do the same.
var remoteControlEnv = []string{"MOZ_MARIONETTE", "MOZ_REMOTE_ALLOW_SYSTEM_ACCESS"}

// parseRemoteControl reports whether a process's command line or environment
// (both NUL-separated, as in /proc/<pid>/cmdline and environ) opens it to
// remote control.
func parseRemoteControl(cmdline, environ []byte) bool {
	for _, arg := range bytes.Split(cmdline, []byte{0}) {
		a := string(arg)
		if !strings.HasPrefix(a, "-") {
			continue
		}
		a = strings.ToLower(strings.TrimLeft(a, "-"))
		if i := strings.IndexByte(a, '='); i >= 0 {
			a = a[:i]
		}
		if remoteControlFlags[a] {
			return true
		}
	}
	for _, kv := range bytes.Split(environ, []byte{0}) {
		key, value, _ := strings.Cut(string(kv), "=")
		for _, name := range remoteControlEnv {
			if key == name && value != "" {
				return true
			}
		}
	}
	return false
}

// remoteControlled reports whether the browser process pid was started for
// remote control.
func remoteControlled(pid int) bool {
	dir := "/proc/" + strconv.Itoa(pid)
	cmdline, err := os.ReadFile(dir + "/cmdline")
	if err != nil {
		return false
	}
	environ, _ := os.ReadFile(dir + "/environ")
	return parseRemoteControl(cmdline, environ)
}
