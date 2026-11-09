package blocker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// BrowserSetup is what stays enforced in Firefox-family browsers whether or not
// a block runs.
type BrowserSetup struct {
	ExtensionID  string // force-installed when set
	ExtensionXPI string
	ForceSearch  bool
	// DisableDevTools blocks the developer tools, which can switch the
	// extension off from its own pages or about:debugging.
	DisableDevTools bool
	// DisablePrivate turns off private windows (no-private).
	DisablePrivate bool
	// RemoveExtensionID is set when uninstalling: browsers remove that
	// extension at their next start. Without it, a policy-installed
	// extension would stay behind and keep enforcing its last rules forever,
	// with no daemon to update them.
	RemoveExtensionID string
}

const googleWebEngine = "Google Web"

// otherPolicyDirs are installation folders that read a distribution policy
// file. The first target list is written always; these only if installed.
var (
	alwaysPolicyFiles = []string{firefoxPolicyPath, "/etc/librewolf/policies/policies.json"}
	installDirs       = []string{"/usr/lib/librewolf", "/usr/share/librewolf", "/usr/lib/firefox", "/usr/lib/firefox-esr"}
)

// ForcedBrowserKillList names other browsers, closed when LibreWolf is forced.
var ForcedBrowserKillList = []string{
	"firefox", "firefox-bin", "firefox-esr", "chrome", "google-chrome", "chromium", "chromium-browser",
	"brave", "brave-browser", "opera", "vivaldi-bin", "msedge", "microsoft-edge", "epiphany",
	"falkon", "konqueror", "midori", "qutebrowser", "palemoon", "waterfox", "floorp", "zen", "tor", "torbrowser",
}

func setupPolicies(s BrowserSetup, policies map[string]interface{}) {
	delete(policies, "ExtensionSettings")
	delete(policies, "SearchEngines")
	delete(policies, "DisableDeveloperTools")
	delete(policies, "DisablePrivateBrowsing")
	if s.DisableDevTools {
		policies["DisableDeveloperTools"] = true
	}
	if s.DisablePrivate {
		policies["DisablePrivateBrowsing"] = true
	}
	if s.RemoveExtensionID != "" {
		policies["ExtensionSettings"] = map[string]interface{}{
			s.RemoveExtensionID: map[string]interface{}{"installation_mode": "blocked"},
		}
	}
	if s.ExtensionID != "" {
		policies["ExtensionSettings"] = map[string]interface{}{
			s.ExtensionID: map[string]interface{}{
				"installation_mode": "force_installed",
				"install_url":       "file://" + s.ExtensionXPI,
				"private_browsing":  true,
			},
		}
	}
	if s.ForceSearch {
		policies["SearchEngines"] = map[string]interface{}{
			"Add": []map[string]string{{
				"Name":        googleWebEngine,
				"URLTemplate": "https://www.google.com/search?q={searchTerms}&udm=14&safe=active",
				"Method":      "GET",
			}},
			"Default":         googleWebEngine,
			"PreventInstalls": true,
			"Remove":          []string{"Bing", "DuckDuckGo", "Ecosia", "Perplexity", "Qwant", "Startpage", "Searx", "MetaGer", "Mojeek"},
		}
	}
}

// EnsureBrowserSetup writes the setup into every Firefox-family policy file.
func EnsureBrowserSetup(s BrowserSetup) error {
	root := map[string]interface{}{}
	if data, err := os.ReadFile(firefoxPolicyPath); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			root = map[string]interface{}{}
		}
	}
	policies, ok := root["policies"].(map[string]interface{})
	if !ok {
		policies = map[string]interface{}{}
	}
	setupPolicies(s, policies)
	root["policies"] = policies

	targets := append([]string{}, alwaysPolicyFiles...)
	for _, dir := range installDirs {
		if dirExists(dir) {
			targets = append(targets, filepath.Join(dir, "distribution", "policies.json"))
		}
	}

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	for _, target := range targets {
		if len(policies) == 0 {
			if err := removePolicyFile(target); err != nil {
				return err
			}
			continue
		}
		if fileHas(target, string(data)) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := atomicWriteFile(target, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

// EnsureFile writes data to path if it differs.
func EnsureFile(path string, data []byte) error {
	if fileHas(path, string(data)) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return atomicWriteFile(path, data, 0644)
}

const (
	safeSearchBegin = "# KC9K-SAFESEARCH-BEGIN"
	safeSearchEnd   = "# KC9K-SAFESEARCH-END"
	// safeSearchFallback is the address in Google's documentation, used when
	// looking up forcesafesearch.google.com fails.
	safeSearchFallback = "216.239.38.120"
)

var safeSearchHosts = []string{"www.google.com", "google.com", "www.google.se", "google.se"}

func stripSafeSearch(content string) string {
	var out []string
	inside := false
	for _, line := range strings.Split(content, "\n") {
		switch {
		case line == safeSearchBegin:
			inside = true
		case line == safeSearchEnd:
			inside = false
		case !inside:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// renderSafeSearch returns hosts content with the SafeSearch section.
func renderSafeSearch(content, ip string) string {
	lines := []string{safeSearchBegin}
	for _, host := range safeSearchHosts {
		lines = append(lines, ip+" "+host)
	}
	lines = append(lines, safeSearchEnd)
	return strings.TrimRight(stripSafeSearch(content), "\n") + "\n" + strings.Join(lines, "\n") + "\n"
}

// SyncSafeSearch keeps Google on its forced SafeSearch address, restoring the
// entries if someone removes them.
func SyncSafeSearch(on bool) error {
	data, err := os.ReadFile(hostsPath)
	if err != nil {
		return err
	}
	content := string(data)
	want := stripSafeSearch(content)
	if on {
		if strings.Contains(content, safeSearchBegin) && strings.Contains(content, safeSearchEnd) {
			return nil
		}
		ip := safeSearchFallback
		if v4, _ := resolveIPs("forcesafesearch.google.com"); len(v4) > 0 {
			ip = v4[0]
		}
		want = renderSafeSearch(content, ip)
	}
	if want == content {
		return nil
	}
	return atomicWriteFile(hostsPath, []byte(want), 0644)
}

const signingPref = `lockPref("xpinstall.signatures.required", false); // kc9k`

var librewolfConfigs = []string{"/usr/share/librewolf/librewolf.cfg", "/usr/lib/librewolf/librewolf.cfg"}

// withSigningPref adds or removes the line that lets the bundled unsigned
// extension load.
func withSigningPref(config string, on bool) string {
	// Leave the file alone when the line is already there, wherever it is,
	// so this and withLocks never take turns rewriting the file.
	if on && strings.Contains(config, "\n"+signingPref+"\n") || on && strings.HasPrefix(config, signingPref+"\n") {
		return config
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimRight(config, "\n"), "\n") {
		if line != signingPref {
			lines = append(lines, line)
		}
	}
	if on {
		lines = append(lines, signingPref)
	}
	return strings.Join(lines, "\n") + "\n"
}

// SyncLibreWolfSigning keeps the setting in place, since a LibreWolf update can
// replace the file.
func SyncLibreWolfSigning(on bool) error {
	for _, path := range librewolfConfigs {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		want := withSigningPref(string(data), on)
		if want != string(data) {
			if err := atomicWriteFile(path, []byte(want), 0644); err != nil {
				return err
			}
		}
	}
	return nil
}
