# Layers: hosts, firewall, DNS

- Covers: `blocker/hosts.go`, `validate.go`, `firewall.go`, `resolve.go`, `patterns.go`, `tor.go`, `cli/dns.go` (the blocking layers live in `internal/blocker/`)
- SPEC: §3.1, §3.13, §4.10 DNS
- Connects to: [enforce-daemon](enforce-daemon.md) (feeds them), [browsers](browsers.md) (policy layer), [device](device.md) (monk mode's cut)

## Code

- `/etc/hosts` (atomic; `validate.go` blocks injection): at most
  `MaxHostsDomains` (20,000) sites, the first in sorted order (every name
  lookup on the computer reads the file); drift check one pass.
- Firewall: nft table `inet kc9k` (address sets, output+forward chains,
  `nft -f`); fallback iptables/ip6tables chain jumped first in OUTPUT and
  FORWARD, re-topped on drift. Real IPs from a built-in DNS client
  bypassing hosts: network server, then Quad9, or `kc9k dns` choice
  (`SetDNSChoice`); loopback answers dropped. DoH resolvers blocked.
  `resolveAll`: 16 parallel, 30 min cache, counted from the run's end; no
  answer kept 2 min. An NXDOMAIN ends a lookup (`responseNoName`: no other
  server asked) and is an answer (kept 30 min; at 2 min 100,000 dead sites
  kept the daemon looking up for good). `ApplyFirewallKnown` (the daemon's) applies the
  addresses known so far and leaves the rest to one background run
  (`knownAddresses`, `LookupsRunning`); `ForgetFailedLookups` asks again.

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **Layers**: hosts/policy authoritative; IP rules best effort (CDNs).
  Containers/VMs: firewall only. Snap/Flatpak browsers may ignore policy.
