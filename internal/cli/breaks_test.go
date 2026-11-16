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
	if got := k.Earn(1800, 0).Balance(); got != 300 {
		t.Errorf("earned %d s in half an hour; want 300", got)
	}
	if got := k.Earn(100000, 0).Balance(); got != 600 {
		t.Errorf("earned %d s; want it capped at 600", got)
	}

	if err := d.ActivateBlock("b", true, 0, 0, false); err != nil { // locked
		t.Fatal(err)
	}
	if err := runCLI(t, "break", "start", "b"); err == nil || !strings.Contains(err.Error(), "less than a minute") {
		t.Errorf("a break with nothing earned: %v", err)
	}
	if err := d.SetBreaks("b", k.Earn(1800, 0)); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "break", "start", "b", "8m"); err == nil {
		t.Error("more break than earned was given")
	}
	// A locked block can take its break; the block stays active and locked.
	mustRun(t, "break", "start", "b", "3m")
	trusted, _ := d.TrustedNow(time.Now())
	until, _ := d.PausedUntil("b")
	if until-trusted < 120 || until-trusted > 180 { // a loaded machine may take a while
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
	if k, _ = d.GetBreaks("b"); k.Balance() < 240 || k.Balance() > 300 {
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
	// The balance kept what ending early gave back (a slow run takes a
	// second or so off it), capped by the new allowance.
	if k, _ = d.GetBreaks("b"); k.Allowance != 300 || k.Balance() < 240 || k.Balance() > 300 {
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
	if !detail.Active || until-trusted < 240 || until-trusted > 300 {
		t.Fatalf("after the first unlock: active %v, paused %d s", detail.Active, until-trusted)
	}
	// Loosening stays refused during the break.
	if err := runWith(t, 0, nil, "list", "remove", "b", "Blocked", "reddit.com"); err == nil {
		t.Error("the block could be loosened during its break")
	}
	// A wrong password ends nothing; the right one, during the break, lifts
	// the lock (§3.4: the block stays on).
	if err := runWith(t, 0, []string{"nope"}, "unlock", "b"); err == nil {
		t.Error("a wrong password unlocked")
	}
	if err := runWith(t, 0, []string{"secret1"}, "unlock", "b"); err != nil {
		t.Fatal(err)
	}
	extras, _ := d.GetLockExtras("b")
	if detail, _ = d.GetBlock("b"); !detail.Active || extras.PasswordHash != "" {
		t.Errorf("unlocking during the break: active %v, password lock left %v; want on and unlocked", detail.Active, extras.PasswordHash != "")
	}
}

// §3.6 reward kept for a length the user picks, with no upper limit
// (owner, 2026-11-16); a longer length, or a change between a period and a
// length, weakens; a shorter one is free while locked.
func TestRewardKeepLength(t *testing.T) {
	d := testDB(t)
	blockWithSite(t)
	mustRun(t, "break", "reward", "b", "10m", "--per", "1h", "--keep", "400d")
	if k, _ := d.GetBreaks("b"); k.Keep != "34560000" {
		t.Fatalf("keep = %q", k.Keep)
	}
	for _, bad := range []string{"30m", "soon", "-1d"} {
		if err := runCLI(t, "break", "reward", "b", "10m", "--per", "1h", "--keep", bad); err == nil {
			t.Errorf("--keep %s accepted", bad)
		}
	}
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runCLI(t, "break", "reward", "b", "10m", "--per", "1h", "--keep", "500d"); err == nil {
		t.Error("a longer keep was taken while locked")
	}
	if err := runCLI(t, "break", "reward", "b", "10m", "--per", "1h", "--keep", "month"); err == nil {
		t.Error("a period instead of a length was taken while locked")
	}
	mustRun(t, "break", "reward", "b", "10m", "--per", "1h", "--keep", "2d")
	if k, _ := d.GetBreaks("b"); k.Keep != "172800" {
		t.Errorf("shorter keep while locked: %q", k.Keep)
	}
	mustRun(t, "break", "reward", "b", "10m", "--per", "1h", "--keep", "one")
	if k, _ := d.GetBreaks("b"); k.Keep != "" {
		t.Errorf("one: %q", k.Keep)
	}
}

// A saved-up reward has no cap: a break can take all of it, also more than
// four hours, and ending it early gives the rest back.
func TestRewardBreakBeyondFourHours(t *testing.T) {
	d := testDB(t)
	blockWithSite(t)
	mustRun(t, "break", "reward", "b", "10m", "--per", "1h", "--keep", "30d")
	mustRun(t, "start", "b")
	trusted := mustTrusted(t, d)
	k, _ := d.GetBreaks("b")
	k = k.Earn(48*3600, trusted) // two days of blocking: 8 hours of break
	if err := d.SetBreaks("b", k); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "break", "start", "b", "6h")
	if until, _ := d.PausedUntil("b"); until < trusted+6*3600-5 {
		t.Errorf("a six-hour break: until %d, now %d", until, trusted)
	}
	mustRun(t, "break", "end", "b")
	if k, _ := d.GetBreaks("b"); k.Balance() < 8*3600-60 {
		t.Errorf("after ending the break at once: %d s left", k.Balance())
	}
}
