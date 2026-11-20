package db

import (
	"testing"

	"github.com/smattermannen/kc9k/internal/blocker"
)

// §3.9 ad trigger: every shipped network is a valid domain, in one
// industry only, and an industry name in an ads list stands for them.
func TestAdIndustries(t *testing.T) {
	seen := map[string]string{}
	for industry, networks := range AdIndustries {
		for _, n := range networks {
			if blocker.NormalizeDomain(n) != n || !blocker.IsValidDomain(n) {
				t.Errorf("%s: %q is not a plain domain", industry, n)
			}
			if other, ok := seen[n]; ok {
				t.Errorf("%q is in %s and %s", n, other, industry)
			}
			seen[n] = industry
		}
	}
	l := WebList{Where: Where{Ads: true}, Entries: []string{"Crypto", "example-ads.com"}}
	got := l.AdNetworks()
	if len(got) != len(AdIndustries["crypto"])+1 {
		t.Errorf("crypto plus one network = %v", got)
	}
}
