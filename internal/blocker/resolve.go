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
	"math/rand"
	"net"
	"os"
	"strings"
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
func SetDNSChoice(choice string) { dnsChoice.Store(choice) }

// resolvConfPaths are read, in order, for the network's DNS servers: the
// upstream list of systemd-resolved first, then the system's own.
var resolvConfPaths = []string{"/run/systemd/resolve/resolv.conf", "/etc/resolv.conf"}

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

// networkDNS reads the network's DNS servers from the first resolv.conf that
// names any besides this computer's own (a local stub reads /etc/hosts).
func networkDNS() []string {
	for _, path := range resolvConfPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if servers := parseResolvConf(string(data)); len(servers) > 0 {
			return servers
		}
	}
	return nil
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
	for _, server := range dnsServers() {
		v4 := realAddresses(queryDNS(server, domain, dnsTypeA))
		v6 := realAddresses(queryDNS(server, domain, dnsTypeAAAA))
		if len(v4) > 0 || len(v6) > 0 {
			return v4, v6
		}
	}
	return nil, nil
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
// returns the addresses found. Any error (timeout, malformed response, etc.)
// yields an empty slice — resolution is best-effort.
func queryDNS(server, domain string, qtype uint16) []string {
	query, err := buildQuery(domain, qtype)
	if err != nil {
		return nil
	}

	conn, err := net.DialTimeout("udp", server, 3*time.Second)
	if err != nil {
		return nil
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	if _, err := conn.Write(query); err != nil {
		return nil
	}

	resp := make([]byte, 1500)
	n, err := conn.Read(resp)
	if err != nil || n < 2 || resp[0] != query[0] || resp[1] != query[1] {
		return nil // no answer, or not the answer to this query
	}

	return parseResponse(resp[:n], qtype)
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
