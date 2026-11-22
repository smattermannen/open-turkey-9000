package blocker

import (
	"path/filepath"
	"testing"

	"github.com/smattermannen/kc9k/internal/platform"
)

// W50: with nothing of KC9K's in Software Restriction Policies (strict
// mode not acting, every cycle) removing them refreshes no policy; after
// a real removal, policy is refreshed once. Real registry: windows-latest
// ("Layers for real", matched by Strict).
func TestStrictSRPRefreshOnlyOnChange(t *testing.T) {
	if !platform.Privileged() {
		t.Skip("needs an administrator")
	}
	oldRefresh, oldRules, oldMade := refreshPolicy, srpRules, srpMadePath
	defer func() { refreshPolicy, srpRules, srpMadePath = oldRefresh, oldRules, oldMade }()
	calls := 0
	refreshPolicy = func() { calls++ }
	dir := t.TempDir()
	srpMadePath = filepath.Join(dir, "srp-made")
	srpRules = func([]string) []srpRule {
		return []srpRule{{filepath.Join(dir, "nowhere"), saferDisallow}}
	}
	if len(srpOurRules(saferDisallow))+len(srpOurRules(saferAllow)) > 0 {
		t.Skip("KC9K's SRP rules are on this computer already")
	}
	for i := 0; i < 3; i++ {
		if err := RemoveNoexec(); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 0 {
		t.Fatalf("idle removal refreshed policy %d times, want 0", calls)
	}
	if err := ApplyNoexec(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("applying refreshed policy %d times, want 1", calls)
	}
	if err := ApplyNoexec(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("applying the same rules again refreshed policy (%d calls)", calls)
	}
	calls = 0
	if err := RemoveNoexec(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("a real removal refreshed policy %d times, want 1", calls)
	}
	if n := len(srpOurRules(saferDisallow)); n != 0 {
		t.Fatalf("%d rules left", n)
	}
	calls = 0
	if err := RemoveNoexec(); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("removal after removal refreshed policy %d times, want 0", calls)
	}
}
