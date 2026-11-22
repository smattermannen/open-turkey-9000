//go:build linux

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
)

// parseRemoteControl reports whether a process's command line or environment
// (both NUL-separated, as in /proc/<pid>/cmdline and environ) opens it to
// remote control.
func parseRemoteControl(cmdline, environ []byte) bool {
	var args, env []string
	for _, arg := range bytes.Split(cmdline, []byte{0}) {
		args = append(args, string(arg))
	}
	for _, kv := range bytes.Split(environ, []byte{0}) {
		env = append(env, string(kv))
	}
	return remoteControlArgs(args, env)
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

// RemoteControlAllowed is remoteControlAllowed for other packages. While it
// holds, KC9K also leaves LibreWolf's developer tools policy out: that
// policy turns off Marionette, which WebDriver needs (testing builds only).
func RemoteControlAllowed() bool { return remoteControlAllowed() }
