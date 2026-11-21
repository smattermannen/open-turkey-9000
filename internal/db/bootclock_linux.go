package db

// bootclock_linux.go — this boot's ID and its boot clock on Linux.

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// bootIDPath and bootClock read this boot's ID and its boot clock
// (seconds, counting suspend). Variables for tests.
var (
	bootIDPath = "/proc/sys/kernel/random/boot_id"
	bootClock  = func() (int64, error) {
		var ts unix.Timespec
		if err := unix.ClockGettime(unix.CLOCK_BOOTTIME, &ts); err != nil {
			return 0, fmt.Errorf("failed to read boot clock: %w", err)
		}
		return int64(ts.Sec), nil
	}
)

func platformBootID() ([]byte, error) { return nil, errors.New("no boot ID path") }
