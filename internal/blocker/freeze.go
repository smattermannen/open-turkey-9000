package blocker

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

// KernelCmdline returns the kernel command line, or "" if it can't be read.
func KernelCmdline() string {
	data, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// LockSessions asks logind to lock every session. Repeating it on a locked
// session is harmless, so the daemon calls it every cycle during a freeze.
func LockSessions() error {
	err := exec.Command("loginctl", "lock-sessions").Run()
	// Cinnamon doesn't react to logind's lock signal, so ask its screensaver
	// directly, as each logged-in user.
	cinnamonErr := lockCinnamon()
	if cinnamonErr != nil {
		return fmt.Errorf("locking the screen: %w", cinnamonErr)
	}
	return err
}

func lockCinnamon() error {
	path, err := exec.LookPath("cinnamon-screensaver-command")
	if err != nil {
		return nil // another desktop: logind's signal covers it
	}
	var firstErr error
	for _, u := range desktopUsers() {
		cmd := asUser(u, path, "--lock")
		cmd.Env = append(cmd.Env, "HOME="+u.HomeDir)
		if err := cmd.Run(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// desktopUsers lists the users (uid >= 1000) with a running session bus, read
// from /run/user so the fast relock loop starts no process just to find them.
func desktopUsers() []*user.User {
	entries, err := os.ReadDir("/run/user")
	if err != nil {
		return nil
	}
	var users []*user.User
	for _, e := range entries {
		uid, err := strconv.ParseUint(e.Name(), 10, 32)
		if err != nil || uid < 1000 {
			continue
		}
		if _, err := os.Stat("/run/user/" + e.Name() + "/bus"); err != nil {
			continue
		}
		if u, err := user.LookupId(e.Name()); err == nil {
			users = append(users, u)
		}
	}
	return users
}

// asUser runs a command as u on u's session bus and X display.
func asUser(u *user.User, name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Env = []string{
		"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/" + u.Uid + "/bus",
		"DISPLAY=:0",
		"XAUTHORITY=" + u.HomeDir + "/.Xauthority",
	}
	uid, _ := strconv.ParseUint(u.Uid, 10, 32)
	gid, _ := strconv.ParseUint(u.Gid, 10, 32)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}}
	return cmd
}

// RelockUnlocked locks again, right away, every Cinnamon screen that was
// unlocked. The daemon calls it twice a second during a lock freeze, so an
// unlock leaves the desktop usable for well under a second instead of up to a
// full enforcement cycle. Desktops without Cinnamon's screensaver are left to
// LockSessions.
func RelockUnlocked() {
	gdbus, err := exec.LookPath("gdbus")
	if err != nil {
		return
	}
	const dest, path, iface = "org.cinnamon.ScreenSaver", "/org/cinnamon/ScreenSaver", "org.cinnamon.ScreenSaver"
	for _, u := range desktopUsers() {
		out, err := asUser(u, gdbus, "call", "--session", "--timeout", "1", "--dest", dest,
			"--object-path", path, "--method", iface+".GetActive").Output()
		if err != nil || strings.Contains(string(out), "true") {
			continue // no Cinnamon screensaver here, or already locked
		}
		_ = asUser(u, gdbus, "call", "--session", "--timeout", "1", "--dest", dest,
			"--object-path", path, "--method", iface+".Lock", "").Run()
	}
}

// NotifyDesktop shows a notification in every desktop session, as its user.
// It is best-effort: without notify-send or a session, nothing happens.
func NotifyDesktop(title, body string) {
	path, err := exec.LookPath("notify-send")
	if err != nil {
		return
	}
	for _, u := range desktopUsers() {
		// "--" so a title starting with "-" (a block name) isn't an option.
		cmd := asUser(u, path, "--app-name=Killer Chicken 9000", "--icon=dialog-warning", "--", title, body)
		if err := cmd.Start(); err == nil {
			go cmd.Wait() //nolint:errcheck // reap it; the result doesn't matter
		}
	}
}
