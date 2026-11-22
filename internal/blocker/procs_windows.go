package blocker

// procs_windows.go — processes on Windows, as /proc gives them on Linux:
// a snapshot with each process's parent, full program path, owner (its
// account's relative ID, the Windows uid: platform package), session and
// start time; killing; and which process owns a TCP connection.

import (
	"encoding/binary"
	"errors"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/smattermannen/kc9k/internal/platform"
	"golang.org/x/sys/windows"
)

// winProc is one process.
type winProc struct {
	PID, PPID uint32
	Exe       string // full path; "" when it can't be read (protected)
	Name      string // program name, lower case, without .exe
	UID       int64  // owner's relative ID; -1 unknown, 18 SYSTEM's well-known RID
	System    bool   // owned by SYSTEM, LOCAL SERVICE or NETWORK SERVICE
	Session   uint32
	Started   time.Time
}

// exeBase is a program path's name, lower case, without .exe.
func exeBase(path string) string {
	return strings.TrimSuffix(strings.ToLower(filepath.Base(path)), ".exe")
}

// processSnapshot is every process now; replaced in tests.
var processSnapshot = func() map[uint32]*winProc {
	out := map[uint32]*winProc{}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return out
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if e.ProcessID == 0 {
			continue
		}
		p := &winProc{PID: e.ProcessID, PPID: e.ParentProcessID, UID: -1,
			Name: exeBase(windows.UTF16ToString(e.ExeFile[:]))}
		_ = windows.ProcessIdToSessionId(p.PID, &p.Session)
		fillProcess(p)
		out[p.PID] = p
	}
	return out
}

// fillProcess reads what needs a handle: path, owner, start time.
func fillProcess(p *winProc) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, p.PID)
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) == nil {
		p.Exe = windows.UTF16ToString(buf[:n])
		p.Name = exeBase(p.Exe)
	}
	var created, exited, kernel, user windows.Filetime
	if windows.GetProcessTimes(h, &created, &exited, &kernel, &user) == nil {
		p.Started = time.Unix(0, created.Nanoseconds())
	}
	var token windows.Token
	if windows.OpenProcessToken(h, windows.TOKEN_QUERY, &token) == nil {
		if u, err := token.GetTokenUser(); err == nil {
			p.UID = platform.RID(u.User.Sid)
			p.System = isServiceAccount(u.User.Sid)
		}
		token.Close()
	}
}

// isServiceAccount reports whether sid is SYSTEM, LOCAL SERVICE or
// NETWORK SERVICE (S-1-5-18, -19, -20).
func isServiceAccount(sid *windows.SID) bool {
	s := sid.String()
	return s == "S-1-5-18" || s == "S-1-5-19" || s == "S-1-5-20"
}

// person reports whether p belongs to a person's account (not a service
// account, not unreadable).
func (p *winProc) person() bool { return p.UID >= 0 && !p.System }

// parentOf is p's parent if it is still the process that started p (a
// parent's ID can be reused after it ends).
func parentOf(procs map[uint32]*winProc, p *winProc) *winProc {
	parent := procs[p.PPID]
	if parent == nil || parent.PID == p.PID {
		return nil
	}
	if !p.Started.IsZero() && !parent.Started.IsZero() && parent.Started.After(p.Started) {
		return nil
	}
	return parent
}

// childrenOf maps each process to the processes it started.
func childrenOf(procs map[uint32]*winProc) map[uint32][]uint32 {
	out := map[uint32][]uint32{}
	for _, p := range procs {
		if parent := parentOf(procs, p); parent != nil {
			out[parent.PID] = append(out[parent.PID], p.PID)
		}
	}
	return out
}

// treeOf is pid and every process below it.
func treeOf(pid uint32, children map[uint32][]uint32) []uint32 {
	out := []uint32{pid}
	for i := 0; i < len(out); i++ {
		out = append(out, children[out[i]]...)
	}
	return out
}

// mainOf is the topmost process of p's program above p: a browser's main
// process for one of its helpers.
func mainOf(procs map[uint32]*winProc, p *winProc) *winProc {
	for {
		parent := parentOf(procs, p)
		if parent == nil || parent.Name != p.Name || parent.Exe != p.Exe {
			return p
		}
		p = parent
	}
}

// errNeverKill: the process is one Windows needs, or (for killProcess) a
// program of Windows' own folder.
var errNeverKill = errors.New("a program Windows needs: never closed")

