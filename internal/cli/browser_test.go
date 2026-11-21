package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

// A setting's locks stack like a block's (§3.4, §6 2026-11-21 By Opus):
// each new one goes last; a kind the setting already has is another lock
// of it, waiting behind the others; a lock already set never changes.
func TestStrengthenSettingLock(t *testing.T) {
	const now, boot = 1000, "boot-a"
	state := func(l db.SettingLock) settingLockState {
		l.Name = "safesearch"
		return settingLockState{lock: l, trusted: now, bootID: boot}
	}
	want := func(qs ...db.QueuedLock) db.SettingLock { return db.SettingLock{Config: qs} }

	next, changes, err := strengthenSettingLock(state(db.SettingLock{}),
		want(db.QueuedLock{Kind: db.LayerTimer, Until: 2000}, db.QueuedLock{Kind: db.LayerText, Chars: 100}), false)
	if err != nil || next.LockedUntil != 2000 || next.LockChars != 100 || len(changes) != 2 || len(next.Config) != 2 {
		t.Fatalf("new locks = %+v, %v, %v", next, changes, err)
	}
	next.PasswordHash = "x"
	for _, q := range []db.QueuedLock{{Kind: db.LayerTimer, Until: 1500}, {Kind: db.LayerText, Chars: 50}, {Kind: db.LayerPassword, PasswordHash: "y"}} {
		got, changes, err := strengthenSettingLock(state(next), want(q), false)
		if err != nil || got.LockedUntil != 2000 || got.LockChars != 100 || len(got.Queue) != 1 || got.Queue[0] != q ||
			len(changes) != 1 || !strings.HasPrefix(changes[0], "another ") {
			t.Errorf("another %s = %+v, %v, %v", q.Kind, got, changes, err)
		}
	}
	// Once one waits in the queue, every new lock does: the order stays.
	next.PasswordHash = ""
	queued, _, _ := strengthenSettingLock(state(next), want(db.QueuedLock{Kind: db.LayerText, Chars: 50}, db.QueuedLock{Kind: db.LayerPassword, PasswordHash: "y"}), false)
	if queued.PasswordHash != "" || len(queued.Queue) != 2 || queued.Queue[1].Kind != db.LayerPassword {
		t.Errorf("after a queued lock: %+v", queued)
	}
	if _, _, err := strengthenSettingLock(state(next), want(db.QueuedLock{Kind: db.LayerDelay, Delay: 60}), false); err == nil {
		t.Error("a delay lock on a setting")
	}

	// Locks that ran out don't count: an expired time lock can be set again
	// to any later time, and a restart lock from an earlier boot is dropped.
	stale := db.SettingLock{LockedUntil: 500, RestartBoot: "boot-old"}
	got, _, err := strengthenSettingLock(state(stale), want(db.QueuedLock{Kind: db.LayerTimer, Until: 1200}), false)
	if err != nil || got.LockedUntil != 1200 || got.RestartBoot != "" || len(got.Queue) != 0 {
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
