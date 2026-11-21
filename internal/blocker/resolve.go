// This file implements a tiny DNS client used by the firewall layer.
//
// WHY NOT net.LookupHost?
// -----------------------
// The firewall needs the REAL IP addresses of a blocked domain so it can drop
// packets to them. But by the time the firewall runs, the /etc/hosts layer has
// already mapped the domain to 0.0.0.0 — so the ordinary system resolver
// (net.LookupHost) and even a local stub resolver like systemd-resolved would
// hand back 0.0.0.0, and the firewall would "block" a useless address.
//
// To get the true addresses we ask a DNS server directly over UDP port 53,
// parsing the raw response ourselves, which bypasses /etc/hosts. By default
// (§4.10) that is the network's own DNS server, the one the computer already
// uses, read from systemd-resolved's upstream list (never its local stub, which
// answers from /etc/hosts), so nobody new learns which sites are blocked; Quad9
// when it can't answer. The user can choose Quad9 only, or their own servers
// (SetDNSChoice). Answers that point at this computer or nowhere (0.0.0.0, ::,
// 127.x) are dropped, so a local resolver that reads /etc/hosts can't fool it.
//
// If every server is unreachable (e.g. the machine is offline) we simply
// return no addresses: the firewall's IP rules are skipped, and the /etc/hosts
// and browser-policy layers continue to do their job.
package blocker

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// errInvalidLabel is returned when a domain label is empty or too long to
// encode in a DNS query.
var errInvalidLabel = errors.New("invalid DNS label")

// quad9 is the fallback server, and the one chosen by DNSQuad9.
const quad9 = "9.9.9.9"

// DNS choices for SetDNSChoice; anything else is a comma-separated list of
// server addresses.
const (
	DNSNetwork = "network" // the network's own server, then Quad9 (default)
	DNSQuad9   = "quad9"
)

var dnsChoice atomic.Value // string

// SetDNSChoice sets which servers resolveIPs asks: DNSNetwork (or ""),
// DNSQuad9, or a comma-separated list of IP addresses.
func SetDNSChoice(choice string) {
	if old, _ := dnsChoice.Swap(choice).(string); old != choice {
		// Other servers may know other addresses.
		resolveCache.Lock()
		resolveCache.entries = map[string]resolved{}
		resolveCache.Unlock()
	}
}

// ValidDNSChoice checks a choice for SetDNSChoice.
func ValidDNSChoice(choice string) error {
	if choice == "" || choice == DNSNetwork || choice == DNSQuad9 {
		return nil
	}
	for _, s := range strings.Split(choice, ",") {
		if net.ParseIP(strings.TrimSpace(s)) == nil {
			return fmt.Errorf("%q is not an IP address (use network, quad9 or IP addresses separated by commas)", s)
		}
	}
	return nil
}

// dnsServers returns the servers to ask, in order, as host:port.
func dnsServers() []string {
	choice, _ := dnsChoice.Load().(string)
	var ips []string
	switch choice {
	case DNSQuad9:
		ips = []string{quad9}
	case "", DNSNetwork:
		ips = append(networkDNS(), quad9)
	default:
		for _, s := range strings.Split(choice, ",") {
			if s = strings.TrimSpace(s); net.ParseIP(s) != nil {
				ips = append(ips, s)
			}
		}
	}
	servers := make([]string, 0, len(ips))
	for _, ip := range ips {
		servers = append(servers, net.JoinHostPort(ip, "53"))
	}
	return servers
}

// parseResolvConf returns the nameservers of a resolv.conf, leaving out
// loopback addresses.
func parseResolvConf(text string) []string {
	var servers []string
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "nameserver" {
			continue
		}
		host := fields[1]
		ip := net.ParseIP(strings.SplitN(host, "%", 2)[0])
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
			continue
		}
		servers = append(servers, host)
	}
	return servers
}

