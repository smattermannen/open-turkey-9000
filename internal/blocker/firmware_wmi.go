package blocker

// firmware_wmi.go — the firmware lock through the firmware's own WMI
// interfaces on Windows (ROADMAP W19; SPEC §6 2026-11-22 By Opus): Dell's
// BIOS attributes, Lenovo's BIOS settings and HP's Instrumented BIOS, the
// same interfaces Linux's firmware-attributes drivers use. A PowerShell
// script reads and writes them (firmware_windows.go); what it answers is
// read here, so it is tested on every system.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// wmiDrivers name each vendor's interface as Linux's driver for it is
// named, so the settings KC9K keeps read the same on both systems.
var wmiDrivers = map[string]string{"dell": "dell-wmi-sysman", "lenovo": "thinklmi", "hp": "hp-bioscfg"}

// wmiReading is the read script's answer.
type wmiReading struct {
	Vendor   string `json:"vendor"`
	Password bool   `json:"password"`
	Settings []struct {
		Name     string   `json:"name"`
		Current  string   `json:"current"`
		Possible []string `json:"possible"`
	} `json:"settings"`
}

// wmiStatus turns the read script's answer into what the lock looks at:
// the settings with a purpose, the value each should have.
func wmiStatus(data []byte) (FirmwareStatus, string, error) {
	var s FirmwareStatus
	var r wmiReading
	if err := json.Unmarshal(data, &r); err != nil {
		return s, "", fmt.Errorf("reading the firmware's settings: %w", err)
	}
	driver := wmiDrivers[r.Vendor]
	if driver == "" {
		return s, "", nil
	}
	s.Drivers, s.Password = []string{driver}, r.Password
	for _, set := range r.Settings {
		p, ok := purposeOf(set.Name)
		if !ok {
			continue
		}
		current := strings.TrimPrefix(strings.TrimSpace(set.Current), "*")
		want := wantedValue(strings.Join(set.Possible, ";"), p.On)
		s.Settings = append(s.Settings, FirmwareSetting{Driver: driver, Name: set.Name, Purpose: p.Name,
			Current: current, Want: want, Done: want != "" && strings.EqualFold(current, want)})
	}
	sort.Slice(s.Settings, func(i, j int) bool { return s.Settings[i].Name < s.Settings[j].Name })
	return s, r.Vendor, nil
}

// wmiChange is one setting to write.
type wmiChange struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// wmiRequest is what the write script is given on its standard input (so
// no password is ever on a command line): the settings to change with the
// current password, then the password to set ("" clears it) when
// SetPassword.
type wmiRequest struct {
	Vendor      string      `json:"vendor"`
	Password    string      `json:"password"`
	Changes     []wmiChange `json:"changes"`
	SetPassword bool        `json:"set_password"`
	NewPassword string      `json:"new_password"`
}

// checkWMIPassword refuses what a vendor's interface can't carry: Lenovo's
// takes the password inside a comma-separated string.
func checkWMIPassword(vendor, password string) error {
	if vendor == "lenovo" && strings.ContainsAny(password, ",;") {
		return fmt.Errorf("Lenovo's firmware interface can't take a password with a comma or semicolon")
	}
	return nil
}

// lockRequest is the request that applies the lock to s: every setting
// not yet as wanted, then the password if none is set. before is what they
// were.
func lockRequest(vendor, password string, s FirmwareStatus) (wmiRequest, map[string]string) {
	req := wmiRequest{Vendor: vendor}
	before := map[string]string{}
	if s.Password {
		req.Password = password
	}
	for _, set := range s.Settings {
		if set.Want == "" || set.Done {
			continue
		}
		req.Changes = append(req.Changes, wmiChange{set.Name, set.Want})
		before[set.Driver+"/"+set.Name] = set.Current
	}
	if !s.Password {
		req.SetPassword, req.NewPassword = true, password
	}
	return req, before
}

// unlockRequest puts back what lockRequest changed (before) and clears the
// password.
func unlockRequest(vendor, password string, before map[string]string) wmiRequest {
	req := wmiRequest{Vendor: vendor, Password: password, SetPassword: true}
	keys := make([]string, 0, len(before))
	for k := range before {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		_, name, ok := strings.Cut(k, "/")
		if ok && name != "" {
			req.Changes = append(req.Changes, wmiChange{name, before[k]})
		}
	}
	return req
}
