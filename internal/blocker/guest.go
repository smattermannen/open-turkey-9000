//go:build linux

// guest.go — tamper protection "close guest profiles" (§4.10): guest
// browsing off in every Chromium-family browser (Chrome and Chromium have it
// off always: the extension can't run there, chromesetup.go) and no guest
// session at LightDM's login screen. Firefox and LibreWolf have no guest
// mode. Paths are vars so tests write into a scratch folder.
package blocker

import (
	"os"
	"path/filepath"
)

var (
	// guestPolicyFiles are the extra managed-policy files, one per
	// Chromium-family browser (next to chromiumPolicyPaths' kc9k.json).
	guestPolicyFiles = func() []string {
		out := make([]string, 0, len(chromiumPolicyPaths))
		for _, p := range chromiumPolicyPaths {
			out = append(out, filepath.Join(filepath.Dir(p), "kc9k-guest.json"))
		}
		return out
	}()
	// lightdmDir is LightDM's configuration; the drop-in goes only where it
	// is installed.
	lightdmDir       = "/etc/lightdm"
	lightdmGuestConf = "lightdm.conf.d/90-kc9k-no-guest.conf"
)

const (
	guestPolicy = "{\n  \"BrowserGuestModeEnabled\": false\n}\n"
	// LightDM reads allow-guest; Mint's greeters read greeter-allow-guest.
	lightdmNoGuest = "# Killer Chicken 9000: tamper protection \"close guest profiles\"\n[Seat:*]\nallow-guest=false\ngreeter-allow-guest=false\n"
)

// SyncGuestOff writes the guest settings when on, and removes them when off.
func SyncGuestOff(on bool) error {
	conf := filepath.Join(lightdmDir, lightdmGuestConf)
	if !on {
		for _, path := range append(append([]string{}, guestPolicyFiles...), conf) {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	}
	for _, path := range guestPolicyFiles {
		if err := EnsureFile(path, []byte(guestPolicy)); err != nil {
			return err
		}
	}
	if info, err := os.Stat(lightdmDir); err == nil && info.IsDir() {
		return EnsureFile(conf, []byte(lightdmNoGuest))
	}
	return nil
}
