// scripts.go — telling programs apart beyond their executable's name:
// Python programs by their script, copies of a blocked program by their
// contents, and download or streaming tools by the address they were given.
package blocker

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// isPython reports whether an executable's name is a Python interpreter.
func isPython(exe string) bool {
	name := strings.ToLower(baseName(exe))
	return name == "python" || name == "python3" || strings.HasPrefix(name, "python3.") ||
		name == "pypy" || name == "pypy3"
}

// processArgs returns a process's arguments (argv), or nil.
func processArgs(pid int) []string {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil || len(data) == 0 {
		return nil
	}
	return strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
}

// pythonScript returns what a Python process runs: the script's full path,
// or a module's name for "-m module". It is "" for anything else, also for
// "python -c …" and an interactive interpreter.
func pythonScript(pid int) string {
	exe, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe")
	if err != nil || !isPython(strings.TrimSuffix(exe, " (deleted)")) {
		return ""
	}
	return scriptFromArgs(processArgs(pid), func() string {
		cwd, _ := os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd")
		return cwd
	})
}

func scriptFromArgs(args []string, cwd func() string) string {
	for i := 1; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-m":
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		case a == "-c" || a == "-":
			return ""
		case a == "-W" || a == "-X" || a == "-Q":
			i++ // these take a value
		case strings.HasPrefix(a, "-"):
		case strings.HasPrefix(a, "/"):
			return a
		default:
			if dir := cwd(); dir != "" {
				return strings.TrimRight(dir, "/") + "/" + strings.TrimPrefix(a, "./")
			}
			return a
		}
	}
	return ""
}

// systemScript reports whether a script (or any program file) is part of the system: root owns it
// and it lies in a system folder. Desktop helpers are such scripts; anything
// in a home folder is not.
func systemScript(path string) bool {
	if !strings.HasPrefix(path, "/") {
		return false
	}
	ok := false
	for _, dir := range []string{"/usr/", "/opt/", "/snap/", "/lib/", "/bin/", "/sbin/"} {
		ok = ok || strings.HasPrefix(path, dir)
	}
	info, err := os.Stat(path)
	if !ok || err != nil {
		return false
	}
	st, isStat := info.Sys().(*syscall.Stat_t)
	return isStat && st.Uid == 0
}

// essentialProcess reports whether a running process is one the desktop
// needs. A Python interpreter counts only when it runs a system script (or
// nothing that can be named): a user's own script is an ordinary program.
func essentialProcess(pid int) bool {
	exe := exeName(pid)
	if !isEssentialApp(exe) {
		return false
	}
	if isPython(exe) {
		if script := pythonScript(pid); strings.HasPrefix(script, "/") && !systemScript(script) {
			return false
		}
	}
	return true
}

// matchesScript reports whether pid is a Python program whose script (or
// module) is named in names: by file name, with or without ".py", by full
// path, or by lying under a listed folder.
func matchesScript(pid int, names []string) bool {
	script := pythonScript(pid)
	if script == "" {
		return false
	}
	base := baseName(script)
	return matchesName(base, names) || matchesName(strings.TrimSuffix(base, ".py"), names) || matchesPath(script, names)
}

// HashPrefix starts an app entry that names a program by its contents:
// "sha256:<hex>:<size>:<path it was taken from>". 'block add-app' adds one
// next to a full path, so a renamed or moved copy of the program is still
// blocked.
const HashPrefix = "sha256:"

// HashEntry returns the contents entry for the program at path.
func HashEntry(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || !strings.HasPrefix(path, "/") {
		return "", false
	}
	sum, err := fileSHA256(path)
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("%s%s:%d:%s", HashPrefix, sum, info.Size(), path), true
}

// HashEntryFor reports whether entry is the contents entry made for path.
func HashEntryFor(entry, path string) bool {
	return strings.HasPrefix(entry, HashPrefix) && strings.HasSuffix(entry, ":"+path)
}

