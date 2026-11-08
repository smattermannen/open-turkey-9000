//go:build devmaster

package blocker

import (
	"os"
	"syscall"
)

// RemoteControlMarker lets the browser tests drive LibreWolf through
// WebDriver in testing builds (TAGS=devmaster). Only root can create it, and
// a normal build ignores it.
const RemoteControlMarker = "/var/lib/kc9k/allow-remote-control"

// remoteControlAllowed reports whether browsers started for remote control
// may run during website blocks: only in testing builds, while the marker
// exists as a root-owned regular file.
func remoteControlAllowed() bool {
	info, err := os.Lstat(RemoteControlMarker)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Uid == 0
}
