package enforce

// wifi_windows.go — the access points in range on Windows (ROADMAP W10),
// from the Native Wifi API of every Wi-Fi interface. Without the WLAN
// service (no Wi-Fi) nothing is seen, which counts as being at every place.

import (
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/winapi"
)

// scanWifi reads what every interface saw last; rescan asks them to look
// again first and waits for the answer (Windows gives it within 4 s).
func scanWifi(rescan bool) ([]db.Network, error) {
	w, err := winapi.OpenWlan()
	if err != nil {
		return nil, err
	}
	defer w.Close()
	ifaces, err := w.Interfaces()
	if err != nil {
		return nil, err
	}
	if rescan {
		for _, i := range ifaces {
			_ = w.Scan(i)
		}
		time.Sleep(4 * time.Second)
	}
	var nets []db.Network
	seen := map[string]bool{}
	var firstErr error
	for _, i := range ifaces {
		list, err := w.BSSList(i)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, b := range list {
			if !seen[b.BSSID] {
				seen[b.BSSID] = true
				nets = append(nets, db.Network{BSSID: strings.ToUpper(b.BSSID), SSID: b.SSID})
			}
		}
	}
	if len(nets) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return nets, nil
}

func startWifiRescan() {
	w, err := winapi.OpenWlan()
	if err != nil {
		return
	}
	defer w.Close()
	ifaces, _ := w.Interfaces()
	for _, i := range ifaces {
		_ = w.Scan(i)
	}
}
