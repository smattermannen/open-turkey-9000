package enforce

import (
	"os"
	"syscall"
)

const applyLockPath = "/run/open-turkey.lock"

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
