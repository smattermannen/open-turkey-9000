package blocker

// apps_windows.go — blocking applications on Windows (ROADMAP W3): what
// the Linux process layer does with /proc, with a process snapshot and
// full program paths. An app entry is a program name ("steam" or
// "steam.exe"), a full path, a folder (ending in \ or /: everything under
// it), or a contents entry (sha256:..., hashentry.go); matched processes
// are killed with everything they started, only processes of people's
// accounts (never SYSTEM's or services'), never the programs Windows needs,
// never KC9K or the processes it runs under.

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// essentialWindowsApps are programs Windows' desktop needs: blocking one
// would end the session over and over.
var essentialWindowsApps = map[string]bool{
	"explorer": true, "csrss": true, "winlogon": true, "wininit": true, "lsass": true, "services": true,
	"smss": true, "svchost": true, "dwm": true, "fontdrvhost": true, "sihost": true, "ctfmon": true,
	"conhost": true, "runtimebroker": true, "taskhostw": true, "searchhost": true, "searchapp": true,
	"startmenuexperiencehost": true, "shellexperiencehost": true, "textinputhost": true, "logonui": true,
	"userinit": true, "dllhost": true, "audiodg": true, "spoolsv": true, "msmpeng": true, "nissrv": true,
	"securityhealthservice": true, "securityhealthsystray": true, "smartscreen": true, "lockapp": true,
	"applicationframehost": true, "systemsettings": true, "wudfhost": true, "dashost": true,
	"kc9k": true, "kc9k-gui": true, "taskmgr": false,
}

