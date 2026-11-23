package blocker

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// W26: KC9K's policies.json mirrored into Firefox's registry key as
// Firefox reads Group Policy, and only KC9K's taken out again.
func TestRegPolicyFirefoxMirror(t *testing.T) {
	const key = `SOFTWARE\KC9K-test\Mozilla\Firefox`
	if k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, key, registry.ALL_ACCESS); err != nil {
		t.Skipf("no write access to HKLM: %v", err)
	} else {
		_ = k.SetStringValue("TheirPolicy", "kept")
		k.Close()
	}
	defer deleteRegTree(registry.LOCAL_MACHINE, `SOFTWARE\KC9K-test`)
	oldKey, oldOwned := firefoxRegKey, firefoxRegOwnedPath
	firefoxRegKey, firefoxRegOwnedPath = key, filepath.Join(t.TempDir(), "owned.json")
	defer func() { firefoxRegKey, firefoxRegOwnedPath = oldKey, oldOwned }()
	doc := `{"policies":{"WebsiteFilter":{"Block":["*://*.example.com/*","*://example.org/*"],"Exceptions":[]},
		"DNSOverHTTPS":{"Enabled":false,"Locked":true},"DisableDeveloperTools":true,"SearchEngines":{"Default":"Google"}}}`
	if err := syncFirefoxRegistry([]byte(doc)); err != nil {
		t.Fatal(err)
	}
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, key+`\WebsiteFilter\Block`, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	if v, _, err := k.GetStringValue("2"); err != nil || v != "*://example.org/*" {
		t.Errorf("WebsiteFilter\\Block\\2 = %q, %v", v, err)
	}
	k.Close()
	k, _ = registry.OpenKey(registry.LOCAL_MACHINE, key+`\DNSOverHTTPS`, registry.QUERY_VALUE)
	if v, _, err := k.GetIntegerValue("Locked"); err != nil || v != 1 {
		t.Errorf("DNSOverHTTPS\\Locked = %d, %v", v, err)
	}
	k.Close()
	if err := syncFirefoxRegistry(nil); err != nil {
		t.Fatal(err)
	}
	k, _ = registry.OpenKey(registry.LOCAL_MACHINE, key, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	defer k.Close()
	subs, _ := k.ReadSubKeyNames(-1)
	if v, _, err := k.GetStringValue("TheirPolicy"); err != nil || v != "kept" || len(subs) != 0 {
		t.Errorf("after removal: their policy %q (%v), keys left %v", v, err, subs)
	}
	if _, _, err := k.GetIntegerValue("DisableDeveloperTools"); err == nil {
		t.Error("KC9K's DisableDeveloperTools stayed")
	}
}

