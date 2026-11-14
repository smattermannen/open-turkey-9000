//go:build roottest

// Run as root on a disposable machine: go test -tags roottest -run Firewall ./internal/blocker
package blocker

import (
	"os/exec"
	"strings"
	"testing"
)

// With nft, the blocked addresses live in our own table's sets; the
// iptables chain is gone, and the drift check notices a deleted table.
func TestFirewallNftables(t *testing.T) {
	if !nftAvailable() {
		t.Skip("no nft")
	}
	defer RemoveFirewall()
	if err := ApplyFirewall([]string{"example.com"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !nftApplied() || !IsFirewallApplied() {
		t.Fatal("freshly applied firewall reported as not applied")
	}
	if _, err := runFWOutput("iptables", "-L", chainName, "-n"); err == nil {
		t.Error("the old iptables chain is still there")
	}
	out, _ := exec.Command("nft", "list", "set", "inet", nftTable, "doh4").Output()
	if !strings.Contains(string(out), "9.9.9.9") {
		t.Errorf("the DoH set lacks Quad9: %s", out)
	}
	if err := exec.Command("nft", "delete", "table", "inet", nftTable).Run(); err != nil {
		t.Fatal(err)
	}
	if IsFirewallApplied() {
		t.Fatal("a deleted table went unnoticed")
	}
	if err := ApplyFirewall([]string{"example.com"}); err != nil || !IsFirewallApplied() {
		t.Fatalf("reapplying: %v", err)
	}
	RemoveFirewall()
	if nftApplied() {
		t.Error("the table is still there after removal")
	}
}

// Without nft, the iptables chain's jump must come first in OUTPUT and
// FORWARD, and the drift check must notice another program's rule above it.
func TestFirewallJumpsStayFirst(t *testing.T) {
	defer RemoveFirewall()
	if err := applyFamily("iptables", []string{"192.0.2.1"}, dohServersIPv4, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	for _, hook := range hookChains {
		if !jumpIsFirst("iptables", hook) {
			t.Fatalf("%s doesn't jump to %s first", hook, chainName)
		}
	}
	if err := runFW("iptables", "-I", "FORWARD", "1", "-j", "ACCEPT"); err != nil {
		t.Fatal(err)
	}
	defer runFW("iptables", "-D", "FORWARD", "-j", "ACCEPT")
	if jumpIsFirst("iptables", "FORWARD") {
		t.Fatal("an ACCEPT above our jump went unnoticed")
	}
	if err := applyFamily("iptables", []string{"192.0.2.1"}, dohServersIPv4, false); err != nil {
		t.Fatalf("reapply: %v", err)
	}
	if !jumpIsFirst("iptables", "FORWARD") {
		t.Fatal("reapplying didn't move our jump back to the top")
	}
	RemoveFirewall()
	for _, hook := range hookChains {
		if out, _ := runFWOutput("iptables", "-S", hook); firstRuleJumps(out, hook) {
			t.Errorf("%s still jumps to %s after removal", hook, chainName)
		}
	}
}
