package blocker

import (
	"strings"
	"testing"
)

// W19: a Lenovo-like answer of the read script, as PowerShell prints it.
func TestWMIStatus(t *testing.T) {
	data := `{"vendor":"lenovo","password":false,"settings":[
		{"name":"USBBootSupport","current":"Enable","possible":["Disable","Enable"]},
		{"name":"BootOrderLock","current":"Disable","possible":["Disable","Enable"]},
		{"name":"F12Option","current":"Enable","possible":["Disable","Enable"]},
		{"name":"WakeOnLAN","current":"ACOnly","possible":["Disable","ACOnly","Enable"]},
		{"name":"NetworkBoot","current":"Disable","possible":["Disable","Enable"]}]}`
	s, vendor, err := wmiStatus([]byte(data))
	if err != nil || vendor != "lenovo" || !s.Available() || s.Drivers[0] != "thinklmi" || s.Password {
		t.Fatalf("status %+v, %q, %v", s, vendor, err)
	}
	got := map[string]FirmwareSetting{}
	for _, set := range s.Settings {
		got[set.Name] = set
	}
	if _, ok := got["WakeOnLAN"]; ok {
		t.Error("a setting with no purpose is looked after")
	}
	if u := got["USBBootSupport"]; u.Want != "Disable" || u.Done || u.Purpose != "USB boot" {
		t.Errorf("USB boot %+v", u)
	}
	if b := got["BootOrderLock"]; b.Want != "Enable" {
		t.Errorf("boot order lock %+v", b)
	}
	if n := got["NetworkBoot"]; !n.Done {
		t.Errorf("network boot already off: %+v", n)
	}
	req, before := lockRequest(vendor, "pw", s)
	if !req.SetPassword || req.NewPassword != "pw" || req.Password != "" || len(req.Changes) != 3 || before["thinklmi/USBBootSupport"] != "Enable" {
		t.Errorf("lock request %+v, before %v", req, before)
	}
	un := unlockRequest(vendor, "pw", before)
	if !un.SetPassword || un.NewPassword != "" || un.Password != "pw" || len(un.Changes) != 3 {
		t.Errorf("unlock request %+v", un)
	}
	// HP marks the current value with "*" in some versions.
	s, _, _ = wmiStatus([]byte(`{"vendor":"hp","password":true,"settings":[{"name":"USB Storage Boot","current":"*Disable","possible":["Disable","Enable"]}]}`))
	if len(s.Settings) != 1 || !s.Settings[0].Done || !s.Password {
		t.Errorf("HP %+v", s)
	}
	if s, _, _ := wmiStatus([]byte(`{"vendor":"","settings":[]}`)); s.Available() {
		t.Error("no vendor's interface counts as available")
	}
	if err := checkWMIPassword("lenovo", "a,b"); err == nil || !strings.Contains(err.Error(), "comma") {
		t.Errorf("a Lenovo password with a comma: %v", err)
	}
}
