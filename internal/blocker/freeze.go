package blocker

import (
	"fmt"
	"os"
	"os/exec"
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
	output, err := exec.Command("loginctl", "list-users", "--no-legend").Output()
	if err != nil {
		return err
	}
	var firstErr error
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		uid, err := strconv.ParseUint(fields[0], 10, 32)
		if err != nil || uid < 1000 {
			continue
		}
		cmd := exec.Command(path, "--lock")
		cmd.Env = []string{
			"DISPLAY=:0",
			"HOME=/home/" + fields[1],
			fmt.Sprintf("DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/%d/bus", uid),
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(uid)}}
		if err := cmd.Run(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
