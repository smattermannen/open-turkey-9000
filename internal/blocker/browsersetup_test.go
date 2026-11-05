package blocker

import (
	"strings"
	"testing"
)

func TestSafeSearchSection(t *testing.T) {
	base := "127.0.0.1 localhost\n"
	once := renderSafeSearch(base, "1.2.3.4")
	if !strings.Contains(once, "1.2.3.4 www.google.com") {
		t.Fatalf("missing entry: %q", once)
	}
	if twice := renderSafeSearch(once, "1.2.3.4"); twice != once {
		t.Fatalf("not stable: %q", twice)
	}
	if got := stripSafeSearch(once); strings.Contains(got, "google") {
		t.Fatalf("not removed: %q", got)
	}
}

func TestSetupPolicies(t *testing.T) {
	policies := map[string]interface{}{"DNSOverHTTPS": true}
	setupPolicies(BrowserSetup{ExtensionID: "x", ExtensionXPI: "/a.xpi", ForceSearch: true}, policies)
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
}
