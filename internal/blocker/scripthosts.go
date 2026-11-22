package blocker

// scripthosts.go — Windows' script hosts (ROADMAP W12): which script an
// interpreter runs, from its arguments, as scripts.go finds a Python
// program's script on Linux. Pure, so it is tested on every system.

import "strings"

// scriptHosts are the interpreters a script is run by, as Windows names
// their programs (lower case, no .exe).
var scriptHosts = map[string]string{
	"python": "python", "python3": "python", "pythonw": "python", "py": "python", "pyw": "python",
	"pypy": "python", "pypy3": "python",
	"powershell": "powershell", "pwsh": "powershell", "powershell_ise": "powershell",
	"cmd": "cmd",
	"wscript": "wsh", "cscript": "wsh",
	"mshta": "mshta",
	"node": "node",
	"java": "java", "javaw": "java",
	"autohotkey": "ahk", "autohotkey64": "ahk", "autohotkey32": "ahk", "autohotkeyu64": "ahk",
	"autohotkeyu32": "ahk", "autohotkeya32": "ahk", "autohotkeyux": "ahk",
}

// scriptExts are the extensions an app entry naming a script may leave out.
var scriptExts = []string{".py", ".pyw", ".ps1", ".bat", ".cmd", ".vbs", ".vbe", ".js", ".jse", ".wsf",
	".hta", ".mjs", ".cjs", ".jar", ".ahk"}

// winAbs reports whether p is an absolute Windows path (C:\..., \\server\...).
func winAbs(p string) bool {
	return (len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/')) || strings.HasPrefix(p, `\\`)
}

// winJoin is p made absolute against cwd, with backslashes.
func winJoin(cwd, p string) string {
	p = strings.ReplaceAll(p, "/", `\`)
	if winAbs(p) || cwd == "" {
		return p
	}
	p = strings.TrimPrefix(p, `.\`)
	return strings.TrimRight(strings.ReplaceAll(cwd, "/", `\`), `\`) + `\` + p
}

// hostScript returns the script the interpreter host runs, given its
// arguments (args[0] is the program) and working folder: a full path, or a
// module's name for "python -m module". It is "" for inline code (-c,
// -Command, -e), an interactive interpreter, or a program that isn't a
// script host.
func hostScript(host string, args []string, cwd string) string {
	kind := scriptHosts[host]
	if kind == "" || len(args) < 2 {
		return ""
	}
	rest := args[1:]
	lower := func(a string) string { return strings.ToLower(a) }
	switch kind {
	case "python":
		for i := 0; i < len(rest); i++ {
			a := rest[i]
			switch {
			case a == "-m":
				if i+1 < len(rest) {
					return rest[i+1]
				}
				return ""
			case a == "-c" || a == "-":
				return ""
			case a == "-W" || a == "-X" || a == "-Q":
				i++
			case strings.HasPrefix(a, "-"):
				// py.exe's version switches (-3, -3.12) and flags
			default:
				return winJoin(cwd, a)
			}
		}
	case "powershell":
		valued := map[string]bool{"executionpolicy": true, "ep": true, "ex": true, "windowstyle": true, "w": true,
			"version": true, "v": true, "inputformat": true, "if": true, "outputformat": true, "of": true, "o": true,
			"configurationname": true, "workingdirectory": true, "wd": true, "psconsolefile": true,
			"settingsfile": true, "custompipename": true}
		for i := 0; i < len(rest); i++ {
			a := lower(rest[i])
			if !strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "/") {
				if strings.HasSuffix(a, ".ps1") {
					return winJoin(cwd, rest[i])
				}
				return "" // a command
			}
			name := strings.TrimLeft(a, "-/")
			switch {
			case name == "file" || name == "f":
				if i+1 < len(rest) {
					return winJoin(cwd, rest[i+1])
				}
				return ""
			case name == "command" || name == "c" || strings.HasPrefix(name, "encodedcommand") || name == "e" || name == "ec":
				return ""
			case valued[name]:
				i++
			}
		}
	case "cmd":
		for i, a := range rest {
			if l := lower(a); l == "/c" || l == "/k" || l == "/r" {
				if i+1 >= len(rest) {
					return ""
				}
				line := strings.TrimSpace(strings.Join(rest[i+1:], " "))
				first := strings.Fields(line + " ")[0]
				if strings.HasPrefix(line, `"`) {
					if end := strings.IndexByte(line[1:], '"'); end >= 0 {
						first = line[1 : 1+end]
					}
				}
				first = strings.Trim(first, `"`)
				if l := lower(first); strings.HasSuffix(l, ".bat") || strings.HasSuffix(l, ".cmd") {
					return winJoin(cwd, first)
				}
				return ""
			}
		}
	case "wsh", "ahk":
		for _, a := range rest {
			if strings.HasPrefix(a, "//") || (kind == "ahk" && strings.HasPrefix(a, "/")) {
				continue
			}
			return winJoin(cwd, a)
		}
	case "mshta":
		a := rest[0]
		if strings.Contains(a, "://") || strings.HasPrefix(lower(a), "javascript:") || strings.HasPrefix(lower(a), "vbscript:") {
			return ""
		}
		return winJoin(cwd, a)
	case "node":
		valued := map[string]bool{"-r": true, "--require": true, "--import": true, "--loader": true,
			"--experimental-loader": true, "--inspect-port": true, "--title": true, "--env-file": true}
		for i := 0; i < len(rest); i++ {
			a := rest[i]
			switch {
			case a == "-e" || a == "--eval" || a == "-p" || a == "--print" || a == "-" || a == "-i":
				return ""
			case valued[a]:
				i++
			case strings.HasPrefix(a, "-"):
			default:
				return winJoin(cwd, a)
			}
		}
	case "java":
		for i, a := range rest {
			if a == "-jar" && i+1 < len(rest) {
				return winJoin(cwd, rest[i+1])
			}
		}
	}
	return ""
}

// scriptNames are the names an app entry may give a script by: its file
// name, without the extension, and for a module its name with "-" for "_"
// (yt_dlp is yt-dlp).
func scriptNames(script string) []string {
	s := strings.ToLower(script)
	if i := strings.LastIndexAny(s, `\/`); i >= 0 {
		s = s[i+1:]
	}
	names := []string{s}
	for _, ext := range scriptExts {
		if strings.HasSuffix(s, ext) {
			names = append(names, strings.TrimSuffix(s, ext))
		}
	}
	if !strings.ContainsAny(script, `\/`) && strings.Contains(s, "_") { // a module
		names = append(names, strings.ReplaceAll(s, "_", "-"))
	}
	if i := strings.IndexByte(s, '.'); i > 0 && !strings.ContainsAny(script, `\/`) { // package.module
		names = append(names, s[:i])
	}
	return names
}
