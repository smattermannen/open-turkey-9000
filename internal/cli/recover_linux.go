package cli

import (
	"os"
	"strings"
)

const (
	recoverIntro       = "Run as root after booting the recovery menu entry."
	recoverNotRoot     = "run it as root, from recovery mode"
	recoverNotRecovery = "this is not a recovery boot: choose the recovery entry in the boot menu first"
)

// recoveryBoot reads the kernel command line.
func recoveryBoot() bool {
	data, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return false
	}
	for _, word := range strings.Fields(string(data)) {
		switch word {
		case "single", "recovery", "S", "1", "emergency", "systemd.unit=rescue.target", "systemd.unit=emergency.target":
			return true
		}
	}
	return false
}
