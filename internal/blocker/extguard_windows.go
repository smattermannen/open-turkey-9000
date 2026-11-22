package blocker

// extguard_windows.go — supported browsers, recognising browsers, and the
// extension guard on Windows (ROADMAP W2). A browser is recognised by
// what it is, not its name: its program's name, the name its maker gave
// it (the version resource's OriginalFilename survives a rename), or the
// engine next to it (xul.dll: Firefox family; chrome.dll or msedge.dll:
// Chromium family), so a renamed or portable browser is still one.

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/smattermannen/kc9k/internal/platform"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// guardedBrowsers are browsers' program names (lower case, no .exe).
var guardedBrowsers = map[string]bool{
	"firefox": true, "librewolf": true, "chrome": true, "chromium": true, "msedge": true,
	"brave": true, "opera": true, "opera_gx": true, "vivaldi": true, "waterfox": true,
	"floorp": true, "zen": true, "palemoon": true, "basilisk": true, "iexplore": true,
	"yandex": true, "thorium": true, "arc": true, "whale": true, "maxthon": true,
	"seamonkey": true, "slimjet": true, "cent": true, "duckduckgo": true, "mullvadbrowser": true,
}

// notBrowsers use a browser engine but aren't browsers.
var notBrowsers = map[string]bool{"thunderbird": true, "betterbird": true, "msedgewebview2": true}

