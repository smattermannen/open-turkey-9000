package cli

import (
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
)

// §3.9 Mode action and custom action (owner 2026-11-17): every trigger takes
// them: a list's and a phrase set's actions, the apps' and window titles',
// a bypass prevention step's. A Mode action (it locks by itself) needs
// --no-confirm-ok when it is set up (R-LOCK-12).
func TestModeAndCustomActions(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	if err := runCLI(t, "list", "action", "b", "Blocked", "mode", "on", "--target", "Work", "--for", "1h"); err == nil {
		t.Error("a Mode action set up without --no-confirm-ok")
	}
	if err := runCLI(t, "list", "action", "b", "Blocked", "mode", "on", "--for", "1h", "--no-confirm-ok"); err == nil {
		t.Error("a Mode action without its mode")
	}
	mustRun(t, "list", "action", "b", "Blocked", "mode", "on", "--target", "Work", "--for", "1h", "--no-confirm-ok")
	mustRun(t, "list", "action", "b", "Blocked", "custom", "on", "--custom", "open https://example.org/study", "--custom", "run notify-send hi")
	if err := runCLI(t, "list", "action", "b", "Blocked", "custom", "on", "--custom", "jump somewhere"); err == nil {
		t.Error("a custom action that is neither open nor run")
	}
	detail, _ := d.GetBlock("b")
	l, _ := detail.Lists.Find(db.DefaultBlockedList)
	mode, _ := l.Action(db.ActionMode)
	custom, _ := l.Action(db.ActionCustom)
	if mode.Param != "Work" || mode.Minutes != 60 || db.DescribeCustom(custom.Param) != "open https://example.org/study; run notify-send hi" {
		t.Errorf("list actions: mode %+v, custom %q", mode, custom.Param)
	}
	mustRun(t, "phrase-set", "create", "b", "P", "--text")
	mustRun(t, "phrase-set", "action", "b", "P", "custom", "on", "--custom", "open ~/Notes")
	mustRun(t, "block", "trigger", "b", "apps", "custom", "on", "--custom", "open ~/Notes")
	mustRun(t, "block", "trigger", "b", "titles", "mode", "on", "--target", "Work", "--for", "30m", "--no-confirm-ok")
	apps, _ := d.TriggerActions("b", db.TriggerApps)
	titles, _ := d.TriggerActions("b", db.TriggerTitles)
	if len(apps) != 2 || apps[1].Name != db.ConsequenceCustom || len(titles) != 2 || titles[1].Param != "Work" || titles[1].Minutes != 30 {
		t.Errorf("trigger actions: apps %+v, titles %+v", apps, titles)
	}
	mustRun(t, "bypass", "set", "--block", "b", "--no-confirm-ok",
		"--step", `2 in 5m: warning, mode Evening Work 1h, custom "open https://example.org/a,b" "run echo hi"`)
	n, _ := d.BlockNepsis("b")
	if len(n.Steps) != 1 || len(n.Steps[0].Actions) != 3 {
		t.Fatalf("ladder: %+v", n)
	}
	a := n.Steps[0].Actions
	if a[1].Param != "Evening Work" || a[1].Minutes != 60 || db.DescribeCustom(a[2].Param) != "open https://example.org/a,b; run echo hi" {
		t.Errorf("ladder actions: %+v", a)
	}
	if out := describeNepsis(n); !strings.Contains(out, "mode Evening Work for 1h0m0s") || !strings.Contains(out, "custom: open") {
		t.Errorf("described: %s", out)
	}
}

// §3.9 Custom action (owner 2026-11-17): a mode opens or runs things when it
// becomes enabled; gui-state lists them.
func TestModeCustom(t *testing.T) {
	d := testDB(t)
	mustRun(t, "mode", "create", "Study")
	mustRun(t, "mode", "custom", "Study", "add", "open https://example.org/notes")
	mustRun(t, "mode", "custom", "Study", "add", "run libreoffice --writer")
	if err := runCLI(t, "mode", "custom", "Study", "add", "fly away"); err == nil {
		t.Error("neither open nor run was taken")
	}
	param, _ := d.ModeCustom("Study")
	if db.DescribeCustom(param) != "open https://example.org/notes; run libreoffice --writer" {
		t.Errorf("custom: %q", param)
	}
	if state := capture(t, "gui-state"); !strings.Contains(state, `"custom":["open https://example.org/notes","run libreoffice --writer"]`) {
		t.Errorf("gui-state: %.200s", state[strings.Index(state, `"name":"Study"`):])
	}
	mustRun(t, "mode", "custom", "Study", "remove", "1")
	mustRun(t, "mode", "custom", "monk", "add", "open ~/Music")
	mustRun(t, "mode", "custom", "Study", "clear")
	if param, _ := d.ModeCustom("Study"); param != "" {
		t.Errorf("cleared, yet %q", param)
	}
}
