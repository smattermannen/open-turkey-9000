//go:build linux

package cli

import (
	"os/exec"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/daemon"
	"github.com/smattermannen/kc9k/internal/db"
)

// systemStopping reports whether the computer is shutting down or
// restarting (systemd's state "stopping"): the daemon stopping then is no
// news for the partners. Replaced in tests.
var systemStopping = func() bool {
	out, _ := exec.Command("systemctl", "is-system-running").Output()
	return strings.TrimSpace(string(out)) == "stopping"
}

// partnerDaemonStopped tells the partners the daemon was stopped by hand
// (§4.15 "KC9K stopped"), sent at once: it may not run again.
func partnerDaemonStopped(database *db.DB) {
	if systemStopping() {
		return
	}
	database.PartnerAlert("Killer Chicken 9000 was stopped on "+deviceName(), "Its daemon was stopped while the computer kept running.")
	if loadSession(database) == nil {
		return
	}
	if c, s, err := signedIn(database); err == nil {
		_, _ = partnerFlush(database, c, s, time.Now(), true)
	}
}

func init() {
	daemon.OnStop = append(daemon.OnStop, partnerDaemonStopped)
}
