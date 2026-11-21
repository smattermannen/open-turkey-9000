//go:build linux

package enforce

import (
	"os"
	"syscall"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"golang.org/x/sys/unix"
)

// sleptSeconds is how long the computer has slept since boot. A variable
// for tests.
var sleptSeconds = func() int64 {
	var boot, mono unix.Timespec
	if unix.ClockGettime(unix.CLOCK_BOOTTIME, &boot) != nil || unix.ClockGettime(unix.CLOCK_MONOTONIC, &mono) != nil {
		return 0
	}
	return boot.Sec - mono.Sec
}

func bootClock() (time.Duration, bool) {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_BOOTTIME, &ts); err != nil {
		return 0, false
	}
	return time.Duration(ts.Nano()), true
}

const applyLockPath = "/run/kc9k.lock"

// applyLock makes the daemon and the CLI take turns writing the layers.
// Otherwise a daemon cycle that read the state just before a CLI change could
// overwrite what the CLI applied. It returns the unlock function. Without
// access to the lock file, as in tests, there is nothing to coordinate with.
func applyLock() func() {
	file, err := os.OpenFile(applyLockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return func() {}
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return func() {}
	}
	return func() { file.Close() }
}

// guardPlatform: Linux's protections are in enforceLockdown.
func guardPlatform(d *db.DB, trusted int64) {}

// platformTaskManagers: none beyond the list.
var platformTaskManagers []string
