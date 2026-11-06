package blocker

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ExtensionGrace is how long a browser may run without the extension checking
// in before ExtensionGuard closes it. It gives a fresh browser time to start
// and load its extensions.
const ExtensionGrace = 60 * time.Second

// guardedBrowsers are the executables treated as browsers that must run the
// extension. "tor" is left out: it is also the name of the Tor daemon. Tor
// Browser and other portable or renamed browsers are recognised by their
// engine instead (see browserdetect.go).
var guardedBrowsers = map[string]bool{
	"firefox": true, "firefox-bin": true, "firefox-esr": true, "librewolf": true,
	"chrome": true, "google-chrome": true, "chromium": true, "chromium-browser": true,
	"brave": true, "brave-browser": true, "opera": true, "vivaldi-bin": true,
	"msedge": true, "microsoft-edge": true, "epiphany": true, "falkon": true,
	"konqueror": true, "midori": true, "qutebrowser": true, "palemoon": true,
	"waterfox": true, "floorp": true, "zen": true,
}

// ExtensionGuard closes browsers that don't run the KC9K extension. The
// extension fetches its rules from the daemon every few seconds; the daemon
// passes each request to CheckIn, which finds the browser behind the
// connection. Enforce closes every browser that hasn't checked in within
// ExtensionGrace.
type ExtensionGuard struct {
	mu       sync.Mutex
	seen     map[int]time.Time // main browser PID -> last check-in
	started  time.Time
	port     int // the daemon's port, the remote end of the extension's socket
	browsers *browserCache
}

// NewExtensionGuard returns a guard for extensions connecting to port.
func NewExtensionGuard(port int) *ExtensionGuard {
	return &ExtensionGuard{seen: map[int]time.Time{}, started: time.Now(), port: port, browsers: newBrowserCache()}
}

// CheckIn records that the browser owning the connection from remoteAddr has
// a working extension.
func (g *ExtensionGuard) CheckIn(remoteAddr string) {
	_, portText, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return
	}
	clientPort, err := strconv.Atoi(portText)
	if err != nil {
		return
	}
	inode := socketInode(clientPort, g.port)
	if inode == "" {
		return
	}
	pid := socketOwner(inode, g.browsers.guarded)
	if pid == 0 {
		return
	}
	main := browserMain(pid)
	g.mu.Lock()
	g.seen[main] = time.Now()
	g.mu.Unlock()
}

// Enforce closes, with all their child processes, the browsers that have run
// for longer than ExtensionGrace without a check-in in the last
// ExtensionGrace. It returns how many browsers it closed.
func (g *ExtensionGuard) Enforce(now time.Time) int {
	if now.Sub(g.started) < ExtensionGrace {
		return 0 // the daemon just started: give running browsers time to check in
	}
	uptime := systemUptime()
	parent, children := processTree()

	g.mu.Lock()
	defer g.mu.Unlock()
	var targets []int
	for pid := range parent {
		name := exeName(pid)
		if exeName(parent[pid]) == name || !g.browsers.guarded(pid) {
			continue // a child process of a browser, or not a browser
		}
		if age, ok := processAge(pid, uptime); !ok || age < ExtensionGrace {
			continue
		}
		if last, ok := g.seen[pid]; ok && now.Sub(last) < ExtensionGrace {
			continue
		}
		log.Printf("browser: closing %s (pid %d): no Killer Chicken extension", name, pid)
		targets = append(targets, pid)
	}
	for pid := range g.seen {
		if _, alive := parent[pid]; !alive {
			delete(g.seen, pid)
		}
	}
	g.browsers.forget(parent)
	for _, pid := range append(targets, descendants(targets, children)...) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	return len(targets)
}

// socketInode finds the inode of the local TCP socket from clientPort to the
// daemon's port, in /proc/net/tcp or tcp6.
func socketInode(clientPort, daemonPort int) string {
	local := fmt.Sprintf(":%04X", clientPort)
	remote := fmt.Sprintf(":%04X", daemonPort)
	for _, table := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		file, err := os.Open(table)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) > 9 && strings.HasSuffix(fields[1], local) && strings.HasSuffix(fields[2], remote) {
				file.Close()
				return fields[9]
			}
		}
		file.Close()
	}
	return ""
}

// socketOwner returns the PID of a browser process holding the socket inode,
// or 0. Only processes isBrowser accepts are searched.
func socketOwner(inode string, isBrowser func(int) bool) int {
	target := "socket:[" + inode + "]"
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || !isBrowser(pid) {
			continue
		}
		fds, err := os.ReadDir("/proc/" + entry.Name() + "/fd")
		if err != nil {
			continue
		}
		for _, fd := range fds {
			if link, err := os.Readlink("/proc/" + entry.Name() + "/fd/" + fd.Name()); err == nil && link == target {
				return pid
			}
		}
	}
	return 0
}

// browserMain walks up from pid to the browser's main process: the topmost
// ancestor running the same executable.
func browserMain(pid int) int {
	name := exeName(pid)
	for {
		ppid, ok := readPPID(pid)
		if !ok || ppid <= 1 || exeName(ppid) != name {
			return pid
		}
		pid = ppid
	}
}

// exeName is the base name of a process's executable, or "" if unknown.
func exeName(pid int) string {
	if pid <= 0 {
		return ""
	}
	exe, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe")
	if err != nil {
		return ""
	}
	return baseName(strings.TrimSuffix(exe, " (deleted)"))
}

// processTree snapshots every process's parent and children.
func processTree() (map[int]int, map[int][]int) {
	parent := map[int]int{}
	children := map[int][]int{}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return parent, children
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		if ppid, ok := readPPID(pid); ok {
			parent[pid] = ppid
			children[ppid] = append(children[ppid], pid)
		}
	}
	return parent, children
}

// systemUptime returns the time since boot, from /proc/uptime.
func systemUptime() time.Duration {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

// clockTicks is the kernel's USER_HZ, the unit of a process start time; it is
// 100 on every Linux architecture KC9K runs on.
const clockTicks = 100

// processAge returns how long pid has been running.
func processAge(pid int, uptime time.Duration) (time.Duration, bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, false
	}
	return parseAge(string(data), uptime)
}

// parseAge reads the start time (field 22) from /proc/<pid>/stat content.
func parseAge(stat string, uptime time.Duration) (time.Duration, bool) {
	i := strings.LastIndexByte(stat, ')')
	if i < 0 {
		return 0, false
	}
	fields := strings.Fields(stat[i+1:])
	// fields[0] is field 3 (state), so field 22 is fields[19].
	if len(fields) < 20 {
		return 0, false
	}
	ticks, err := strconv.ParseInt(fields[19], 10, 64)
	if err != nil {
		return 0, false
	}
	return uptime - time.Duration(ticks)*time.Second/clockTicks, true
}
