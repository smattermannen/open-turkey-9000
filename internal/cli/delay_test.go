package cli

import (
	"fmt"
	"testing"
	"time"
)

// R-LOCK-1, R-LOCK-10: a block with allow windows is free to weaken without
// a lock (nothing is queued any more); its schedule lock keeps it outside
// its allow windows.
func TestAllowWindowsAndScheduleLock(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add-app", "b", "steam", "lutris")
	// An allow window at least five hours from now, so the block is on now.
	h := (time.Now().Hour() + 6) % 24
	mustRun(t, "schedule", "add", "b", "--allow", "--days", "daily", "--from", fmt.Sprintf("%02d:00", h), "--to", fmt.Sprintf("%02d:55", h))
	mustRun(t, "block", "remove-app", "b", "steam")
	if pending, _ := d.ListPendingEdits(); len(pending) != 0 {
		t.Fatalf("%d edits queued; want none", len(pending))
	}
	if detail, _ := d.GetBlock("b"); len(detail.Apps) != 1 {
		t.Fatalf("apps = %v; the removal should apply at once", detail.Apps)
	}

	mustRun(t, "schedule", "lock", "b", "on", "--lead", "1h")
	if on, lead, _ := d.ScheduleLock("b"); !on || lead != 60 {
		t.Fatalf("schedule lock = %v %d", on, lead)
	}
	if err := runCLI(t, "list", "remove", "b", "Exceptions", "file://*"); err != nil {
		t.Errorf("removing an exception (a strengthening) was refused: %v", err)
	}
	if err := runCLI(t, "schedule", "lock", "b", "on", "--lead", "10m"); err == nil {
		t.Error("the lead time was shortened while locked")
	}
	if err := runCLI(t, "schedule", "lock", "b", "off"); err == nil {
		t.Error("the schedule lock was taken off while locked")
	}
	if err := runCLI(t, "schedule", "remove", "b", "1"); err == nil {
		t.Error("the last allow window was removed while locked")
	}
	mustRun(t, "schedule", "lock", "b", "on", "--lead", "2h")
	if _, lead, _ := d.ScheduleLock("b"); lead != 120 {
		t.Errorf("a longer lead time didn't apply: %d", lead)
	}
}
