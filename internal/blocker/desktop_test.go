//go:build linux

package blocker

import (
	"os"
	"path/filepath"
	"testing"
)

// R-DESKTOP-1: warn on Wayland or a desktop other than Cinnamon.
func TestDesktopWarning(t *testing.T) {
	old := sessionInfo
	t.Cleanup(func() { sessionInfo = old })
	for _, c := range []struct {
		kind, desktop string
		warn          bool
	}{{"x11", "cinnamon", false}, {"", "", false}, {"wayland", "cinnamon", true}, {"x11", "mate", true}, {"wayland", "gnome", true}} {
		sessionInfo = func() (string, string) { return c.kind, c.desktop }
		if got := DesktopWarning(); (got != "") != c.warn {
			t.Errorf("%s/%s: %q", c.kind, c.desktop, got)
		}
	}
}

// §3.7: RunningApps finds an app that runs, whether or not it is in front.
func TestRunningApps(t *testing.T) {
	self := filepath.Base(os.Args[0])
	got := RunningApps([]string{self, "kc9k-no-such-app"})
	if len(got) != 1 || got[0] != self {
		t.Errorf("RunningApps = %v; want [%s]", got, self)
	}
}

// §3.1 folder entry: a file manager window showing the folder, by title.
func TestFolderWindow(t *testing.T) {
	folders := []string{"/home/me/games/"}
	for title, want := range map[string]string{
		"games":                   "/home/me/games/",
		"games — Dolphin":         "/home/me/games/",
		"/home/me/games - Thunar": "/home/me/games/",
		"Downloads":               "",
		"gamesaves":               "",
	} {
		if got := folderWindow(title, folders); got != want {
			t.Errorf("%q: %q; want %q", title, got, want)
		}
	}
	if fileManagers["nemo-desktop"] || !fileManagers["nemo"] {
		t.Error("the desktop's icons aren't a folder window")
	}
}