// neverKillExtra are, with essentialWindowsApps, the programs no kill
// path ends (W50: the extension guard once closed dwm, fontdrvhost,
// sihost, ctfmon, svchost... every 5 s until the laptop hung).
var neverKillExtra = map[string]bool{
	"shellhost": true, "lsaiso": true, "registry": true, "memcompression": true, "memory compression": true,
	"system": true, "secure system": true, "searchindexer": true, "wmiprvse": true, "taskhost": true,
}

// sessionCritical reports whether a process is one Windows or KC9K needs
// (the hard deny-list every Windows kill path goes through): PID 0 and 4,
// Windows' session and shell programs in Windows' folder (or unreadable:
// protected), KC9K's own programs. A copy named svchost.exe in Downloads
// is not one.
func sessionCritical(pid uint32, name, exe string) bool {
	if pid <= 4 {
		return true
	}
	if (essentialWindowsApps[name] || neverKillExtra[name]) && (exe == "" || underSystemRoot(exe)) {
		return true
	}
	if strings.HasPrefix(name, "kc9k") && (exe == "" || kc9kProgram(exe)) {
		return true
	}
	return false
}

// kc9kProgram reports whether exe is in KC9K's folder or this program's.
func kc9kProgram(exe string) bool {
	dir := strings.ToLower(filepath.Dir(filepath.Clean(exe)))
	if dir == strings.ToLower(filepath.Join(programFilesDir(), "KC9K")) {
		return true
	}
	self, err := os.Executable()
	return err == nil && dir == strings.ToLower(filepath.Dir(self))
}

// killProcess ends a process that is not Windows' own: never a
// session-critical one, never a program under %SystemRoot% (nor one whose
// path can't be read). For what KC9K found by itself: browsers, trees
// below a blocked app, people's own programs.
func killProcess(p *winProc) error { return killChecked(p, false) }

// killNamedProcess ends a process a rule names (a blocked app, a tamper
// tool, a fetch tool naming a blocked site, a blocked window's owner): a
// program in Windows' folder may be one (taskmgr, cmd, curl), a
// session-critical one never.
func killNamedProcess(p *winProc) error { return killChecked(p, true) }

func killChecked(p *winProc, named bool) error {
	if p == nil {
		return errors.New("no such process")
	}
	if sessionCritical(p.PID, p.Name, p.Exe) || (!named && (p.Exe == "" || underSystemRoot(p.Exe))) {
		noteRefusedKill(p)
		return errNeverKill
	}
	return terminateProcess(p.PID)
}

// refusedKills notes refused kills, logged once per process.
var refusedKills = struct {
	sync.Mutex
	m map[uint32]string
}{m: map[uint32]string{}}

func noteRefusedKill(p *winProc) {
	refusedKills.Lock()
	defer refusedKills.Unlock()
	if refusedKills.m[p.PID] == p.Name {
		return
	}
	if len(refusedKills.m) > 1000 {
		refusedKills.m = map[uint32]string{}
	}
	refusedKills.m[p.PID] = p.Name
	log.Printf("kill refused: %s (pid %d, %s) is Windows' own", p.Name, p.PID, p.Exe)
}

// terminateProcess ends pid at once; replaced in tests. It checks the
// deny-list once more on the process the handle names (a PID can be
// reused since the snapshot).
var terminateProcess = func(pid uint32) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	exe := ""
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) == nil {
		exe = windows.UTF16ToString(buf[:n])
	}
	if exe == "" || sessionCritical(pid, exeBase(exe), exe) {
		return errNeverKill
	}
	return windows.TerminateProcess(h, 1)
}

var (
	iphlpapi                = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable = iphlpapi.NewProc("GetExtendedTcpTable")
)

const tcpTableOwnerPIDConnections = 4

// tcpOwner is the process owning the TCP connection from local port
// localPort to remote port remotePort on this computer, or 0.
func tcpOwner(localPort, remotePort int) uint32 {
	for _, family := range []uint32{windows.AF_INET, windows.AF_INET6} {
		size := uint32(0)
		procGetExtendedTcpTable.Call(0, uintptr(unsafe.Pointer(&size)), 0, uintptr(family), tcpTableOwnerPIDConnections, 0)
		if size == 0 {
			continue
		}
		buf := make([]byte, size+1024)
		size = uint32(len(buf))
		if r, _, _ := procGetExtendedTcpTable.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0,
			uintptr(family), tcpTableOwnerPIDConnections, 0); r != 0 {
			continue
		}
		if pid := findTCPOwner(buf[:size], family == windows.AF_INET6, localPort, remotePort); pid != 0 {
			return pid
		}
	}
	return 0
}

