package cli

// forwardIfNeeded: on Linux the sudo wrapper runs kc9k-bin as root, so
// nothing is forwarded (see windows_windows.go).
func forwardIfNeeded() (int, bool) { return 0, false }
