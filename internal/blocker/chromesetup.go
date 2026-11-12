package blocker

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ChromeSetup is what stays enforced in Google Chrome and Chromium whether or
// not a block runs, written as a policy file of its own beside the blocklist
// (kc9k.json, see browser.go), so the two never touch each other's settings.
type ChromeSetup struct {
	// ExtensionID is force-installed from UpdateURL (see webext.PackChrome).
	ExtensionID string
	UpdateURL   string
	// DisableDevTools blocks the developer tools everywhere (block-devtools).
	// They are always blocked on the extension itself.
	DisableDevTools bool
	ForceSearch     bool
}

// ChromeSetupPaths are the policy files, one per browser (tests point it
// elsewhere). As with the blocklist, a missing folder is made, so the setup
// is there if the browser is installed later.
var ChromeSetupPaths = []string{
	"/etc/opt/chrome/policies/managed/kc9k-setup.json", // Google Chrome
	"/etc/chromium/policies/managed/kc9k-setup.json",   // Chromium
}

// chromeBinaries are where Google Chrome and Chromium are installed (the
// .deb packages; a portable copy is a browser without the extension, see
// browserdetect.go).
var chromeBinaries = []string{"/opt/google/chrome/chrome", "/usr/lib/chromium/chromium"}

// ChromeInstalled reports whether Google Chrome or Chromium is installed,
// browsers that run the extension (see EnsureChromeSetup).
func ChromeInstalled() bool {
	for _, path := range chromeBinaries {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// ExtensionBrowserInstalled reports whether a browser that runs the KC9K
// extension is installed: LibreWolf, Google Chrome or Chromium.
func ExtensionBrowserInstalled() bool {
	return LibreWolfVersion() != "" || ChromeInstalled()
}

// chromeSetupPolicy returns the policies for s.
//
// Chrome doesn't run extensions in incognito windows or as a guest unless
// the user allows it, which policy can't do, so both are turned off: they
// would be browsers without the extension. Developer mode is off, so no
// other extension can be loaded unpacked under KC9K's ID, and the developer
// tools can't be opened on the extension.
func chromeSetupPolicy(s ChromeSetup) map[string]interface{} {
	devTools := 1 // not on extensions installed by policy
	if s.DisableDevTools {
		devTools = 2 // not at all
	}
	policy := map[string]interface{}{
		"IncognitoModeAvailability":      1,
		"BrowserGuestModeEnabled":        false,
		"ExtensionDeveloperModeSettings": 1,
		"DeveloperToolsAvailability":     devTools,
	}
	if s.ExtensionID != "" {
		policy["ExtensionSettings"] = map[string]interface{}{
			s.ExtensionID: map[string]interface{}{
				"installation_mode": "force_installed",
				"update_url":        s.UpdateURL,
				"toolbar_pin":       "force_pinned",
			},
		}
	}
	if s.ForceSearch {
		policy["DefaultSearchProviderEnabled"] = true
		policy["DefaultSearchProviderName"] = googleWebEngine
		policy["DefaultSearchProviderSearchURL"] = "https://www.google.com/search?q={searchTerms}&udm=14&safe=active"
	}
	return policy
}

// EnsureChromeSetup writes the setup for Google Chrome and Chromium. Chrome
// reads policy changes while it runs: a new extension is installed within
// seconds.
func EnsureChromeSetup(s ChromeSetup) error {
	data, err := json.MarshalIndent(chromeSetupPolicy(s), "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	for _, path := range ChromeSetupPaths {
		if fileHas(path, string(data)) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := atomicWriteFile(path, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

// RemoveChromeSetup removes the setup when uninstalling. Chrome uninstalls an
// extension it was made to install as soon as the policy is gone.
func RemoveChromeSetup() error {
	for _, path := range ChromeSetupPaths {
		if err := removePolicyFile(path); err != nil {
			return err
		}
	}
	return nil
}
