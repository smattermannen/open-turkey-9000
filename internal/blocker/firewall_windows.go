package blocker

// firewall_windows.go — the firewall layer on Windows: filters in the
// Windows Filtering Platform under KC9K's own provider and sublayer, not
// Windows Firewall rules (the first thing an administrator turns off;
// turning Windows Firewall off leaves WFP filters of other providers in
// force). The same job as nftables on Linux: drop the real addresses of
// blocked sites and the DNS-over-HTTPS resolvers; with cut (monk mode),
// everything but loopback. Filters are persistent: they hold from boot,
// before the service starts, and after it stops.

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"

	"github.com/smattermannen/kc9k/internal/winapi"
	"golang.org/x/sys/windows"
)

var (
	wfpProvider = windows.GUID{Data1: 0x6b9d0a5c, Data2: 0x5f42, Data3: 0x4b8e, Data4: [8]byte{0x9b, 0x1d, 0x4b, 0x43, 0xc9, 0xa9, 0x00, 0x01}}
	wfpSublayer = windows.GUID{Data1: 0x6b9d0a5c, Data2: 0x5f42, Data3: 0x4b8e, Data4: [8]byte{0x9b, 0x1d, 0x4b, 0x43, 0xc9, 0xa9, 0x00, 0x02}}
)

// wfpChunk is how many addresses one filter holds (conditions on one
// field are ORed).
const wfpChunk = 500

// Filter names, so a listing tells what each is for.
const (
	wfpNameBlocked = "KC9K blocked site addresses"
	wfpNameDoH     = "KC9K DNS-over-HTTPS resolvers"
	wfpNameCutOK   = "KC9K monk mode: loopback"
	wfpNameCut     = "KC9K monk mode: no network"
)

var wfpLayers = []windows.GUID{winapi.LayerConnectV4, winapi.LayerConnectV6, winapi.LayerAcceptV4, winapi.LayerAcceptV6}

// wfpSecurity guards KC9K's WFP objects (strict mode, ROADMAP W5); nil is
// the engine's default.
var wfpSecurity = func() *windows.SECURITY_DESCRIPTOR { return nil }

// wfpLast is what the last apply put in, to check it is all still there.
var wfpLast struct {
	sync.Mutex
	names map[string]int // filter name -> how many
	cut   bool
}

// ApplyFirewallCut drops the addresses of domains (looked up first) and
// the DoH resolvers; with cut, everything but loopback.
func ApplyFirewallCut(domains []string, cut bool) error {
	valid := ValidDomains(domains)
	if len(valid) == 0 && !cut {
		return RemoveFirewall()
	}
	var ipv4, ipv6 []string
	if !cut {
		ipv4, ipv6 = resolveAll(valid)
	}
	return applyWFP(ipv4, ipv6, cut)
}

// ApplyFirewallKnown drops the addresses looked up so far and reports
// whether that was every domain's (see the Linux one).
func ApplyFirewallKnown(domains []string) (complete bool, err error) {
	valid := ValidDomains(domains)
	if len(valid) == 0 {
		return true, RemoveFirewall()
	}
	ipv4, ipv6, complete := knownAddresses(valid)
	return complete, applyWFP(ipv4, ipv6, false)
}

// wfpFilters are the filters for these addresses.
func wfpFilters(ipv4, ipv6 []string, cut bool) []winapi.Filter {
	var out []winapi.Filter
	add := func(name string, layer windows.GUID, weight uint8, permit bool, conds []winapi.Condition) {
		key, _ := windows.GenerateGUID()
		out = append(out, winapi.Filter{Key: key, Name: name, Layer: layer, Sublayer: wfpSublayer,
			Provider: wfpProvider, Weight: weight, Permit: permit, Conditions: conds})
	}
	if cut {
		for _, layer := range wfpLayers {
			add(wfpNameCutOK, layer, 15, true, []winapi.Condition{winapi.Loopback()})
			add(wfpNameCut, layer, 1, false, nil)
		}
	}
	v4 := func(ips []string) []winapi.Condition {
		var conds []winapi.Condition
		for _, s := range ips {
			if ip := net.ParseIP(s).To4(); ip != nil {
				conds = append(conds, winapi.V4Address([4]byte(ip)))
			}
		}
		return conds
	}
	v6 := func(ips []string) []winapi.Condition {
		var conds []winapi.Condition
		for _, s := range ips {
			if ip := net.ParseIP(s); ip != nil && ip.To4() == nil {
				conds = append(conds, winapi.V6Address([16]byte(ip.To16())))
			}
		}
		return conds
	}
	chunks := func(layer windows.GUID, conds []winapi.Condition) {
		for len(conds) > 0 {
			n := min(len(conds), wfpChunk)
			add(wfpNameBlocked, layer, 10, false, conds[:n])
			conds = conds[n:]
		}
	}
	chunks(winapi.LayerConnectV4, v4(ipv4))
	chunks(winapi.LayerConnectV6, v6(ipv6))
	// DNS over HTTPS (443), and over TLS and QUIC (853), W26: conditions
	// on one field are ORed, so either port to any of them.
	add(wfpNameDoH, winapi.LayerConnectV4, 10, false, append(v4(dohServersIPv4), winapi.RemotePort(443), winapi.RemotePort(853)))
	add(wfpNameDoH, winapi.LayerConnectV6, 10, false, append(v6(dohServersIPv6), winapi.RemotePort(443), winapi.RemotePort(853)))
	return out
}

