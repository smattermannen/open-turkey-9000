//go:build linux

package enforce

// wifi_linux.go — the access points in range, from NetworkManager.

import (
	"os/exec"

	"github.com/smattermannen/kc9k/internal/db"
)

func scanWifi(rescan bool) ([]db.Network, error) {
	mode := "no"
	if rescan {
		mode = "yes"
	}
	out, err := exec.Command("nmcli", "-t", "-f", "BSSID,SSID", "dev", "wifi", "list", "--rescan", mode).Output()
	if err != nil {
		return nil, err
	}
	return parseNmcli(string(out)), nil
}

func startWifiRescan() { _ = exec.Command("nmcli", "dev", "wifi", "rescan").Run() }
