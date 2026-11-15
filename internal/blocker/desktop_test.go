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
