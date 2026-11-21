package blocker

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeFirmware builds a firmware-attributes tree like think-lmi's (Lenovo)
// and points FirmwareRoot at it. Never the real /sys.
func fakeFirmware(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	old := FirmwareRoot
	FirmwareRoot = root
	t.Cleanup(func() { FirmwareRoot = old })
	write := func(rel, value string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	attr := func(name, current, possible string) {
		write("thinklmi/attributes/"+name+"/current_value", current)
		write("thinklmi/attributes/"+name+"/possible_values", possible)
		write("thinklmi/attributes/"+name+"/type", "enumeration")
	}
	attr("USBBootSupport", "Enable", "Enable;Disable")
	attr("IPv4NetworkStack", "Enable", "Enable;Disable")
	attr("BootDeviceListF12Option", "Enable", "Disable;Enable")
	attr("BootOrderLock", "Disable", "Disable;Enable")
	attr("BootMode", "UEFI Only", "UEFI Only;Both")
	attr("USBPortAccess", "Enable", "Enable;Disable")
	write("thinklmi/attributes/BootOrder/current_value", "NVMe0:USBHDD")
	write("thinklmi/attributes/BootOrder/type", "string")
	write("thinklmi/attributes/pending_reboot", "0")
	write("thinklmi/authentication/Admin/is_enabled", "0")
	write("thinklmi/authentication/Admin/role", "bios-admin")
	write("thinklmi/authentication/Admin/current_password", "")
	write("thinklmi/authentication/Admin/new_password", "")
	return root
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Vendors name the settings differently; each purpose is found by its words.
func TestFirmwarePurposeNames(t *testing.T) {
	for name, want := range map[string]string{
		"USBBootSupport":          "USB boot",
		"USB Storage Boot":        "USB boot",
		"Removable Media Boot":    "USB boot",
		"Network (PXE) Boot":      "network boot",
		"PXEBootToLAN":            "network boot",
		"UefiNwStack":             "network boot",
		"IPv6NetworkStack":        "network boot",
		"BootDeviceListF12Option": "one-time boot menu",
		"BootMenu":                "one-time boot menu",
		"BootOrderLock":           "boot order lock",
		"Fixed Boot Order":        "boot order lock",
		"BootOrder":               "",
		"USBPortAccess":           "",
		"LegacyUSBBoot":           "",
		"BootMenuTimeout":         "",
		"SecureBoot":              "",
	} {
		p, ok := purposeOf(name)
		if got := map[bool]string{true: p.Name}[ok]; got != want {
			t.Errorf("%q → %q, want %q", name, got, want)
		}
	}
	if wantedValue("Enabled;Disabled", false) != "Disabled" || wantedValue("Enable;Disable", true) != "Enable" ||
		wantedValue("First;Last", false) != "" {
		t.Error("wantedValue picks the wrong value")
	}
}

// §3.8 firmware lock: the settings first, then the password; with a
// password, every change authenticates; off puts back what it changed.
func TestFirmwareLockOnFakeFirmware(t *testing.T) {
	root := fakeFirmware(t)
	attrs := filepath.Join(root, "thinklmi", "attributes")
	admin := filepath.Join(root, "thinklmi", "authentication", "Admin")

	s, err := FirmwareLockStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !s.Available() || s.Password || s.Locked() || len(s.Settings) != 4 || len(s.Missing()) != 0 {
		t.Fatalf("status before = %+v (missing %v)", s, s.Missing())
	}
	if _, err := ApplyFirmwareLock(""); err == nil {
		t.Error("a lock without a password")
	}
	before, err := ApplyFirmwareLock("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"USBBootSupport": "Disable", "IPv4NetworkStack": "Disable",
		"BootDeviceListF12Option": "Disable", "BootOrderLock": "Enable", "USBPortAccess": "Enable\n", "BootMode": "UEFI Only\n"} {
		if got := readFile(t, filepath.Join(attrs, name, "current_value")); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if got := readFile(t, filepath.Join(admin, "new_password")); got != "hunter2" {
		t.Errorf("new_password = %q", got)
	}
	if before["thinklmi/USBBootSupport"] != "Enable" || before["thinklmi/BootOrderLock"] != "Disable" || len(before) != 4 {
		t.Errorf("before = %v", before)
	}
	// The firmware now has the password.
	if err := os.WriteFile(filepath.Join(admin, "is_enabled"), []byte("1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if s, _ = FirmwareLockStatus(); !s.Locked() {
		t.Errorf("not locked: %+v", s)
	}
	// Something changed USB boot back (the firmware's own setup, with the
	// password): applying again needs the password and only fixes that one.
	_ = os.WriteFile(filepath.Join(attrs, "USBBootSupport", "current_value"), []byte("Enable\n"), 0644)
	_ = os.WriteFile(filepath.Join(admin, "new_password"), nil, 0644)
	if _, err := ApplyFirmwareLock("x"); err != nil {
		t.Fatal(err) // the given password is passed on; the firmware judges it
	}
	if got := readFile(t, filepath.Join(admin, "current_password")); got != "x" {
		t.Errorf("not authenticated before the change: %q", got)
	}
	if got := readFile(t, filepath.Join(admin, "new_password")); got != "" {
		t.Errorf("an existing password was changed: %q", got)
	}
	if err := RemoveFirmwareLock("hunter2", before); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"USBBootSupport": "Enable", "IPv4NetworkStack": "Enable",
		"BootDeviceListF12Option": "Enable", "BootOrderLock": "Disable"} {
		if got := readFile(t, filepath.Join(attrs, name, "current_value")); got != want {
			t.Errorf("after off %s = %q, want %q", name, got, want)
		}
	}
	if readFile(t, filepath.Join(admin, "current_password")) != "hunter2" || readFile(t, filepath.Join(admin, "new_password")) != "" {
		t.Error("password not cleared with the current one")
	}
	// A saved key can't lead outside the firmware folder.
	if err := RemoveFirmwareLock("hunter2", map[string]string{"../../etc/x": "1", "thinklmi/../x": "1"}); err != nil {
		t.Fatal(err)
	}
}

// No firmware-attributes folder: nothing to change, and the lock says so.
func TestFirmwareLockUnavailable(t *testing.T) {
	old := FirmwareRoot
	FirmwareRoot = filepath.Join(t.TempDir(), "none")
	t.Cleanup(func() { FirmwareRoot = old })
	s, err := FirmwareLockStatus()
	if err != nil || s.Available() || s.Locked() {
		t.Fatalf("status = %+v, %v", s, err)
	}
	if _, err := ApplyFirmwareLock("pw"); err == nil {
		t.Error("applied without firmware settings")
	}
}
