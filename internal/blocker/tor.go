// tor.go — tamper protection "close private windows and Tor" (§4.10):
// Tor Browser and a user's own tor are closed.
package blocker

import (
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// torExe reports whether an executable path is Tor Browser's or tor.
func torExe(path string) bool {
	lower := strings.ToLower(path)
	for _, part := range []string{"/tor-browser", "/torbrowser", "/tor browser"} {
		if strings.Contains(lower, part) {
			return true
		}
	}
	name := baseName(path)
	return name == "tor" || name == "torbrowser-launcher"
}

// CloseTor kills every regular user's Tor Browser and tor process and
// returns how many.
func CloseTor() int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	killed := 0
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || !ownedByUser(pid) {
			continue
		}
		exe, err := os.Readlink("/proc/" + e.Name() + "/exe")
		if err != nil || !torExe(strings.TrimSuffix(exe, " (deleted)")) {
			continue
		}
		if syscall.Kill(pid, syscall.SIGKILL) == nil {
			log.Printf("tamper: closed %s (pid %d): Tor", baseName(exe), pid)
			killed++
		}
	}
	return killed
}
