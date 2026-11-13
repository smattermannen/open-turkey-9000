package blocker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestChromeSetup(t *testing.T) {
	dir := t.TempDir()
	saved := ChromeSetupPaths
	ChromeSetupPaths = []string{filepath.Join(dir, "chrome", "kc9k-setup.json"), filepath.Join(dir, "chromium", "kc9k-setup.json")}
	t.Cleanup(func() { ChromeSetupPaths = saved })

	read := func(path string) map[string]interface{} {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var policy map[string]interface{}
		if err := json.Unmarshal(data, &policy); err != nil {
			t.Fatal(err)
		}
		return policy
	}

	if err := EnsureChromeSetup(ChromeSetup{ExtensionID: "abc", UpdateURL: "http://127.0.0.1:47613/chrome/update.xml"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range ChromeSetupPaths {
		policy := read(path)
		ext := policy["ExtensionSettings"].(map[string]interface{})["abc"].(map[string]interface{})
		if ext["installation_mode"] != "force_installed" || ext["update_url"] != "http://127.0.0.1:47613/chrome/update.xml" {
			t.Errorf("%s: extension settings %v", path, ext)
		}
		// Windows the extension can't run in are off; developer tools never
		// reach the extension.
		if policy["IncognitoModeAvailability"] != 1.0 || policy["BrowserGuestModeEnabled"] != false ||
			policy["DeveloperToolsAvailability"] != 1.0 || policy["ExtensionDeveloperModeSettings"] != 1.0 {
			t.Errorf("%s: %v", path, policy)
		}
		if _, ok := policy["DefaultSearchProviderSearchURL"]; ok {
			t.Errorf("%s: search engine forced while force-search is off", path)
		}
	}

	if err := EnsureChromeSetup(ChromeSetup{DisableDevTools: true, ForceSearch: true}); err != nil {
		t.Fatal(err)
	}
	policy := read(ChromeSetupPaths[0])
	if policy["DeveloperToolsAvailability"] != 2.0 || policy["DefaultSearchProviderName"] != googleWebEngine {
		t.Errorf("block-devtools and force-search: %v", policy)
	}

	if err := RemoveChromeSetup(); err != nil {
		t.Fatal(err)
	}
	for _, path := range ChromeSetupPaths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s still there after removal", path)
		}
	}
}
