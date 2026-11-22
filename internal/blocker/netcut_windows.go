package blocker

// netcut_windows.go — monk mode's physical cut on Windows: Wi-Fi and wired
// network adapters switched off and on again (NetAdapter, through
// PowerShell, whose answers aren't translated: netsh's are). The WFP cut
// (firewall_windows.go) holds whatever this does.

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/smattermannen/kc9k/internal/winapi"
	"golang.org/x/sys/windows"
)

type netAdapter struct {
	Name              string `json:"Name"`
	AdminStatus       int    `json:"AdminStatus"` // 1 up, 2 down
	PhysicalMediaType string `json:"PhysicalMediaType"`
}

// netAdapters lists the physical adapters; replaced in tests.
var netAdapters = func() ([]netAdapter, error) {
	out, err := powershell(`Get-NetAdapter -Physical -IncludeHidden | Select-Object Name,@{n='AdminStatus';e={[int]$_.AdminStatus}},PhysicalMediaType | ConvertTo-Json -Compress`)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return nil, nil
	}
	if !strings.HasPrefix(text, "[") {
		text = "[" + text + "]"
	}
	var list []netAdapter
	if err := json.Unmarshal([]byte(text), &list); err != nil {
		return nil, err
	}
	return list, nil
}

// adapterCache keeps the adapter list a minute: monk mode's cut and the
// places' Wi-Fi check ask every cycle, and each list starts PowerShell
// (W50 audit). A change KC9K makes drops it.
var adapterCache struct {
	sync.Mutex
	list []netAdapter
	at   time.Time
}

// adapters is netAdapters, at most once a minute.
func adapters() ([]netAdapter, error) {
	adapterCache.Lock()
	defer adapterCache.Unlock()
	if !adapterCache.at.IsZero() && time.Since(adapterCache.at) < time.Minute {
		return adapterCache.list, nil
	}
	list, err := netAdapters()
	if err != nil {
		return nil, err
	}
	adapterCache.list, adapterCache.at = list, time.Now()
	return list, nil
}

func forgetAdapters() {
	adapterCache.Lock()
	adapterCache.at = time.Time{}
	adapterCache.Unlock()
}

func powershell(script string) ([]byte, error) {
	ps := filepath.Join(systemRoot(), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, ps, "-NoProfile", "-NonInteractive", "-Command", script).Output()
}

func isWifi(a netAdapter) bool {
	m := strings.ToLower(a.PhysicalMediaType)
	return strings.Contains(m, "802.11") || strings.Contains(m, "wireless")
}

// WifiRadio reports whether Wi-Fi is on: a Wi-Fi adapter enabled and its
// radio's software switch on (Windows' "Wi-Fi off" and airplane mode turn
// the switch off and leave the adapter enabled).
func WifiRadio() (on, known bool) {
	list, err := adapters()
	if err != nil {
		return false, false
	}
	for _, a := range list {
		if isWifi(a) {
			known = true
			on = on || a.AdminStatus == 1
		}
	}
	if on {
		if radios, ok := wifiSoftwareRadios(); ok {
			on = false
			for _, r := range radios {
				on = on || r.Software == 1
			}
		}
	}
	return on, known
}

// wlanRadio is one radio of one Wi-Fi interface.
type wlanRadio struct {
	iface    windows.GUID
	phy      uint32
	Software uint32 // 1 on, 2 off
	Hardware uint32
}

// wifiSoftwareRadios lists every Wi-Fi radio; false without the WLAN
// service. A variable for tests.
var wifiSoftwareRadios = func() ([]wlanRadio, bool) {
	w, err := winapi.OpenWlan()
	if err != nil {
		return nil, false
	}
	defer w.Close()
	ifaces, err := w.Interfaces()
	if err != nil {
		return nil, false
	}
	var out []wlanRadio
	for _, i := range ifaces {
		states, err := w.Radios(i)
		if err != nil {
			continue
		}
		for _, s := range states {
			out = append(out, wlanRadio{i, s.Phy, s.Software, s.Hardware})
		}
	}
	return out, len(out) > 0
}

// SetWifiRadio switches every Wi-Fi adapter on or off; on also turns each
// radio's software switch on.
func SetWifiRadio(on bool) error {
	list, err := adapters()
	if err != nil {
		return err
	}
	for _, a := range list {
		if isWifi(a) && (a.AdminStatus == 1) != on {
			if err := setAdapter(a.Name, on); err != nil {
				return err
			}
		}
	}
	if !on {
		return nil
	}
	w, err := winapi.OpenWlan()
	if err != nil {
		return nil // no WLAN service: the adapters are all there is
	}
	defer w.Close()
	ifaces, _ := w.Interfaces()
	for _, i := range ifaces {
		states, err := w.Radios(i)
		if err != nil {
			continue
		}
		for _, s := range states {
			if s.Software != 1 {
				if err := w.SetRadioOn(i, s.Phy); err != nil {
					return fmt.Errorf("Wi-Fi radio: %w", err)
				}
			}
		}
	}
	return nil
}

// EthernetLinks are the wired adapters and whether each is on.
func EthernetLinks() map[string]bool {
	links := map[string]bool{}
	list, err := adapters()
	if err != nil {
		return links
	}
	for _, a := range list {
		if !isWifi(a) {
			links[a.Name] = a.AdminStatus == 1
		}
	}
	return links
}

// SetLink switches the adapter name on or off.
func SetLink(name string, up bool) error { return setAdapter(name, up) }

func setAdapter(name string, on bool) error {
	verb := "Disable-NetAdapter"
	if on {
		verb = "Enable-NetAdapter"
	}
	defer forgetAdapters()
	quoted := "'" + strings.ReplaceAll(name, "'", "''") + "'"
	if out, err := powershell(verb + " -Name " + quoted + " -Confirm:$false -IncludeHidden"); err != nil {
		return fmt.Errorf("%s %s: %v: %s", verb, name, err, strings.TrimSpace(string(out)))
	}
	return nil
}
