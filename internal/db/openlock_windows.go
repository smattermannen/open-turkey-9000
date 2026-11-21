package db

import (
	"os"

	"golang.org/x/sys/windows"
)

// tryLockFile takes an exclusive lock on file without waiting; busy
// reports that another process holds it. The lock goes with the handle.
func tryLockFile(file *os.File) (busy bool, err error) {
	var ol windows.Overlapped
	err = windows.LockFileEx(windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol)
	return err == windows.ERROR_LOCK_VIOLATION, err
}
