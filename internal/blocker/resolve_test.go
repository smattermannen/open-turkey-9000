package blocker

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBuildQuery(t *testing.T) {
	q, err := buildQuery("example.com", dnsTypeA)
	if err != nil {
		t.Fatalf("buildQuery error: %v", err)
	}

	// 12-byte header + labels (1+7)+(1+3)+1 + 4 (qtype+qclass) = 29.
	if len(q) != 29 {
		t.Fatalf("query length = %d; want 29", len(q))
	}
	// Flags: standard query, recursion desired.
	if q[2] != 0x01 || q[3] != 0x00 {
		t.Errorf("flags = %02x%02x; want 0100", q[2], q[3])
	}
	// QDCOUNT = 1.
	if q[4] != 0x00 || q[5] != 0x01 {
		t.Errorf("qdcount = %02x%02x; want 0001", q[4], q[5])
	}
	// Question labels.
	if q[12] != 7 || string(q[13:20]) != "example" {
		t.Errorf("first label wrong: %v %q", q[12], q[13:20])
	}
	if q[20] != 3 || string(q[21:24]) != "com" {
		t.Errorf("second label wrong: %v %q", q[20], q[21:24])
	}
	if q[24] != 0 {
		t.Errorf("root terminator = %d; want 0", q[24])
	}
	// QTYPE = A (1), QCLASS = IN (1).
	if q[25] != 0x00 || q[26] != 0x01 || q[27] != 0x00 || q[28] != 0x01 {
		t.Errorf("qtype/qclass wrong: %v", q[25:29])
	}
}

func TestBuildQueryInvalidLabel(t *testing.T) {
	long := ""
	for i := 0; i < 64; i++ {
		long += "a"
	}
	if _, err := buildQuery(long+".com", dnsTypeA); err == nil {
		t.Error("expected error for over-long label, got nil")
	}
}

func TestParseResponseA(t *testing.T) {
	// A DNS response for example.com -> 93.184.216.34, using a compression
	// pointer (0xC00C) for the answer name.
	msg := []byte{
		0x12, 0x34, // id
		0x81, 0x80, // flags: response, recursion available
		0x00, 0x01, // qdcount
		0x00, 0x01, // ancount
		0x00, 0x00, // nscount
		0x00, 0x00, // arcount
		// question: example.com A IN
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0,
		0x00, 0x01, // qtype A
		0x00, 0x01, // qclass IN
		// answer
		0xC0, 0x0C, // name pointer -> offset 12
		0x00, 0x01, // type A
		0x00, 0x01, // class IN
		0x00, 0x00, 0x01, 0x2C, // ttl
		0x00, 0x04, // rdlength
		93, 184, 216, 34, // rdata
	}

	got := parseResponse(msg, dnsTypeA)
	want := []string{"93.184.216.34"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseResponse A = %v; want %v", got, want)
	}

	// Asking for AAAA against the same message yields nothing.
	if got := parseResponse(msg, dnsTypeAAAA); len(got) != 0 {
		t.Errorf("parseResponse AAAA = %v; want empty", got)
	}
}

func TestParseResponseTruncatedIsSafe(t *testing.T) {
	// A deliberately too-short / malformed message must not panic and must
	// return no addresses.
	for _, msg := range [][]byte{
		nil,
		{0x00},
		{0x12, 0x34, 0x81, 0x80, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0xC0},
	} {
		if got := parseResponse(msg, dnsTypeA); len(got) != 0 {
			t.Errorf("parseResponse(%v) = %v; want empty", msg, got)
		}
	}
}

func TestParseResolvConf(t *testing.T) {
	got := parseResolvConf("# comment\nnameserver 127.0.0.53\nnameserver 10.0.0.1\nnameserver fe80::1%wlan0\nnameserver ::1\noptions edns0\nsearch .\n")
	want := []string{"10.0.0.1", "fe80::1%wlan0"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("parseResolvConf = %v; want %v", got, want)
	}
}

