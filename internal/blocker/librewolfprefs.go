// librewolfprefs.go — settings in LibreWolf that would switch the extension
// off while it keeps checking in, locked.
//
// LibreWolf reads its configuration from librewolf.cfg, then your personal
// overrides from ~/.librewolf/librewolf.overrides.cfg, which it loads after
// its own file through autoadmin.global_config_url. Locks KC9K adds to
// librewolf.cfg could therefore be undone by an unlockPref line in the
// overrides file. Instead, KC9K:
//
//   - appends its locks to the end of librewolf.cfg;
//   - points autoadmin.global_config_url at a cleaned copy of your overrides
//     file, kept in /var/lib/kc9k/librewolf/<user>.cfg, which holds only
//     plain pref(), defaultPref() and lockPref() lines with literal values,
//     minus anything touching the settings below.
//
// Your overrides keep working, and nothing in them can unlock these.
package blocker

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
)

// protectedPrefs are LibreWolf settings that would stop the extension from
// working while it still checks in, each locked to its safe value:
// restricted and quarantined domains are sites extensions may not touch,
// and the two devtools switches open a console that runs privileged code.
var protectedPrefs = []struct {
	name  string
	value string // as JavaScript
}{
	{"extensions.webextensions.restrictedDomains", `""`},
	{"extensions.quarantinedDomains.enabled", "false"},
	{"extensions.quarantinedDomains.list", `""`},
	{"devtools.chrome.enabled", "false"},
	{"devtools.debugger.remote-enabled", "false"},
}

// protectedPrefixes may not be set by the overrides file either: the
// overrides mechanism itself, autoconfig, add-on signing, and the protected
// settings.
var protectedPrefixes = []string{
	"autoadmin.", "general.config.", "xpinstall.", "extensions.webextensions.restricteddomains",
	"extensions.quarantineddomains.", "devtools.chrome.", "devtools.debugger.",
}

// LibreWolfOverridesDir holds the cleaned overrides files, one per user.
var LibreWolfOverridesDir = "/var/lib/kc9k/librewolf"

const (
	locksBegin = "// BEGIN kc9k locks: settings that would switch the Killer Chicken extension off"
	locksEnd   = "// END kc9k locks"
)

// LibreWolfLocksVersion identifies the locks KC9K applies, so the daemon can
// tell when they changed and LibreWolf has to restart to pick them up.
func LibreWolfLocksVersion() string {
	sum := sha256.Sum256([]byte(locksSection()))
	return hex.EncodeToString(sum[:6])
}

// locksSection renders KC9K's part of librewolf.cfg.
func locksSection() string {
	lines := []string{locksBegin}
	for _, p := range protectedPrefs {
		lines = append(lines, fmt.Sprintf("lockPref(%q, %s);", p.name, p.value))
	}
	lines = append(lines,
		"// Your overrides, cleaned by Killer Chicken 9000 (see /var/lib/kc9k/librewolf).",
		`let kc9kUser = getenv("USER") || "";`,
		`lockPref("autoadmin.global_config_url", /^[a-z_][a-z0-9_-]{0,31}$/.test(kc9kUser) ? "file://`+LibreWolfOverridesDir+`/" + kc9kUser + ".cfg" : "");`,
		locksEnd)
	return strings.Join(lines, "\n")
}

// withLocks returns librewolf.cfg content with KC9K's section at the end,
// or without it.
func withLocks(config string, on bool) string {
	// Leave the file alone when the current section is already there,
	// wherever it is, so this and withSigningPref never take turns
	// rewriting the file.
	if on && strings.Contains(config, locksSection()+"\n") {
		return config
	}
	var kept []string
	inside := false
	for _, line := range strings.Split(strings.TrimRight(config, "\n"), "\n") {
		switch {
		case line == locksBegin:
			inside = true
		case line == locksEnd:
			inside = false
		case !inside:
			kept = append(kept, line)
		}
	}
	out := strings.TrimRight(strings.Join(kept, "\n"), "\n")
	if on {
		out += "\n\n" + locksSection()
	}
	return out + "\n"
}

// SyncLibreWolfLocks keeps KC9K's locks at the end of LibreWolf's config and
// the cleaned overrides of every desktop user up to date, or removes both.
// Without LibreWolf installed there is nothing to do.
func SyncLibreWolfLocks(on bool) error {
	installed := false
	for _, path := range librewolfConfigs {
		if _, err := os.Stat(path); err == nil {
			installed = true
		}
	}
	if !installed && on {
		return nil
	}
	for _, path := range librewolfConfigs {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if want := withLocks(string(data), on); want != string(data) {
			if err := atomicWriteFile(path, []byte(want), 0644); err != nil {
				return err
			}
		}
	}
	if !on {
		return removeCleanedOverrides()
	}
	return syncCleanedOverrides(desktopUsers())
}

// syncCleanedOverrides writes each user's cleaned overrides file. Every
// desktop user gets one, even an empty one, so LibreWolf always finds it.
func syncCleanedOverrides(users []*user.User) error {
	if err := os.MkdirAll(LibreWolfOverridesDir, 0755); err != nil {
		return err
	}
	var firstErr error
	for _, u := range users {
		source := filepath.Join(u.HomeDir, ".librewolf", "librewolf.overrides.cfg")
		content := cleanOverrides(readUserFile(source), source)
		target := filepath.Join(LibreWolfOverridesDir, u.Username+".cfg")
		if fileHas(target, content) {
			continue
		}
		if err := atomicWriteFile(target, []byte(content), 0644); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func removeCleanedOverrides() error {
	files, _ := filepath.Glob(filepath.Join(LibreWolfOverridesDir, "*.cfg"))
	for _, f := range files {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.Remove(LibreWolfOverridesDir); err != nil && !os.IsNotExist(err) && !strings.Contains(err.Error(), "not empty") {
		return err
	}
	return nil
}

// readUserFile reads a file in a user's home folder, as the root daemon: only
// a regular file, at most 256 KiB, so a link to /dev/zero or a huge file
// can't stall the daemon. Whatever it reads only passes cleanOverrides.
func readUserFile(path string) string {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	data, _ := io.ReadAll(io.LimitReader(f, 256<<10))
	return string(data)
}

// prefLine matches one plain preference statement with a literal value:
// pref("name", "text"), defaultPref("name", 12) or lockPref("name", true),
// optionally followed by a // comment.
var prefLine = regexp.MustCompile(`^\s*(pref|defaultPref|lockPref)\(\s*"([A-Za-z0-9._\-\[\]@{}]+)"\s*,\s*("[^"\\\n]*"|-?[0-9]+|true|false)\s*\)\s*;?\s*(//.*)?$`)

// cleanOverrides keeps only the plain preference lines of an overrides file
// that don't touch protected settings, so the result can't run code or
// unlock anything. Everything else is dropped, and counted in a comment.
func cleanOverrides(content, source string) string {
	out := []string{"// Generated by Killer Chicken 9000 from " + source + ".",
		"// Only plain preference lines are kept; edit the original, not this file."}
	dropped := 0
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		m := prefLine.FindStringSubmatch(line)
		if m == nil || isProtectedPref(m[2]) {
			dropped++
			continue
		}
		out = append(out, fmt.Sprintf("%s(%q, %s);", m[1], m[2], m[3]))
	}
	if dropped > 0 {
		out = append(out, fmt.Sprintf("// %d line(s) were left out: code, or settings that protect the extension.", dropped))
	}
	return strings.Join(out, "\n") + "\n"
}

func isProtectedPref(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range protectedPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}