// DNS record type constants.
const (
	dnsTypeA    = 1  // IPv4 address
	dnsTypeAAAA = 28 // IPv6 address
)

// resolveIPs returns the real IPv4 and IPv6 addresses of a domain, bypassing
// /etc/hosts. It returns the first resolver's answer; if none respond, both
// slices are empty.
func resolveIPs(domain string) (ipv4 []string, ipv6 []string) {
	ipv4, ipv6, _ = resolveName(domain)
	return ipv4, ipv6
}

// resolveName is resolveIPs, and noName: a server answered that the name
// doesn't exist.
func resolveName(domain string) (ipv4 []string, ipv6 []string, noName bool) {
	for _, server := range dnsServers() {
		a, noName := queryDNS(server, domain, dnsTypeA)
		if noName {
			// The name doesn't exist: the next server won't know it either.
			// Huge lists hold many such sites; asking Quad9 too tripled the
			// lookups and it stopped answering (2026-11-21, desktop).
			return nil, nil, true
		}
		aaaa, _ := queryDNS(server, domain, dnsTypeAAAA)
		v4, v6 := realAddresses(a), realAddresses(aaaa)
		if len(v4) > 0 || len(v6) > 0 {
			return v4, v6, false
		}
	}
	return nil, nil, false
}

// resolveWorkers is how many lookups resolveAll runs at once.
const resolveWorkers = 16

// Lookups are kept for a while, so the daemon re-applying the firewall
// (a list changed, a block started) asks again only for new domains. A
// name that doesn't exist is an answer: kept as long as addresses (100,000
// such sites asked again every 2 minutes kept the daemon looking up for
// good; 2026-11-21, desktop). No answer at all is asked again soon.
const (
	resolveKeep       = 30 * time.Minute
	resolveKeepFailed = 2 * time.Minute
)

var resolveCache = struct {
	sync.Mutex
	entries map[string]resolved
}{entries: map[string]resolved{}}

type resolved struct {
	v4, v6 []string
	noName bool
	until  time.Time
}

// lookupIPs is resolveName; a variable for tests.
var lookupIPs = resolveName

// resolveAll looks up every domain's real addresses, several at once and
// from the cache where it can, and returns them without duplicates, sorted.
func resolveAll(domains []string) (ipv4, ipv6 []string) {
	results, due := fromCache(domains, time.Now())
	found := lookupAll(due)
	for i, domain := range domains {
		if r, ok := found[domain]; ok {
			results[i] = r
		}
	}
	return mergeAddresses(results)
}

// fromCache returns each domain's last lookup, an expired one too (its
// addresses stand until it is looked up again), and the domains due for a
// lookup: never looked up, or expired.
func fromCache(domains []string, now time.Time) (results []resolved, due []string) {
	results = make([]resolved, len(domains))
	resolveCache.Lock()
	defer resolveCache.Unlock()
	for i, domain := range domains {
		r, ok := resolveCache.entries[domain]
		if ok {
			results[i] = r
		}
		if !ok || !now.Before(r.until) {
			due = append(due, domain)
		}
	}
	return results, due
}

// lookupAll looks up domains, resolveWorkers at once, and keeps the answers
// in the cache. They count from when the last lookup ends, so a long run
// (100,000 sites) has every answer fresh at its end.
func lookupAll(domains []string) map[string]resolved {
	results := make([]resolved, len(domains))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < min(resolveWorkers, len(domains)); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i].v4, results[i].v6, results[i].noName = lookupIPs(domains[i])
			}
		}()
	}
	for i := range domains {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	now := time.Now()
	found := make(map[string]resolved, len(domains))
	resolveCache.Lock()
	defer resolveCache.Unlock()
	for domain, r := range resolveCache.entries {
		if now.After(r.until) {
			delete(resolveCache.entries, domain)
		}
	}
	for i, domain := range domains {
		r := results[i]
		r.until = now.Add(resolveKeep)
		if len(r.v4)+len(r.v6) == 0 && !r.noName {
			r.until = now.Add(resolveKeepFailed)
		}
		resolveCache.entries[domain] = r
		found[domain] = r
	}
	return found
}

