package blocker

// browser_windows.go — browser policies on Windows (ROADMAP W2). Chromium
// browsers read policies from the registry only (HKLM\SOFTWARE\Policies\
// <vendor>\<browser>), which works on Windows Home: no Group Policy Editor.
// The blocklist and the setup each own their values in the same key and
// never touch the other's. Firefox-family browsers read
// distribution\policies.json in their installation folder: KC9K keeps its
// own copy in its data folder (firefoxPolicyPath, SYSTEM only) and writes
// it to every installation it finds.

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/smattermannen/kc9k/internal/platform"
	"github.com/smattermannen/kc9k/internal/winapi"
	"golang.org/x/sys/windows/registry"
)

// firefoxPolicyPath is KC9K's own copy of the Firefox-family policies.
var firefoxPolicyPath = filepath.Join(platform.DataDir, "firefox", "policies.json")

// alwaysPolicyFiles are written whether or not a browser is installed.
var alwaysPolicyFiles = []string{firefoxPolicyPath}

// otherBrowserNames names browsers besides LibreWolf by their programs'
// names (without .exe), for closing them all.
var otherBrowserNames = []string{
	"firefox", "chrome", "msedge", "brave", "opera", "opera_gx", "vivaldi", "chromium", "iexplore",
	"waterfox", "floorp", "zen", "palemoon", "yandex", "browser", "arc", "thorium", "librewolf-portable",
	"tor", "seamonkey", "maxthon", "whale", "duckduckgo",
}

// librewolfConfigs are LibreWolf's autoconfig files.
var librewolfConfigs = []string{
	filepath.Join(programFilesDir(), "LibreWolf", "librewolf.cfg"),
}

func programFilesDir() string {
	if d := os.Getenv("ProgramFiles"); d != "" {
		return d
	}
	return `C:\Program Files`
}

func programFilesX86Dir() string {
	if d := os.Getenv("ProgramFiles(x86)"); d != "" {
		return d
	}
	return `C:\Program Files (x86)`
}

// firefoxInstallDirs are the Firefox-family installations: the usual
// folders, those the registry's App Paths name, and per-user ones.
func firefoxInstallDirs() []string {
	seen := map[string]bool{}
	var out []string
	add := func(dir string) {
		key := strings.ToLower(filepath.Clean(dir))
		if dir == "" || seen[key] || !dirExists(dir) {
			return
		}
		seen[key] = true
		out = append(out, dir)
	}
	for _, base := range []string{programFilesDir(), programFilesX86Dir()} {
		for _, name := range []string{"Mozilla Firefox", "Firefox Developer Edition", "Firefox Nightly", "LibreWolf", "Waterfox", "Floorp", "Zen Browser"} {
			add(filepath.Join(base, name))
		}
	}
	for _, exe := range []string{"firefox.exe", "librewolf.exe", "waterfox.exe", "floorp.exe", "zen.exe"} {
		if path := appPath(exe); path != "" {
			add(filepath.Dir(path))
		}
	}
	// Per-user installations (%LOCALAPPDATA%\Mozilla Firefox) of every
	// account.
	users, _ := filepath.Glob(filepath.Join(os.Getenv("SystemDrive")+`\`, "Users", "*", "AppData", "Local", "Mozilla Firefox"))
	for _, d := range users {
		add(d)
	}
	return out
}

// appPath is the program the registry's App Paths name for exe, or "".
func appPath(exe string) string {
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		k, err := registry.OpenKey(root, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\`+exe, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := k.GetStringValue("")
		k.Close()
		if err == nil && v != "" {
			return strings.Trim(v, `"`)
		}
	}
	return ""
}

// chromiumBrowser is one Chromium browser's policy key.
type chromiumBrowser struct {
	name string
	key  string // under HKLM
	// incognito is its name for the private window policy.
	incognito string
	// pin is how its ExtensionSettings keeps the extension's button in
	// the toolbar: Chrome's toolbar_pin, Edge's own toolbar_state.
	pin [2]string
}