// applyWFP replaces KC9K's filters with these, in one transaction.
func applyWFP(ipv4, ipv6 []string, cut bool) error {
	e, err := winapi.OpenEngine()
	if err != nil {
		return fmt.Errorf("filtering engine: %w", err)
	}
	defer e.Close()
	want := wfpFilters(ipv4, ipv6, cut)
	err = e.Transaction(func() error {
		if err := e.AddProvider(wfpProvider, "Killer Chicken 9000", wfpSecurity()); err != nil {
			return fmt.Errorf("provider: %w", err)
		}
		if err := e.AddSublayer(wfpSublayer, wfpProvider, "Killer Chicken 9000", 0xF000, wfpSecurity()); err != nil {
			return fmt.Errorf("sublayer: %w", err)
		}
		if err := deleteWFPFilters(e); err != nil {
			return err
		}
		for _, f := range want {
			if err := e.AddFilter(f, wfpSecurity()); err != nil {
				return fmt.Errorf("filter %q: %w", f.Name, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	names := map[string]int{}
	for _, f := range want {
		names[f.Name]++
	}
	wfpLast.Lock()
	wfpLast.names, wfpLast.cut = names, cut
	wfpLast.Unlock()
	return nil
}

// deleteWFPFilters deletes every filter of KC9K's provider.
func deleteWFPFilters(e *winapi.Engine) error {
	for _, layer := range wfpLayers {
		list, err := e.Filters(wfpProvider, layer)
		if err != nil {
			if winapi.NotFound(err) {
				continue
			}
			return fmt.Errorf("list filters: %w", err)
		}
		for _, f := range list {
			if err := e.DeleteFilter(f.Key); err != nil {
				return fmt.Errorf("delete filter %q: %w", f.Name, err)
			}
		}
	}
	return nil
}

// wfpPresent counts KC9K's filters by name.
func wfpPresent() (map[string]int, error) {
	e, err := winapi.OpenEngine()
	if err != nil {
		return nil, err
	}
	defer e.Close()
	names := map[string]int{}
	for _, layer := range wfpLayers {
		list, err := e.Filters(wfpProvider, layer)
		if err != nil {
			if winapi.NotFound(err) {
				continue
			}
			return nil, err
		}
		for _, f := range list {
			names[f.Name]++
		}
	}
	return names, nil
}

// RemoveFirewall deletes KC9K's filters, sublayer and provider.
func RemoveFirewall() error {
	e, err := winapi.OpenEngine()
	if err != nil {
		return fmt.Errorf("filtering engine: %w", err)
	}
	defer e.Close()
	err = e.Transaction(func() error {
		if err := deleteWFPFilters(e); err != nil {
			return err
		}
		if err := e.DeleteSublayer(wfpSublayer); err != nil {
			return err
		}
		return e.DeleteProvider(wfpProvider)
	})
	if err == nil {
		wfpLast.Lock()
		wfpLast.names = nil
		wfpLast.Unlock()
	}
	return err
}

// IsFirewallApplied reports whether every filter of the last apply is in
// place (one deleted by hand makes it false, so the service applies again);
// with no apply in this process, whether the DoH filters are there.
func IsFirewallApplied() bool {
	have, err := wfpPresent()
	if err != nil {
		return false
	}
	wfpLast.Lock()
	want := wfpLast.names
	wfpLast.Unlock()
	if want == nil {
		return have[wfpNameDoH] == 2
	}
	return sameCounts(have, want)
}

// IsFirewallCut reports whether everything but loopback is dropped.
func IsFirewallCut() bool {
	have, err := wfpPresent()
	return err == nil && have[wfpNameCut] == len(wfpLayers) && have[wfpNameCutOK] == len(wfpLayers)
}

func sameCounts(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// wfpDescribe lists KC9K's filters by name (diagnostics, tests).
func wfpDescribe() string {
	have, err := wfpPresent()
	if err != nil {
		return err.Error()
	}
	var lines []string
	for k, v := range have {
		lines = append(lines, fmt.Sprintf("%s: %d", k, v))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}
