package cli

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
)

// W31 (§4.1 ladder positions): gui-state gives the global ladder's step
// and each block's, 0-based, so the Overview can say where each stands.
func TestGuiStateNepsisLevel(t *testing.T) {
	d := testDB(t)
	mustRun(t, "nepsis", "set", "--step", "3 in 10m: warning", "--step", "3 in 10m: close-browser",
		"--quiet", "2h", "--no-confirm-ok")
	if state := capture(t, "gui-state"); !strings.Contains(state, `"nepsis_level":0`) {
		t.Fatalf("no global ladder level: %.300s", state)
	}
	// The ladder stands on its second step (as the daemon stores it after firing).
	now := time.Now().Unix()
	if err := d.SetSetting(db.NepsisLadderKey(""), fmt.Sprintf("1 %d %d 0", now, now)); err != nil {
		t.Fatal(err)
	}
	if state := capture(t, "gui-state"); !strings.Contains(state, `"nepsis_level":1`) {
		t.Errorf("the global ladder's step 2 isn't in gui-state")
	}
}
