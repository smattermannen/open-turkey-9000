package blocker

import "testing"

func TestTorExe(t *testing.T) {
	for path, want := range map[string]bool{
		"/home/a/.local/share/torbrowser/tbb/x86_64/tor-browser/Browser/firefox.real": true,
		"/home/a/tor-browser/Browser/TorBrowser/Tor/tor":                              true,
		"/usr/bin/tor":                    true,
		"/usr/bin/torbrowser-launcher":    true,
		"/usr/lib/firefox/firefox":        false,
		"/usr/bin/torrent-client":         false,
		"/home/a/projects/factory/editor": false,
	} {
		if got := torExe(path); got != want {
			t.Errorf("torExe(%q) = %v", path, got)
		}
	}
}
