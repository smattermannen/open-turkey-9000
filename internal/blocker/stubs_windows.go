package blocker

// stubs_windows.go — every layer function enforce and the CLI call that is
// not built on Windows yet. Each says so: an error names the ROADMAP item
// that builds it; a query answers "nothing there". An item that builds a
// function moves it out of this file into its own _windows.go.

import (
	"fmt"
)

// errNotBuilt is the error of a layer not built on Windows yet.
func errNotBuilt(item string) error {
	return fmt.Errorf("not built on Windows yet (ROADMAP %s)", item)
}

// RemoteControlAllowed: browsers started for remote control never run
// during website blocks on Windows (no testing build there yet).
func RemoteControlAllowed() bool { return false }

// ApplyFirmwareLock is not built on Windows yet (ROADMAP W5).
func ApplyFirmwareLock(password string) (map[string]string, error) {
	return nil, errNotBuilt("W5")
}

// FirmwareLockStatus is not built on Windows yet (ROADMAP W5).
func FirmwareLockStatus() (FirmwareStatus, error) {
	return FirmwareStatus{}, errNotBuilt("W5")
}

// RemoveFirmwareLock is not built on Windows yet (ROADMAP W5).
func RemoveFirmwareLock(password string, before map[string]string) error {
	return errNotBuilt("W5")
}

// FirmwareRoot: the firmware lock is not built on Windows yet (ROADMAP W5).
var FirmwareRoot = ""
