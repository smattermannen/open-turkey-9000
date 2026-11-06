// =============================================================================
// Package blocker — Killer Chicken 9000's firewall module
// =============================================================================
//
// This file blocks sites at the network layer using iptables (IPv4) and
// ip6tables (IPv6), the firewalls built into the Linux kernel.
//
// WHAT DOES THE FIREWALL LAYER ADD?
// ---------------------------------
// The /etc/hosts layer already stops name resolution for blocked domains. The
// firewall is a second, independent layer that:
//
//  1. Drops DNS-over-HTTPS (DoH) traffic to well-known resolvers. Modern
//     browsers can resolve names over HTTPS, bypassing /etc/hosts entirely;
//     dropping port 443 to those resolvers forces them back onto system DNS,
//     where /etc/hosts applies. This is the firewall's most important job.
//
//  2. Drops packets to the real IP addresses of blocked domains, so that even
//     a hard-coded IP or a cached address can't reach the site.
//
// RESOLVING THE REAL IPs
// ----------------------
// By the time the firewall runs, /etc/hosts already maps blocked domains to
// 0.0.0.0, so the ordinary resolver would return 0.0.0.0. We therefore query
// public resolvers directly (see resolve.go) to learn the true addresses.
//
// CAVEAT — SHARED IPs / CDNs
// --------------------------
// A blocked domain may share IP addresses with unrelated sites (common behind
// CDNs like Cloudflare or Fastly), and addresses change over time. Blocking by
// IP can therefore over-block or go stale. That is why the firewall is a
// secondary layer: the authoritative blocks are /etc/hosts and browser policy.
// We keep the IP rules because they raise the effort required to bypass, but
// they are intentionally best-effort.
//
// =============================================================================
package blocker

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// chainName is the name of our custom chain in both iptables and ip6tables.
const chainName = "KC9K"

// dohServersIPv4 and dohServersIPv6 hold the addresses of well-known
// DNS-over-HTTPS resolvers. We drop TCP port 443 to these so browsers can't
// use them to resolve names and bypass /etc/hosts. We do NOT touch port 53, so
// ordinary system DNS keeps working for sites that are not blocked.
var dohServersIPv4 = []string{
	"1.1.1.1", "1.0.0.1", // Cloudflare
	"8.8.8.8", "8.8.4.4", // Google
	"9.9.9.9", "149.112.112.112", // Quad9
	"208.67.222.222", "208.67.220.220", // OpenDNS
	"94.140.14.14", "94.140.15.15", // AdGuard
	"185.228.168.9", "185.228.169.9", // CleanBrowsing
	"76.76.2.0", "76.76.10.0", // Control D
	"194.242.2.2", // Mullvad
}

var dohServersIPv6 = []string{
	"2606:4700:4700::1111", "2606:4700:4700::1001", // Cloudflare
	"2001:4860:4860::8888", "2001:4860:4860::8844", // Google
	"2620:fe::fe", "2620:fe::9", // Quad9
	"2620:119:35::35", "2620:119:53::53", // OpenDNS
	"2a10:50c0::ad1:ff", "2a10:50c0::ad2:ff", // AdGuard
}

// =============================================================================
// Public functions
// =============================================================================

// ApplyFirewall configures the firewall to block the given domains.
//
// It rebuilds the KC9K chain from scratch (simpler and safer than
// diffing), jumps to it first from OUTPUT and FORWARD, drops the resolved IPs of each domain,
// and drops the DoH resolvers. IPv6 rules are applied the same way when
// ip6tables is available; IPv6 failures are best-effort and never abort the
// IPv4 rules.
//
// With no valid domains, the firewall is removed entirely.
func ApplyFirewall(domains []string) error {
	valid := ValidDomains(domains)
	if len(valid) == 0 {
		return RemoveFirewall()
	}

	// Resolve every domain up front (bypassing /etc/hosts) into v4/v6 sets.
	var ipv4, ipv6 []string
	for _, domain := range valid {
		v4, v6 := resolveIPs(domain)
		ipv4 = append(ipv4, v4...)
		ipv6 = append(ipv6, v6...)
	}

	if err := applyFamily("iptables", ipv4, dohServersIPv4); err != nil {
		return err
	}

	// IPv6 is best-effort: many machines have it disabled or lack ip6tables.
	if ip6tablesAvailable() {
		// An error here shouldn't undo the working IPv4 rules; the daemon logs
		// the returned error but keeps enforcing.
		if err := applyFamily("ip6tables", ipv6, dohServersIPv6); err != nil {
			return fmt.Errorf("IPv6 firewall (best-effort) failed: %w", err)
		}
	}

	return nil
}