// mergeAddresses returns the lookups' addresses without duplicates, sorted.
func mergeAddresses(results []resolved) (ipv4, ipv6 []string) {
	seen4, seen6 := map[string]bool{}, map[string]bool{}
	for _, r := range results {
		for _, ip := range r.v4 {
			if !seen4[ip] {
				seen4[ip] = true
				ipv4 = append(ipv4, ip)
			}
		}
		for _, ip := range r.v6 {
			if !seen6[ip] {
				seen6[ip] = true
				ipv6 = append(ipv6, ip)
			}
		}
	}
	sort.Strings(ipv4)
	sort.Strings(ipv6)
	return ipv4, ipv6
}

// background is the one lookup run knownAddresses keeps going.
var background struct {
	sync.Mutex
	running bool
}

// knownAddresses is resolveAll without waiting on the network (principle 9):
// it returns the addresses looked up so far, and whether every domain was
// looked up; the rest are looked up in the background, one run at a time
// (LookupsRunning). 100,000 sites take half an hour or more over a slow
// network (2026-11-21, desktop): the daemon's cycle and every kc9k command
// would wait that long.
func knownAddresses(domains []string) (ipv4, ipv6 []string, complete bool) {
	results, due := fromCache(domains, time.Now())
	ipv4, ipv6 = mergeAddresses(results)
	if len(due) == 0 {
		return ipv4, ipv6, true
	}
	background.Lock()
	defer background.Unlock()
	if !background.running {
		background.running = true
		go func() {
			start := time.Now()
			found := lookupAll(due)
			named := 0
			for _, r := range found {
				if len(r.v4)+len(r.v6) > 0 {
					named++
				}
			}
			log.Printf("firewall: looked up %d sites in %s, %d with addresses", len(due), time.Since(start).Round(time.Second), named)
			background.Lock()
			background.running = false
			background.Unlock()
		}()
	}
	return ipv4, ipv6, false
}

// LookupsRunning reports whether knownAddresses' background lookups run.
func LookupsRunning() bool {
	background.Lock()
	defer background.Unlock()
	return background.running
}

// ForgetFailedLookups drops the lookups that got no answer, so the next
// apply asks again (the network is back after monk mode's cut).
func ForgetFailedLookups() {
	resolveCache.Lock()
	defer resolveCache.Unlock()
	for domain, r := range resolveCache.entries {
		if len(r.v4)+len(r.v6) == 0 && !r.noName {
			delete(resolveCache.entries, domain)
		}
	}
}

// realAddresses drops answers that point nowhere or at this computer, as a
// resolver that reads /etc/hosts would give for a blocked site.
func realAddresses(ips []string) []string {
	out := ips[:0]
	for _, s := range ips {
		if ip := net.ParseIP(s); ip != nil && !ip.IsUnspecified() && !ip.IsLoopback() {
			out = append(out, s)
		}
	}
	return out
}

