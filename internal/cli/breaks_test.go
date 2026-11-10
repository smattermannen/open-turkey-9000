package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

func TestBreakAllowance(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	if err := runCLI(t, "break", "allowance", "b", "70m", "--per", "1h"); err == nil {
		t.Error("a break longer than the time it is earned over was accepted")
	}
	mustRun(t, "break", "allowance", "b", "10m", "--per", "1h")
	k, _ := d.GetBreaks("b")
	if k.Allowance != 600 || k.Per != 3600 || k.BalanceMs != 0 {
		t.Fatalf("breaks = %+v", k)
	}
	// Half an hour of blocking earns five minutes; it stops at one allowance.
	if got := k.Earn(1800).Balance(); got != 300 {
		t.Errorf("earned %d s in half an hour; want 300", got)
	}
	if got := k.Earn(100000).Balance(); got != 600 {
		t.Errorf("earned %d s; want it capped at 600", got)
	}

	if err := d.ActivateBlock("b", true, 0, 0, false); err != nil { // locked
		t.Fatal(err)
	}
	if err := runCLI(t, "break", "start", "b"); err == nil || !strings.Contains(err.Error(), "less than a minute") {
		t.Errorf("a break with nothing earned: %v", err)
	}
	if err := d.SetBreaks("b", k.Earn(1800)); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "break", "start", "b", "8m"); err == nil {
		t.Error("more break than earned was given")
	}
	// A locked block can take its break; the block stays active and locked.
	mustRun(t, "break", "start", "b", "3m")
	trusted, _ := d.TrustedNow(time.Now())
	until, _ := d.PausedUntil("b")
	if until-trusted < 170 || until-trusted > 180 {
		t.Errorf("paused for %d s; want 180", until-trusted)
	}
	if k, _ = d.GetBreaks("b"); k.Balance() != 120 {
		t.Errorf("balance after taking 3 of 5 minutes = %d s", k.Balance())
	}
	detail, _ := d.GetBlock("b")
	if !detail.Active || !detail.Locked {
		t.Error("the break ended or unlocked the block")
	}
	if rules, _ := enforce.BuildRules(d); len(rules.Lists) != 0 {
		t.Error("the block is enforced during its break")
	}
	if err := runCLI(t, "break", "start", "b"); err == nil {
		t.Error("a second break started during the first")
	}
	// Ending early gives the rest back.
	mustRun(t, "break", "end", "b")
	if k, _ = d.GetBreaks("b"); k.Balance() < 295 || k.Balance() > 300 {
		t.Errorf("balance after ending early = %d s; want about 300", k.Balance())
	}
	if rules, _ := enforce.BuildRules(d); len(rules.Lists) != 1 {
		t.Error("the block isn't enforced after its break")
	}

	// Held: more break is refused, less is allowed.
	if err := runCLI(t, "break", "allowance", "b", "20m", "--per", "1h"); err == nil {
		t.Error("a larger allowance was accepted on a locked block")
	}
	if err := runCLI(t, "break", "allowance", "b", "10m", "--per", "30m"); err == nil {
		t.Error("a faster allowance was accepted on a locked block")
	}
	if err := runCLI(t, "break", "relock", "b", "5m"); err == nil {
		t.Error("relock was accepted on a locked block")
	}
	mustRun(t, "break", "allowance", "b", "5m", "--per", "1h")
	if k, _ = d.GetBreaks("b"); k.Allowance != 300 || k.Balance() != 300 {
		t.Errorf("after lowering: %+v", k)
	}
	mustRun(t, "break", "allowance", "b", "off")
	if k, _ = d.GetBreaks("b"); k != (db.Breaks{}) {
		t.Errorf("after off: %+v", k)
	}
	// Stopping the block ends its pause.
	if err := d.SetPause("b", trusted+500); err != nil {
		t.Fatal(err)
	}
	if err := d.DeactivateBlock("b"); err != nil {
		t.Fatal(err)
	}
	if until, _ := d.PausedUntil("b"); until != 0 {
		t.Error("a pause outlived its block's run")
	}
}

func TestRelockUnlock(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "break", "relock", "b", "5m")
	hash, _ := db.HashPassword("secret1")
	if err := d.ActivateBlock("b", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := d.SetLockExtras("b", db.LockExtras{PasswordHash: hash}); err != nil {
		t.Fatal(err)
	}
	// The first unlock is a break.
	if err := runWith(t, 0, []string{"secret1"}, "unlock", "b"); err != nil {
		t.Fatal(err)
	}
	detail, _ := d.GetBlock("b")
	trusted, _ := d.TrustedNow(time.Now())
	until, _ := d.PausedUntil("b")
	if !detail.Active || until-trusted < 290 || until-trusted > 300 {
		t.Fatalf("after the first unlock: active %v, paused %d s", detail.Active, until-trusted)
	}
	// Loosening stays refused during the break.
	if err := runWith(t, 0, nil, "list", "remove", "b", "Blocked", "reddit.com"); err == nil {
		t.Error("the block could be loosened during its break")
	}
	// A wrong password ends nothing; the right one, during the break, ends the block.
	if err := runWith(t, 0, []string{"nope"}, "unlock", "b"); err == nil {
		t.Error("a wrong password unlocked")
	}
	if err := runWith(t, 0, []string{"secret1"}, "unlock", "b"); err != nil {
		t.Fatal(err)
	}
	if detail, _ = d.GetBlock("b"); detail.Active {
		t.Error("unlocking during the break didn't end the block")
	}
}
