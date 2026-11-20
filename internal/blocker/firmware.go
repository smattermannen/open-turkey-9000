// firmware.go — factory reset protection's firmware lock (§3.8, §4.10).
//
// Linux exposes firmware (BIOS/UEFI) settings of Dell, Lenovo and HP
// computers under /sys/class/firmware-attributes/<driver>/: each setting is
// a folder under attributes/ (current_value, possible_values, type), the
// firmware passwords are under authentication/ (Admin: is_enabled,
// current_password, new_password). The firmware lock turns off booting from
// USB and the network and the one-time boot menu, locks the boot order, and
// then sets a firmware password that a trusted person keeps. Without that
// password the boot menu stays open and the settings can be put back.
//
// Every write goes through FirmwareRoot, which tests point at a generated
// tree; nothing here runs a program.
package blocker

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FirmwareRoot is the firmware-attributes class folder.
var FirmwareRoot = "/sys/class/firmware-attributes"

// FirmwarePurpose is what one firmware setting is changed for.
type FirmwarePurpose struct {
	Name string   // shown to the user, e.g. "USB boot"
	On   bool     // the lock wants it on (else off)
	Keys []string // matched against the setting's name, lower case, letters and digits only
	Skip []string // names containing one of these don't match
}

// firmwarePurposes are the settings the lock changes. Vendors name them
// differently (Lenovo USBBootSupport, HP "USB Storage Boot", Dell
// UefiNwStack), so a setting matches when its folded name has every word of
// one key.
var firmwarePurposes = []FirmwarePurpose{
	{Name: "USB boot", Keys: []string{"usb boot", "usbstorage boot", "removable boot", "external boot", "bootfromusb"}, Skip: []string{"order", "legacy"}},
	{Name: "network boot", Keys: []string{"pxe", "network boot", "netboot", "lanboot", "networkstack", "nwstack", "httpboot", "wakeonlanboot"}, Skip: []string{"order"}},
	{Name: "one-time boot menu", Keys: []string{"bootmenu", "f12", "bootdevicelist", "onetimeboot"}, Skip: []string{"timeout", "delay"}},
	{Name: "boot order lock", On: true, Keys: []string{"bootorderlock", "lockbootorder", "fixedbootorder"}},
}

// FirmwareSetting is one firmware setting the lock looks after.
type FirmwareSetting struct {
	Driver  string `json:"driver"`
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
	Current string `json:"current"`
	Want    string `json:"want"` // "" when no possible value fits
	Done    bool   `json:"done"`
}

// FirmwareStatus is what the firmware lets KC9K see.
type FirmwareStatus struct {
	Drivers       []string          `json:"drivers"`
	Password      bool              `json:"password"` // an admin (setup) password is set
	Settings      []FirmwareSetting `json:"settings"`
	PendingReboot bool              `json:"pending_reboot"`
}

// Available reports whether the computer's firmware can be changed from Linux.
func (s FirmwareStatus) Available() bool { return len(s.Drivers) > 0 }

// Locked reports whether the firmware lock is complete: a password, and every
// setting found as the lock wants it.
func (s FirmwareStatus) Locked() bool {
	if !s.Password || !s.Available() {
		return false
	}
	for _, set := range s.Settings {
		if set.Want != "" && !set.Done {
			return false
		}
	}
	return true
}

// Missing lists the purposes no setting was found for: they are done by hand
// in the firmware's own setup.
func (s FirmwareStatus) Missing() []string {
	var out []string
	for _, p := range firmwarePurposes {
		found := false
		for _, set := range s.Settings {
			if set.Purpose == p.Name && set.Want != "" {
				found = true
			}
		}
		if !found {
			out = append(out, p.Name)
		}
	}
	return out
}

func foldName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// purposeOf returns the purpose a setting's name matches, if any.
func purposeOf(name string) (FirmwarePurpose, bool) {
	folded := foldName(name)
	for _, p := range firmwarePurposes {
		skipped := false
		for _, s := range p.Skip {
			if strings.Contains(folded, s) {
				skipped = true
			}
		}
		if skipped {
			continue
		}
		for _, key := range p.Keys {
			all := true
			for _, word := range strings.Fields(key) {
				if !strings.Contains(folded, word) {
					all = false
				}
			}
			if all {
				return p, true
			}
		}
	}
	return FirmwarePurpose{}, false
}

var (
	firmwareOffValues = []string{"disabled", "disable", "off", "no", "false"}
	firmwareOnValues  = []string{"enabled", "enable", "on", "yes", "true"}
)