// queryDNS sends a single UDP DNS query for one record type to one server and
// returns the addresses found, and whether the server answered that the name
// doesn't exist (NXDOMAIN). Any error (timeout, malformed response, etc.)
// yields an empty slice — resolution is best-effort.
func queryDNS(server, domain string, qtype uint16) (ips []string, noName bool) {
	query, err := buildQuery(domain, qtype)
	if err != nil {
		return nil, false
	}

	conn, err := net.DialTimeout("udp", server, 3*time.Second)
	if err != nil {
		return nil, false
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	if _, err := conn.Write(query); err != nil {
		return nil, false
	}

	resp := make([]byte, 1500)
	n, err := conn.Read(resp)
	if err != nil || n < 2 || resp[0] != query[0] || resp[1] != query[1] {
		return nil, false // no answer, or not the answer to this query
	}

	return parseResponse(resp[:n], qtype), responseNoName(resp[:n])
}

// responseNoName reports whether a DNS response says the name doesn't exist
// (response code 3, NXDOMAIN).
func responseNoName(msg []byte) bool {
	return len(msg) >= 12 && msg[2]&0x80 != 0 && msg[3]&0x0F == 3
}

// buildQuery assembles a minimal DNS query packet: a 12-byte header with the
// recursion-desired flag, followed by a single question section.
func buildQuery(domain string, qtype uint16) ([]byte, error) {
	var b []byte

	// Header: random ID, flags 0x0100 (standard query, recursion desired),
	// QDCOUNT=1, ANCOUNT=NSCOUNT=ARCOUNT=0.
	id := uint16(rand.Intn(0xffff))
	header := make([]byte, 12)
	binary.BigEndian.PutUint16(header[0:2], id)
	binary.BigEndian.PutUint16(header[2:4], 0x0100)
	binary.BigEndian.PutUint16(header[4:6], 1)
	b = append(b, header...)

	// Question: QNAME as length-prefixed labels, then QTYPE and QCLASS (IN=1).
	for _, label := range strings.Split(domain, ".") {
		if len(label) == 0 || len(label) > 63 {
			return nil, errInvalidLabel
		}
		b = append(b, byte(len(label)))
		b = append(b, label...)
	}
	b = append(b, 0) // root label terminator

	qt := make([]byte, 4)
	binary.BigEndian.PutUint16(qt[0:2], qtype)
	binary.BigEndian.PutUint16(qt[2:4], 1) // class IN
	b = append(b, qt...)

	return b, nil
}

// parseResponse walks a DNS response and extracts the addresses of the
// requested record type. It performs strict bounds checking so a malformed or
// hostile packet can never cause an out-of-range panic.
func parseResponse(msg []byte, qtype uint16) []string {
	if len(msg) < 12 {
		return nil
	}

	qdCount := int(binary.BigEndian.Uint16(msg[4:6]))
	anCount := int(binary.BigEndian.Uint16(msg[6:8]))

	off := 12

	// Skip the question section.
	for i := 0; i < qdCount; i++ {
		off = skipName(msg, off)
		if off < 0 || off+4 > len(msg) {
			return nil
		}
		off += 4 // QTYPE + QCLASS
	}

	var ips []string
	for i := 0; i < anCount; i++ {
		off = skipName(msg, off)
		if off < 0 || off+10 > len(msg) {
			return nil
		}
		recType := binary.BigEndian.Uint16(msg[off : off+2])
		off += 8 // TYPE(2) + CLASS(2) + TTL(4)
		rdLen := int(binary.BigEndian.Uint16(msg[off : off+2]))
		off += 2
		if off+rdLen > len(msg) {
			return nil
		}

		if recType == qtype {
			if qtype == dnsTypeA && rdLen == 4 {
				ips = append(ips, net.IP(msg[off:off+4]).String())
			} else if qtype == dnsTypeAAAA && rdLen == 16 {
				ips = append(ips, net.IP(msg[off:off+16]).String())
			}
		}
		off += rdLen
	}

	return ips
}

// skipName advances past a DNS name at offset off, handling both the
// length-prefixed label form and compression pointers (0xC0). It returns the
// offset just after the name, or -1 if the name is malformed.
func skipName(msg []byte, off int) int {
	for {
		if off >= len(msg) {
			return -1
		}
		length := int(msg[off])
		if length == 0 {
			// Root label: name ends here.
			return off + 1
		}
		if length&0xC0 == 0xC0 {
			// Compression pointer: two bytes, and the name ends after them.
			if off+2 > len(msg) {
				return -1
			}
			return off + 2
		}
		// Ordinary label: skip the length byte plus the label bytes.
		off += 1 + length
	}
}

// DNSServers lists the servers resolveIPs asks now, in order.
func DNSServers() []string { return dnsServers() }
