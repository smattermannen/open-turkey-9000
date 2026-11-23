# Layers: hosts, firewall, DNS

- Covers: `blocker/hosts.go`, `validate.go`, `firewall.go`, `resolve.go`, `patterns.go`, `tor.go`, `cli/dns.go` (the blocking layers live in `internal/blocker/`)
- SPEC: §3.1, §3.13, §4.10 DNS
- Connects to: [enforce-daemon](enforce-daemon.md) (feeds them), [browsers](browsers.md) (policy layer), [device](device.md) (monk mode's cut)

- Hosts/policy authoritative; IP rules best effort (CDNs). Containers/VMs:
  firewall only. Snap/Flatpak browsers may ignore policy.
- `/etc/hosts` (atomic; `validate.go` blocks injection): ≤
  `MaxHostsDomains` (20,000) sites, first in sorted order (every lookup
  reads the file); drift check one pass.
- Firewall: nft table `inet kc9k` (address sets, output+forward, `nft -f`);
  fallback iptables/ip6tables chain jumped first in OUTPUT and FORWARD,
  re-topped on drift. DoH resolvers blocked.
- IPs from a built-in DNS client bypassing hosts: network server, then
  Quad9, or `kc9k dns` choice (`SetDNSChoice`); loopback answers dropped.
  `resolveAll`: 16 parallel, 30 min cache from run's end; no answer kept 2
  min; NXDOMAIN is an answer, ends the lookup (`responseNoName`), kept 30
  min (else dead sites keep the daemon looking up forever).
- `ApplyFirewallKnown` (daemon) applies known addresses, leaves the rest to
  one background run (`knownAddresses`, `LookupsRunning`);
  `ForgetFailedLookups` asks again.
