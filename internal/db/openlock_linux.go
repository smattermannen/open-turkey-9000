package db

import (
	"os"
	"syscall"
)

// tryLockFile takes an exclusive lock on file without waiting; busy
// reports that another process holds it.
func tryLockFile(file *os.File) (busy bool, err error) {
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	return err == syscall.EWOULDBLOCK, err
}
