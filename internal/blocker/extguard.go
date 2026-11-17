package blocker

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// DefaultExtensionGrace is how long a browser may run without the extension
// checking in before ExtensionGuard closes it, unless the user chose
// otherwise (§4.10 tamper protection). It gives a fresh browser time to
// start and load its extensions.
const DefaultExtensionGrace = 60 * time.Second

// extensionGrace is the grace in force (nanoseconds); SetExtensionGrace.
var extensionGrace atomic.Int64

func init() { extensionGrace.Store(int64(DefaultExtensionGrace)) }

// SetExtensionGrace sets the grace (10 s to 10 min).
func SetExtensionGrace(d time.Duration) {
	extensionGrace.Store(int64(min(max(d, 10*time.Second), 10*time.Minute)))
}

// ExtensionGrace returns the grace in force.
func ExtensionGrace() time.Duration { return time.Duration(extensionGrace.Load()) }

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
// ExtensionGrace().
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
// a working extension, and reports whether it found that browser and which
// supported browser it is ("" for another).
func (g *ExtensionGuard) CheckIn(remoteAddr string) (bool, string) {
	_, portText, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false, ""
	}
	clientPort, err := strconv.Atoi(portText)
	if err != nil {
		return false, ""
	}
	inode := socketInode(clientPort, g.port)
	if inode == "" {
		return false, ""
	}
	pid := socketOwner(inode, g.browsers.guarded)
	if pid == 0 {
		return false, ""
	}
	main := browserMain(pid)
	g.mu.Lock()
	g.seen[main] = time.Now()
	g.mu.Unlock()
	return true, supportedExe(main)
}

// Enforce closes, with all their child processes, the browsers that have run
// for longer than ExtensionGrace() without a check-in in the last
// ExtensionGrace(). It returns how many browsers it closed.
//
// Browser settings only take effect when a browser starts, so browsers that
// started before settingsChanged (KC9K changed something that protects the
// extension: its version, blocked developer tools, locked settings) are
// closed too, ExtensionGrace() after the change. Zero means no such change.
//
// With unsupported set (block unsupported browsers, §4.10), every browser
// that isn't an installed supported browser (see SupportedBrowsers) is
// closed at once: it can never run the extension.
func (g *ExtensionGuard) Enforce(now, settingsChanged time.Time, unsupported bool) int {
	if now.Sub(g.started) < ExtensionGrace() {
		return 0 // the daemon just started: give running browsers time to check in
	}
	uptime := systemUptime()
	parent, children := processTree()

	g.mu.Lock()
	defer g.mu.Unlock()
	var targets []int
	for pid := range parent {
		name := exeName(pid)
		if exeName(parent[pid]) == name || !ownedByUser(pid) || !g.browsers.guarded(pid) {
			continue // a child process of a browser, a system program, or not a browser
		}
		p := guardedProcess{remote: remoteControlled(pid) && !remoteControlAllowed(), supported: supportedExe(pid) != ""}
		p.age, p.ageKnown = processAge(pid, uptime)
		p.lastSeen, p.seen = g.seen[pid]
		if reason := closeReason(p, now, settingsChanged, unsupported); reason != "" {
			log.Printf("browser: closing %s (pid %d): %s", name, pid, reason)
			targets = append(targets, pid)
		}
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

// RunningSupportedBrowsers returns the supported browsers running now in
// regular users' sessions (SupportedBrowsers IDs), each with how long its
// longest-running main process has run.
func RunningSupportedBrowsers() map[string]time.Duration {
	uptime := systemUptime()
	parent, _ := processTree()
	out := map[string]time.Duration{}
	for pid := range parent {
		id := supportedExe(pid)
		if id == "" || exeName(parent[pid]) == exeName(pid) || !ownedByUser(pid) {
			continue // not a supported browser, or one of its child processes
		}
		if age, ok := processAge(pid, uptime); ok && age >= out[id] {
			out[id] = age
		}
	}
	return out
}

// guardedProcess is what the guard knows of a running browser.
type guardedProcess struct {
	remote    bool // started for remote control, which isn't allowed
	supported bool // an installed supported browser (SupportedBrowsers)
	age       time.Duration
	ageKnown  bool
	lastSeen  time.Time // its extension's last check-in
	seen      bool
}

// closeReason says why the guard closes p, or "" to leave it open.
func closeReason(p guardedProcess, now, settingsChanged time.Time, unsupported bool) string {
	if p.remote {
		return "started for remote control"
	}
	if unsupported && !p.supported {
		return "not a supported browser (block unsupported browsers is on)"
	}
	if !settingsChanged.IsZero() && now.Sub(settingsChanged) >= ExtensionGrace() &&
		p.ageKnown && now.Add(-p.age).Before(settingsChanged) {
		return "started before KC9K changed its browser settings"
	}
	if !p.ageKnown || p.age < ExtensionGrace() {
		return ""
	}
	if p.seen && now.Sub(p.lastSeen) < ExtensionGrace() {
		return ""
	}
	return "no Killer Chicken extension"
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

// ClientUser names the local user owning the TCP connection from
// remoteAddr ("127.0.0.1:port") to the daemon's port, by the socket's uid
// in /proc/net/tcp; "" when unknown or not a regular user.
func ClientUser(remoteAddr string, daemonPort int) string {
	_, portText, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return ""
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return ""
	}
	local := fmt.Sprintf(":%04X", port)
	remote := fmt.Sprintf(":%04X", daemonPort)
	for _, table := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := os.ReadFile(table)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) > 7 && strings.HasSuffix(fields[1], local) && strings.HasSuffix(fields[2], remote) {
				uid, err := strconv.ParseUint(fields[7], 10, 32)
				if err != nil || !regularUser(uid) {
					return ""
				}
				if u, err := user.LookupId(fields[7]); err == nil {
					return u.Username
				}
				return ""
			}
		}
	}
	return ""
}
