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

// §4.10 guided setup: the checklist for strict mode and no-escape, in its
// order, follows the settings and says what to run.
func TestGuidedSetup(t *testing.T) {
	testDB(t)
	notTerminal(t)
	fakeFirmwareRoot(t)
	dir := t.TempDir()
	shadow, grub := filepath.Join(dir, "shadow"), filepath.Join(dir, "grub.cfg")
	_ = os.WriteFile(shadow, []byte("root:$6$abc:19000:0:99999:7:::\nme:!:1:::\n"), 0o600)
	_ = os.WriteFile(grub, []byte("menuentry 'Linux Mint 22' --class linuxmint --class gnu-linux {\n}\n"+
		"menuentry 'Windows Boot Manager (on /dev/nvme0n1p1)' --class windows --class os $menuentry_id_option 'osprober-efi-1' {\n}\n"), 0o644)
	oldShadow, oldGrub := shadowPath, grubCfgPath
	shadowPath, grubCfgPath = shadow, grub
	t.Cleanup(func() { shadowPath, grubCfgPath = oldShadow, oldGrub })

	steps := func() map[string]setupStep {
		var list []setupStep
		if err := json.Unmarshal([]byte(capture(t, "guided-setup", "--json")), &list); err != nil {
			t.Fatal(err)
		}
		if list[len(list)-1].ID != "no-escape" || list[0].ID != "settings-password" {
			t.Errorf("order: first %s, last %s", list[0].ID, list[len(list)-1].ID)
		}
		out := map[string]setupStep{}
		for _, s := range list {
			out[s.ID] = s
		}
		return out
	}
	s := steps()
	for _, id := range []string{"settings-password", "no-weakening", "tamper", "strict", "strict-lock", "reset-guard", "firmware", "no-escape"} {
		if s[id].State != stepTodo || s[id].Command == "" {
			t.Errorf("%s: %+v", id, s[id])
		}
	}
	if s["root-password"].State != stepTodo || s["firmware"].Command != "kc9k firmware-lock on" {
		t.Errorf("root %+v, firmware %+v", s["root-password"], s["firmware"])
	}
	if o := s["other-systems"]; o.State != stepInfo || !strings.Contains(o.Detail, "Windows Boot Manager") || strings.Contains(o.Detail, "Mint") {
		t.Errorf("other systems: %+v", o)
	}
	for _, args := range [][]string{{"strict", "on"}, {"reset-guard", "on"}, {"no-weakening", "on"},
		{"tamper", "clock", "on"}, {"tamper", "task-managers", "on"}, {"tamper", "private", "on"}} {
		mustRun(t, args...)
	}
	_ = os.WriteFile(shadow, []byte("root:!:19000:0:99999:7:::\n"), 0o600)
	s = steps()
	for _, id := range []string{"strict", "reset-guard", "no-weakening", "root-password"} {
		if s[id].State != stepDone {
			t.Errorf("%s not done: %+v", id, s[id])
		}
	}
	if s["tamper"].State != stepTodo || !strings.Contains(s["tamper"].Detail, "unsupported-browsers") || strings.Contains(s["tamper"].Detail, "clock") {
		t.Errorf("tamper: %+v", s["tamper"])
	}
	_ = os.Remove(shadow)
	blocker.FirmwareRoot = filepath.Join(dir, "none")
	s = steps()
	if s["root-password"].State != stepUnknown || s["firmware"].State != stepByHand {
		t.Errorf("root %+v, firmware %+v", s["root-password"], s["firmware"])
	}
	if out := capture(t, "guided-setup"); !strings.Contains(out, "[x]") || !strings.Contains(out, "→ kc9k no-escape on --yes") {
		t.Errorf("text checklist:\n%s", out)
	}
}
