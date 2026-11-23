package blocker

import (
	"runtime"
	"strings"
	"testing"
)

// §4.10: SafeSearch pins Google, Bing, DuckDuckGo and YouTube, each for its
// own browser setting (§6 2026-11-18).
func TestSafeSearchSection(t *testing.T) {
	base := "127.0.0.1 localhost\n"
	ips := map[string]string{"forcesafesearch.google.com": "1.2.3.4", "strict.bing.com": "5.6.7.8",
		"safe.duckduckgo.com": "9.9.9.1", "restrict.youtube.com": "1.1.1.2"}
	all := safeSearchFor([]string{"google", "bing", "duckduckgo", "youtube"})
	once := renderSafeSearch(base, all, ips)
	for _, want := range []string{"1.2.3.4 www.google.com", "5.6.7.8 bing.com", "9.9.9.1 duckduckgo.com", "1.1.1.2 www.youtube.com"} {
		if !strings.Contains(once, want+"\n") {
			t.Fatalf("missing %q: %q", want, once)
		}
	}
	google := safeSearchFor([]string{"google"})
	if !safeSearchComplete(once, all) || safeSearchComplete(renderSafeSearch(base, all, map[string]string{"forcesafesearch.google.com": "1.2.3.4"}), all) {
		t.Error("completeness: an old Google-only section must be rewritten")
	}
	if safeSearchComplete(once, google) {
		t.Error("Bing, DuckDuckGo and YouTube stay pinned after their settings are off")
	}
	onlyGoogle := renderSafeSearch(once, google, ips)
	if !safeSearchComplete(onlyGoogle, google) || strings.Contains(onlyGoogle, "bing") || strings.Contains(onlyGoogle, "youtube") {
		t.Errorf("Google alone: %q", onlyGoogle)
	}
	if twice := renderSafeSearch(once, all, ips); twice != once {
		t.Fatalf("not stable: %q", twice)
	}
	if got := stripSafeSearch(once); strings.Contains(got, "google") || strings.Contains(got, "bing") {
		t.Fatalf("not removed: %q", got)
	}
}

func TestSetupPolicies(t *testing.T) {
	policies := map[string]interface{}{"DNSOverHTTPS": true}
	setupPolicies(BrowserSetup{ExtensionID: "x", ExtensionXPI: "/a.xpi", ForceSearch: true, DisableDevTools: true, DisablePrivate: true}, policies)
	if policies["DisableDeveloperTools"] != true {
		t.Error("developer tools should be blocked")
	}
	if policies["DisablePrivateBrowsing"] != true {
		t.Error("private windows should be off")
	}
	if _, ok := policies["ExtensionSettings"]; !ok {
		t.Error("the extension should be forced")
	}
	if _, ok := policies["SearchEngines"]; !ok {
		t.Error("the search engine should be forced")
	}
	setupPolicies(BrowserSetup{}, policies)
	if len(policies) != 1 {
		t.Errorf("only unrelated policies should remain: %v", policies)
	}
	// Uninstalling: the extension is blocked, which removes it, and nothing
	// else of ours stays.
	setupPolicies(BrowserSetup{RemoveExtensionID: "x"}, policies)
	settings, ok := policies["ExtensionSettings"].(map[string]interface{})
	if !ok || len(policies) != 2 {
		t.Fatalf("after uninstall: %v", policies)
	}
	if mode := settings["x"].(map[string]interface{})["installation_mode"]; mode != "blocked" {
		t.Errorf("extension mode after uninstall = %v, want blocked", mode)
	}
}

func TestSigningPref(t *testing.T) {
	base := "// config\n"
	on := withSigningPref(base, true)
	if !strings.Contains(on, signingPref) || withSigningPref(on, true) != on {
		t.Fatalf("adding is not stable: %q", on)
	}
	if off := withSigningPref(on, false); off != base {
		t.Fatalf("not removed: %q", off)
	}
}

// 2.12: the extension's install_url is a real file: URL on both systems
// (Firefox and LibreWolf on Windows got file://C:\..., which isn't one).
func TestFileURL(t *testing.T) {
	cases := map[string]string{"/usr/share/kc9k/kc9k-1.2.xpi": "file:///usr/share/kc9k/kc9k-1.2.xpi"}
	if runtime.GOOS == "windows" {
		cases = map[string]string{`C:\Program Files\KC9K\share\kc9k-1.2.xpi`: "file:///C:/Program%20Files/KC9K/share/kc9k-1.2.xpi"}
	}
	for in, want := range cases {
		if got := fileURL(in); got != want {
			t.Errorf("fileURL(%q) = %q, want %q", in, got, want)
		}
	}
}