func TestDNSServers(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "stub")
	upstream := filepath.Join(dir, "upstream")
	if err := os.WriteFile(stub, []byte("nameserver 127.0.0.53\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := resolvConfPaths
	t.Cleanup(func() { resolvConfPaths = old; SetDNSChoice("") })

	// Only a local stub: Quad9 alone.
	resolvConfPaths = []string{upstream, stub}
	SetDNSChoice("")
	if got := dnsServers(); strings.Join(got, " ") != "9.9.9.9:53" {
		t.Errorf("stub only: %v", got)
	}
	// The network's own server first, then Quad9.
	if err := os.WriteFile(upstream, []byte("nameserver 192.168.1.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := dnsServers(); strings.Join(got, " ") != "192.168.1.1:53 9.9.9.9:53" {
		t.Errorf("network: %v", got)
	}
	SetDNSChoice(DNSQuad9)
	if got := dnsServers(); strings.Join(got, " ") != "9.9.9.9:53" {
		t.Errorf("quad9: %v", got)
	}
	SetDNSChoice("1.0.0.1, 2606:4700::1111")
	if got := dnsServers(); strings.Join(got, " ") != "1.0.0.1:53 [2606:4700::1111]:53" {
		t.Errorf("own: %v", got)
	}
	if ValidDNSChoice("1.2.3.4,nope") == nil {
		t.Error("a name was taken for an address")
	}
	if got := realAddresses([]string{"0.0.0.0", "1.2.3.4", "::", "127.0.0.1", "2001:db8::1"}); strings.Join(got, " ") != "1.2.3.4 2001:db8::1" {
		t.Errorf("realAddresses = %v", got)
	}
}

func TestResolveAllParallelCachedDeduped(t *testing.T) {
	var mu sync.Mutex
	calls := map[string]int{}
	running, most := 0, 0
	old := lookupIPs
	t.Cleanup(func() { lookupIPs = old; SetDNSChoice("x"); SetDNSChoice("") })
	lookupIPs = func(domain string) ([]string, []string) {
		mu.Lock()
		calls[domain]++
		running++
		most = max(most, running)
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		running--
		mu.Unlock()
		if domain == "gone.example" {
			return nil, nil
		}
		return []string{"192.0.2.1", "192.0.2." + strconv.Itoa(len(domain))}, []string{"2001:db8::1"}
	}
	SetDNSChoice("fresh") // empties the cache
	var domains []string
	for i := 0; i < 40; i++ {
		domains = append(domains, "d"+strconv.Itoa(i)+".example")
	}
	domains = append(domains, "gone.example")
	start := time.Now()
	v4, v6 := resolveAll(domains)
	if time.Since(start) > 400*time.Millisecond || most < 2 {
		t.Errorf("lookups weren't parallel: %s, at most %d at once", time.Since(start), most)
	}
	if len(v6) != 1 || v4[0] != "192.0.2.1" || len(v4) != 3 { // .1, .10, .11
		t.Errorf("v4 %v v6 %v", v4, v6)
	}
	resolveAll(domains)
	if calls["d1.example"] != 1 || calls["gone.example"] != 1 {
		t.Errorf("the cache wasn't used: %v", calls)
	}
}

func TestRestoreRules(t *testing.T) {
	got := restoreRules([]string{"192.0.2.1"}, []string{"9.9.9.9"})
	want := "*filter\n:KC9K - [0:0]\n-A KC9K -d 192.0.2.1 -j DROP\n-A KC9K -d 9.9.9.9 -p tcp --dport 443 -j DROP\nCOMMIT\n"
	if got != want {
		t.Errorf("restoreRules =\n%s\nwant\n%s", got, want)
	}
}

func TestNftRules(t *testing.T) {
	got := nftRules([]string{"192.0.2.1", "192.0.2.2"}, nil)
	for _, want := range []string{
		"add table inet kc9k\ndelete table inet kc9k\ntable inet kc9k {",
		"set blocked4 {\n\t\ttype ipv4_addr\n\t\tflags interval\n\t\tauto-merge\n\t\telements = { 192.0.2.1, 192.0.2.2 }\n\t}",
		"set blocked6 {\n\t\ttype ipv6_addr\n\t\tflags interval\n\t\tauto-merge\n\t}",
		"chain output {\n\t\ttype filter hook output priority filter - 10; policy accept;\n\t\tip daddr @blocked4 drop",
		"chain forward {",
		"ip daddr @doh4 tcp dport 443 drop",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("nftRules lacks %q:\n%s", want, got)
		}
	}
}
