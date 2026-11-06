//go:build roottest

// Run as root on a disposable machine: go test -tags roottest -run Firewall ./internal/blocker
package blocker

import "testing"

// The jump to our chain must come first in OUTPUT and FORWARD, and the drift
// check must notice when another program (Docker, ufw) puts a rule above it.
func TestFirewallJumpsStayFirst(t *testing.T) {
	defer RemoveFirewall()
	if err := ApplyFirewall([]string{"example.com"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	for _, hook := range hookChains {
		if !jumpIsFirst("iptables", hook) {
			t.Fatalf("%s doesn't jump to %s first", hook, chainName)
		}
	}
	if !IsFirewallApplied() {
		t.Fatal("freshly applied firewall reported as not applied")
	}

	// Another program inserts an ACCEPT above our jump in FORWARD.
	if err := runFW("iptables", "-I", "FORWARD", "1", "-j", "ACCEPT"); err != nil {
		t.Fatal(err)
	}
	defer runFW("iptables", "-D", "FORWARD", "-j", "ACCEPT")
	if IsFirewallApplied() {
		t.Fatal("an ACCEPT above our jump went unnoticed")
	}
	if err := ApplyFirewall([]string{"example.com"}); err != nil {
		t.Fatalf("reapply: %v", err)
	}
	if !jumpIsFirst("iptables", "FORWARD") || !IsFirewallApplied() {
		t.Fatal("reapplying didn't move our jump back to the top")
	}

	RemoveFirewall()
	for _, hook := range hookChains {
		if out, _ := runFWOutput("iptables", "-S", hook); firstRuleJumps(out, hook) {
			t.Errorf("%s still jumps to %s after removal", hook, chainName)
		}
	}
}
