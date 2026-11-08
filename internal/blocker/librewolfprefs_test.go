package blocker

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

// A stand-in for LibreWolf's librewolf.cfg: the same shape at the end.
const sampleConfig = `// LibreWolf settings
defaultPref("extensions.webextensions.restrictedDomains", "");
defaultPref("extensions.quarantinedDomains.enabled", false);
pref("devtools.debugger.remote-enabled", false);

let profile_directory;
if (profile_directory = getenv('USERPROFILE') || getenv('HOME')) {
  defaultPref('autoadmin.global_config_url', ` + "`file://${profile_directory}/.librewolf/librewolf.overrides.cfg`" + `);
}
`

func TestWithLocksStable(t *testing.T) {
	on := withLocks(sampleConfig, true)
	if !strings.Contains(on, locksSection()) {
		t.Fatal("section not added")
	}
	// Both of KC9K's parts at the end of the file must settle, not take
	// turns moving to the end.
	both := withSigningPref(on, true)
	for i := 0; i < 3; i++ {
		next := withSigningPref(withLocks(both, true), true)
		if next != both {
			t.Fatalf("round %d rewrote the file:\n%s", i, next)
		}
	}
	off := withLocks(withSigningPref(both, false), false)
	if off != sampleConfig {
		t.Fatalf("not restored:\n%q\nwant\n%q", off, sampleConfig)
	}
}

func TestCleanOverrides(t *testing.T) {
	input := strings.Join([]string{
		`// my settings`,
		`defaultPref("webgl.disabled", false);`,
		`pref("privacy.resistFingerprinting", false); // keep my timezone`,
		`lockPref("browser.startup.page", 3)`,
		`pref("browser.startup.homepage", "https://example.org/");`,
		`unlockPref("extensions.webextensions.restrictedDomains");`,
		`pref("extensions.webextensions.restrictedDomains", "reddit.com");`,
		`lockPref("Extensions.QuarantinedDomains.list", "reddit.com");`,
		`pref("autoadmin.global_config_url", "file:///home/u/evil.cfg");`,
		`pref("devtools.chrome.enabled", true);`,
		`pref("xpinstall.signatures.required", false);`,
		`var n = "devtools.chrome.enabled"; pref(n, true);`,
		`pref("a.b", "x\"y");`,
		`pref("a.b", "x"); unlockPref("devtools.chrome.enabled");`,
		`Components.utils.import("resource://gre/modules/Services.jsm");`,
	}, "\n")
	out := cleanOverrides(input, "/home/u/.librewolf/librewolf.overrides.cfg")
	for _, kept := range []string{
		`defaultPref("webgl.disabled", false);`,
		`pref("privacy.resistFingerprinting", false);`,
		`lockPref("browser.startup.page", 3);`,
		`pref("browser.startup.homepage", "https://example.org/");`,
	} {
		if !strings.Contains(out, kept+"\n") {
			t.Errorf("dropped a plain line: %s", kept)
		}
	}
	for _, gone := range []string{"restrictedDomains", "reddit.com", "evil", "devtools.chrome", "signatures", "Components", "unlockPref", `x\"y`} {
		if strings.Contains(out, gone) {
			t.Errorf("kept something it shouldn't (%s):\n%s", gone, out)
		}
	}
	if !strings.Contains(out, "// 10 line(s) were left out") {
		t.Errorf("wrong count of dropped lines:\n%s", out)
	}
}

func TestSyncCleanedOverrides(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(filepath.Join(home, ".librewolf"), 0755); err != nil {
		t.Fatal(err)
	}
	old := LibreWolfOverridesDir
	LibreWolfOverridesDir = filepath.Join(dir, "cleaned")
	defer func() { LibreWolfOverridesDir = old }()

	alice := &user.User{Username: "alice", HomeDir: home}
	overrides := filepath.Join(home, ".librewolf", "librewolf.overrides.cfg")
	if err := os.WriteFile(overrides, []byte(`pref("webgl.disabled", false);`+"\n"+`unlockPref("devtools.chrome.enabled");`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := syncCleanedOverrides([]*user.User{alice}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(LibreWolfOverridesDir, "alice.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `pref("webgl.disabled", false);`) || strings.Contains(string(data), "unlockPref") {
		t.Errorf("cleaned copy:\n%s", data)
	}

	// A link to another file is not followed.
	if err := os.Remove(overrides); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/passwd", overrides); err != nil {
		t.Fatal(err)
	}
	if err := syncCleanedOverrides([]*user.User{alice}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(LibreWolfOverridesDir, "alice.cfg"))
	if strings.Contains(string(data), "root") || strings.Count(string(data), "\n") != 2 {
		t.Errorf("followed the link:\n%s", data)
	}

	if err := removeCleanedOverrides(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(LibreWolfOverridesDir); !os.IsNotExist(err) {
		t.Error("cleaned overrides left behind")
	}
}

// Runs the config with stand-ins for LibreWolf's autoconfig functions and
// checks the outcome: it must parse, lock the protected settings, and point
// the overrides at the cleaned copy for a valid user name only.
func TestLockedConfigRuns(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed")
	}
	script := `
const fs = require("fs");
const prefs = {}, locked = {};
const pref = (n, v) => { if (!locked[n]) prefs[n] = v; };
const defaultPref = pref;
const lockPref = (n, v) => { prefs[n] = v; locked[n] = true; };
const unlockPref = (n) => { delete locked[n]; };
const getenv = (n) => ({HOME: "/home/u", USER: process.argv[3]})[n];
eval(fs.readFileSync(process.argv[2], "utf8"));
console.log(JSON.stringify({prefs, locked}));
`
	dir := t.TempDir()
	harness := filepath.Join(dir, "harness.js")
	cfg := filepath.Join(dir, "librewolf.cfg")
	if err := os.WriteFile(harness, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte(withSigningPref(withLocks(sampleConfig, true), true)), 0644); err != nil {
		t.Fatal(err)
	}
	for user, wantURL := range map[string]string{
		"tjena":       "file://" + LibreWolfOverridesDir + "/tjena.cfg",
		"../../evil":  "",
		"":            "",
		"Robert'); x": "",
	} {
		out, err := exec.Command(node, harness, cfg, user).CombinedOutput()
		if err != nil {
			t.Fatalf("config failed to run for user %q: %v\n%s", user, err, out)
		}
		text := string(out)
		if !strings.Contains(text, `"autoadmin.global_config_url":"`+wantURL+`"`) {
			t.Errorf("user %q: overrides URL wrong:\n%s", user, text)
		}
		for _, p := range protectedPrefs {
			if !strings.Contains(text, `"`+p.name+`":true`) {
				t.Errorf("user %q: %s not locked:\n%s", user, p.name, text)
			}
		}
	}
}
