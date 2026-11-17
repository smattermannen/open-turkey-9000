package blocker

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
)

// RunCustom carries out a custom action (§3.9, R-CUSTOM-1; owner
// 2026-11-17) in the desktop session of the signed-in user, as that user,
// never as root: each of opens is opened with xdg-open (an address, a file
// or a folder; ~/ is the user's home), each of runs is run with sh -c (a
// program or a command). Nothing is waited for. It reports whether a
// session was found.
func RunCustom(opens, runs []string) bool {
	users := desktopUsers()
	if len(users) == 0 {
		return false
	}
	u := users[0]
	for _, target := range opens {
		if strings.HasPrefix(target, "~/") {
			target = filepath.Join(u.HomeDir, target[2:])
		}
		startAsUser(u, "xdg-open", target)
	}
	for _, command := range runs {
		startAsUser(u, "/bin/sh", "-c", command)
	}
	return true
}

// startAsUser starts a program as u in u's desktop session (its display,
// session bus, home and a usual PATH), detached, without waiting.
func startAsUser(u *user.User, name string, args ...string) {
	cmd := asUser(u, name, args...)
	cmd.Env = append(cmd.Env,
		"HOME="+u.HomeDir, "USER="+u.Username, "LOGNAME="+u.Username,
		"XDG_RUNTIME_DIR=/run/user/"+u.Uid,
		"PATH=/usr/local/bin:/usr/bin:/bin:/usr/local/games:/usr/games:"+filepath.Join(u.HomeDir, ".local/bin"),
		"LANG="+os.Getenv("LANG"))
	cmd.Dir = u.HomeDir
	cmd.SysProcAttr.Setsid = true
	if path, err := exec.LookPath(name); err == nil {
		cmd.Path = path
	}
	if err := cmd.Start(); err == nil {
		go cmd.Wait() //nolint:errcheck // reap it
	}
}

var _ = syscall.Setsid // documents that startAsUser detaches with Setsid

// SessionUsers returns the users signed in to a desktop session.
func SessionUsers() []string {
	var out []string
	for _, u := range desktopUsers() {
		out = append(out, u.Username)
	}
	return out
}