var (
	chromePin = [2]string{"toolbar_pin", "force_pinned"}
	edgePin   = [2]string{"toolbar_state", "force_shown"}
)

var chromiumBrowsers = []chromiumBrowser{
	{"Google Chrome", `SOFTWARE\Policies\Google\Chrome`, "IncognitoModeAvailability", chromePin},
	{"Chromium", `SOFTWARE\Policies\Chromium`, "IncognitoModeAvailability", chromePin},
	{"Microsoft Edge", `SOFTWARE\Policies\Microsoft\Edge`, "InPrivateModeAvailability", edgePin},
	{"Brave", `SOFTWARE\Policies\BraveSoftware\Brave`, "IncognitoModeAvailability", chromePin},
	{"Vivaldi", `SOFTWARE\Policies\Vivaldi`, "IncognitoModeAvailability", chromePin},
}

// blocklistValues are the values the blocklist owns in each key.
var blocklistValues = []string{"URLBlocklist", "URLAllowlist", "DnsOverHttpsMode", "BuiltInDnsClientEnabled"}

// setupValues are the values the setup owns in each key (with the
// browser's name for private windows).
var setupValues = []string{"BrowserGuestModeEnabled", "ExtensionDeveloperModeSettings", "DeveloperToolsAvailability",
	"ExtensionSettings", "ExtensionInstallForcelist", "DefaultSearchProviderEnabled", "DefaultSearchProviderName",
	"DefaultSearchProviderSearchURL"}

// writeRegPolicy makes key hold policy's values, and removes the values
// in owned that policy doesn't set. Lists are subkeys of numbered strings;
// dictionaries are JSON strings; booleans and numbers are DWORDs.
func writeRegPolicy(keyPath string, policy map[string]any, owned []string) error {
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, keyPath, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("policy key %s: %w", keyPath, err)
	}
	defer k.Close()
	for _, name := range owned {
		if _, set := policy[name]; set {
			continue
		}
		_ = k.DeleteValue(name)
		_ = deleteRegTree(k, name)
	}
	names := make([]string, 0, len(policy))
	for name := range policy {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := writeRegValue(k, name, policy[name]); err != nil {
			return fmt.Errorf("policy %s\\%s: %w", keyPath, name, err)
		}
	}
	return nil
}

func writeRegValue(k registry.Key, name string, v any) error {
	switch v := v.(type) {
	case bool:
		n := uint32(0)
		if v {
			n = 1
		}
		_ = deleteRegTree(k, name)
		return k.SetDWordValue(name, n)
	case int:
		_ = deleteRegTree(k, name)
		return k.SetDWordValue(name, uint32(v))
	case string:
		_ = deleteRegTree(k, name)
		return k.SetStringValue(name, v)
	case []string:
		_ = k.DeleteValue(name)
		_ = deleteRegTree(k, name)
		sub, _, err := registry.CreateKey(k, name, registry.ALL_ACCESS)
		if err != nil {
			return err
		}
		defer sub.Close()
		for i, s := range v {
			if err := sub.SetStringValue(strconv.Itoa(i+1), s); err != nil {
				return err
			}
		}
		return nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_ = deleteRegTree(k, name)
		return k.SetStringValue(name, string(data))
	}
}

