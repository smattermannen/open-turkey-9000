//go:build linux

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
	// One connection throughout: Xvfb starts afresh (idle time too) when its
	// last client leaves.
	c, err := openUserDisplay(display, xauth, fileOwner(t, xauth))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	idle := func() uint64 {
		_ = c.conn.SetDeadline(time.Now().Add(x11Timeout))
		ms, err := c.idleMillis()
		if err != nil {
			t.Fatal(err)
		}
		return ms
	}
	before := idle()
	time.Sleep(1500 * time.Millisecond)
	after := idle()
	if after < before+1000 {
		t.Fatalf("idle went from %d to %d ms in 1.5 s without input", before, after)
	}
	if _, err := exec.LookPath("xdotool"); err != nil {
		t.Skip("xdotool not installed: input not tried")
	}
	move := startClient(t, display, xauth, "xdotool", "mousemove", "40", "40")
	move.Wait()
	if ms := idle(); ms >= after {
		t.Errorf("idle %d ms right after input (was %d)", ms, after)
	}
}

// §3.7 Use: the X server's idle time decides who is away (cinnamon-
// screensaver 6 doesn't answer GetSessionIdleTime, and the session bus is
// the user's own); when nothing can tell, the person counts as there.
func TestAwayFromDesktopUsesX11Idle(t *testing.T) {
	saved, savedLocked := x11IdleSeconds, screenLocked
	t.Cleanup(func() { x11IdleSeconds, screenLocked = saved, savedLocked })
	screenLocked = func(string) bool { return false }
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
