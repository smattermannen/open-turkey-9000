package enforce

import (
	"testing"

	"github.com/smattermannen/kc9k/internal/blocker"
)

// W10: the Native Wifi API on a real Windows. GitHub's runner has no WLAN
// service: nothing is seen, with an error, and nothing breaks; a computer
// with Wi-Fi lists what is in range.
func TestWlanScanForReal(t *testing.T) {
	nets, err := scanWifi(false)
	t.Logf("networks: %d, error: %v", len(nets), err)
	for _, n := range nets {
		if len(n.BSSID) != 17 {
			t.Errorf("BSSID %q", n.BSSID)
		}
	}
	startWifiRescan()
	on, known := blocker.WifiRadio() // the real one (TestMain fakes wifiRadio)
	t.Logf("Wi-Fi on %v, known %v", on, known)
}
