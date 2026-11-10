package cli

import (
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
)

func TestLoosenDelay(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add-app", "b", "steam")
	if n, _ := d.LoosenDelay("b"); n != db.DefaultLoosenDelay {
		t.Fatalf("default delay = %d", n)
	}
	if err := runCLI(t, "pending", "delay", "b", "10m"); err == nil {
		t.Error("a delay under an hour was accepted")
	}
	// Without allow windows the block isn't queued at all: any delay sets.
	mustRun(t, "pending", "delay", "b", "48h")
	if n, _ := d.LoosenDelay("b"); n != 48*3600 {
		t.Fatalf("delay = %d", n)
	}
	// With an allow window: a loosening edit waits the block's own delay,
	// a longer delay applies at once, a shorter one is queued.
	mustRun(t, "schedule", "add", "b", "--allow", "--days", "daily", "--from", "12:00", "--to", "13:00")
	mustRun(t, "block", "remove-app", "b", "steam")
	pending, _ := d.ListPendingEdits()
	if len(pending) != 1 {
		t.Fatalf("%d pending edits; want the removal", len(pending))
	}
	if wait := time.Until(time.Unix(pending[0].ApplyAt, 0)); wait < 47*time.Hour || wait > 48*time.Hour {
		t.Errorf("the removal waits %v; want 48h", wait)
	}
	mustRun(t, "pending", "delay", "b", "72h")
	if n, _ := d.LoosenDelay("b"); n != 72*3600 {
		t.Errorf("a longer delay didn't apply at once: %d", n)
	}
	mustRun(t, "pending", "delay", "b", "2h")
	if n, _ := d.LoosenDelay("b"); n != 72*3600 {
		t.Errorf("a shorter delay applied at once: %d", n)
	}
	pending, _ = d.ListPendingEdits()
	if len(pending) != 2 || pending[1].Edit.Action != db.EditSetDelay {
		t.Fatalf("pending = %+v", pending)
	}
	if err := d.ApplyEdit("b", pending[1].Edit); err != nil {
		t.Fatal(err)
	}
	if n, _ := d.LoosenDelay("b"); n != 2*3600 {
		t.Errorf("the queued shorter delay didn't apply: %d", n)
	}
}
