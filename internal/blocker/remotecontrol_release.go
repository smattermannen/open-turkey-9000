//go:build !devmaster

package blocker

// remoteControlAllowed reports whether browsers started for remote control
// may run during website blocks. Never, in a normal build.
func remoteControlAllowed() bool { return false }
