package blocker

import (
	"os/exec"
	"time"
)

// ConfirmDesktop asks the user a yes/no question in a dialog on the desktop
// session and reports whether they confirmed. Run from root (a hotkey runs
// through sudo), it shows the dialog as the session's user. Without zenity or
// a session, or when nobody answers within a minute, the answer is no: a lock
// is never set unconfirmed (R-LOCK-12).
func ConfirmDesktop(title, text, okLabel string) bool {
	path, err := exec.LookPath("zenity")
	if err != nil {
		return false
	}
	users := desktopUsers()
	if len(users) == 0 {
		return false
	}
	cmd := asUser(users[0], path, "--question", "--title="+title, "--text="+text,
		"--ok-label="+okLabel, "--cancel-label=Cancel", "--no-markup", "--timeout=60")
	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return false
	}
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err == nil
	case <-time.After(70 * time.Second):
		_ = cmd.Process.Kill()
		return false
	}
}
