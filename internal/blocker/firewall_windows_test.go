package blocker

import (
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
	"time"
)

// TestWFPBlocks adds KC9K's real filters and checks connections are
// refused, then removes them. It changes the computer's firewall: only on
// a disposable machine (KC9K_TEST_WFP=1, set on the windows-latest runner).
func TestWFPBlocks(t *testing.T) {
	if os.Getenv("KC9K_TEST_WFP") != "1" {
		t.Skip("changes the firewall; set KC9K_TEST_WFP=1 on a disposable machine")
	}
	dial := func(addr string) error {
		c, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err == nil {
			c.Close()
		}
		return err
	}
	refused := func(err error) bool {
		// WFP refuses a blocked connect at once: WSAEACCES.
		var errno syscall.Errno
		return errors.As(err, &errno) && errno == 10013
	}
	if err := dial("1.0.0.1:80"); err != nil {
		t.Skipf("no network to test with: %v", err)
	}
	t.Cleanup(func() {
		if err := RemoveFirewall(); err != nil {
			t.Errorf("RemoveFirewall: %v", err)
		}
	})
	if err := applyWFP([]string{"1.0.0.1"}, nil, false); err != nil {
		t.Fatalf("applyWFP: %v", err)
	}
	t.Log(wfpDescribe())
	if !IsFirewallApplied() {
		t.Error("IsFirewallApplied is false right after applying")
	}
	if IsFirewallCut() {
		t.Error("IsFirewallCut is true without cut")
	}
	if err := dial("1.0.0.1:80"); !refused(err) {
		t.Errorf("a blocked address was reached or failed otherwise: %v", err)
	}
	if err := dial("1.1.1.1:443"); !refused(err) {
		t.Errorf("a DNS-over-HTTPS resolver was reached on 443: %v", err)
	}
	if err := dial("1.1.1.1:80"); err != nil {
		t.Errorf("an address that isn't blocked was refused: %v", err)
	}

	// A filter deleted by hand makes the layer count as not applied.
	have, _ := wfpPresent()
	if have[wfpNameBlocked] != 1 || have[wfpNameDoH] != 2 {
		t.Errorf("filters = %v", have)
	}

	// Applying again replaces, never adds.
	if err := applyWFP([]string{"1.0.0.1", "192.0.2.7"}, nil, false); err != nil {
		t.Fatal(err)
	}
	if have, _ := wfpPresent(); have[wfpNameBlocked] != 1 || have[wfpNameDoH] != 2 {
		t.Errorf("after a second apply, filters = %v", have)
	}

	if err := RemoveFirewall(); err != nil {
		t.Fatal(err)
	}
	if have, _ := wfpPresent(); len(have) != 0 {
		t.Errorf("filters left after RemoveFirewall: %v", have)
	}
	if err := dial("1.0.0.1:80"); err != nil {
		t.Errorf("still blocked after RemoveFirewall: %v", err)
	}
}

// TestWFPLayout checks the filters built for addresses, without the engine.
func TestWFPLayout(t *testing.T) {
	ips := make([]string, 0, wfpChunk+1)
	for i := 0; i <= wfpChunk; i++ {
		ips = append(ips, net.IPv4(10, 0, byte(i>>8), byte(i)).String())
	}
	fs := wfpFilters(ips, []string{"2001:db8::1"}, false)
	count := map[string]int{}
	for _, f := range fs {
		count[f.Name]++
	}
	if count[wfpNameBlocked] != 3 || count[wfpNameDoH] != 2 || count[wfpNameCut] != 0 {
		t.Errorf("filters = %v", count)
	}
	cut := wfpFilters(nil, nil, true)
	count = map[string]int{}
	for _, f := range cut {
		count[f.Name]++
	}
	if count[wfpNameCut] != len(wfpLayers) || count[wfpNameCutOK] != len(wfpLayers) {
		t.Errorf("cut filters = %v", count)
	}
}
