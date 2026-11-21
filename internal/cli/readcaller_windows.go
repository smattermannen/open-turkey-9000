package cli

import (
	"os"
	"runtime"
	"strconv"

	"github.com/smattermannen/kc9k/internal/platform"
	"github.com/smattermannen/kc9k/internal/winapi"
	"golang.org/x/sys/windows"
)

// readAsCallerUpTo reads a user-supplied path, up to limit bytes, with the
// caller's rights: a command the service runs (LocalSystem) opens the file
// while impersonating the caller's token, which the service hands down
// (platform.EnvCallerToken), as the Linux one reads as SUDO_UID. Without
// the service the process is the caller.
func readAsCallerUpTo(path string, limit int64) ([]byte, error) {
	if platform.Privileged() {
		if v := os.Getenv(platform.EnvCallerToken); v != "" {
			h, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return nil, err
			}
			runtime.LockOSThread()
			if err := winapi.ImpersonateLoggedOnUser(windows.Token(h)); err != nil {
				runtime.UnlockOSThread()
				return nil, err
			}
			f, err := os.Open(path)
			if rerr := windows.RevertToSelf(); rerr != nil {
				panic(rerr) // never go on as the caller
			}
			runtime.UnlockOSThread()
			if err != nil {
				return nil, err
			}
			defer f.Close()
			return readLimited(f, limit)
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readLimited(f, limit)
}
