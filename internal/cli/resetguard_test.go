package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

// The guard's sites are list entries like any other, unchanged by
// normalizing, and all addresses.
func TestResetGuardSitesValid(t *testing.T) {
	got, err := prepareEntries(db.Where{URL: true}, enforce.ResetGuardSites)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range got {
		if e != enforce.ResetGuardSites[i] || !urlEntry(e) {
			t.Errorf("%q → %q (address %v)", enforce.ResetGuardSites[i], e, urlEntry(e))
		}
	}
}

// §3.8 installer media guard: acts only while something is locked; off is
// refused then.
func TestResetGuard(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "reset-guard", "on")
	guarded := func() bool {
		rules, err := enforce.BuildRules(d)
		if err != nil {
			t.Fatal(err)
		}
		return slices.Contains(rules.Block, "releases.ubuntu.com")
	}
	if guarded() {
		t.Error("the guard acts with nothing locked")
	}
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if !guarded() {
		t.Error("the guard doesn't act while a block is locked")
	}
	if err := runCLI(t, "reset-guard", "off"); err == nil {
		t.Error("the guard turned off while locked")
	}
	if !strings.Contains(capture(t, "reset-guard"), "on") {
		t.Error("reset-guard doesn't show on")
	}
}
