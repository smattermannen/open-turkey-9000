package blocker

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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
	if output, err := exec.Command("loginctl", "lock-sessions").CombinedOutput(); err != nil {
		return fmt.Errorf("loginctl lock-sessions: %s (error: %w)", strings.TrimSpace(string(output)), err)
	}
	return nil
}