// readRegValue reads a value as writeRegValue wrote it, for comparing.
func readRegValue(k registry.Key, name string, like any) (any, bool) {
	switch like.(type) {
	case bool:
		n, _, err := k.GetIntegerValue(name)
		return n == 1, err == nil
	case int:
		n, _, err := k.GetIntegerValue(name)
		return int(n), err == nil
	case []string:
		sub, err := registry.OpenKey(k, name, registry.QUERY_VALUE)
		if err != nil {
			return nil, false
		}
		defer sub.Close()
		names, err := sub.ReadValueNames(0)
		if err != nil {
			return nil, false
		}
		var out []string
		for i := 1; i <= len(names); i++ {
			s, _, err := sub.GetStringValue(strconv.Itoa(i))
			if err != nil {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		s, _, err := k.GetStringValue(name)
		if err != nil {
			return nil, false
		}
		if _, isString := like.(string); isString {
			return s, true
		}
		want, _ := json.Marshal(like)
		return string(want) == s, true
	}
}

// regPolicyHas reports whether key holds every value of policy.
func regPolicyHas(keyPath string, policy map[string]any) bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return false
	}
	defer k.Close()
	for name, want := range policy {
		got, ok := readRegValue(k, name, want)
		if !ok {
			return false
		}
		switch w := want.(type) {
		case []string:
			if !sameSet(got.([]string), w) || len(got.([]string)) != len(w) {
				return false
			}
		case map[string]any, []any:
			if got != true {
				return false
			}
		default:
			if got != want {
				return false
			}
		}
	}
	return true
}

// regValuesPresent reports whether key holds any of names.
func regValuesPresent(keyPath string, names []string) bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return false
	}
	defer k.Close()
	for _, n := range names {
		if _, _, err := k.GetValue(n, nil); err == nil {
			return true
		}
		if sub, err := registry.OpenKey(k, n, registry.QUERY_VALUE); err == nil {
			sub.Close()
			return true
		}
	}
	return false
}

// removeRegValues deletes names (values or list subkeys) from key.
func removeRegValues(keyPath string, names []string) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.ALL_ACCESS)
	if err == registry.ErrNotExist {
		return nil
	}
	if err != nil {
		return err
	}
	defer k.Close()
	for _, n := range names {
		_ = k.DeleteValue(n)
		_ = deleteRegTree(k, n)
	}
	return nil
}

