package blocker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/winapi"
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

// W14: "close guest profiles" turns the Guest account off and, when it
// ends, back on only if it was on. Needs an administrator (CI).
func TestWinGuestAccount(t *testing.T) {
	if os.Getenv("KC9K_TEST_WFP") == "" {
		t.Skip("changes the computer's Guest account: KC9K_TEST_WFP=1 (CI) only")
	}
	guest, ok := guestAccount()
	if !ok {
		t.Fatal("no Guest account (relative ID 501) found")
	}
	defer func() { _ = winapi.SetAccountDisabled(guest.Name, guest.Disabled) }()
	old := guestOffPath
	guestOffPath = filepath.Join(t.TempDir(), "guest-off")
	defer func() { guestOffPath = old }()
	if err := winapi.SetAccountDisabled(guest.Name, false); err != nil {
		t.Fatal(err)
	}
	if off, err := syncGuestAccount(true); err != nil || !off {
		t.Fatalf("turning the Guest account off: %v, %v", off, err)
	}
	if g, _ := guestAccount(); !g.Disabled {
		t.Error("the Guest account is still on")
	}
	if off, err := syncGuestAccount(true); err != nil || off {
		t.Errorf("a second cycle changed it again: %v, %v", off, err)
	}
	if _, err := syncGuestAccount(false); err != nil {
		t.Fatal(err)
	}
	if g, _ := guestAccount(); g.Disabled {
		t.Error("the Guest account, on before, wasn't put back on")
	}
	if _, err := os.Stat(guestOffPath); err == nil {
		t.Error("the note stayed")
	}
	// Off before: stays off afterwards.
	_ = winapi.SetAccountDisabled(guest.Name, true)
	_, _ = syncGuestAccount(true)
	_, _ = syncGuestAccount(false)
	if g, _ := guestAccount(); !g.Disabled {
		t.Error("the Guest account, off before, was turned on")
	}
}

// W18: winget is asked for the whole computer, silently, with its own
// package; a browser already there isn't installed again.
func TestWinInstallBrowserWinget(t *testing.T) {
	var got []string
	old := installRun
	installRun = func(name string, args ...string) ([]byte, error) {
		got = append([]string{name}, args...)
		return nil, nil
	}
	defer func() { installRun = old }()
	if wingetPath() == "" {
		t.Skip("no winget here")
	}
	err := InstallBrowser("librewolf")
	if LibreWolfVersion() != "" {
		if len(got) != 0 {
			t.Errorf("an installed LibreWolf installed again: %v", got)
		}
		return
	}
	want := "install --id LibreWolf.LibreWolf --exact --scope machine --silent"
	if len(got) == 0 || !strings.Contains(strings.Join(got, " "), want) {
		t.Errorf("winget ran %v, want %q", got, want)
	}
	if err == nil {
		t.Error("a fake install reported LibreWolf installed")
	}
}
