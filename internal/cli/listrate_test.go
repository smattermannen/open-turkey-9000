package cli

import "testing"

// §3.9 rate: setting or raising it weakens (refused while locked); lowering
// it or off strengthens; it reaches the rules and a duplicate.
func TestListRate(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "list", "create", "b", "Words", "--text")
	mustRun(t, "list", "add", "b", "Words", "casino")
	mustRun(t, "list", "rate", "b", "Words", "block", "2.5")
	rate := func() int {
		l := list(t, d, "b", "Words")
		a, _ := l.Action("block")
		return a.Rate
	}
	if rate() != 25 {
		t.Fatalf("rate = %d; want 25", rate())
	}
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runCLI(t, "list", "rate", "b", "Words", "block", "5"); err == nil {
		t.Error("raised the rate on a locked block")
	}
	mustRun(t, "list", "rate", "b", "Words", "block", "1")
	if rate() != 10 {
		t.Errorf("lowered rate = %d", rate())
	}
	mustRun(t, "list", "rate", "b", "Words", "block", "off")
	if rate() != 0 {
		t.Errorf("rate off = %d", rate())
	}
	if err := runCLI(t, "list", "rate", "b", "Words", "block", "0.5"); err == nil {
		t.Error("set a rate on a locked block")
	}
	if err := runCLI(t, "list", "rate", "b", "Words", "block", "200"); err == nil {
		t.Error("a rate over 100 % accepted")
	}
}
