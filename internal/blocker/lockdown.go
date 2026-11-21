package blocker

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Lockdown takes root away from the desktop user while a block is held. sudo
// and pkexec cover every terminal, including text consoles, so there is no need
// to block terminal programs. Root-equivalent group sockets (docker, lxd,
// libvirt) are closed too; see sockets.go. Recovery mode and booting another system still
// work; see freeze.BypassParam for the exit the daemon itself honours.
const (
	lockdownSudoers = "/etc/sudoers.d/zz-kc9k-lockdown"
	lockdownPolkit  = "/etc/polkit-1/rules.d/00-kc9k-lockdown.rules"
	binaryPath      = "/usr/local/bin/kc9k-bin"
)

// lockdownPolkitRule denies every privileged desktop action except power,
// session, network, colour (night light) and backlight ones, so people can
// still log out, join a network and change screen brightness.
const lockdownPolkitRule = `polkit.addRule(function(action, subject) {
    if (action.id.indexOf("org.freedesktop.login1.") === 0) { return polkit.Result.NOT_HANDLED; }
    if (action.id.indexOf("org.freedesktop.NetworkManager.") === 0) { return polkit.Result.NOT_HANDLED; }
    if (action.id.indexOf("org.freedesktop.color-manager.") === 0) { return polkit.Result.NOT_HANDLED; }
    if (/^org\.(cinnamon|gnome)\.settings-daemon\.plugins\.power\.backlight-helper$/.test(action.id)) { return polkit.Result.NOT_HANDLED; }
    return polkit.Result.NO;
});
`

var validUser = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)

// LockdownSudoers renders the sudoers rules. sudo applies the last match, and
// this file sorts last, so it overrides every earlier grant.
func LockdownSudoers(user string) (string, error) {
	rule := "ALL ALL=(ALL:ALL) !ALL\n%sudo ALL=(root) NOPASSWD: " + binaryPath + "\n"
	if user == "" {
		return rule, nil
	}
	if !validUser.MatchString(user) {
		return "", fmt.Errorf("invalid user name %q", user)
	}
	return rule + user + " ALL=(root) NOPASSWD: " + binaryPath + "\n", nil
}

func fileHas(path, content string) bool {
	data, err := os.ReadFile(path)
	return err == nil && string(data) == content
}

// ApplyLockdown installs the lockdown if it isn't already in place. A sudoers
// file with an error would break sudo entirely, so visudo checks it first.
func ApplyLockdown(user string) error {
	rules, err := LockdownSudoers(user)
	if err != nil {
		return err
	}
	if !fileHas(lockdownSudoers, rules) {
		check, err := os.CreateTemp("", "kc9k-sudoers-*")
		if err != nil {
			return err
		}
		defer os.Remove(check.Name())
		if _, err := check.WriteString(rules); err != nil {
			check.Close()
			return err
		}
		check.Close()
		if output, err := exec.Command("visudo", "-cf", check.Name()).CombinedOutput(); err != nil {
			return fmt.Errorf("visudo rejected the lockdown rules: %s (error: %w)", strings.TrimSpace(string(output)), err)
		}
		if err := atomicWriteFile(lockdownSudoers, []byte(rules), 0440); err != nil {
			return err
		}
	}
	if !fileHas(lockdownPolkit, lockdownPolkitRule) {
		if err := atomicWriteFile(lockdownPolkit, []byte(lockdownPolkitRule), 0644); err != nil {
			return err
		}
	}
	return LockPrivilegedSockets()
}

// RemoveLockdown restores normal administrator rights.
func RemoveLockdown() error {
	for _, path := range []string{lockdownSudoers, lockdownPolkit} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return UnlockPrivilegedSockets()
}

// LogoutUsers ends the sessions of every regular user.
func LogoutUsers() error {
	output, err := exec.Command("loginctl", "list-users", "--no-legend").Output()
	if err != nil {
		return fmt.Errorf("loginctl list-users: %w", err)
	}
	var firstErr error
	for _, name := range usersToLogOut(string(output)) {
		if err := exec.Command("loginctl", "terminate-user", name).Run(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// usersToLogOut reads `loginctl list-users --no-legend` (UID USER, and on
// newer systemd LINGER STATE) and returns the regular users to end. A user
// already "closing" is left alone: terminating it again while its sessions
// end gains nothing, and on systemd 255 (Ubuntu 24.04) a session ended twice
// over while it was still opening left logind spinning at 100% CPU, so no
// one could log in after the freeze (B2, 2026-11-20).
func usersToLogOut(listUsers string) []string {
	var names []string
	for _, line := range strings.Split(listUsers, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || len(fields[0]) < 4 {
			continue // no user, or a system account below uid 1000
		}
		if len(fields) >= 4 && fields[len(fields)-1] == "closing" {
			continue
		}
		names = append(names, fields[1])
	}
	return names
}

// PowerOff shuts the computer down.
func PowerOff() error {
	return exec.Command("systemctl", "poweroff").Run()
}