func parseHashEntry(entry string) (sum string, size int64, ok bool) {
	if !strings.HasPrefix(entry, HashPrefix) {
		return "", 0, false
	}
	parts := strings.SplitN(entry[len(HashPrefix):], ":", 3)
	if len(parts) < 2 || len(parts[0]) != 64 {
		return "", 0, false
	}
	size, err := strconv.ParseInt(parts[1], 10, 64)
	return parts[0], size, err == nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type fileID struct {
	dev, ino    uint64
	size, mtime int64
}

var (
	hashMu    sync.Mutex
	hashCache = map[fileID]string{}
)

// matchesHash reports whether pid's executable has the contents of a listed
// contents entry. Only files of a listed size are read, each once. Root-owned
// programs in system folders are left out.
func matchesHash(pid int, names []string) bool {
	var wanted map[string]int64
	for _, n := range names {
		if sum, size, ok := parseHashEntry(n); ok {
			if wanted == nil {
				wanted = map[string]int64{}
			}
			wanted[sum] = size
		}
	}
	if wanted == nil {
		return false
	}
	path := "/proc/" + strconv.Itoa(pid) + "/exe"
	// The system's own programs are never matched by contents: a copy of a
	// system tool in a blocked folder must not take the tool itself down.
	if real, err := os.Readlink(path); err != nil || systemScript(strings.TrimSuffix(real, " (deleted)")) {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	sized := false
	for _, size := range wanted {
		sized = sized || size == info.Size()
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !sized || !ok {
		return false
	}
	id := fileID{uint64(st.Dev), st.Ino, info.Size(), info.ModTime().UnixNano()}
	hashMu.Lock()
	sum, cached := hashCache[id]
	hashMu.Unlock()
	if !cached {
		if sum, err = fileSHA256(path); err != nil {
			return false
		}
		hashMu.Lock()
		if len(hashCache) > 4096 {
			hashCache = map[fileID]string{}
		}
		hashCache[id] = sum
		hashMu.Unlock()
	}
	_, hit := wanted[sum]
	return hit
}

// fetchTools are programs that download or play what an address points to.
// One of them given a blocked site's address is a way around the browser.
var fetchTools = map[string]bool{
	"yt-dlp": true, "youtube-dl": true, "yt-dlp_linux": true, "gallery-dl": true, "streamlink": true,
	"mpv": true, "vlc": true, "celluloid": true, "smplayer": true, "mplayer": true, "ffplay": true,
	"ffmpeg": true, "totem": true, "haruna": true, "curl": true, "wget": true, "wget2": true,
	"aria2c": true, "axel": true, "http": true, "https": true, "xh": true, "lynx": true, "w3m": true,
	"links": true, "links2": true, "elinks": true, "browsh": true, "freetube": true,
}

// namesDomain reports whether an argument names a domain (or a subdomain of
// it) as a host: at the start, after "//", "@", "." or "=", and followed by
// the end, "/", ":", "?" or "#". The domain inside a longer word doesn't count.
func namesDomain(arg, domain string) bool {
	arg = strings.ToLower(arg)
	for from := 0; ; {
		i := strings.Index(arg[from:], domain)
		if i < 0 {
			return false
		}
		i += from
		end := i + len(domain)
		before := i == 0 || strings.ContainsRune("/@.=", rune(arg[i-1]))
		after := end == len(arg) || strings.ContainsRune("/:?#", rune(arg[end]))
		if before && after {
			return true
		}
		from = i + 1
	}
}

// toolNaming returns the fetch tool a process is, if its arguments name one
// of the domains; "" otherwise.
func toolNaming(comm, exe, script string, args, domains []string) string {
	tool := ""
	for _, name := range []string{comm, baseName(exe), baseName(script)} {
		if fetchTools[strings.ToLower(name)] {
			tool = strings.ToLower(name)
			break
		}
	}
	if tool == "" || len(args) < 2 {
		return ""
	}
	for _, arg := range args[1:] {
		for _, d := range domains {
			if d != "" && namesDomain(arg, d) {
				return tool
			}
		}
	}
	return ""
}

// KillNaming kills the user's download and streaming tools that were given
// the address of one of the domains, and returns their names.
func KillNaming(domains []string) []string {
	if len(domains) == 0 {
		return nil
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var tools []string
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 1 || pid == os.Getpid() || !ownedByUser(pid) {
			continue
		}
		exe, _ := os.Readlink("/proc/" + entry.Name() + "/exe")
		tool := toolNaming(readComm(pid), strings.TrimSuffix(exe, " (deleted)"), pythonScript(pid), processArgs(pid), domains)
		if tool != "" && syscall.Kill(pid, syscall.SIGKILL) == nil {
			tools = append(tools, tool)
		}
	}
	return tools
}