// findTCPOwner reads a MIB_TCPTABLE_OWNER_PID (or the IPv6 one): a count,
// then rows; ports are in network byte order in the low 16 bits.
func findTCPOwner(table []byte, v6 bool, localPort, remotePort int) uint32 {
	if len(table) < 4 {
		return 0
	}
	n := int(binary.LittleEndian.Uint32(table))
	row, lp, rp, pid := 24, 8, 16, 20 // MIB_TCPROW_OWNER_PID: state, laddr, lport, raddr, rport, pid
	if v6 {
		row, lp, rp, pid = 56, 20, 44, 52 // laddr[16], scope, lport, raddr[16], scope, rport, state, pid
	}
	port := func(b []byte) int { return int(b[0])<<8 | int(b[1]) }
	for i := 0; i < n; i++ {
		off := 4 + i*row
		if off+row > len(table) {
			break
		}
		r := table[off : off+row]
		if port(r[lp:]) == localPort && port(r[rp:]) == remotePort {
			return binary.LittleEndian.Uint32(r[pid:])
		}
	}
	return 0
}

// localPort is the port of a "host:port" address, or -1.
func localPort(addr string) int {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return -1
	}
	n := 0
	for _, c := range p {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// processEnvironment is pid's environment ("KEY=value" each), read from its
// process parameters (PEB, x64 layout); nil if it can't be read.
func processEnvironment(pid uint32) []string {
	env, _ := processParams(pid, true)
	return env
}

// processCwd is pid's working folder, or "".
func processCwd(pid uint32) string {
	_, cwd := processParams(pid, false)
	return cwd
}

// processParams reads pid's environment (withEnv) and working folder from
// its RTL_USER_PROCESS_PARAMETERS.
func processParams(pid uint32, withEnv bool) (env []string, cwd string) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, pid)
	if err != nil {
		return nil, ""
	}
	defer windows.CloseHandle(h)
	var pbi windows.PROCESS_BASIC_INFORMATION
	if err := windows.NtQueryInformationProcess(h, windows.ProcessBasicInformation, unsafe.Pointer(&pbi), uint32(unsafe.Sizeof(pbi)), nil); err != nil {
		return nil, ""
	}
	read := func(addr uintptr, buf []byte) bool {
		var n uintptr
		return windows.ReadProcessMemory(h, addr, &buf[0], uintptr(len(buf)), &n) == nil && int(n) == len(buf)
	}
	word := make([]byte, 8)
	// PEB.ProcessParameters at 0x20.
	if !read(uintptr(unsafe.Pointer(pbi.PebBaseAddress))+0x20, word) {
		return nil, ""
	}
	params := uintptr(binary.LittleEndian.Uint64(word))
	if params == 0 {
		return nil, ""
	}
	// CurrentDirectory.DosPath (UNICODE_STRING) at 0x38: length, then the
	// buffer's address at 0x40.
	str := make([]byte, 16)
	if read(params+0x38, str) {
		n := int(binary.LittleEndian.Uint16(str[0:2]))
		at := uintptr(binary.LittleEndian.Uint64(str[8:16]))
		if n > 0 && n <= 2*32767 && n%2 == 0 && at != 0 {
			buf := make([]byte, n)
			if read(at, buf) {
				u := make([]uint16, n/2)
				for i := range u {
					u[i] = binary.LittleEndian.Uint16(buf[2*i:])
				}
				cwd = windows.UTF16ToString(u)
			}
		}
	}
	if !withEnv {
		return nil, cwd
	}
	// Environment at 0x80, EnvironmentSize at 0x3F0.
	if !read(params+0x80, word) {
		return nil, cwd
	}
	block := uintptr(binary.LittleEndian.Uint64(word))
	if block == 0 || !read(params+0x3F0, word) {
		return nil, cwd
	}
	size := binary.LittleEndian.Uint64(word)
	if size == 0 || size > 1<<20 || size%2 != 0 {
		return nil, cwd
	}
	data := make([]byte, size)
	if !read(block, data) {
		return nil, cwd
	}
	return splitEnvBlock(data), cwd
}

// splitEnvBlock splits a UTF-16 environment block (NUL-separated, ended by
// two NULs) into its entries.
func splitEnvBlock(block []byte) []string {
	u := make([]uint16, len(block)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(block[2*i:])
	}
	var out []string
	start := 0
	for i, c := range u {
		if c != 0 {
			continue
		}
		if i == start {
			break
		}
		out = append(out, windows.UTF16ToString(u[start:i]))
		start = i + 1
	}
	return out
}
