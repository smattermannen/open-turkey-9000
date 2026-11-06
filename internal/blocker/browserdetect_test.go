package blocker

import "testing"

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
