package enforce

import (
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
)

// R-BREAK-2: a reward is earned only while the computer is in use.
func TestRewardOnlyWhileInUse(t *testing.T) {
	d := testDB(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetBreaks("b", db.Breaks{Allowance: 600, Per: 3600}); err != nil {
		t.Fatal(err)
	}
	inUse := false
	oldUse, oldLast := computerInUse, lastEarned
	computerInUse = func() bool { return inUse }
	t.Cleanup(func() { computerInUse, lastEarned = oldUse, oldLast })

	enforced := []db.ActiveBlockDetail{{BlockName: "b"}}
	lastEarned = 0
	earnBreaks(d, enforced, 1000) // the first cycle only starts the clock
	earnBreaks(d, enforced, 1036)
	if k, _ := d.GetBreaks("b"); k.BalanceMs != 0 {
		t.Fatalf("idle computer earned %d ms", k.BalanceMs)
	}
	inUse = true
	earnBreaks(d, enforced, 1072)
	if k, _ := d.GetBreaks("b"); k.BalanceMs != 6000 {
		t.Fatalf("36 s in use earned %d ms; want 6000", k.BalanceMs)
	}
}