// wantedValue picks, from a setting's possible values (separated by ';'),
// the one meaning off (or on).
func wantedValue(possible string, on bool) string {
	words := firmwareOffValues
	if on {
		words = firmwareOnValues
	}
	values := strings.Split(strings.TrimSpace(possible), ";")
	for _, w := range words {
		for _, v := range values {
			if strings.EqualFold(strings.TrimSpace(v), w) {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

func readTrim(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// adminDir is a driver's admin (setup) password folder, "" without one.
func adminDir(driver string) string {
	base := filepath.Join(FirmwareRoot, driver, "authentication")
	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		dir := filepath.Join(base, e.Name())
		role := strings.ToLower(readTrim(filepath.Join(dir, "role")))
		if strings.EqualFold(e.Name(), "Admin") || strings.EqualFold(e.Name(), "Setup Password") || role == "bios-admin" {
			return dir
		}
	}
	return ""
}

// FirmwareLockStatus reads the firmware's settings.
func FirmwareLockStatus() (FirmwareStatus, error) {
	var s FirmwareStatus
	entries, err := os.ReadDir(FirmwareRoot)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	for _, e := range entries {
		driver := e.Name()
		attrs := filepath.Join(FirmwareRoot, driver, "attributes")
		if _, err := os.Stat(attrs); err != nil {
			continue
		}
		s.Drivers = append(s.Drivers, driver)
		if dir := adminDir(driver); dir != "" && readTrim(filepath.Join(dir, "is_enabled")) == "1" {
			s.Password = true
		}
		if readTrim(filepath.Join(attrs, "pending_reboot")) == "1" {
			s.PendingReboot = true
		}
		settings, _ := os.ReadDir(attrs)
		for _, a := range settings {
			if !a.IsDir() {
				continue
			}
			p, ok := purposeOf(a.Name())
			if !ok {
				continue
			}
			dir := filepath.Join(attrs, a.Name())
			current := readTrim(filepath.Join(dir, "current_value"))
			want := ""
			if t := readTrim(filepath.Join(dir, "type")); t == "" || t == "enumeration" {
				want = wantedValue(readTrim(filepath.Join(dir, "possible_values")), p.On)
			}
			s.Settings = append(s.Settings, FirmwareSetting{
				Driver: driver, Name: a.Name(), Purpose: p.Name,
				Current: current, Want: want, Done: want != "" && strings.EqualFold(current, want),
			})
		}
	}
	sort.Slice(s.Settings, func(i, j int) bool { return s.Settings[i].Name < s.Settings[j].Name })
	return s, nil
}

// writeSysfs writes one value the way the firmware drivers read it: the
// value alone, no newline.
func writeSysfs(path, value string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(value); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// authenticate gives the drivers that have a password the current one before
// a change.
func authenticate(driver, password string) error {
	dir := adminDir(driver)
	if dir == "" || readTrim(filepath.Join(dir, "is_enabled")) != "1" {
		return nil
	}
	if password == "" {
		return fmt.Errorf("the firmware already has a password: it is needed to change its settings")
	}
	return writeSysfs(filepath.Join(dir, "current_password"), password)
}

// ApplyFirmwareLock changes every setting the lock looks after, then sets the
// password, so the settings are written before a password is needed. With a
// password already set, the given one is that password and stays. It returns
// the values the settings had, by name, to put back when the lock is removed.
func ApplyFirmwareLock(password string) (map[string]string, error) {
	if password == "" {
		return nil, fmt.Errorf("the firmware lock needs a password")
	}
	status, err := FirmwareLockStatus()
	if err != nil {
		return nil, err
	}
	if !status.Available() {
		return nil, fmt.Errorf("this computer's firmware settings can't be changed from Linux (no %s): set the firmware password by hand", FirmwareRoot)
	}
	before := map[string]string{}
	for _, set := range status.Settings {
		if set.Want == "" || set.Done {
			continue
		}
		if err := authenticate(set.Driver, password); err != nil {
			return before, err
		}
		path := filepath.Join(FirmwareRoot, set.Driver, "attributes", set.Name, "current_value")
		if err := writeSysfs(path, set.Want); err != nil {
			return before, fmt.Errorf("firmware setting %s: %w", set.Name, err)
		}
		before[set.Driver+"/"+set.Name] = set.Current
	}
	for _, driver := range status.Drivers {
		dir := adminDir(driver)
		if dir == "" || readTrim(filepath.Join(dir, "is_enabled")) == "1" {
			continue
		}
		if err := writeSysfs(filepath.Join(dir, "new_password"), password); err != nil {
			return before, fmt.Errorf("firmware password: %w", err)
		}
	}
	return before, nil
}

// RemoveFirmwareLock puts back the settings the lock changed (before, from
// ApplyFirmwareLock) and clears the firmware password, which is needed.
func RemoveFirmwareLock(password string, before map[string]string) error {
	status, err := FirmwareLockStatus()
	if err != nil {
		return err
	}
	if !status.Available() {
		return fmt.Errorf("no firmware settings found under %s", FirmwareRoot)
	}
	keys := make([]string, 0, len(before))
	for k := range before {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		driver, name, ok := strings.Cut(key, "/")
		if !ok || strings.Contains(name, "/") || strings.Contains(driver, "..") || strings.Contains(name, "..") {
			continue
		}
		if err := authenticate(driver, password); err != nil {
			return err
		}
		if err := writeSysfs(filepath.Join(FirmwareRoot, driver, "attributes", name, "current_value"), before[key]); err != nil {
			return fmt.Errorf("firmware setting %s: %w", name, err)
		}
	}
	for _, driver := range status.Drivers {
		dir := adminDir(driver)
		if dir == "" || readTrim(filepath.Join(dir, "is_enabled")) != "1" {
			continue
		}
		if err := writeSysfs(filepath.Join(dir, "current_password"), password); err != nil {
			return err
		}
		if err := writeSysfs(filepath.Join(dir, "new_password"), ""); err != nil {
			return fmt.Errorf("firmware password: %w", err)
		}
	}
	return nil
}
