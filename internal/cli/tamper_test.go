package cli

import (
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

// fakeSupportedBrowser makes a supported browser count as installed, or not.
func fakeSupportedBrowser(t *testing.T, installed bool) {
	t.Helper()
	old := extensionBrowserInstalled
	extensionBrowserInstalled = func() bool { return installed }
	t.Cleanup(func() { extensionBrowserInstalled = old })
}

func unsupportedOn(t *testing.T, d *db.DB) bool {
	t.Helper()
	return enforce.UnsupportedBrowsersOn(d)
}

// §4.10: block unsupported browsers is a tamper protection setting. It can't
// be turned on with no supported browser installed (every browser would be
// closed), and once on it can't be turned off while anything is locked, its
// own lock included.
func TestUnsupportedBrowsersSetting(t *testing.T) {
	d := testDB(t)
	notTerminal(t)

	fakeSupportedBrowser(t, false)
	if err := runCLI(t, "tamper", "unsupported-browsers", "on"); err == nil || !strings.Contains(err.Error(), "no supported browser") {
		t.Fatalf("turned on with no supported browser installed: %v", err)
	}
	if unsupportedOn(t, d) {
		t.Fatal("on after a refusal")
	}
	fakeSupportedBrowser(t, true)
	mustRun(t, "tamper", "unsupported-browsers", "on")
	if !unsupportedOn(t, d) {
		t.Fatal("not on")
	}
	if enforce.IsBrowserSetting(db.UnsupportedBrowsers) || enforce.IsBrowserSetting(forceLibreWolfAlias) {
		t.Error("block unsupported browsers is still listed as a browser setting")
	}

	// A block's lock keeps it on (it guards blocks).
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	if err := d.ActivateBlock("b", true, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "tamper", "unsupported-browsers", "off"); err == nil {
		t.Error("turned off while a block is locked")
	}
	if err := d.DeactivateBlock("b"); err != nil {
		t.Fatal(err)
	}

	// So does any other lock (§4.10 tamper protection: anything locked).
	mustRun(t, "browser", "safesearch", "on", "--for", "1h", "--yes")
	if err := runCLI(t, "tamper", "unsupported-browsers", "off"); err == nil {
		t.Error("turned off while a browser setting is locked")
	}
	if err := d.ClearSettingLock("safesearch"); err != nil {
		t.Fatal(err)
	}

	// Its own timer lock: only extended, counts as a setting lock.
	mustRun(t, "tamper", "unsupported-browsers", "on", "--for", "1h", "--yes")
	if err := runCLI(t, "tamper", "unsupported-browsers", "off"); err == nil || !strings.Contains(err.Error(), "locked until") {
		t.Errorf("off under its own timer lock: %v", err)
	}
	if err := runCLI(t, "tamper", "unsupported-browsers", "on", "--for", "10m", "--yes"); err == nil {
		t.Error("its timer lock was shortened")
	}
	trusted := mustTrusted(t, d)
	if locked, _ := enforce.AnySettingLocked(d, trusted); !locked {
		t.Error("its lock doesn't count as a setting lock (strict mode, no-escape, uninstall)")
	}
	if !enforce.GuardsBlocks(db.UnsupportedBrowsers) {
		t.Error("it doesn't guard blocks")
	}
	if err := d.ClearSettingLock(db.UnsupportedBrowsers); err != nil {
		t.Fatal(err)
	}

	// A password lock: off points to unlock; unlock needs the password.
	if err := runWith(t, 0, []string{"partner"}, "tamper", "unsupported-browsers", "on", "--password-stdin", "--yes"); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "tamper", "unsupported-browsers", "off"); err == nil || !strings.Contains(err.Error(), "tamper unlock unsupported-browsers") {
		t.Errorf("off under a password lock: %v", err)
	}
	if err := runWith(t, 0, []string{"wrong"}, "tamper", "unlock", "unsupported-browsers"); err == nil {
		t.Error("a wrong password unlocked it")
	}
	// Unlocked while a block is locked: the lock goes, the setting stays on.
	if err := d.ActivateBlock("b", true, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"partner"}, "tamper", "unlock", "unsupported-browsers"); err != nil {
		t.Fatal(err)
	}
	if !unsupportedOn(t, d) {
		t.Error("unlock turned it off while a block is locked")
	}
	if settingLocked(db.UnsupportedBrowsers)(t, d) {
		t.Error("unlock left the password lock")
	}
	if err := d.DeactivateBlock("b"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "tamper", "unsupported-browsers", "off")
	if unsupportedOn(t, d) {
		t.Error("still on after off")
	}
}

