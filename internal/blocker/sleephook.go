// sleephook.go — a systemd sleep hook (R-MODE-5): before the computer
// sleeps, `kc9k-bin wallpaper before-sleep` puts the wallpaper default on
// the desktop, so a laptop closed at one place and opened at another
// doesn't show the first place's wallpaper while it can't yet tell where
// it is. The daemon writes the hook while a mode switched by places has
// wallpapers, and removes it otherwise; uninstalling removes it.
package blocker

import (
	"bytes"
	"os"
	"path/filepath"
)

// SleepHookPath is where systemd looks for programs to run before sleep
// ("pre") and after waking ("post"). A variable for tests.
var SleepHookPath = "/usr/lib/systemd/system-sleep/kc9k"

const sleepHook = `#!/bin/sh
# Killer Chicken 9000: before the computer sleeps, the wallpaper default goes
# on the desktop, so it doesn't wake at another place showing the last
# place's wallpaper. Written by the KC9K daemon; removed when not needed.
if [ "$1" = pre ] && [ -x /usr/local/bin/kc9k-bin ]; then
	/usr/local/bin/kc9k-bin wallpaper before-sleep >/dev/null 2>&1
fi
exit 0
`

// SyncSleepHook writes the hook (on) or removes it. Without systemd's
// folder for such hooks it does nothing.
func SyncSleepHook(on bool) error {
	if !on {
		if err := os.Remove(SleepHookPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	dir := filepath.Dir(SleepHookPath)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil
	}
	if have, err := os.ReadFile(SleepHookPath); err == nil && bytes.Equal(have, []byte(sleepHook)) {
		return nil
	}
	tmp := SleepHookPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(sleepHook), 0o755); err != nil {
		return err
	}
	_ = os.Chmod(tmp, 0o755)
	return os.Rename(tmp, SleepHookPath)
}
