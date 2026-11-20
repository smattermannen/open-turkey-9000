package db

import (
	"os"
	"syscall"
	"time"
)

// setupLockWait bounds the wait for another process's setup: one stopped
// while holding the lock must not hold every later command up for ever.
var setupLockWait = 30 * time.Second

// setupLock serialises the schema setup in OpenDB across processes: on a
// fresh install the daemon and `kc9k-bin browser install-librewolf` create
// the same new database at once (install.sh). SQLite's busy_timeout does not
// cover that: a write after a read whose snapshot another process has since
// changed fails with "database is locked" at once, and two processes may
// both find a column missing and both add it. The lock file sits next to the
// database; if it can't be opened (a read-only directory), setup runs
// unlocked as before, and so after setupLockWait. Returns the release
// function.
func setupLock(dbPath string) func() {
	file, err := os.OpenFile(dbPath+".setup-lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return func() {}
	}
	deadline := time.Now().Add(setupLockWait)
	for {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() { file.Close() }
		}
		if err != syscall.EWOULDBLOCK || time.Now().After(deadline) {
			file.Close()
			return func() {}
		}
		time.Sleep(20 * time.Millisecond)
	}
}
