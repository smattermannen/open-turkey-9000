//go:build linux

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
// An Electron app shipped unpacked (resources/app, no archive) holds no
// asar; its renderers carry Electron's --app-path switch, which Chromium's
// never do (B3). A process is judged once it has had electronSettle to start
// them.
package blocker

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
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

// fuseMagic is the statfs type of a FUSE file system, such as the one an
// AppImage mounts its files on.
const fuseMagic = 0x65735546

// exeOwnedByUser reports whether the process runs an executable the user put
// in place rather than one root installed. Stat through /proc/<pid>/exe
// reaches the real file even when it sits in another mount namespace.
func exeOwnedByUser(pid int) bool {
	exe := "/proc/" + strconv.Itoa(pid) + "/exe"
	var uid uint32
	info, err := os.Stat(exe)
	if err == nil {
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			uid = st.Uid
		}
	}
	var fsInfo syscall.Statfs_t
	if err == nil {
		err = syscall.Statfs(exe, &fsInfo)
	}
	return userPlacedExe(uid, int64(fsInfo.Type), err)
}

// userPlacedExe judges an executable by its owner and file system type, or
// by the error reading them. A file on a FUSE file system counts as the
// user's whatever owner it shows: an AppImage (a portable LibreWolf, or a
// browser repacked under another name) mounts its files there, shown as
// root's, and root itself is refused access to them (permission denied).
// Any other error means the process is gone.
func userPlacedExe(uid uint32, fsType int64, err error) bool {
	if err != nil {
		return errors.Is(err, fs.ErrPermission)
	}
	return uid != 0 || fsType == fuseMagic
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
	return !strings.Contains(string(maps), ".asar") && !holdsAsar(pid) && !electronRenderer(pid, 3)
}

// electronSettle is how long a new process with a browser engine is given to
// start its renderers before it is judged.
const electronSettle = 15 * time.Second

// electronRenderer reports whether a renderer with Electron's --app-path
// switch runs among pid's descendants, depth levels down (Electron's
// renderers are children of its zygote, a child of the main process).
func electronRenderer(pid, depth int) bool {
	if depth == 0 {
		return false
	}
	for _, child := range childPIDs(pid) {
		if cmd, err := os.ReadFile("/proc/" + strconv.Itoa(child) + "/cmdline"); err == nil && electronRendererCmdline(string(cmd)) {
			return true
		}
		if electronRenderer(child, depth-1) {
			return true
		}
	}
	return false
}

// electronRendererCmdline reports whether /proc/<pid>/cmdline content is an
// Electron renderer's: --type=renderer with --app-path=. Chromium rewrites
// its children's process titles, joining the arguments with spaces.
func electronRendererCmdline(cmdline string) bool {
	renderer, appPath := false, false
	for _, arg := range strings.FieldsFunc(cmdline, func(r rune) bool { return r == 0 || r == ' ' }) {
		renderer = renderer || arg == "--type=renderer"
		appPath = appPath || strings.HasPrefix(arg, "--app-path=")
	}
	return renderer && appPath
}

// childPIDs lists a process's children, from each of its threads.
func childPIDs(pid int) []int {
	dir := "/proc/" + strconv.Itoa(pid) + "/task"
	tasks, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []int
	for _, t := range tasks {
		data, err := os.ReadFile(dir + "/" + t.Name() + "/children")
		if err != nil {
			continue
		}
		for _, f := range strings.Fields(string(data)) {
			if child, err := strconv.Atoi(f); err == nil {
				out = append(out, child)
			}
		}
	}
	return out
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
	if verdict {
		if age, ok := processAge(pid, systemUptime()); !ok || age < electronSettle {
			return false // judged again once it has had time to start its renderers
		}
	}
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
