//go:build linux

package blocker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// firefoxPolicyPath is Firefox's shared policy file.
var firefoxPolicyPath = "/etc/firefox/policies/policies.json"

// chromiumPolicyPaths lists the managed-policy file we write for each
// Chromium-family browser. They share the same URLBlocklist format but each
// reads a different directory. Writing a file whose directory doesn't exist
// yet is fine — os.MkdirAll creates it, so the policy is ready if the user
// installs that browser later.
var chromiumPolicyPaths = []string{
	"/etc/chromium/policies/managed/kc9k.json",   // Chromium
	"/etc/opt/chrome/policies/managed/kc9k.json", // Google Chrome
	"/etc/brave/policies/managed/kc9k.json",      // Brave
	"/etc/opt/edge/policies/managed/kc9k.json",   // Microsoft Edge
	"/etc/opt/opera/policies/managed/kc9k.json",  // Opera
	"/etc/vivaldi/policies/managed/kc9k.json",    // Vivaldi
}

// ApplyBrowserPolicies writes the blocking policies for every browser.
//
// Firefox and Chromium use DIFFERENT pattern formats (treating them the same
// once let subdomains through), so we build two sets:
//
//   - Firefox (WebsiteFilter): match patterns like "*://*.domain.com/*", plus a
//     catch-all "*://*/*" with Exceptions in allowlist mode.
//   - Chromium family (URLBlocklist / URLAllowlist): a bare "domain.com" filter
//     (which matches root, subdomains and paths), or "*" plus URLAllowlist in
//     allowlist mode.
//
// With an empty (nothing-to-block) policy, the files are removed instead of
// written empty, so an apps-only block leaves no policy files behind.
func ApplyBrowserPolicies(p BrowserPolicy) error {
	sets := setsOf(p)
	if sets.empty {
		return RemoveBrowserPolicies()
	}

	if err := applyFirefoxPolicy(sets.ffBlock, sets.ffExceptions); err != nil {
		return fmt.Errorf("failed to apply Firefox policy: %w", err)
	}

	for _, path := range chromiumPolicyPaths {
		if err := applyChromiumPolicy(path, sets.crBlock, sets.crAllow); err != nil {
			return fmt.Errorf("failed to apply policy at %s: %w", path, err)
		}
	}

	return nil
}

// RemoveBrowserPolicies removes our blocking policies from every browser.
//
// Firefox's file may hold other policies, so we strip only our keys (and
// delete the file if nothing else remains). Each Chromium file is ours alone,
// so we delete it.
func RemoveBrowserPolicies() error {
	if err := removeFirefoxPolicy(); err != nil {
		return fmt.Errorf("failed to remove Firefox policy: %w", err)
	}
	for _, path := range chromiumPolicyPaths {
		if err := removePolicyFile(path); err != nil {
			return fmt.Errorf("failed to remove policy at %s: %w", path, err)
		}
	}
	return nil
}

// IsBrowserPoliciesApplied reports whether the policies are correct for every
// browser whose policy directory exists. It is strict: any missing filter, any
// tampered blocklist, or a missing DoH-off setting makes it return false so the
// daemon reapplies. With no domains, "applied" means our policies are absent.
func IsBrowserPoliciesApplied(p BrowserPolicy) bool {
	sets := setsOf(p)
	if sets.empty {
		return !BrowserPoliciesPresent()
	}

	if dirExists(filepath.Dir(firefoxPolicyPath)) {
		if !checkFirefoxPolicy(sets.ffBlock, sets.ffExceptions) {
			return false
		}
	}

	for _, path := range chromiumPolicyPaths {
		if dirExists(filepath.Dir(path)) {
			if !checkChromiumPolicy(path, sets.crBlock, sets.crAllow) {
				return false
			}
		}
	}

	return true
}

// BrowserPoliciesPresent reports whether any of our policy artifacts still
// exist (any Chromium file, or Firefox's WebsiteFilter key). The daemon uses it
// to decide whether an idle cleanup is needed.
func BrowserPoliciesPresent() bool {
	for _, path := range chromiumPolicyPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	content, err := os.ReadFile(firefoxPolicyPath)
	if err != nil {
		return false
	}
	var root map[string]interface{}
	if json.Unmarshal(content, &root) != nil {
		return false
	}
	policies, ok := root["policies"].(map[string]interface{})
	if !ok {
		return false
	}
	_, hasFilter := policies["WebsiteFilter"]
	_, hasDoH := policies["DNSOverHTTPS"]
	return hasFilter || hasDoH
}

// applyChromiumPolicy writes our managed-policy file for one Chromium-family
// browser: the URL blocklist (and, in allowlist mode, the allowlist) plus
// DNS-over-HTTPS turned off.
func applyChromiumPolicy(path string, block, allow []string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	policy := map[string]interface{}{
		"URLBlocklist":            block,
		"DnsOverHttpsMode":        "off",
		"BuiltInDnsClientEnabled": false,
	}
	if len(allow) > 0 {
		policy["URLAllowlist"] = allow
	}
	return writeJSON(path, policy)
}

// checkChromiumPolicy verifies URLBlocklist and URLAllowlist are exactly the
// expected sets and that DNS-over-HTTPS is off.
func checkChromiumPolicy(path string, expectedBlock, expectedAllow []string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var policy map[string]interface{}
	if json.Unmarshal(content, &policy) != nil {
		return false
	}
	if !sameSet(stringsFromInterface(policy["URLBlocklist"]), expectedBlock) {
		return false
	}
	if !sameSet(stringsFromInterface(policy["URLAllowlist"]), expectedAllow) {
		return false
	}
	mode, ok := policy["DnsOverHttpsMode"].(string)
	return ok && mode == "off"
}
