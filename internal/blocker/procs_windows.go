package blocker

// procs_windows.go — processes on Windows, as /proc gives them on Linux:
// a snapshot with each process's parent, full program path, owner (its
// account's relative ID, the Windows uid: platform package), session and
// start time; killing; and which process owns a TCP connection.

import (
	"encoding/binary"
	"net"
	"path/filepath"
	"strings"
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

// killProcess ends pid at once; replaced in tests.
var killProcess = func(pid uint32) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
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
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, pid)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(h)
	var pbi windows.PROCESS_BASIC_INFORMATION
	if err := windows.NtQueryInformationProcess(h, windows.ProcessBasicInformation, unsafe.Pointer(&pbi), uint32(unsafe.Sizeof(pbi)), nil); err != nil {
		return nil
	}
	read := func(addr uintptr, buf []byte) bool {
		var n uintptr
		return windows.ReadProcessMemory(h, addr, &buf[0], uintptr(len(buf)), &n) == nil && int(n) == len(buf)
	}
	word := make([]byte, 8)
	// PEB.ProcessParameters at 0x20.
	if !read(uintptr(unsafe.Pointer(pbi.PebBaseAddress))+0x20, word) {
		return nil
	}
	params := uintptr(binary.LittleEndian.Uint64(word))
	// RTL_USER_PROCESS_PARAMETERS: Environment at 0x80, EnvironmentSize at 0x3F0.
	if params == 0 || !read(params+0x80, word) {
		return nil
	}
	env := uintptr(binary.LittleEndian.Uint64(word))
	if env == 0 || !read(params+0x3F0, word) {
		return nil
	}
	size := binary.LittleEndian.Uint64(word)
	if size == 0 || size > 1<<20 || size%2 != 0 {
		return nil
	}
	block := make([]byte, size)
	if !read(env, block) {
		return nil
	}
	return splitEnvBlock(block)
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
