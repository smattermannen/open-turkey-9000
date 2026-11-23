//go:build linux

package cli

import (
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
)

// §4.15 "KC9K stopped": the daemon stopped by hand is an alert, a
// shutdown isn't.
func TestPartnerDaemonStopped(t *testing.T) {
	d := testDB(t)
	old := systemStopping
	t.Cleanup(func() { systemStopping = old })
	if err := d.SetSetting(db.SettingPartnerActive, "on"); err != nil {
		t.Fatal(err)
	}
	systemStopping = func() bool { return true }
	partnerDaemonStopped(d)
	if out, _ := d.PartnerOutbox(); len(out) != 0 {
		t.Errorf("an alert at shutdown: %+v", out)
	}
	systemStopping = func() bool { return false }
	partnerDaemonStopped(d)
	if out, _ := d.PartnerOutbox(); len(out) != 1 || !strings.Contains(out[0].Subject, "was stopped") {
		t.Errorf("no alert for a daemon stopped by hand: %+v", out)
	}
}
