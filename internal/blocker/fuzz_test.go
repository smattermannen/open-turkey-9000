package blocker

import (
	"strings"
	"testing"
)

// A domain that passes IsValidDomain can't break out of a hosts line, an
// nft set or a JSON string: only lowercase letters, digits, dots, hyphens.
func FuzzDomainValidation(f *testing.F) {
	for _, seed := range []string{"reddit.com", "https://user@www.Example.com:443/path", "a\nb.com", "x.com ", "*.xxx",
		"[::1]", "a..com", "-a.com", "xn--bcher-kva.example"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		d := NormalizeDomain(input)
		if !IsValidDomain(d) {
			return
		}
		if strings.Trim(d, "abcdefghijklmnopqrstuvwxyz0123456789.-") != "" {
			t.Errorf("%q normalised to %q, accepted with other bytes", input, d)
		}
		if NormalizeDomain(d) != d {
			t.Errorf("normalising %q again gives %q", d, NormalizeDomain(d))
		}
	})
}
