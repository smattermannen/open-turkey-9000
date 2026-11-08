// browserdetect.go — recognising browsers by what they are, not their name.
//
// The extension guard used to know browsers only by executable name, so a
// renamed or portable copy (a Firefox tarball, Tor Browser, a browser copied
// to a new name) ran without the extension. A browser can't hide its engine:
// a Gecko browser maps libxul.so, and a Chromium-family browser maps its ICU
// data and V8 snapshot. Those markers are what this file looks for.
//
// Installed applications built on the same engines (Thunderbird, Zotero,
// Spotify and Steam through CEF, Electron apps such as Discord or VS Code)
// must not be closed. They are installed by root, while a copy that dodges the
// name list has to be put somewhere by the user, so only executables the user
// owns are judged by their engine; root-owned ones are judged by name, as
// before. Electron apps keep their app.asar open or mapped, which no browser
// does, so a user-installed Electron app isn't mistaken for a browser either.
package blocker

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// geckoMarkers and chromiumMarkers are files only a browser engine maps.
var (
	geckoMarkers    = []string{"libxul.so"}
	chromiumMarkers = []string{"icudtl.dat", "v8_context_snapshot.bin", "snapshot_blob.bin"}
)

// mapsBrowserEngine reports whether /proc/<pid>/maps content shows a Gecko or
// Chromium engine.
func mapsBrowserEngine(maps string) bool {
	for _, line := range strings.Split(maps, "\n") {
		i := strings.LastIndexByte(line, '/')
		if i < 0 {
			continue
		}
		file := strings.TrimSuffix(line[i+1:], " (deleted)")
		for _, marker := range geckoMarkers {
			if file == marker {
				return true
			}
		}
		for _, marker := range chromiumMarkers {
			if file == marker {
				return true
			}
		}
	}
	return false
}

// holdsAsar reports whether the process has an Electron app archive open.
func holdsAsar(pid int) bool {
	dir := "/proc/" + strconv.Itoa(pid) + "/fd"
	fds, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, fd := range fds {
		if link, err := os.Readlink(dir + "/" + fd.Name()); err == nil && strings.HasSuffix(link, ".asar") {
			return true
		}
	}
	return false
}

// exeOwnedByUser reports whether the process runs an executable that belongs
// to an ordinary user rather than root. Stat through /proc/<pid>/exe reaches
// the real file even when it sits in another mount namespace.
func exeOwnedByUser(pid int) bool {
	info, err := os.Stat("/proc/" + strconv.Itoa(pid) + "/exe")
	if err != nil {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Uid != 0
}

// engineBrowser reports whether pid runs a browser engine from an executable
// the user put in place: a portable, renamed or copied browser.
func engineBrowser(pid int) bool {
	if !exeOwnedByUser(pid) {
		return false
	}
	maps, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/maps")
	if err != nil || !mapsBrowserEngine(string(maps)) {
		return false
	}
	// Electron reads its app.asar through an open file or a memory map.
	return !strings.Contains(string(maps), ".asar") && !holdsAsar(pid)
}

// browserVerdict caches whether a process is a guarded browser, keyed by its
// start time so a reused PID is judged again.
type browserVerdict struct {
	start   string
	guarded bool
}

// browserCache remembers verdicts, so a process's memory map is read once in
// its life rather than on every check.
type browserCache struct {
	mu       sync.Mutex
	verdicts map[int]browserVerdict
}

func newBrowserCache() *browserCache {
	return &browserCache{verdicts: map[int]browserVerdict{}}
}

// guarded reports whether pid is a browser that must run the extension: a
// known browser name, or a browser engine in a user-owned executable.
func (c *browserCache) guarded(pid int) bool {
	if guardedBrowsers[exeName(pid)] {
		return true
	}
	start := startTicks(pid)
	if start == "" {
		return false
	}
	c.mu.Lock()
	v, ok := c.verdicts[pid]
	c.mu.Unlock()
	if ok && v.start == start {
		return v.guarded
	}
	verdict := engineBrowser(pid)
	c.mu.Lock()
	c.verdicts[pid] = browserVerdict{start: start, guarded: verdict}
	c.mu.Unlock()
	return verdict
}

// forget drops verdicts for processes that are gone.
func (c *browserCache) forget(alive map[int]int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for pid := range c.verdicts {
		if _, ok := alive[pid]; !ok {
			delete(c.verdicts, pid)
		}
	}
}

// startTicks returns a process's start time (field 22 of /proc/<pid>/stat) as
// text, or "" if the process is gone.
func startTicks(pid int) string {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return ""
	}
	return parseStartField(string(data))
}

func parseStartField(stat string) string {
	i := strings.LastIndexByte(stat, ')')
	if i < 0 {
		return ""
	}
	fields := strings.Fields(stat[i+1:])
	if len(fields) < 20 {
		return ""
	}
	return fields[19]
}
