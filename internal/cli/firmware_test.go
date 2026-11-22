package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
)

// fakeFirmwareRoot gives the CLI a generated firmware-attributes tree (an
// HP-like one); the real /sys is never reached.
func fakeFirmwareRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	old := blocker.FirmwareRoot
	blocker.FirmwareRoot = root
	t.Cleanup(func() { blocker.FirmwareRoot = old })
	write := func(rel, value string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, current := range map[string]string{"USB Storage Boot": "Enable", "Network (PXE) Boot": "Enable"} {
		write("hp-bioscfg/attributes/"+name+"/current_value", current)
		write("hp-bioscfg/attributes/"+name+"/possible_values", "Disable;Enable")
		write("hp-bioscfg/attributes/"+name+"/type", "enumeration")
	}
	write("hp-bioscfg/authentication/Setup Password/is_enabled", "0")
	write("hp-bioscfg/authentication/Setup Password/current_password", "")
	write("hp-bioscfg/authentication/Setup Password/new_password", "")
	return root
}

// §3.8 firmware lock, §4.10 Device protection: on is a lock (R-LOCK-12),
// the password comes from standard input; off is refused while anything
// is locked (logged, R-WEAK-2), and puts back what it changed.
func TestFirmwareLockCommand(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	root := fakeFirmwareRoot(t)
	setup := filepath.Join(root, "hp-bioscfg", "authentication", "Setup Password")
	usb := filepath.Join(root, "hp-bioscfg", "attributes", "USB Storage Boot", "current_value")
	read := func(path string) string { data, _ := os.ReadFile(path); return string(data) }

	if err := runWith(t, 0, []string{"secret"}, "firmware-lock", "on"); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("not confirmed: %v", err)
	}
	if read(usb) != "Enable" || read(filepath.Join(setup, "new_password")) != "" {
		t.Fatal("changed before it was confirmed")
	}
	if err := runWith(t, 0, []string{"secret"}, "firmware-lock", "on", "--yes"); err != nil {
		t.Fatal(err)
	}
	if read(usb) != "Disable" || read(filepath.Join(setup, "new_password")) != "secret" {
		t.Fatalf("usb %q, password %q", read(usb), read(filepath.Join(setup, "new_password")))
	}
	if v, _ := d.GetSetting(SettingFirmwareLock); v != "on" {
		t.Errorf("setting = %q", v)
	}
	if v, _ := d.GetSetting(SettingFirmwareBefore); !strings.Contains(v, `"hp-bioscfg/USB Storage Boot":"Enable"`) {
		t.Errorf("before = %q", v)
	}
	_ = os.WriteFile(filepath.Join(setup, "is_enabled"), []byte("1"), 0o644) // the firmware took it

	var status struct {
		Locked  bool     `json:"locked"`
		Missing []string `json:"missing"`
	}
	if err := json.Unmarshal([]byte(capture(t, "firmware-lock", "--json")), &status); err != nil {
		t.Fatal(err)
	}
	if !status.Locked || len(status.Missing) != 2 { // HP here has no boot menu or boot order lock setting
		t.Errorf("status = %+v", status)
	}

	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runWith(t, 0, []string{"secret"}, "firmware-lock", "off"); err == nil {
		t.Fatal("firmware lock removed while a block is locked")
	}
	if read(usb) != "Disable" {
		t.Error("changed by a refused off")
	}
	events, _ := d.RecentEvents(0, 20)
	logged := false
	for _, e := range events {
		logged = logged || (e.Kind == db.EventRefused && strings.Contains(e.Detail, "firmware lock"))
	}
	if !logged {
		t.Error("the refusal isn't logged (R-WEAK-2)")
	}
	if err := d.DeactivateBlock("b"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"secret"}, "firmware-lock", "off"); err != nil {
		t.Fatal(err)
	}
	if read(usb) != "Enable" || read(filepath.Join(setup, "current_password")) != "secret" || read(filepath.Join(setup, "new_password")) != "" {
		t.Errorf("off: usb %q, current %q, new %q", read(usb), read(filepath.Join(setup, "current_password")), read(filepath.Join(setup, "new_password")))
	}
}

// A test's database never reaches the real firmware.
func TestFirmwareLockDevicelessWithTestDB(t *testing.T) {
	testDB(t)
	notTerminal(t)
	if !firmwareDeviceless() {
		t.Fatal("KC9K_DB with the real firmware folder would touch it")
	}
	if err := runWith(t, 0, []string{"secret"}, "firmware-lock", "on", "--yes"); err == nil {
		t.Error("firmware-lock on did something with a test database")
	}
}
