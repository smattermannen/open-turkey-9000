package cli

import (
	"strings"
	"testing"
)

// §4.3 (2026-11-21): an opened block is a draft for its Breaks & allowance
// and Bypass prevention tabs too. Their commands on the draft are kept, the
// last of each kind, shown in gui-state, and run at Save edits through
// their own gates: free ones apply, weakening ones on a locked block are
// refused and named. Cancel drops them.
func TestDraftKeepsBreaksAndBypass(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "block", "draft", "b")
	draft := "b (draft)"
	mustRun(t, "allowance", "set", draft, "2h")
	mustRun(t, "allowance", "set", draft, "1h", "--per", "week") // replaces the one before
	mustRun(t, "pomodoro", "set", draft, "25m", "5m")
	mustRun(t, "pomodoro", "off", draft)
	mustRun(t, "bypass", "set", "--count", "3", "--within", "10m", "--then", "warning", "--block", draft)
	kept := guiStateWith(t).Blocks
	var shown []string
	for _, b := range kept {
		if b.Name == "b" {
			shown = b.DraftKept
		}
	}
	if len(shown) != 3 || shown[0] != "Allowance: 1h, per week" || shown[1] != "Pomodoro: off" ||
		!strings.HasPrefix(shown[2], "Bypass prevention: ") {
		t.Errorf("gui-state draft_kept: %q", shown)
	}
	if q, _ := d.GetQuota("b"); q != nil {
		t.Fatalf("applied before save: %+v", q)
	}
	mustRun(t, "block", "draft", "b", "save")
	if q, _ := d.GetQuota("b"); q == nil || q.Limit != 3600 || q.Period != "week" {
		t.Errorf("allowance not saved: %+v", q)
	}
	if n, _ := d.BlockNepsis("b"); len(n.Steps) == 0 {
		t.Errorf("bypass prevention not saved: %+v", n)
	}
	if p, _ := d.GetPomodoro("b"); p != nil {
		t.Errorf("Pomodoro off kept as on: %+v", p)
	}

	// Locked: a longer allowance weakens and is refused at save, named; a
	// shorter one is free.
	mustRun(t, "stop", "b")
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	mustRun(t, "block", "draft", "b")
	mustRun(t, "allowance", "set", draft, "5h", "--per", "week")
	mustRun(t, "break", "reward", draft, "10m", "--per", "1h")
	err := runCLI(t, "block", "draft", "b", "save")
	if err == nil || !strings.Contains(err.Error(), "Allowance: 5h, per week") || !strings.Contains(err.Error(), "Reward") {
		t.Errorf("weakening kept settings on a locked block: %v", err)
	}
	if q, _ := d.GetQuota("b"); q == nil || q.Limit != 3600 {
		t.Errorf("a refused allowance applied: %+v", q)
	}
	mustRun(t, "block", "draft", "b")
	mustRun(t, "allowance", "set", draft, "30m", "--per", "week")
	mustRun(t, "block", "draft", "b", "save")
	if q, _ := d.GetQuota("b"); q == nil || (q.Limit != 1800 && q.PendingLimit != 1800) {
		t.Errorf("a shorter allowance wasn't saved: %+v", q)
	}
	// Cancel drops what was kept.
	mustRun(t, "block", "draft", "b")
	mustRun(t, "allowance", "remove", draft)
	mustRun(t, "block", "draft", "b", "cancel")
	if q, _ := d.GetQuota("b"); q == nil {
		t.Error("a cancelled draft removed the allowance")
	}
	if v, _ := d.GetSetting(settingDraftQueue + draft); v != "" {
		t.Errorf("kept edits left after cancel: %s", v)
	}
}
