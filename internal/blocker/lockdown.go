package blocker

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Lockdown takes root away from the desktop user while a block is locked. sudo
// and pkexec cover every terminal, including text consoles, so there is no need
// to block terminal programs; a root password is kept out of login and su by
// the PAM guard (pamguard.go). Root-equivalent group sockets (docker, lxd,
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
	if err := ApplyPAMGuard(); err != nil {
		return err
	}
	// Either failing leaves the other done.
	return errors.Join(restrictUserNamespaces(), LockPrivilegedSockets())
}

// RemoveLockdown restores normal administrator rights.
func RemoveLockdown() error {
	for _, path := range []string{lockdownSudoers, lockdownPolkit} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := RemovePAMGuard(); err != nil {
		return err
	}
	return errors.Join(restoreUserNamespaces(), UnlockPrivilegedSockets())
}

// LockdownInstalled reports whether any part of the lockdown is in place.
func LockdownInstalled() bool {
	for _, path := range []string{lockdownSudoers, lockdownPolkit} {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return PAMGuardInstalled()
}

// A user namespace lets a user mount a fresh tmpfs, which allows running
// programs, and run anything copied there: `unshare -Urm` gets round the
// noexec folders (B3). Ubuntu 24.04 stops it with AppArmor's
// apparmor_restrict_unprivileged_userns (namespaces of programs without a
// profile get no capabilities; browsers, Flatpak and the like have
// profiles); Linux Mint 22 turns that off. During the lockdown it is on, and
// the earlier value comes back after. A kernel without the setting is left
// as it is.
var (
	usernsRestrictPath = "/proc/sys/kernel/apparmor_restrict_unprivileged_userns"
	usernsStatePath    = "/run/kc9k-userns-restrict"
)

// restrictUserNamespaces turns the restriction on, first remembering the
// value it had (once: a later call doesn't overwrite it).
func restrictUserNamespaces() error {
	data, err := os.ReadFile(usernsRestrictPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	value := strings.TrimSpace(string(data))
	if value == "1" {
		return nil
	}
	if _, err := os.Stat(usernsStatePath); os.IsNotExist(err) {
		if err := atomicWriteFile(usernsStatePath, []byte(value+"\n"), 0o600); err != nil {
			return err
		}
	}
	if err := os.WriteFile(usernsRestrictPath, []byte("1\n"), 0o644); err != nil {
		return fmt.Errorf("restricting user namespaces: %w", err)
	}
	return nil
}

// restoreUserNamespaces puts back the value restrictUserNamespaces found.
func restoreUserNamespaces() error {
	data, err := os.ReadFile(usernsStatePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if value := strings.TrimSpace(string(data)); value == "0" {
		if err := os.WriteFile(usernsRestrictPath, []byte(value+"\n"), 0o644); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("restoring user namespaces: %w", err)
		}
	}
	return os.Remove(usernsStatePath)
}

// LogoutUsers ends the sessions of every person (personUser), guests too.
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
		if len(fields) < 2 {
			continue
		}
		if uid, err := strconv.ParseUint(fields[0], 10, 32); err != nil || !personUser(uid) {
			continue // a system account, not a person
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
