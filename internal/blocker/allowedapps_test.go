package blocker

import "testing"

// R-FREEZE-1: only allowed programs and system processes keep running.
func TestAllowedAppKeep(t *testing.T) {
	for _, c := range []struct {
		exe              string
		allowed, ancestr bool
		keep             bool
	}{
		{"/usr/bin/cinnamon", false, false, true},
		{"/usr/libexec/gvfsd", false, false, true},
		{"/usr/lib/xorg/Xorg", false, false, true},
		{"/usr/bin/pipewire", false, false, true},
		{"/usr/local/bin/kc9k-bin", false, false, true},
		{"/usr/bin/steam", false, false, false},
		{"/home/me/game.AppImage", false, false, false},
		{"/usr/bin/libreoffice", true, false, true},
		{"/usr/lib/libreoffice/program/soffice.bin", false, true, true},
		{"/usr/bin/gnome-terminal-server", false, false, false},
		{"/usr/lib/firefox/firefox", false, false, false},
		{"/usr/lib/chromium/chromium", false, false, false},
		{"/usr/lib/libreoffice/program/soffice.bin", false, false, false},
	} {
		if got := allowedAppKeep(c.exe, c.allowed, c.ancestr); got != c.keep {
			t.Errorf("%s: keep = %v; want %v", c.exe, got, c.keep)
		}
	}
}