// W26: a name the hosts file sends to 0.0.0.0 gets no IPv6 address from
// Windows either (else ":: name" lines are needed).
func TestWFPHostsIPv6(t *testing.T) {
	if os.Getenv("KC9K_TEST_WFP") != "1" {
		t.Skip("changes the hosts file; KC9K_TEST_WFP=1 (CI) only")
	}
	const name = "www.google.com"
	if ips, err := net.LookupIP(name); err != nil || len(ips) == 0 {
		t.Skipf("no DNS here: %v", err)
	}
	orig, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.WriteFile(hostsPath, orig, 0o644); hostsWritten() }()
	if err := os.WriteFile(hostsPath, append(append([]byte{}, orig...), []byte("\r\n0.0.0.0 "+name+"\r\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	hostsWritten()
	time.Sleep(time.Second)
	ips, _ := net.DefaultResolver.LookupIP(context.Background(), "ip6", name)
	t.Logf("AAAA of %s with only 0.0.0.0 in hosts: %v", name, ips)
	for _, ip := range ips {
		if !ip.IsUnspecified() {
			t.Errorf("an IPv6 address got past the hosts file: %v", ip)
		}
	}
}

// 2.12: Firefox (installed and the Microsoft Store's) and Edge are
// supported browsers on Windows once KC9K can give them the extension;
// recognised by their program's path alone.
func TestWinBrowserSupport(t *testing.T) {
	oldStore, oldSigned := chromeStoreReady, mozillaSigned
	t.Cleanup(func() { chromeStoreReady, mozillaSigned = oldStore, oldSigned })
	pf := programFilesDir()
	paths := map[string]string{
		filepath.Join(pf, "Mozilla Firefox", "firefox.exe"):                                                                                            "firefox",
		strings.ToUpper(filepath.Join(pf, "Mozilla Firefox", "FIREFOX.EXE")):                                                                           "firefox",
		filepath.Join(pf, "WindowsApps", "Mozilla.Firefox_133.0.0.0_x64__n80bbvh6b1yt2", "VFS", "ProgramFiles", "Firefox Package Root", "firefox.exe"): "firefox",
		filepath.Join(programFilesX86Dir(), "Microsoft", "Edge", "Application", "msedge.exe"):                                                          "edge",
		filepath.Join(pf, "Google", "Chrome", "Application", "chrome.exe"):                                                                             "chrome",
		filepath.Join(pf, "LibreWolf", "librewolf.exe"):                                                                                                "librewolf",
		filepath.Join(pf, "WindowsApps", "Mozilla.Firefox_1_x64__n", "evil", "firefox.exe"):                                                            "",
		filepath.Join(`C:\Users\me\Downloads`, "Mozilla Firefox", "firefox.exe"):                                                                       "",
	}
	// Nothing ready: only LibreWolf.
	chromeStoreReady, mozillaSigned = func() bool { return false }, func() bool { return false }
	for p, id := range paths {
		want := ""
		if id == "librewolf" {
			want = id
		}
		if got := supportedExeOf(p); got != want {
			t.Errorf("nothing ready: %s → %q, want %q", p, got, want)
		}
	}
	for id, want := range map[string]string{"chrome": WaitingChromeWebStore, "edge": WaitingChromeWebStore, "firefox": WaitingMozillaSignature, "librewolf": ""} {
		b, ok := SupportedBrowserByID(id)
		if !ok || b.Waiting() != want {
			t.Errorf("%s waits for %q, want %q", id, b.Waiting(), want)
		}
	}
	// Store listing and Mozilla's signature there: all of them.
	chromeStoreReady, mozillaSigned = func() bool { return true }, func() bool { return true }
	for p, want := range paths {
		if got := supportedExeOf(p); got != want {
			t.Errorf("all ready: %s → %q, want %q", p, got, want)
		}
	}
}

// 2.12, 2.17: with the store ID set, Chrome and Edge get the extension
// force-installed from the Chrome Web Store, each pinned in its own words.
func TestWinEdgeReadyWithStoreID(t *testing.T) {
	old := chromeStoreID
	chromeStoreID = "abcdefghijklmnopabcdefghijklmnop"
	t.Cleanup(func() { chromeStoreID = old })
	for _, b := range chromiumBrowsers {
		p := chromeSetupFor(ChromeSetup{ExtensionID: "x"}, b)
		list, _ := p["ExtensionInstallForcelist"].([]string)
		if len(list) != 1 || list[0] != chromeStoreID+";"+chromeStoreUpdateURL {
			t.Errorf("%s forcelist %v", b.name, list)
		}
		settings, _ := p["ExtensionSettings"].(map[string]any)
		ext, _ := settings[chromeStoreID].(map[string]any)
		if ext["installation_mode"] != "force_installed" || ext["update_url"] != chromeStoreUpdateURL {
			t.Errorf("%s settings %v", b.name, ext)
		}
		wantKey := "toolbar_pin"
		if b.name == "Microsoft Edge" {
			wantKey = "toolbar_state"
		}
		if _, ok := ext[wantKey]; !ok {
			t.Errorf("%s: no %s in %v", b.name, wantKey, ext)
		}
	}
	chromeStoreID = ""
	if _, ok := chromeSetupFor(ChromeSetup{ExtensionID: "x"}, chromiumBrowsers[2])["ExtensionSettings"]; ok {
		t.Error("Edge got an extension without the store ID")
	}
}
