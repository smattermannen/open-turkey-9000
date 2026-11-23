package blocker

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// fileURL is path as a file: URL, as browsers' policies take it:
// file:///usr/share/... on Linux, file:///C:/Program%20Files/... on
// Windows (where "file://" + path gave file://C:\Program Files\..., no URL).
func fileURL(path string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

const googleWebEngine = "Google Web"

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
				"install_url":       fileURL(s.ExtensionXPI),
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
	for _, dir := range firefoxInstallDirs() {
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
)

// safeSearchTarget pins search sites to the address their owner gives for
// forced SafeSearch (§4.10): Google, Bing, DuckDuckGo, YouTube Restricted
// Mode, each for its own browser setting (§6 2026-11-18). It works in every
// browser, the extension or not.
type safeSearchTarget struct {
	engine   string   // google, bing, duckduckgo, youtube
	safe     string   // the documented safe name, looked up for its address
	fallback string   // its address when the lookup fails ("" = leave out)
	hosts    []string // the names pinned to it
}

var safeSearchTargets = []safeSearchTarget{
	{"google", "forcesafesearch.google.com", "216.239.38.120", []string{"www.google.com", "google.com", "www.google.se", "google.se"}},
	{"bing", "strict.bing.com", "", []string{"www.bing.com", "bing.com"}},
	{"duckduckgo", "safe.duckduckgo.com", "", []string{"duckduckgo.com", "www.duckduckgo.com"}},
	{"youtube", "restrict.youtube.com", "216.239.38.120", []string{"www.youtube.com", "youtube.com", "m.youtube.com",
		"youtubei.googleapis.com", "youtube.googleapis.com", "www.youtube-nocookie.com"}},
}

// safeSearchFor returns the targets of the engines named.
func safeSearchFor(engines []string) []safeSearchTarget {
	var out []safeSearchTarget
	for _, t := range safeSearchTargets {
		for _, e := range engines {
			if e == t.engine {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

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

// renderSafeSearch returns hosts content with the SafeSearch section for
// targets; ips gives each target's address by its safe name (missing: left
// out).
func renderSafeSearch(content string, targets []safeSearchTarget, ips map[string]string) string {
	lines := []string{safeSearchBegin}
	for _, t := range targets {
		if ip := ips[t.safe]; ip != "" {
			for _, host := range t.hosts {
				lines = append(lines, ip+" "+host)
			}
		}
	}
	lines = append(lines, safeSearchEnd)
	return strings.TrimRight(stripSafeSearch(content), "\n") + "\n" + strings.Join(lines, "\n") + "\n"
}

// safeSearchComplete reports whether the section pins exactly the targets'
// hosts: every one of them, and no other engine's.
func safeSearchComplete(content string, targets []safeSearchTarget) bool {
	start, end := strings.Index(content, safeSearchBegin), strings.Index(content, safeSearchEnd)
	if start < 0 || end < start {
		return false
	}
	pinned := map[string]bool{}
	for _, line := range strings.Split(content[start+len(safeSearchBegin):end], "\n") {
		if f := strings.Fields(line); len(f) == 2 {
			pinned[f[1]] = true
		}
	}
	want := 0
	for _, t := range targets {
		for _, host := range t.hosts {
			if !pinned[host] {
				return false
			}
			want++
		}
	}
	return len(pinned) == want
}

// SyncSafeSearch keeps the search sites of the engines named (google, bing,
// duckduckgo, youtube) on their forced SafeSearch addresses, restoring the
// entries if someone removes them, and the others' entries out.
func SyncSafeSearch(engines []string) error {
	data, err := os.ReadFile(hostsPath)
	if err != nil {
		return err
	}
	content := string(data)
	want := stripSafeSearch(content)
	if targets := safeSearchFor(engines); len(targets) > 0 {
		if safeSearchComplete(content, targets) {
			return nil
		}
		ips := map[string]string{}
		for _, t := range targets {
			ips[t.safe] = t.fallback
			if v4, _ := resolveIPs(t.safe); len(v4) > 0 {
				ips[t.safe] = v4[0]
			}
		}
		want = renderSafeSearch(content, targets, ips)
	}
	if want == content {
		return nil
	}
	return writeHosts(want)
}

const signingPref = `lockPref("xpinstall.signatures.required", false); // kc9k`

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

// BrowserNames names every browser, LibreWolf included, for closing them all.
func BrowserNames() []string {
	return append([]string{"librewolf"}, otherBrowserNames...)
}
