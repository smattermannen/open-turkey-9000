# Layers: hosts, firewall, DNS

- Covers: `blocker/hosts.go`, `validate.go`, `firewall.go`, `resolve.go`, `patterns.go`, `tor.go`, `cli/dns.go` (the blocking layers live in `internal/blocker/`)
- SPEC: §3.1, §3.13, §4.10 DNS
- Connects to: [enforce-daemon](enforce-daemon.md) (feeds them), [browsers](browsers.md) (policy layer), [device](device.md) (monk mode's cut)

## Code

- `/etc/hosts` (atomic; `validate.go` blocks injection).
- Firewall: nft table `inet kc9k` (address sets, output+forward chains,
  `nft -f`); fallback iptables/ip6tables chain jumped first in OUTPUT and
  FORWARD, re-topped on drift. Real IPs from a built-in DNS client
  bypassing hosts: network server, then Quad9, or `kc9k dns` choice
  (`SetDNSChoice`); loopback answers dropped. DoH resolvers blocked.
  `resolveAll`: 16 parallel, 30 min cache.

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **Layers**: hosts/policy authoritative; IP rules best effort (CDNs).
  Containers/VMs: firewall only. Snap/Flatpak browsers may ignore policy.
