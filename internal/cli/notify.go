// notify.go — best-effort desktop notifications from the command line.
package cli

import "github.com/smattermannen/kc9k/internal/blocker"

// notify shows a desktop notification in every desktop session, as its user
// (see blocker.NotifyDesktop). It is best-effort and never fails a command.
// It used to run "sudo -u <user> notify-send" from this root process, which
// strict mode's sudo rules refuse, with its own user lookup.
func notify(title, body string) {
	blocker.NotifyDesktop(title, body)
}
