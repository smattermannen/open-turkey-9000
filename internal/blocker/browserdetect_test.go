package blocker

import (
	"io/fs"
	"syscall"
	"testing"
)

func TestMapsBrowserEngine(t *testing.T) {
	cases := []struct {
		name string
		maps string
		want bool
	}{
		{"firefox", "7f1c000-7f2c000 r-xp 00000000 08:01 1234 /home/u/firefox/libxul.so\n", true},
		{"tor browser", "7f1c000-7f2c000 r-xp 00000000 08:01 1234 /home/u/tor-browser/Browser/libxul.so (deleted)\n", true},
		{"chromium", "55d0-55e0 r--p 00000000 08:01 99 /home/u/chrome-linux64/icudtl.dat\n", true},
		{"electron snapshot", "55d0-55e0 r--p 00000000 08:01 99 /opt/x/v8_context_snapshot.bin\n", true},
		{"ordinary program", "55d0-55e0 r-xp 00000000 08:01 9 /usr/bin/python3.12\n7f00-7f10 r-xp 0 08:01 7 /usr/lib/x86_64-linux-gnu/libc.so.6\n", false},
		{"lookalike name", "55d0-55e0 r-xp 00000000 08:01 9 /home/u/notlibxul.so\n55d0-55e0 r-xp 0 08:01 9 /home/u/libxul.so.bak\n", false},
		{"anonymous mappings", "7ffd-7ffe rw-p 00000000 00:00 0 [stack]\n", false},
	}
	for _, c := range cases {
		if got := mapsBrowserEngine(c.maps); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestParseStartField(t *testing.T) {
	// A command name with spaces and parentheses must not shift the fields.
	stat := "4242 (Web Content (x)) S 1 4242 4242 0 -1 4194560 100 0 0 0 5 3 0 0 20 0 30 0 987654 1000 200"
	if got := parseStartField(stat); got != "987654" {
		t.Errorf("start = %q, want 987654", got)
	}
	if got := parseStartField("garbage"); got != "" {
		t.Errorf("garbage gave %q", got)
	}
}

func TestFirstRuleJumps(t *testing.T) {
	ours := "-P FORWARD DROP\n-A FORWARD -j KC9K\n-A FORWARD -j DOCKER-USER\n"
	if !firstRuleJumps(ours, "FORWARD") {
		t.Error("our jump first, but not recognised")
	}
	dockerFirst := "-P FORWARD DROP\n-A FORWARD -j DOCKER-USER\n-A FORWARD -j KC9K\n"
	if firstRuleJumps(dockerFirst, "FORWARD") {
		t.Error("Docker's rule above ours went unnoticed")
	}
	if firstRuleJumps("-P OUTPUT ACCEPT\n", "OUTPUT") {
		t.Error("an empty chain has no jump")
	}
	if firstRuleJumps("-A OUTPUT -j KC9K-OTHER\n", "OUTPUT") {
		t.Error("a chain with a longer name isn't ours")
	}
}

// Block unsupported browsers and the extension guard (§4.10): an AppImage's
// files sit on FUSE, shown as root's and closed to root itself, so a browser
// run from one (a portable LibreWolf, or one repacked under another name) is
// the user's, judged by its engine and never a supported browser.
func TestUserPlacedExe(t *testing.T) {
	const ext4 = 0xef53
	cases := []struct {
		name   string
		uid    uint32
		fsType int64
		err    error
		want   bool
	}{
		{"installed by root", 0, ext4, nil, false},
		{"user's copy", 1000, ext4, nil, true},
		{"AppImage, root's on FUSE", 0, fuseMagic, nil, true},
		{"AppImage, root refused", 0, 0, &fs.PathError{Op: "stat", Path: "/proc/1/exe", Err: syscall.EACCES}, true},
		{"process gone", 0, 0, &fs.PathError{Op: "stat", Path: "/proc/1/exe", Err: syscall.ENOENT}, false},
	}
	for _, c := range cases {
		if got := userPlacedExe(c.uid, c.fsType, c.err); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// B3: an Electron app shipped unpacked holds no asar; its renderer's
// --app-path switch shows it isn't a browser. Chromium's renderers have none.
func TestElectronRendererCmdline(t *testing.T) {
	cases := map[string]bool{
		"/opt/el2/b3unpacked\x00--type=renderer\x00--user-data-dir=/home/t/.config/b3\x00--app-path=/opt/el2/resources/app\x00": true,
		"/opt/chrome/chrome\x00--type=renderer\x00--lang=en-US\x00":                                                             false,
		"/opt/el2/b3unpacked\x00--type=gpu-process\x00--app-path=/x\x00":                                                        false,
		"/opt/el2/b3unpacked\x00--type=renderer-ish\x00--app-path=/x\x00":                                                       false,
		// as Chromium leaves it after rewriting the process title (seen in the VM)
		"/home/tester/b3el/dist/b3unpacked --type=renderer --user-data-dir=/home/tester/.config/b3unpacked --app-path=/home/tester/b3el/dist/resources/app --enable-sandbox\x00": true,
		"/opt/chrome/chrome --type=renderer --lang=en-US\x00": false,
	}
	for cmd, want := range cases {
		if got := electronRendererCmdline(cmd); got != want {
			t.Errorf("%q: got %v, want %v", cmd, got, want)
		}
	}
}