// R-WEAK-4 and the settings password apply to turning it off.
func TestUnsupportedBrowsersWeakening(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	fakeSupportedBrowser(t, true)
	if err := runWith(t, 0, []string{"secret1"}, "password", "set"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "tamper", "unsupported-browsers", "on")
	if err := runWith(t, 0, nil, "tamper", "unsupported-browsers", "off"); err == nil {
		t.Error("turned off without the settings password")
	}
	if err := runWith(t, 0, []string{"secret1"}, "no-weakening", "on"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"secret1"}, "tamper", "unsupported-browsers", "off"); err != nil {
		t.Fatalf("off with the settings password and nothing locked: %v", err)
	}
	if unsupportedOn(t, d) {
		t.Error("still on")
	}
}

// The old command stays as a hidden alias, working on the new setting and
// its lock.
func TestForceLibreWolfAlias(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	fakeSupportedBrowser(t, true)
	mustRun(t, "browser", "force-librewolf", "on")
	if !unsupportedOn(t, d) {
		t.Fatal("browser force-librewolf on didn't turn on block unsupported browsers")
	}
	if v, _ := d.GetSetting("browser.force-librewolf"); v != "" {
		t.Errorf("the old setting was written: %q", v)
	}
	if err := runWith(t, 0, []string{"partner"}, "browser", "force-librewolf", "on", "--password-stdin", "--yes"); err != nil {
		t.Fatal(err)
	}
	if !settingLocked(db.UnsupportedBrowsers)(t, d) {
		t.Fatal("the alias's lock isn't on the new setting")
	}
	if err := runWith(t, 0, []string{"partner"}, "browser", "unlock", "force-librewolf"); err != nil {
		t.Fatal(err)
	}
	if unsupportedOn(t, d) {
		t.Error("browser unlock force-librewolf didn't turn it off")
	}
	if strings.Contains(browserCmd.Short, forceLibreWolfAlias) {
		t.Error("the alias is shown in the browser command's help")
	}
}

// §4.10: allow file:// blocking is a tamper protection setting: on is free,
// off is refused while anything is locked; it goes in a project.
func TestTamperFileURLs(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "tamper", "file-urls", "on")
	if v, _ := d.GetSetting("tamper.file-urls"); v != "on" {
		t.Fatalf("setting = %q", v)
	}
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runCLI(t, "tamper", "file-urls", "off"); err == nil {
		t.Error("turned off while a block is locked")
	}
	if !strings.Contains(capture(t, "project", "export"), `"file-urls": true`) {
		t.Error("not in the project")
	}
}

// §4.10: the extension's grace (longer weakens) and closing task managers
// while locked (off refused while locked).
func TestTamperGraceAndTaskManagers(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "tamper", "grace", "30")
	mustRun(t, "tamper", "task-managers", "on")
	if enforce.ExtensionGraceSeconds(d) != 30 {
		t.Fatalf("grace = %d", enforce.ExtensionGraceSeconds(d))
	}
	if err := runCLI(t, "tamper", "grace", "5"); err == nil {
		t.Error("a 5 s grace accepted")
	}
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runCLI(t, "tamper", "grace", "120"); err == nil {
		t.Error("a longer grace while locked")
	}
	mustRun(t, "tamper", "grace", "20")
	if err := runCLI(t, "tamper", "task-managers", "off"); err == nil {
		t.Error("task managers allowed again while locked")
	}
	if !strings.Contains(capture(t, "gui-state"), `"anything_locked":true`) {
		t.Error("gui-state doesn't say something is locked")
	}
}

// §4.10 close private windows and Tor: turns private windows off in the
// browsers; off refused while locked.
func TestTamperPrivate(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "tamper", "private", "on")
	if !enforce.PrivateTamperOn(d) || !strings.Contains(capture(t, "gui-state"), `"tamper_private":true`) {
		t.Fatal("private tamper not on")
	}
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runCLI(t, "tamper", "private", "off"); err == nil {
		t.Error("private windows allowed again while locked")
	}
}
