package blocker

import (
	"fmt"
	"os"
	"path/filepath"
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
	lookupIPs = func(string) ([]string, []string) { return []string{"192.0.2.1"}, nil }
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
