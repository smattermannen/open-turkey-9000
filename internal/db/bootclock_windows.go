package db

// bootclock_windows.go — this boot's ID and its boot clock on Windows.
// The ID is the kernel's boot counter (BootId, which Windows raises at
// every boot); the clock is GetTickCount64, which counts sleep and
// hibernation as CLOCK_BOOTTIME does on Linux.

import (
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// bootIDPath is empty on Windows: the ID comes from the registry. Tests
// set it to a file.
var (
	bootIDPath = ""
	bootClock  = func() (int64, error) {
		return int64(windows.DurationSinceBoot().Seconds()), nil
	}
)

func platformBootID() ([]byte, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Control\Session Manager\Memory Management\PrefetchParameters`, registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("failed to read the boot ID: %w", err)
	}
	defer k.Close()
	n, _, err := k.GetIntegerValue("BootId")
	if err != nil {
		return nil, fmt.Errorf("failed to read the boot ID: %w", err)
	}
	return []byte(fmt.Sprintf("windows-boot-%d", n)), nil
}
