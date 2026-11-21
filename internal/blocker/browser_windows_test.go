package blocker

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

// TestRegPolicy writes, checks and removes policies in a scratch key of
// HKLM (needs an administrator: skipped otherwise).
func TestRegPolicy(t *testing.T) {
	const key = `SOFTWARE\KC9K-test\Policies`
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, key, registry.ALL_ACCESS)
	if err != nil {
		t.Skipf("no write access to HKLM: %v", err)
	}
	k.Close()
	t.Cleanup(func() {
		root, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE`, registry.ALL_ACCESS)
		if err == nil {
			_ = deleteRegTree(root, "KC9K-test")
			root.Close()
		}
	})
	b := chromiumBrowser{name: "Test", key: key, incognito: "IncognitoModeAvailability"}
	block := chromiumBlocklist([]string{"example.com", "example.org"}, []string{"ok.example.com"})
	if err := writeRegPolicy(key, block, blocklistValues); err != nil {
		t.Fatal(err)
	}
	if !regPolicyHas(key, block) {
		t.Fatal("the blocklist written isn't read back")
	}
	// A list changed by hand is noticed.
	k, _ = registry.OpenKey(registry.LOCAL_MACHINE, key+`\URLBlocklist`, registry.SET_VALUE)
	_ = k.DeleteValue("2")
	k.Close()
	if regPolicyHas(key, block) {
		t.Error("a removed list entry went unnoticed")
	}
	// The setup owns other values in the same key and leaves the
	// blocklist's alone.
	setup := chromeSetupFor(ChromeSetup{ExtensionID: "x", ForceSearch: true}, b)
	if err := writeRegPolicy(key, setup, append([]string{b.incognito}, setupValues...)); err != nil {
		t.Fatal(err)
	}
	if !regPolicyHas(key, setup) {
		t.Error("the setup written isn't read back")
	}
	if !regValuesPresent(key, []string{"URLBlocklist"}) {
		t.Error("writing the setup removed the blocklist")
	}
	// Turning force-search off removes its values.
	setup = chromeSetupFor(ChromeSetup{}, b)
	if err := writeRegPolicy(key, setup, append([]string{b.incognito}, setupValues...)); err != nil {
		t.Fatal(err)
	}
	if regValuesPresent(key, []string{"DefaultSearchProviderSearchURL"}) {
		t.Error("a setup value no longer wanted is still there")
	}
	if err := removeRegValues(key, blocklistValues); err != nil {
		t.Fatal(err)
	}
	if regValuesPresent(key, []string{"URLBlocklist", "URLAllowlist"}) {
		t.Error("the blocklist is still there after removal")
	}
}
