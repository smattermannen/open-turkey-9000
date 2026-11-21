package blocker

// netcut_windows.go — monk mode's physical cut on Windows: Wi-Fi and wired
// network adapters switched off and on again (NetAdapter, through
// PowerShell, whose answers aren't translated: netsh's are). The WFP cut
// (firewall_windows.go) holds whatever this does.

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
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

func powershell(script string) ([]byte, error) {
	ps := filepath.Join(systemRoot(), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	return exec.Command(ps, "-NoProfile", "-NonInteractive", "-Command", script).Output()
}

func isWifi(a netAdapter) bool {
	m := strings.ToLower(a.PhysicalMediaType)
	return strings.Contains(m, "802.11") || strings.Contains(m, "wireless")
}

// WifiRadio reports whether Wi-Fi is on: any Wi-Fi adapter switched on.
func WifiRadio() (on, known bool) {
	list, err := netAdapters()
	if err != nil {
		return false, false
	}
	for _, a := range list {
		if isWifi(a) {
			known = true
			on = on || a.AdminStatus == 1
		}
	}
	return on, known
}

// SetWifiRadio switches every Wi-Fi adapter on or off.
func SetWifiRadio(on bool) error {
	list, err := netAdapters()
	if err != nil {
		return err
	}
	for _, a := range list {
		if isWifi(a) {
			if err := setAdapter(a.Name, on); err != nil {
				return err
			}
		}
	}
	return nil
}

// EthernetLinks are the wired adapters and whether each is on.
func EthernetLinks() map[string]bool {
	links := map[string]bool{}
	list, err := netAdapters()
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
	quoted := "'" + strings.ReplaceAll(name, "'", "''") + "'"
	if out, err := powershell(verb + " -Name " + quoted + " -Confirm:$false -IncludeHidden"); err != nil {
		return fmt.Errorf("%s %s: %v: %s", verb, name, err, strings.TrimSpace(string(out)))
	}
	return nil
}
