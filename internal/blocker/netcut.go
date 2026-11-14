// netcut.go — the two physical ways monk mode cuts the connection
// (R-MONK-2): the Wi-Fi radio and the wired interfaces. The third, the
// firewall dropping everything except loopback, is in firewall.go.
package blocker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// sysNet is where the kernel lists network interfaces (replaced in tests).
var sysNet = "/sys/class/net"

// WifiRadio reports whether the Wi-Fi radio is switched on, as
// NetworkManager sees it. known is false when that can't be told (no
// NetworkManager): then there is nothing to switch.
func WifiRadio() (on, known bool) {
	out, err := exec.Command("nmcli", "radio", "wifi").Output()
	if err != nil {
		return false, false
	}
	switch strings.TrimSpace(string(out)) {
	case "enabled":
		return true, true
	case "disabled":
		return false, true
	}
	return false, false
}

// SetWifiRadio switches the Wi-Fi radio on or off.
func SetWifiRadio(on bool) error {
	state := "off"
	if on {
		state = "on"
	}
	if out, err := exec.Command("nmcli", "radio", "wifi", state).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to switch the Wi-Fi radio %s: %v: %s", state, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// EthernetLinks returns every wired (Ethernet) interface of the computer
// and whether it is up: real devices only (USB adapters and phone tethering
// included), not Wi-Fi, loopback, bridges, tunnels or containers' virtual
// pairs, which carry nothing once the real ones and the firewall are shut.
func EthernetLinks() map[string]bool {
	links := map[string]bool{}
	entries, err := os.ReadDir(sysNet)
	if err != nil {
		return links
	}
	for _, e := range entries {
		dir := filepath.Join(sysNet, e.Name())
		// 1 is ARPHRD_ETHER.
		if readTrimmed(filepath.Join(dir, "type")) != "1" {
			continue
		}
		// A device behind it: virtual interfaces have none.
		if _, err := os.Stat(filepath.Join(dir, "device")); err != nil {
			continue
		}
		if exists(filepath.Join(dir, "wireless")) || exists(filepath.Join(dir, "phy80211")) {
			continue
		}
		flags, err := strconv.ParseUint(strings.TrimPrefix(readTrimmed(filepath.Join(dir, "flags")), "0x"), 16, 32)
		if err != nil {
			continue
		}
		links[e.Name()] = flags&1 != 0 // IFF_UP
	}
	return links
}

// SetLink brings a network interface up or down.
func SetLink(name string, up bool) error {
	state := "down"
	if up {
		state = "up"
	}
	if out, err := exec.Command("ip", "link", "set", "dev", name, state).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring %s %s: %v: %s", name, state, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func readTrimmed(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
