//go:build linux

// idle.go — how long a desktop user has gone without input, from the X
// server itself (the MIT-SCREEN-SAVER extension's QueryInfo), over the same
// cgo-free client as window titles (x11.go). Cinnamon's screensaver used to
// tell this (GetSessionIdleTime), but cinnamon-screensaver 6 (Linux Mint 22)
// answers "not implemented": without this, idle time counted as use (§3.7).
package blocker

import (
	"encoding/binary"
	"fmt"
	"os/user"
	"path/filepath"
	"strconv"
)

// idleMillis asks the X server how many milliseconds have passed since the
// last keyboard or pointer input on the display.
func (c *x11Conn) idleMillis() (uint64, error) {
	// Xwayland sees only input meant for X11 windows: someone typing in
	// Wayland apps would look idle there.
	if _, err := c.extensionOpcode("XWAYLAND"); err == nil {
		return 0, fmt.Errorf("Xwayland can't tell the session's idle time")
	}
	op, err := c.extensionOpcode("MIT-SCREEN-SAVER")
	if err != nil {
		return 0, err
	}
	// ScreenSaverQueryInfo (minor 1) on the root window. The reply's idle
	// field (milliseconds since the last input) is at byte 16.
	query := make([]byte, 8)
	query[0], query[1] = op, 1
	binary.LittleEndian.PutUint16(query[2:], 2)
	binary.LittleEndian.PutUint32(query[4:], c.root)
	head, _, err := c.request(query, true)
	if err != nil {
		return 0, err
	}
	return uint64(binary.LittleEndian.Uint32(head[16:20])), nil
}

// x11IdleSeconds is how long u's X session on :0 has had no input; ok is
// false when it can't tell (no X session there, or no extension). A
// variable for tests.
var x11IdleSeconds = func(u *user.User) (seconds uint64, ok bool) {
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return 0, false
	}
	c, err := openUserDisplay(":0", filepath.Join(u.HomeDir, ".Xauthority"), uid)
	if err != nil {
		return 0, false
	}
	defer c.Close()
	ms, err := c.idleMillis()
	if err != nil {
		return 0, false
	}
	return ms / 1000, true
}
