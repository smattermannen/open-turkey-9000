package blocker

import (
	"slices"
	"testing"
)

// W12: the script each of Windows' script hosts runs, from its arguments.
func TestHostScript(t *testing.T) {
	cwd := `C:\Users\u\Downloads`
	cases := []struct {
		host string
		args []string
		want string
	}{
		{"python", []string{"python.exe", "bot.py", "--fast"}, `C:\Users\u\Downloads\bot.py`},
		{"pythonw", []string{"pythonw.exe", "-u", `D:\games\run.pyw`}, `D:\games\run.pyw`},
		{"py", []string{"py.exe", "-3.12", "-m", "yt_dlp", "https://youtube.com/x"}, "yt_dlp"},
		{"python", []string{"python.exe", "-c", "print(1)"}, ""},
		{"python", []string{"python.exe"}, ""},
		{"python", []string{"python.exe", "-X", "utf8", `.\tools\a.py`}, `C:\Users\u\Downloads\tools\a.py`},
		{"powershell", []string{"powershell.exe", "-ExecutionPolicy", "Bypass", "-File", "game.ps1"}, `C:\Users\u\Downloads\game.ps1`},
		{"pwsh", []string{"pwsh.exe", "-NoProfile", `C:\s\x.ps1`}, `C:\s\x.ps1`},
		{"powershell", []string{"powershell.exe", "-Command", "Get-Date"}, ""},
		{"powershell", []string{"powershell.exe", "-enc", "AAAA"}, ""},
		{"powershell", []string{"powershell.exe", "Get-Date"}, ""},
		{"cmd", []string{"cmd.exe", "/c", "run.bat", "x"}, `C:\Users\u\Downloads\run.bat`},
		{"cmd", []string{"cmd.exe", "/c", `"C:\a b\go.cmd" now`}, `C:\a b\go.cmd`},
		{"cmd", []string{"cmd.exe", "/c", "dir"}, ""},
		{"cmd", []string{"cmd.exe"}, ""},
		{"wscript", []string{"wscript.exe", "//B", "//Nologo", "a.vbs"}, `C:\Users\u\Downloads\a.vbs`},
		{"mshta", []string{"mshta.exe", `C:\x\app.hta`}, `C:\x\app.hta`},
		{"mshta", []string{"mshta.exe", "https://example.com/a.hta"}, ""},
		{"node", []string{"node.exe", "--require", "dotenv/config", "server.js"}, `C:\Users\u\Downloads\server.js`},
		{"node", []string{"node.exe", "-e", "1"}, ""},
		{"javaw", []string{"javaw.exe", "-Xmx2G", "-jar", "Game.jar"}, `C:\Users\u\Downloads\Game.jar`},
		{"autohotkey64", []string{"AutoHotkey64.exe", "/restart", "keys.ahk"}, `C:\Users\u\Downloads\keys.ahk`},
		{"notepad", []string{"notepad.exe", "a.py"}, ""},
	}
	for _, c := range cases {
		if got := hostScript(c.host, c.args, cwd); got != c.want {
			t.Errorf("%s %v: %q, want %q", c.host, c.args[1:], got, c.want)
		}
	}
}

func TestScriptNames(t *testing.T) {
	if got := scriptNames(`C:\Users\u\Bot.PY`); !slices.Contains(got, "bot.py") || !slices.Contains(got, "bot") {
		t.Errorf("script names %v", got)
	}
	if got := scriptNames("yt_dlp"); !slices.Contains(got, "yt-dlp") {
		t.Errorf("a module's names %v lack yt-dlp", got)
	}
	if got := scriptNames(`C:\x\run_me.py`); slices.Contains(got, "run-me") {
		t.Errorf("a file's underscores changed: %v", got)
	}
}