// protectedWindowsFolders hold programs the system needs.
func protectedWindowsFolders() []string {
	root := systemRoot()
	return []string{filepath.VolumeName(root) + `\`, root, filepath.Join(root, "System32"), filepath.Join(root, "SysWOW64"),
		programFilesDir(), programFilesX86Dir(), filepath.Join(filepath.VolumeName(root)+`\`, "Users"),
		os.Getenv("ProgramData")}
}

// appKey is an entry as compared: lower case, without .exe for names.
func appKey(entry string) string {
	e := strings.ToLower(strings.TrimSpace(entry))
	if !isPathEntry(e) {
		e = strings.TrimSuffix(e, ".exe")
	}
	return e
}

func isPathEntry(e string) bool { return strings.ContainsAny(e, `\/`) || filepath.VolumeName(e) != "" }

func isFolderEntry(e string) bool { return strings.HasSuffix(e, `\`) || strings.HasSuffix(e, "/") }

func isEssentialWindowsApp(entry string) bool {
	if isPathEntry(entry) && !isFolderEntry(entry) {
		return essentialWindowsApps[exeBase(entry)] && underSystemRoot(entry)
	}
	return essentialWindowsApps[appKey(entry)]
}

func underSystemRoot(path string) bool {
	return strings.HasPrefix(strings.ToLower(filepath.Clean(path)), strings.ToLower(systemRoot())+`\`)
}

// ValidAppEntry rejects folders and programs that can't safely be blocked.
func ValidAppEntry(entry string) error {
	if isFolderEntry(entry) || (isPathEntry(entry) && dirExists(entry)) {
		clean := strings.ToLower(filepath.Clean(entry))
		for _, f := range protectedWindowsFolders() {
			if f != "" && clean == strings.ToLower(filepath.Clean(f)) {
				return fmt.Errorf("the folder %s can't be blocked, because the system needs it", entry)
			}
		}
		return nil
	}
	if isEssentialWindowsApp(entry) {
		return fmt.Errorf("%s can't be blocked: Windows needs it, and blocking it would close your session over and over", entry)
	}
	return nil
}

// appMatcher matches processes against app entries.
type appMatcher struct {
	names   map[string]string // name -> entry
	paths   map[string]string // full path -> entry
	folders []struct{ dir, entry string }
	hashes  map[string]int64 // sha256 -> size
	hashEnt map[string]string
}

func newAppMatcher(entries []string) *appMatcher {
	m := &appMatcher{names: map[string]string{}, paths: map[string]string{}, hashes: map[string]int64{}, hashEnt: map[string]string{}}
	for _, e := range entries {
		if isEssentialWindowsApp(e) {
			continue
		}
		if sum, size, ok := parseHashEntry(e); ok {
			m.hashes[sum], m.hashEnt[sum] = size, e
			continue
		}
		k := appKey(e)
		switch {
		case isFolderEntry(k):
			m.folders = append(m.folders, struct{ dir, entry string }{strings.ToLower(filepath.Clean(k)) + `\`, e})
		case isPathEntry(k):
			m.paths[strings.ToLower(filepath.Clean(k))] = e
		default:
			m.names[k] = e
		}
	}
	return m
}

func (m *appMatcher) empty() bool {
	return len(m.names)+len(m.paths)+len(m.folders)+len(m.hashes) == 0
}

// match is the entry p matches, or "".
func (m *appMatcher) match(p *winProc) string {
	if essentialWindowsApps[p.Name] && (p.Exe == "" || underSystemRoot(p.Exe)) {
		return ""
	}
	if e, ok := m.names[p.Name]; ok {
		return e
	}
	if p.Exe == "" {
		return ""
	}
	exe := strings.ToLower(filepath.Clean(p.Exe))
	if e, ok := m.paths[exe]; ok {
		return e
	}
	for _, f := range m.folders {
		if strings.HasPrefix(exe, f.dir) {
			return f.entry
		}
	}
	if len(m.hashes) > 0 && !underSystemRoot(p.Exe) {
		if sum := cachedHash(p.Exe, m.hashes); sum != "" {
			return m.hashEnt[sum]
		}
	}
	return ""
}

var programHashes = struct {
	sync.Mutex
	m map[string]string // path|size|mtime -> sha256
}{m: map[string]string{}}

// cachedHash is the sha256 of exe if its size is one of wanted's.
func cachedHash(exe string, wanted map[string]int64) string {
	info, err := os.Stat(exe)
	if err != nil {
		return ""
	}
	sized := false
	for _, size := range wanted {
		sized = sized || size == info.Size()
	}
	if !sized {
		return ""
	}
	key := fmt.Sprintf("%s|%d|%d", strings.ToLower(exe), info.Size(), info.ModTime().UnixNano())
	programHashes.Lock()
	sum, ok := programHashes.m[key]
	programHashes.Unlock()
	if !ok {
		if sum, err = fileSHA256(exe); err != nil {
			return ""
		}
		programHashes.Lock()
		if len(programHashes.m) > 4096 {
			programHashes.m = map[string]string{}
		}
		programHashes.m[key] = sum
		programHashes.Unlock()
	}
	if _, hit := wanted[sum]; hit {
		return sum
	}
	return ""
}

// noDescendWindows are programs whose children aren't theirs to kill.
var noDescendWindows = map[string]bool{"explorer": true, "cmd": true, "powershell": true, "pwsh": true,
	"windowsterminal": true, "openconsole": true, "wt": true, "svchost": true, "services": true}

// protectedPIDs are this process and the processes above it.
func protectedPIDs(procs map[uint32]*winProc) map[uint32]bool {
	out := map[uint32]bool{0: true, 4: true}
	p := procs[uint32(os.Getpid())]
	for i := 0; p != nil && i < 64; i++ {
		out[p.PID] = true
		p = parentOf(procs, p)
	}
	return out
}

// KillBlocked kills every person's process of the apps in processNames.
func KillBlocked(processNames []string) (int, error) {
	return killBlockedWin(processNames, false, time.Time{}, nil)
}

// KillBlockedFor is KillBlocked for the users uids only (nil: everyone;
// empty: no one), politely: asking apps with windows to close first.
func KillBlockedFor(processNames []string, uids []uint64, politely bool, now time.Time) (int, error) {
	if uids != nil && len(uids) == 0 {
		return 0, nil
	}
	return killBlockedWin(processNames, politely, now, uids)
}

func killBlockedWin(names []string, politely bool, now time.Time, uids []uint64) (int, error) {
	m := newAppMatcher(names)
	if m.empty() {
		return 0, nil
	}
	procs := processSnapshot()
	children := childrenOf(procs)
	var roots []int
	for _, p := range procs {
		if m.match(p) != "" && ownedByAnyUID(p, uids) {
			roots = append(roots, int(p.PID))
		}
	}
	asked := 0
	if politely {
		wait, fresh := polite.sort(roots, now, func(pid int) string {
			if p := procs[uint32(pid)]; p != nil {
				return p.Name
			}
			return ""
		})
		kept := roots[:0]
		for _, pid := range roots {
			if !wait[pid] {
				kept = append(kept, pid)
			}
		}
		roots = kept
		asked = len(fresh)
		askToCloseWin(fresh, procs)
	}
	protected := protectedPIDs(procs)
	killed := 0
	for _, root := range roots {
		tree := []uint32{uint32(root)}
		if p := procs[uint32(root)]; p != nil && !noDescendWindows[p.Name] {
			tree = treeOf(uint32(root), children)
		}
		for i := len(tree) - 1; i >= 0; i-- {
			pid := tree[i]
			p := procs[pid]
			if protected[pid] || p == nil || !ownedByAnyUID(p, uids) {
				continue
			}
			protected[pid] = true
			if killProcess(pid) == nil {
				killed++
			}
		}
	}
	return killed + asked, nil
}

// RunningApps returns the entries of names that have a process now.
func RunningApps(names []string) []string {
	procs := processSnapshot()
	var out []string
	for _, n := range names {
		m := newAppMatcher([]string{n})
		if m.empty() {
			continue
		}
		for _, p := range procs {
			if p.person() && m.match(p) != "" {
				out = append(out, n)
				break
			}
		}
	}
	return out
}

// systemProgram reports whether a person's process is still the system's
// (Frozen Chicken's allowed apps keep it): Windows' own programs and KC9K.
func systemProgram(p *winProc) bool {
	if essentialWindowsApps[p.Name] {
		return true
	}
	if p.Exe == "" {
		return true // unreadable: not ours to judge
	}
	return underSystemRoot(p.Exe) || strings.HasPrefix(strings.ToLower(p.Exe), strings.ToLower(filepath.Join(programFilesDir(), "KC9K"))+`\`)
}

// CloseAllExcept kills every program of people that isn't an allowed app,
// started by one, or Windows' own (Frozen Chicken's allowed apps only).
func CloseAllExcept(allowed []string) int {
	m := newAppMatcher(allowed)
	procs := processSnapshot()
	keep := func(p *winProc) bool {
		for i := 0; p != nil && i < 64; i++ {
			if m.match(p) != "" {
				return true
			}
			p = parentOf(procs, p)
		}
		return false
	}
	protected := protectedPIDs(procs)
	closed := 0
	for _, p := range procs {
		if !p.person() || protected[p.PID] || systemProgram(p) || keep(p) {
			continue
		}
		if killProcess(p.PID) == nil {
			log.Printf("freeze: closed %s (pid %d): not an allowed app", p.Name, p.PID)
			closed++
		}
	}
	return closed
}

// torProgram reports whether exe is Tor or Tor Browser.
func torProgram(p *winProc) bool {
	lower := strings.ToLower(p.Exe)
	for _, part := range []string{`\tor browser\`, `\torbrowser\`, `\tor-browser\`} {
		if strings.Contains(lower, part) {
			return true
		}
	}
	return p.Name == "tor"
}

// CloseTor kills Tor and Tor Browser (tamper protection "close private
// windows and Tor").
func CloseTor() int {
	procs := processSnapshot()
	n := 0
	for _, p := range procs {
		if p.person() && torProgram(p) && killProcess(p.PID) == nil {
			n++
		}
	}
	return n
}

// KillNamingFor kills download and player tools (curl, yt-dlp, vlc...)
// whose command line names a blocked domain, and returns their names.
func KillNamingFor(domains []string, uids []uint64) []string {
	if len(domains) == 0 || (uids != nil && len(uids) == 0) {
		return nil
	}
	var tools []string
	for _, p := range processSnapshot() {
		if !fetchTools[p.Name] || !ownedByAnyUID(p, uids) {
			continue
		}
		args := commandLine(p.PID)
		for _, arg := range args[min(1, len(args)):] {
			hit := false
			for _, d := range domains {
				if d != "" && namesDomain(arg, d) {
					hit = true
					break
				}
			}
			if hit {
				if killProcess(p.PID) == nil {
					tools = append(tools, p.Name)
				}
				break
			}
		}
	}
	return tools
}

var (
	ntdll                         = windows.NewLazySystemDLL("ntdll.dll")
	procNtQueryInformationProcess = ntdll.NewProc("NtQueryInformationProcess")
)

// commandLine is pid's command line, split as Windows programs split it.
func commandLine(pid uint32) []string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(h)
	const processCommandLineInformation = 60
	buf := make([]byte, 8192)
	var n uint32
	r, _, _ := procNtQueryInformationProcess.Call(uintptr(h), processCommandLineInformation,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&n)))
	if r != 0 {
		return nil
	}
	us := (*windows.NTUnicodeString)(unsafe.Pointer(&buf[0]))
	if us.Buffer == nil || us.Length == 0 {
		return nil
	}
	text := windows.UTF16ToString(unsafe.Slice(us.Buffer, us.Length/2))
	args, err := windows.DecomposeCommandLine(text)
	if err != nil {
		return strings.Fields(text)
	}
	return args
}
