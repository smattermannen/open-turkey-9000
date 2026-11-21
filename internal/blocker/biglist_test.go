//go:build linux

package blocker

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// Principle 9: a list of 100,000 sites never slows the computer. The hosts
// layer keeps MaxHostsDomains (every lookup on the computer reads the file),
// its drift check is one pass, and looking the sites up for the firewall
// runs in parallel and is kept.
func TestHugeSiteList(t *testing.T) {
	domains := make([]string, 100000)
	for i := range domains {
		domains[i] = fmt.Sprintf("site%d.example", i)
	}
	path := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(path, []byte("127.0.0.1 localhost\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := hostsPath
	hostsPath = path
	t.Cleanup(func() { hostsPath = old })

	start := time.Now()
	if err := ApplyHosts(domains); err != nil {
		t.Fatal(err)
	}
	applied := time.Since(start)
	data, _ := os.ReadFile(path)
	if lines := strings.Count(string(data), "\n"); lines > 2*MaxHostsDomains+3 {
		t.Errorf("hosts has %d lines; want at most %d", lines, 2*MaxHostsDomains+3)
	}
	start = time.Now()
	if !IsHostsApplied(domains) {
		t.Error("the applied hosts section isn't recognised")
	}
	checked := time.Since(start)
	if applied > 2*time.Second || checked > time.Second {
		t.Errorf("100,000 sites: hosts written in %v, checked in %v", applied, checked)
	}
	// Tampering is still caught.
	tampered := strings.Replace(string(data), "0.0.0.0 site1.example\n", "1.2.3.4 site1.example\n", 1)
	_ = os.WriteFile(path, []byte(tampered), 0o644)
	if IsHostsApplied(domains) {
		t.Error("a changed address went unnoticed")
	}

	oldLookup := lookupIPs
	t.Cleanup(func() { lookupIPs = oldLookup })
	lookupIPs = func(string) ([]string, []string, bool) { return []string{"192.0.2.1"}, nil, false }
	start = time.Now()
	resolveAll(domains)
	first := time.Since(start)
	start = time.Now()
	resolveAll(domains)
	again := time.Since(start)
	if first > 5*time.Second || again > time.Second {
		t.Errorf("100,000 lookups (instant answers): first %v, from the cache %v", first, again)
	}
	t.Logf("hosts %v, check %v, lookups %v, cached %v", applied, checked, first, again)
}

// Chrome and Chromium take only ChromiumPolicyLimit entries of a URL list
// (checked 2026-11-17). With 100,000 sites, the policy holds what
// /etc/hosts can't block first: patterns and sites past MaxHostsDomains;
// at most the limit, the same each time (its drift check compares).
func TestChromiumPolicyLimit(t *testing.T) {
	domains := make([]string, 100000)
	for i := range domains {
		domains[i] = fmt.Sprintf("site%d.example", i)
	}
	p := BrowserPolicy{BlockDomains: domains, BlockPatterns: []string{"*casino*"}}
	set := chromiumBlockSet(p)
	if len(set) != ChromiumPolicyLimit {
		t.Fatalf("%d entries", len(set))
	}
	inHosts := map[string]bool{}
	for _, d := range hostsDomains(domains) {
		inHosts[d] = true
	}
	for _, e := range set {
		if inHosts[e] {
			t.Fatalf("%s is in /etc/hosts, yet took a place", e)
		}
	}
	if strings.Join(set, ",") != strings.Join(chromiumBlockSet(p), ",") {
		t.Error("not the same each time")
	}
	small := chromiumBlockSet(BrowserPolicy{BlockDomains: domains[:10]})
	if len(small) != 10 {
		t.Errorf("a small list was cut: %d", len(small))
	}
	allow := chromiumAllowSet(BrowserPolicy{AllowMode: true, AllowDomains: domains})
	if len(allow) != ChromiumPolicyLimit {
		t.Errorf("allowlist %d entries", len(allow))
	}
}

// Firefox and LibreWolf take only FirefoxPolicyLimit entries of
// WebsiteFilter (checked 2026-11-21: of 200,004, the 1,001st on were
// ignored). With 100,000 sites, Block holds what /etc/hosts can't block
// first: patterns, file URLs, sites past MaxHostsDomains; at most the
// limit, the same each time; the allowlist's catch-all always stays.
func TestFirefoxPolicyLimit(t *testing.T) {
	domains := make([]string, 100000)
	for i := range domains {
		domains[i] = fmt.Sprintf("site%d.example", i)
	}
	p := BrowserPolicy{BlockDomains: domains, BlockPatterns: []string{"*casino*"},
		BlockFiles: []string{"file:///home/me/games/*"}}
	set := firefoxBlockSet(p)
	if len(set) != FirefoxPolicyLimit {
		t.Fatalf("%d entries", len(set))
	}
	inHosts := map[string]bool{}
	for _, d := range hostsDomains(domains) {
		inHosts[fmt.Sprintf("*://%s/*", d)] = true
		inHosts[fmt.Sprintf("*://*.%s/*", d)] = true
	}
	patterns, _ := translated(p.BlockPatterns)
	for _, want := range append(patterns, FilePatterns(p.BlockFiles)...) {
		if !slices.Contains(set, want) {
			t.Errorf("%s left out", want)
		}
	}
	for _, e := range set {
		if inHosts[e] {
			t.Fatalf("%s is in /etc/hosts, yet took a place", e)
		}
	}
	if strings.Join(set, ",") != strings.Join(firefoxBlockSet(p), ",") {
		t.Error("not the same each time")
	}
	// Fewer sites than /etc/hosts holds: they fill the places.
	if got := firefoxBlockSet(BrowserPolicy{BlockDomains: domains[:5000]}); len(got) != FirefoxPolicyLimit {
		t.Errorf("5,000 sites: %d entries", len(got))
	}
	small := firefoxBlockSet(BrowserPolicy{BlockDomains: domains[:10]})
	if len(small) != 20 {
		t.Errorf("a small list was cut: %d", len(small))
	}
	many := make([]string, 2*FirefoxPolicyLimit)
	for i := range many {
		many[i] = fmt.Sprintf("*casino%d*", i)
	}
	all := firefoxBlockSet(BrowserPolicy{AllowMode: true, BlockPatterns: many})
	if len(all) != FirefoxPolicyLimit || !slices.Contains(all, "*://*/*") {
		t.Error("the allowlist's catch-all was left out")
	}
	allow := firefoxExceptionSet(BrowserPolicy{AllowMode: true, AllowDomains: domains})
	if len(allow) != FirefoxPolicyLimit {
		t.Errorf("exceptions %d entries", len(allow))
	}
}

// Principle 9: the daemon's drift check builds a policy's sets once, not
// every cycle; any change to the policy builds them again (a stale set
// could leave a newly blocked site out).
func TestPolicySetsFollowThePolicy(t *testing.T) {
	domains := make([]string, 100000)
	for i := range domains {
		domains[i] = fmt.Sprintf("site%d.example", i)
	}
	p := BrowserPolicy{BlockDomains: domains}
	setsOf(p)
	start := time.Now()
	setsOf(p)
	if took := time.Since(start); took > 100*time.Millisecond {
		t.Errorf("the same policy again took %s", took)
	}
	small := BrowserPolicy{BlockDomains: []string{"a.example"}}
	if got := setsOf(small).ffBlock; !slices.Contains(got, "*://a.example/*") {
		t.Errorf("first policy: %v", got)
	}
	small.BlockDomains[0] = "b.example" // same length, changed in place
	if got := setsOf(small).ffBlock; !slices.Contains(got, "*://b.example/*") || slices.Contains(got, "*://a.example/*") {
		t.Errorf("changed in place: %v", got)
	}
	small.AllowMode = true
	if got := setsOf(small).crBlock; !slices.Contains(got, "*") {
		t.Errorf("allowlist mode: %v", got)
	}
	if !setsOf(BrowserPolicy{}).empty {
		t.Error("an empty policy isn't empty")
	}
}