// applyFamily builds the KC9K chain for one address family (iptables or
// ip6tables): create/flush the chain, ensure the OUTPUT jump, drop each IP, and
// drop the DoH resolvers on port 443.
func applyFamily(cmd string, ips, dohServers []string) error {
	// Create the chain (ignore "already exists").
	_ = runFW(cmd, "-N", chainName)

	// Flush it to start clean.
	if err := runFW(cmd, "-F", chainName); err != nil {
		return fmt.Errorf("failed to flush %s chain %s: %w", cmd, chainName, err)
	}

	// Jump to our chain first from OUTPUT (this computer's own traffic) and
	// FORWARD (containers and virtual machines behind a bridge or NAT).
	for _, hook := range hookChains {
		if err := ensureFirstJump(cmd, hook); err != nil {
			return err
		}
	}

	// Drop the resolved IPs of blocked domains.
	for _, ip := range ips {
		if err := runFW(cmd, "-A", chainName, "-d", ip, "-j", "DROP"); err != nil {
			return fmt.Errorf("failed to drop IP %s (%s): %w", ip, cmd, err)
		}
	}

	// Drop DoH resolvers on TCP 443.
	for _, ip := range dohServers {
		if err := runFW(cmd, "-A", chainName, "-d", ip, "-p", "tcp", "--dport", "443", "-j", "DROP"); err != nil {
			return fmt.Errorf("failed to drop DoH server %s (%s): %w", ip, cmd, err)
		}
	}

	return nil
}

// RemoveFirewall tears down the KC9K chain in both families.
//
// Order matters: remove the OUTPUT and FORWARD jumps, flush the chain, then delete it —
// iptables refuses to delete a chain that is still referenced or non-empty.
// The function is idempotent and swallows errors (a missing chain is the
// desired end state).
func RemoveFirewall() error {
	removeFamily("iptables")
	if ip6tablesAvailable() {
		removeFamily("ip6tables")
	}
	return nil
}

func removeFamily(cmd string) {
	for _, hook := range hookChains {
		removeJumps(cmd, hook)
	}
	_ = runFW(cmd, "-F", chainName)
	_ = runFW(cmd, "-X", chainName)
}

// hookChains are the built-in chains that jump to ours. OUTPUT carries this
// computer's own traffic. FORWARD carries traffic of Docker containers and
// libvirt virtual machines, which have their own /etc/hosts and would
// otherwise pass the firewall untouched. (Rootless Podman and VirtualBox NAT
// send through this computer's own sockets, so OUTPUT covers them.)
var hookChains = []string{"OUTPUT", "FORWARD"}

// removeJumps deletes every jump from hook to our chain.
func removeJumps(cmd, hook string) {
	for i := 0; i < 10 && runFW(cmd, "-D", hook, "-j", chainName) == nil; i++ {
	}
}

// ensureFirstJump makes the jump to our chain the first rule of hook. Docker,
// ufw and others insert their own rules at the top, and an ACCEPT placed
// before our jump would let blocked traffic through.
func ensureFirstJump(cmd, hook string) error {
	if jumpIsFirst(cmd, hook) {
		return nil
	}
	removeJumps(cmd, hook)
	if err := runFW(cmd, "-I", hook, "1", "-j", chainName); err != nil {
		return fmt.Errorf("failed to insert %s jump from %s to %s: %w", cmd, hook, chainName, err)
	}
	return nil
}

// jumpIsFirst reports whether hook's first rule jumps to our chain.
func jumpIsFirst(cmd, hook string) bool {
	output, err := runFWOutput(cmd, "-S", hook)
	return err == nil && firstRuleJumps(output, hook)
}

// firstRuleJumps reads "iptables -S <hook>" output: policy lines ("-P") come
// first, then the rules ("-A") in order.
func firstRuleJumps(listing, hook string) bool {
	for _, line := range strings.Split(listing, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "-A "+hook+" ") {
			return line == "-A "+hook+" -j "+chainName
		}
	}
	return false
}

// IsFirewallApplied reports whether our firewall is active. We treat the IPv4
// chain existing with at least one rule as the signal, because the DoH rules
// are always present whenever the firewall is applied. (IPv6 is best-effort
// and not required for the layer to count as active.)
func IsFirewallApplied() bool {
	output, err := runFWOutput("iptables", "-L", chainName, "-n")
	if err != nil {
		return false
	}
	// "iptables -L CHAIN" prints two header lines; rules start on line 3.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) <= 2 {
		return false
	}
	// A populated chain does nothing unless OUTPUT and FORWARD jump to it
	// before anything else can accept the traffic.
	for _, hook := range hookChains {
		if !jumpIsFirst("iptables", hook) {
			return false
		}
	}
	if ip6tablesAvailable() {
		if _, err := runFWOutput("ip6tables", "-L", chainName, "-n"); err == nil {
			for _, hook := range hookChains {
				if !jumpIsFirst("ip6tables", hook) {
					return false
				}
			}
		}
	}
	return true
}

// ip6tablesAvailable reports whether the ip6tables binary is present.
func ip6tablesAvailable() bool {
	_, err := exec.LookPath("ip6tables")
	return err == nil
}

// =============================================================================
// Helpers
// =============================================================================

// runFW runs an iptables/ip6tables command, capturing stderr for diagnostics.
// Both binaries require root.
func runFW(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	var stderr bytes.Buffer
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return fmt.Errorf("failed to run %s %s: %s (error: %w)", cmd, strings.Join(args, " "), stderrStr, err)
		}
		return fmt.Errorf("failed to run %s %s: %w", cmd, strings.Join(args, " "), err)
	}
	return nil
}

// runFWOutput runs an iptables/ip6tables command and returns its combined
// output (used for listing rules).
func runFWOutput(cmd string, args ...string) (string, error) {
	c := exec.Command(cmd, args...)
	output, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run %s %s: %s (error: %w)", cmd, strings.Join(args, " "), strings.TrimSpace(string(output)), err)
	}
	return string(output), nil
}