// SupportedBrowsers on Windows: LibreWolf with the bundled extension;
// Google Chrome and Microsoft Edge with the extension from the Chrome Web
// Store (supported once ChromeStoreExtensionID is set).
var SupportedBrowsers = []SupportedBrowser{
	{ID: "librewolf", Name: "LibreWolf", Exes: []string{
		filepath.Join(programFilesDir(), "LibreWolf", "librewolf.exe")}},
	{ID: "chrome", Name: "Google Chrome", Exes: []string{
		filepath.Join(programFilesDir(), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(programFilesX86Dir(), "Google", "Chrome", "Application", "chrome.exe")}},
	{ID: "edge", Name: "Microsoft Edge", Exes: []string{
		filepath.Join(programFilesX86Dir(), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(programFilesDir(), "Microsoft", "Edge", "Application", "msedge.exe")}},
}

// Installed reports whether the browser is installed (for Chrome and
// Edge: and the store extension exists to give them).
func (b SupportedBrowser) Installed() bool {
	if b.ID != "librewolf" && ChromeStoreExtensionID == "" {
		return false
	}
	for _, exe := range b.Exes {
		if _, err := os.Stat(exe); err == nil {
			return true
		}
	}
	return false
}

// SupportedBrowserByID finds a supported browser.
func SupportedBrowserByID(id string) (SupportedBrowser, bool) {
	for _, b := range SupportedBrowsers {
		if b.ID == id {
			return b, true
		}
	}
	return SupportedBrowser{}, false
}

// supportedExeOf is the ID of the supported browser whose program path is
// exe, or "".
func supportedExeOf(exe string) string {
	for _, b := range SupportedBrowsers {
		if !b.Installed() {
			continue
		}
		for _, e := range b.Exes {
			if strings.EqualFold(filepath.Clean(e), filepath.Clean(exe)) {
				return b.ID
			}
		}
	}
	return ""
}

// wingetIDs are the supported browsers' packages in winget's own source.
var wingetIDs = map[string]string{"librewolf": "LibreWolf.LibreWolf", "chrome": "Google.Chrome"}

// wingetPath finds winget (App Installer) for SYSTEM, which has it on no
// PATH: the newest App Installer package's winget.exe.
func wingetPath() string {
	matches, _ := filepath.Glob(filepath.Join(programFilesDir(), "WindowsApps", "Microsoft.DesktopAppInstaller_*_x64__8wekyb3d8bbwe", "winget.exe"))
	if len(matches) == 0 {
		if p, err := exec.LookPath("winget.exe"); err == nil {
			return p
		}
		return ""
	}
	sort.Strings(matches) // versions sort as text within one major version
	return matches[len(matches)-1]
}

// InstallBrowser installs a supported browser for the whole computer with
// winget (W18; SPEC §6 2026-11-22 By Opus): the download comes from the
// browser's own publisher through winget's source.
func InstallBrowser(id string) error {
	pkg := wingetIDs[id]
	if pkg == "" {
		return errorf("unknown browser %q (use librewolf or chrome)", id)
	}
	b, _ := SupportedBrowserByID(id)
	for _, exe := range b.Exes {
		if fileExists(exe) {
			if id == "chrome" && ChromeStoreExtensionID == "" {
				return errorf("Google Chrome is installed; it gets the extension once its Chrome Web Store listing exists")
			}
			return nil
		}
	}
	winget := wingetPath()
	if winget == "" {
		return errorf("winget (App Installer) isn't installed: install %s from its website, or App Installer from the Microsoft Store first", b.Name)
	}
	out, err := installRun(winget, "install", "--id", pkg, "--exact", "--scope", "machine", "--silent",
		"--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity")
	if err != nil {
		text := strings.TrimSpace(string(out))
		if len(text) > 600 {
			text = text[len(text)-600:]
		}
		return errorf("winget install %s: %v: %s", pkg, err, text)
	}
	for _, exe := range b.Exes {
		if fileExists(exe) {
			return nil
		}
	}
	return errorf("winget finished but %s is missing", b.Exes[0])
}

// installRun runs the installer; a variable for tests.
var installRun = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// ChromeInstalled reports whether Chrome or Edge can run the extension.
func ChromeInstalled() bool {
	for _, b := range SupportedBrowsers {
		if b.ID != "librewolf" && b.Installed() {
			return true
		}
	}
	return false
}

// ExtensionBrowserInstalled reports whether a browser that runs the
// extension is installed.
func ExtensionBrowserInstalled() bool { return LibreWolfVersion() != "" || ChromeInstalled() }

// LibreWolfVersion is the installed LibreWolf's version, or "".
func LibreWolfVersion() string {
	for _, root := range []string{`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`} {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, root, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		names, _ := k.ReadSubKeyNames(0)
		for _, n := range names {
			sub, err := registry.OpenKey(k, n, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			display, _, _ := sub.GetStringValue("DisplayName")
			version, _, _ := sub.GetStringValue("DisplayVersion")
			sub.Close()
			if strings.HasPrefix(display, "LibreWolf") && version != "" {
				k.Close()
				return version
			}
		}
		k.Close()
	}
	if _, err := os.Stat(SupportedBrowsers[0].Exes[0]); err == nil {
		return "installed"
	}
	return ""
}

// browserVerdicts caches whether a program is a browser, by path.
var browserVerdicts = struct {
	sync.Mutex
	m map[string]bool
}{m: map[string]bool{}}

// isBrowserProc reports whether p is a browser's process.
func isBrowserProc(p *winProc) bool {
	if notBrowsers[p.Name] {
		return false
	}
	if guardedBrowsers[p.Name] {
		return true
	}
	if p.Exe == "" {
		return false
	}
	key := strings.ToLower(p.Exe)
	browserVerdicts.Lock()
	v, ok := browserVerdicts.m[key]
	browserVerdicts.Unlock()
	if ok {
		return v
	}
	v = isBrowserProgram(p.Exe)
	browserVerdicts.Lock()
	browserVerdicts.m[key] = v
	browserVerdicts.Unlock()
	return v
}

// isBrowserProgram looks at the program file and its folder.
func isBrowserProgram(exe string) bool {
	if orig := exeBase(originalFilename(exe)); orig != "" {
		if notBrowsers[orig] {
			return false
		}
		if guardedBrowsers[orig] {
			return true
		}
	}
	dir := filepath.Dir(exe)
	if fileExists(filepath.Join(dir, "xul.dll")) {
		return true
	}
	for _, engine := range []string{"chrome.dll", "msedge.dll"} {
		if fileExists(filepath.Join(dir, engine)) {
			return true
		}
		if m, _ := filepath.Glob(filepath.Join(dir, "*", engine)); len(m) > 0 {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// originalFilename is the OriginalFilename of exe's version resource.
func originalFilename(exe string) string {
	size, err := windows.GetFileVersionInfoSize(exe, nil)
	if err != nil || size == 0 {
		return ""
	}
	data := make([]byte, size)
	if windows.GetFileVersionInfo(exe, 0, size, unsafe.Pointer(&data[0])) != nil {
		return ""
	}
	var trans *[2]uint16
	var n uint32
	if windows.VerQueryValue(unsafe.Pointer(&data[0]), `\VarFileInfo\Translation`, unsafe.Pointer(&trans), &n) != nil || n < 4 {
		return ""
	}
	var name *uint16
	path := `\StringFileInfo\` + hex4(trans[0]) + hex4(trans[1]) + `\OriginalFilename`
	if windows.VerQueryValue(unsafe.Pointer(&data[0]), path, unsafe.Pointer(&name), &n) != nil || n == 0 {
		return ""
	}
	return windows.UTF16PtrToString(name)
}

func hex4(v uint16) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[v>>12&0xf], digits[v>>8&0xf], digits[v>>4&0xf], digits[v&0xf]})
}

// ExtensionGuard closes browsers without a working extension (see the
// Linux one).
type ExtensionGuard struct {
	mu      sync.Mutex
	seen    map[uint32]time.Time // main browser PID -> last check-in
	started time.Time
	port    int
}

// NewExtensionGuard makes a guard for the rules server on port.
func NewExtensionGuard(port int) *ExtensionGuard {
	return &ExtensionGuard{seen: map[uint32]time.Time{}, started: time.Now(), port: port}
}

// CheckIn notes a check-in from the browser whose connection is
// remoteAddr; it reports whether a browser made it, and which supported
// browser.
func (g *ExtensionGuard) CheckIn(remoteAddr string) (bool, string) {
	port := localPort(remoteAddr)
	if port < 0 {
		return false, ""
	}
	pid := tcpOwner(port, g.port)
	if pid == 0 {
		return false, ""
	}
	procs := processSnapshot()
	p := procs[pid]
	if p == nil || !isBrowserProc(p) {
		return false, ""
	}
	main := mainOf(procs, p)
	g.mu.Lock()
	g.seen[main.PID] = time.Now()
	g.mu.Unlock()
	return true, supportedExeOf(main.Exe)
}

// ownedByAnyUID reports whether p belongs to one of uids (nil: any
// person).
func ownedByAnyUID(p *winProc, uids []uint64) bool {
	if !p.person() {
		return false
	}
	if uids == nil {
		return true
	}
	for _, uid := range uids {
		if p.UID == int64(uid) {
			return true
		}
	}
	return false
}

// EnforceFor closes the browsers of uids (nil: every person's) that have
// run longer than the grace without their extension checking in.
func (g *ExtensionGuard) EnforceFor(now, settingsChanged time.Time, unsupported bool, uids []uint64) int {
	if uids != nil && len(uids) == 0 {
		return 0
	}
	if now.Sub(g.started) < ExtensionGrace() {
		return 0
	}
	procs := processSnapshot()
	children := childrenOf(procs)
	g.mu.Lock()
	defer g.mu.Unlock()
	var targets []uint32
	for _, p := range procs {
		if !isBrowserProc(p) || !ownedByAnyUID(p, uids) || mainOf(procs, p) != p {
			continue
		}
		gp := guardedProcess{supported: supportedExeOf(p.Exe) != "", remote: remoteControlledWin(p.PID) && !RemoteControlAllowed()}
		if !p.Started.IsZero() {
			gp.age, gp.ageKnown = now.Sub(p.Started), true
		}
		gp.lastSeen, gp.seen = g.seen[p.PID]
		if reason := closeReason(gp, now, settingsChanged, unsupported); reason != "" {
			log.Printf("browser: closing %s (pid %d): %s", p.Name, p.PID, reason)
			targets = append(targets, p.PID)
		}
	}
	for pid := range g.seen {
		if procs[pid] == nil {
			delete(g.seen, pid)
		}
	}
	for _, pid := range targets {
		tree := treeOf(pid, children)
		for i := len(tree) - 1; i >= 0; i-- {
			_ = killProcess(tree[i])
		}
	}
	return len(targets)
}

// RunningSupportedBrowsers is RunningSupportedBrowsersFor every person.
func RunningSupportedBrowsers() map[string]time.Duration { return RunningSupportedBrowsersFor(nil) }

// RunningSupportedBrowsersFor returns, per supported browser running for
// uids, how long its oldest main process has run.
func RunningSupportedBrowsersFor(uids []uint64) map[string]time.Duration {
	procs := processSnapshot()
	out := map[string]time.Duration{}
	for _, p := range procs {
		id := supportedExeOf(p.Exe)
		if id == "" || mainOf(procs, p) != p || !ownedByAnyUID(p, uids) || p.Started.IsZero() {
			continue
		}
		if age := time.Since(p.Started); age >= out[id] {
			out[id] = age
		}
	}
	return out
}

// ClientUser is the account whose program made the connection from
// remoteAddr to the rules server (for statistics per user), or "".
func ClientUser(remoteAddr string, daemonPort int) string {
	port := localPort(remoteAddr)
	if port < 0 {
		return ""
	}
	pid := tcpOwner(port, daemonPort)
	if pid == 0 {
		return ""
	}
	p := processSnapshot()[pid]
	if p == nil || !p.person() {
		return ""
	}
	return accountName(p.PID)
}

// accountName is the name of pid's owner.
func accountName(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	var token windows.Token
	if windows.OpenProcessToken(h, windows.TOKEN_QUERY, &token) != nil {
		return ""
	}
	defer token.Close()
	u, err := token.GetTokenUser()
	if err != nil {
		return ""
	}
	return platform.AccountName(u.User.Sid)
}

func errorf(format string, args ...any) error { return fmt.Errorf(format, args...) }

// remoteControlledWin reports whether the browser process pid was started
// for remote control (W11): its command line (--remote-debugging-port,
// --marionette, --disable-extensions...) or environment (MOZ_MARIONETTE).
func remoteControlledWin(pid uint32) bool {
	return remoteControlArgs(commandLine(pid), processEnvironment(pid))
}

// RemoteControlAllowed: browsers started for remote control never run
// during website blocks on Windows (no testing build there).
func RemoteControlAllowed() bool { return false }
