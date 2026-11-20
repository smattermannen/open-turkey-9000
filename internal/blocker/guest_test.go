package blocker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// §4.10 close guest profiles: a policy per Chromium-family browser and a
// LightDM drop-in where LightDM is installed; off removes both.
func TestSyncGuestOff(t *testing.T) {
	dir := t.TempDir()
	oldFiles, oldDir := guestPolicyFiles, lightdmDir
	guestPolicyFiles = []string{filepath.Join(dir, "brave", "kc9k-guest.json"), filepath.Join(dir, "chrome", "kc9k-guest.json")}
	lightdmDir = filepath.Join(dir, "lightdm")
	t.Cleanup(func() { guestPolicyFiles, lightdmDir = oldFiles, oldDir })

	if err := SyncGuestOff(true); err != nil {
		t.Fatal(err)
	}
	for _, p := range guestPolicyFiles {
		if data, err := os.ReadFile(p); err != nil || !strings.Contains(string(data), `"BrowserGuestModeEnabled": false`) {
			t.Errorf("%s: %q %v", p, data, err)
		}
	}
	conf := filepath.Join(lightdmDir, lightdmGuestConf)
	if _, err := os.Stat(conf); !os.IsNotExist(err) {
		t.Error("a LightDM drop-in without LightDM")
	}
	if err := os.MkdirAll(lightdmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SyncGuestOff(true); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(conf); !strings.Contains(string(data), "allow-guest=false") {
		t.Errorf("drop-in: %q", data)
	}
	if err := SyncGuestOff(false); err != nil {
		t.Fatal(err)
	}
	for _, p := range append(guestPolicyFiles, conf) {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s left behind", p)
		}
	}
	if err := SyncGuestOff(false); err != nil { // nothing there: fine
		t.Fatal(err)
	}
}
