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
