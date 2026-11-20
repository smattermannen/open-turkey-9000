package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

// A setting's locks can be added to and made stronger, never weaker.
func TestStrengthenSettingLock(t *testing.T) {
	const now, boot = 1000, "boot-a"
	state := func(l db.SettingLock) settingLockState {
		l.Name = "safesearch"
		return settingLockState{lock: l, trusted: now, bootID: boot}
	}

	next, changes, err := strengthenSettingLock(state(db.SettingLock{}), db.SettingLock{LockedUntil: 2000, LockChars: 100}, false)
	if err != nil || next.LockedUntil != 2000 || next.LockChars != 100 || len(changes) != 2 {
		t.Fatalf("new locks = %+v, %v, %v", next, changes, err)
	}
	if _, _, err := strengthenSettingLock(state(next), db.SettingLock{LockedUntil: 1500}, false); err == nil {
		t.Error("an earlier end should be refused")
	}
	if _, _, err := strengthenSettingLock(state(next), db.SettingLock{LockChars: 50}, true); err == nil {
		t.Error("a shorter challenge should be refused")
	}
	if got, _, err := strengthenSettingLock(state(next), db.SettingLock{LockChars: 400}, true); err != nil || got.LockChars != 400 {
		t.Errorf("a longer challenge = %+v, %v", got, err)
	}
	if got, changes, err := strengthenSettingLock(state(next), db.SettingLock{LockChars: 30}, false); err != nil || got.LockChars != 100 || len(changes) != 0 {
		t.Errorf("--lock without --lock-chars on a locked setting = %+v, %v, %v; want unchanged", got, changes, err)
	}

	withPassword := db.SettingLock{PasswordHash: "x"}
	if _, _, err := strengthenSettingLock(state(withPassword), db.SettingLock{PasswordHash: "y"}, false); err == nil {
		t.Error("replacing a password should be refused")
	}

	// Locks that ran out don't count: an expired time lock can be set again
	// to any later time, and a restart lock from an earlier boot is dropped.
	stale := db.SettingLock{LockedUntil: 500, RestartBoot: "boot-old"}
	got, _, err := strengthenSettingLock(state(stale), db.SettingLock{LockedUntil: 1200}, false)
	if err != nil || got.LockedUntil != 1200 || got.RestartBoot != "" {
		t.Errorf("over stale locks = %+v, %v", got, err)
	}
}

// §4.10, §6 2026-11-18: SafeSearch is a browser setting per engine, each
// with its own locks; 'browser safesearch' switches all four, its lock flags
// lock each, confirmed together (R-LOCK-12).
func TestSafeSearchPerEngine(t *testing.T) {
	notTerminal(t)
	d := testDB(t)
	mustRun(t, "browser", "safesearch-bing", "on")
	if got := enforce.SafeSearchOn(d); len(got) != 1 || got[0] != "bing" {
		t.Errorf("engines on = %v", got)
	}
	if r, _ := enforce.BuildRules(d); !r.SafeSearch || len(r.SafeSearchEngines) != 1 {
		t.Errorf("rules: %v %v", r.SafeSearch, r.SafeSearchEngines)
	}
	if err := runCLI(t, "browser", "safesearch", "on", "--for", "1h"); err == nil {
		t.Error("locks on every engine without --yes")
	}
	if got := enforce.SafeSearchOn(d); len(got) != 1 {
		t.Errorf("an unconfirmed lock turned engines on: %v", got)
	}
	mustRun(t, "browser", "safesearch", "on", "--for", "1h", "--yes")
	for _, e := range enforce.SafeSearchEngines {
		name := enforce.SafeSearchSetting(e)
		if l, _ := d.GetSettingLock(name); l.LockedUntil == 0 || !enforce.BrowserOn(d, name) {
			t.Errorf("%s: on %v, lock %+v", name, enforce.BrowserOn(d, name), l)
		}
	}
	if err := runCLI(t, "browser", "safesearch-youtube", "off"); err == nil {
		t.Error("a locked engine's SafeSearch was turned off")
	}
	if err := runCLI(t, "browser", "safesearch", "off"); err == nil || !enforce.BrowserOn(d, "safesearch-google") {
		t.Errorf("safesearch off while locked: %v", err)
	}
	if err := runCLI(t, "browser", "unlock", "safesearch"); err == nil || !strings.Contains(err.Error(), "safesearch-<engine>") {
		t.Errorf("unlock safesearch: %v", err)
	}
}

// A project file from before (one "safesearch") turns every engine's on.
func TestOldProjectSafeSearch(t *testing.T) {
	d := testDB(t)
	file := filepath.Join(t.TempDir(), "old.json")
	os.WriteFile(file, []byte(`{"kc9k_project":1,"blocks":[],"browser_options":{"safesearch":true}}`), 0o644)
	mustRun(t, "project", "import", file)
	if got := enforce.SafeSearchOn(d); len(got) != 4 {
		t.Errorf("engines on = %v", got)
	}
}
