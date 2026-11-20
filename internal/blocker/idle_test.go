package blocker

import (
	"os/exec"
	"os/user"
	"testing"
	"time"
)

// §3.7 Use: "input in the last 3 minutes". The X server tells how long it
// has had no input (MIT-SCREEN-SAVER), and new input starts that again.
func TestX11IdleMillis(t *testing.T) {
	display, xauth := startX(t)
	idle := func() uint64 {
		c, err := openUserDisplay(display, xauth, fileOwner(t, xauth))
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		ms, err := c.idleMillis()
		if err != nil {
			t.Fatal(err)
		}
		return ms
	}
	time.Sleep(1500 * time.Millisecond)
	if ms := idle(); ms < 1000 {
		t.Fatalf("idle %d ms after 1.5 s without input", ms)
	}
	if _, err := exec.LookPath("xdotool"); err != nil {
		t.Skip("xdotool not installed: input not tried")
	}
	move := startClient(t, display, xauth, "xdotool", "mousemove", "40", "40")
	move.Wait()
	if ms := idle(); ms > 1000 {
		t.Errorf("idle %d ms right after input", ms)
	}
}

// §3.7 Use: the X server's idle time decides who is away (cinnamon-
// screensaver 6 doesn't answer GetSessionIdleTime, and the session bus is
// the user's own); when nothing can tell, the person counts as there.
func TestAwayFromDesktopUsesX11Idle(t *testing.T) {
	saved := x11IdleSeconds
	t.Cleanup(func() { x11IdleSeconds = saved })
	u := &user.User{Uid: "1000", HomeDir: "/nonexistent"}
	for _, c := range []struct {
		idle uint64
		ok   bool
		away bool
	}{
		{idle: 600, ok: true, away: true},
		{idle: 30, ok: true, away: false},
		{idle: 0, ok: false, away: false},
	} {
		x11IdleSeconds = func(*user.User) (uint64, bool) { return c.idle, c.ok }
		if got := awayFromDesktop(u, "", 180); got != c.away {
			t.Errorf("idle %d s (known %v): away %v, want %v", c.idle, c.ok, got, c.away)
		}
	}
}
