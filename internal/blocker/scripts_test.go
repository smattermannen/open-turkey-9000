package blocker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScriptFromArgs(t *testing.T) {
	cwd := func() string { return "/home/u/games" }
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"python3", "/usr/bin/lutris"}, "/usr/bin/lutris"},
		{[]string{"python3", "-u", "play.py", "--fast"}, "/home/u/games/play.py"},
		{[]string{"python3", "./play.py"}, "/home/u/games/play.py"},
		{[]string{"python3", "-W", "ignore", "-m", "pygame.examples.aliens"}, "pygame.examples.aliens"},
		{[]string{"python3", "-c", "print(1)"}, ""},
		{[]string{"python3"}, ""},
	}
	for _, c := range cases {
		if got := scriptFromArgs(c.args, cwd); got != c.want {
			t.Errorf("%v: %q; want %q", c.args, got, c.want)
		}
	}
	if systemScript("/home/u/games/play.py") || systemScript("relative.py") {
		t.Error("a script outside the system folders counts as the system's")
	}
}

func TestHashEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "game")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	entry, ok := HashEntry(path)
	if !ok {
		t.Fatal("no entry for a regular file")
	}
	sum, size, ok := parseHashEntry(entry)
	if !ok || len(sum) != 64 || size != 17 || !HashEntryFor(entry, path) || HashEntryFor(entry, path+"x") {
		t.Errorf("entry %q: sum %q size %d", entry, sum, size)
	}
	if _, ok := HashEntry(dir); ok {
		t.Error("a folder got a contents entry")
	}
	// This test's own program is found by its contents, under any name.
	self, _ := os.Executable()
	own, ok := HashEntry(self)
	if !ok {
		t.Skip("can't read the test binary")
	}
	if !matchesHash(os.Getpid(), []string{"other", own}) {
		t.Error("a process wasn't matched by its executable's contents")
	}
	if matchesHash(os.Getpid(), []string{entry}) || matchesHash(os.Getpid(), []string{"plainname"}) {
		t.Error("a process matched contents that aren't its own")
	}
}

func TestToolNaming(t *testing.T) {
	domains := []string{"youtube.com", "reddit.com"}
	cases := []struct {
		comm, exe, script string
		args              []string
		want              string
	}{
		{"mpv", "/usr/bin/mpv", "", []string{"mpv", "https://www.youtube.com/watch?v=x"}, "mpv"},
		{"yt-dlp", "/usr/bin/python3.12", "/usr/bin/yt-dlp", []string{"python3", "/usr/bin/yt-dlp", "youtube.com/watch?v=x"}, "yt-dlp"},
		{"curl", "/usr/bin/curl", "", []string{"curl", "-s", "https://old.reddit.com:443/r/x"}, "curl"},
		{"curl", "/usr/bin/curl", "", []string{"curl", "https://example.org/"}, ""},
		{"mpv", "/usr/bin/mpv", "", []string{"mpv", "/home/u/notyoutube.com.mkv"}, ""},
		{"mpv", "/usr/bin/mpv", "", []string{"mpv", "/home/u/myyoutube.computer.mp4"}, ""},
		// Editors and anything else are never touched.
		{"nano", "/usr/bin/nano", "", []string{"nano", "youtube.com"}, ""},
		{"bash", "/usr/bin/bash", "", []string{"bash", "-c", "echo reddit.com"}, ""},
	}
	for _, c := range cases {
		if got := toolNaming(c.comm, c.exe, c.script, c.args, domains); got != c.want {
			t.Errorf("%v: %q; want %q", c.args, got, c.want)
		}
	}
}