// deleteRegTree deletes subkey name of k and everything under it.
func deleteRegTree(k registry.Key, name string) error {
	sub, err := registry.OpenKey(k, name, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	children, _ := sub.ReadSubKeyNames(0)
	for _, c := range children {
		_ = deleteRegTree(sub, c)
	}
	sub.Close()
	return registry.DeleteKey(k, name)
}

// chromiumBlocklist is the blocklist's policy for one browser.
func chromiumBlocklist(block, allow []string) map[string]any {
	p := map[string]any{"URLBlocklist": block, "DnsOverHttpsMode": "off", "BuiltInDnsClientEnabled": false}
	if len(allow) > 0 {
		p["URLAllowlist"] = allow
	}
	return p
}

// ApplyBrowserPolicies writes the blocking policies for every browser.
func ApplyBrowserPolicies(p BrowserPolicy) error {
	sets := setsOf(p)
	if sets.empty {
		return RemoveBrowserPolicies()
	}
	if err := applyFirefoxPolicy(sets.ffBlock, sets.ffExceptions); err != nil {
		return fmt.Errorf("failed to apply Firefox policy: %w", err)
	}
	if err := distributeFirefoxPolicy(); err != nil {
		return fmt.Errorf("failed to apply Firefox policy: %w", err)
	}
	for _, b := range chromiumBrowsers {
		if err := writeRegPolicy(b.key, chromiumBlocklist(sets.crBlock, sets.crAllow), blocklistValues); err != nil {
			return err
		}
	}
	return nil
}

// distributeFirefoxPolicy writes KC9K's copy to every installation.
func distributeFirefoxPolicy() error {
	data, err := os.ReadFile(firefoxPolicyPath)
	if err != nil {
		return err
	}
	if err := syncFirefoxRegistry(data); err != nil {
		return err
	}
	for _, dir := range firefoxInstallDirs() {
		target := filepath.Join(dir, "distribution", "policies.json")
		if fileHas(target, string(data)) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := atomicWriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// RemoveBrowserPolicies removes the blocking policies from every browser.
func RemoveBrowserPolicies() error {
	if err := removeFirefoxPolicy(); err != nil {
		return fmt.Errorf("failed to remove Firefox policy: %w", err)
	}
	if _, err := os.Stat(firefoxPolicyPath); err == nil {
		if err := distributeFirefoxPolicy(); err != nil {
			return err
		}
	} else {
		if err := syncFirefoxRegistry(nil); err != nil {
			return err
		}
		for _, dir := range firefoxInstallDirs() {
			if err := removeFirefoxBlocking(filepath.Join(dir, "distribution", "policies.json")); err != nil {
				return err
			}
		}
	}
	for _, b := range chromiumBrowsers {
		if err := removeRegValues(b.key, blocklistValues); err != nil {
			return err
		}
	}
	return nil
}

// removeFirefoxBlocking takes WebsiteFilter and DNSOverHTTPS out of a
// policy file without KC9K's copy to go by.
func removeFirefoxBlocking(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var root map[string]any
	if json.Unmarshal(data, &root) != nil {
		return nil
	}
	policies, _ := root["policies"].(map[string]any)
	if policies == nil {
		return nil
	}
	delete(policies, "WebsiteFilter")
	delete(policies, "DNSOverHTTPS")
	if len(policies) == 0 {
		return removePolicyFile(path)
	}
	return writeJSON(path, root)
}

// IsBrowserPoliciesApplied reports whether every browser has the policies
// (with nothing to block: that none are left).
func IsBrowserPoliciesApplied(p BrowserPolicy) bool {
	sets := setsOf(p)
	if sets.empty {
		return !BrowserPoliciesPresent()
	}
	if !checkFirefoxPolicy(sets.ffBlock, sets.ffExceptions) {
		return false
	}
	data, err := os.ReadFile(firefoxPolicyPath)
	if err != nil {
		return false
	}
	for _, dir := range firefoxInstallDirs() {
		if !fileHas(filepath.Join(dir, "distribution", "policies.json"), string(data)) {
			return false
		}
	}
	want := chromiumBlocklist(sets.crBlock, sets.crAllow)
	for _, b := range chromiumBrowsers {
		if !regPolicyHas(b.key, want) {
			return false
		}
	}
	return true
}

// BrowserPoliciesPresent reports whether any blocking policy is left.
func BrowserPoliciesPresent() bool {
	for _, b := range chromiumBrowsers {
		if regValuesPresent(b.key, []string{"URLBlocklist", "URLAllowlist"}) {
			return true
		}
	}
	paths := []string{firefoxPolicyPath}
	for _, dir := range firefoxInstallDirs() {
		paths = append(paths, filepath.Join(dir, "distribution", "policies.json"))
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var root map[string]any
		if json.Unmarshal(content, &root) != nil {
			continue
		}
		policies, _ := root["policies"].(map[string]any)
		_, hasFilter := policies["WebsiteFilter"]
		_, hasDoH := policies["DNSOverHTTPS"]
		if hasFilter || hasDoH {
			return true
		}
	}
	return false
}

// ChromeStoreExtensionID is the extension's ID in the Chrome Web Store
// (its unlisted listing, SPEC §6 2026-11-21): Chrome and Edge on Windows
// force-install only store extensions. Empty until the owner publishes
// the listing; until then Chrome and Edge get no extension, and the
// extension guard treats them as running without it. Setting it is the
// only step left for both (2.17): Edge force-installs Chrome Web Store
// extensions from the same update URL.
const ChromeStoreExtensionID = ""

// chromeStoreID is ChromeStoreExtensionID; tests set it.
var chromeStoreID = ChromeStoreExtensionID

// chromeStoreUpdateURL is the Chrome Web Store's update URL.
const chromeStoreUpdateURL = "https://clients2.google.com/service/update2/crx"

// chromeSetupFor is the setup's policy for browser b.
func chromeSetupFor(s ChromeSetup, b chromiumBrowser) map[string]any {
	devTools := 1
	if s.DisableDevTools {
		devTools = 2
	}
	p := map[string]any{
		b.incognito:                      1,
		"BrowserGuestModeEnabled":        false,
		"ExtensionDeveloperModeSettings": 1,
		"DeveloperToolsAvailability":     devTools,
	}
	if s.ExtensionID != "" && chromeStoreID != "" {
		p["ExtensionSettings"] = map[string]any{
			chromeStoreID: map[string]any{
				"installation_mode": "force_installed",
				"update_url":        chromeStoreUpdateURL,
				b.pin[0]:            b.pin[1],
			},
		}
		p["ExtensionInstallForcelist"] = []string{chromeStoreID + ";" + chromeStoreUpdateURL}
	}
	if s.ForceSearch {
		p["DefaultSearchProviderEnabled"] = true
		p["DefaultSearchProviderName"] = googleWebEngine
		p["DefaultSearchProviderSearchURL"] = "https://www.google.com/search?q={searchTerms}&udm=14&safe=active"
	}
	return p
}

// EnsureChromeSetup writes the setup's policy for every Chromium browser.
func EnsureChromeSetup(s ChromeSetup) error {
	for _, b := range chromiumBrowsers {
		want := chromeSetupFor(s, b)
		if regPolicyHas(b.key, want) && !regExtra(b, want) {
			continue
		}
		if err := writeRegPolicy(b.key, want, append([]string{b.incognito}, setupValues...)); err != nil {
			return err
		}
	}
	return nil
}

// regExtra reports whether b's key holds a setup value want doesn't set.
func regExtra(b chromiumBrowser, want map[string]any) bool {
	var extra []string
	for _, n := range setupValues {
		if _, ok := want[n]; !ok {
			extra = append(extra, n)
		}
	}
	return regValuesPresent(b.key, extra)
}

// RemoveChromeSetup removes the setup's policy from every Chromium browser.
func RemoveChromeSetup() error {
	for _, b := range chromiumBrowsers {
		if err := removeRegValues(b.key, append([]string{b.incognito}, setupValues...)); err != nil {
			return err
		}
	}
	return nil
}

// SyncGuestOff: guest browsing is off in every Chromium browser on Windows
// whatever the setting (chromeSetupFor), and Windows has no guest session
// at sign-in (its Guest account is off unless an administrator turns it
// on). Nothing more to write.
func SyncGuestOff(on bool) error {
	_, err := syncGuestAccount(on)
	return err
}

// guestOffPath notes that "close guest profiles" turned the Guest account
// off, and that it was on before.
var guestOffPath = filepath.Join(platform.DataDir, "guest-off")

// guestAccount is the built-in Guest account (relative ID 501) by its
// local name, which Windows translates.
func guestAccount() (winapi.Account, bool) {
	accounts, err := winapi.LocalAccounts()
	if err != nil {
		return winapi.Account{}, false
	}
	for _, a := range accounts {
		if a.RID == 501 {
			return a, true
		}
	}
	return winapi.Account{}, false
}

// syncGuestAccount keeps the Guest account off while on (W14): Windows'
// guest session at sign-in. It is put back on afterwards only if it was on
// when the setting turned it off. It reports whether it turned it off now.
func syncGuestAccount(on bool) (bool, error) {
	guest, ok := guestAccount()
	if !ok {
		return false, nil
	}
	_, statErr := os.Stat(guestOffPath)
	noted := statErr == nil
	switch {
	case on && !guest.Disabled:
		if !noted {
			if err := os.WriteFile(guestOffPath, []byte("the Guest account was on\n"), 0o600); err != nil {
				return false, err
			}
		}
		log.Printf("guest: turning the Guest account off (close guest profiles)")
		return true, winapi.SetAccountDisabled(guest.Name, true)
	case !on && noted:
		if guest.Disabled {
			if err := winapi.SetAccountDisabled(guest.Name, false); err != nil {
				return false, err
			}
		}
		return false, os.Remove(guestOffPath)
	}
	return false, nil
}
