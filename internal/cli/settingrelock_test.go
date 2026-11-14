package cli

import (
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

// §3.4 re-lock on setting locks: unlocking lifts the text and password
// locks for the re-lock time and leaves the setting on; then they are on
// again by themselves, unless the setting was turned off meanwhile.
func TestSettingRelock(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	lockOf := func(name string) db.SettingLock {
		t.Helper()
		l, err := d.GetSettingLock(name)
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	trusted := func() int64 {
		n, err := d.TrustedNow(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	if err := runCLI(t, "browser", "web-only", "on", "--relock", "10m", "--for", "1h", "--yes"); err == nil {
		t.Error("--relock without a text or password lock accepted")
	}
	if err := runWith(t, 0, []string{"pw"}, "browser", "safesearch", "on", "--password-stdin", "--relock", "30s", "--yes"); err == nil {
		t.Error("--relock under a minute accepted")
	}
	if err := runWith(t, 0, []string{"pw"}, "browser", "safesearch", "on", "--password-stdin", "--relock", "30m", "--yes"); err != nil {
		t.Fatal(err)
	}
	if l := lockOf("safesearch"); l.Relock != 1800 || !l.InForce(trusted(), "boot") {
		t.Fatalf("lock = %+v", l)
	}
	// Longer re-lock weakens; shorter strengthens.
	if err := runCLI(t, "browser", "safesearch", "on", "--relock", "1h", "--yes"); err == nil {
		t.Error("a longer re-lock was accepted")
	}
	mustRun(t, "browser", "safesearch", "on", "--relock", "20m", "--yes")
	if l := lockOf("safesearch"); l.Relock != 1200 {
		t.Errorf("re-lock = %d; want 1200", l.Relock)
	}

	// Unlock: lifted, setting stays on, no lock counts.
	if err := runWith(t, 0, []string{"pw"}, "browser", "unlock", "safesearch"); err != nil {
		t.Fatal(err)
	}
	l := lockOf("safesearch")
	if !enforce.BrowserOn(d, "safesearch") || l.InForce(trusted(), "boot") || !l.PendingRelock(trusted()) {
		t.Fatalf("after unlock: on %v, lock %+v", enforce.BrowserOn(d, "safesearch"), l)
	}
	if err := runWith(t, 0, []string{"pw"}, "browser", "unlock", "safesearch"); err == nil {
		t.Error("unlocking a lifted lock again succeeded")
	}
	if locked, _ := enforce.AnyLock(d, time.Now(), trusted()); locked {
		t.Error("a lifted setting lock still counts as locked")
	}

	// The re-lock time passes: locked again, off refused.
	l.LiftedUntil = trusted() - 1
	if err := d.SetSettingLock(l); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "browser", "safesearch", "off"); err == nil {
		t.Error("turned off after the re-lock came back on")
	}
	if locked, _ := enforce.AnyLock(d, time.Now(), trusted()); !locked {
		t.Error("the re-locked setting doesn't count as locked")
	}

	// Unlock, then turn off meanwhile: the re-lock is gone with it.
	if err := runWith(t, 0, []string{"pw"}, "browser", "unlock", "safesearch"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "browser", "safesearch", "off")
	if l := lockOf("safesearch"); l != (db.SettingLock{Name: "safesearch"}) {
		t.Errorf("after off during the lift: %+v", l)
	}

	// A new lock during a lift replaces the lifted ones at once.
	if err := runWith(t, 0, []string{"pw"}, "browser", "web-only", "on", "--password-stdin", "--relock", "5m", "--yes"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"pw"}, "browser", "unlock", "web-only"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"new"}, "browser", "web-only", "on", "--password-stdin", "--yes"); err != nil {
		t.Fatal(err)
	}
	if l := lockOf("web-only"); !l.TextOrPasswordLocked(trusted()) || l.Relock != 0 || !db.CheckPassword(l.PasswordHash, "new") {
		t.Errorf("new lock over a lifted one: %+v", l)
	}
}

// Re-lock on the global ladder: a weaker ladder during the lift keeps the
// re-lock; turning bypass prevention off ends it.
func TestBypassRelock(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	if err := runWith(t, 0, []string{"pw"}, "bypass", "set", "--count", "3", "--within", "10m", "--then", "warning",
		"--password-stdin", "--relock", "10m", "--yes"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"pw"}, "bypass", "unlock"); err != nil {
		t.Fatal(err)
	}
	trusted, _ := d.TrustedNow(time.Now())
	if l, _ := d.GetSettingLock(db.NepsisLockName); !l.PendingRelock(trusted) {
		t.Fatalf("after unlock: %+v", l)
	}
	mustRun(t, "bypass", "set", "--count", "5", "--within", "10m", "--then", "warning")
	if l, _ := d.GetSettingLock(db.NepsisLockName); !l.PendingRelock(trusted) {
		t.Errorf("a weaker ladder dropped the re-lock: %+v", l)
	}
	mustRun(t, "bypass", "off")
	if l, _ := d.GetSettingLock(db.NepsisLockName); l.PasswordHash != "" {
		t.Errorf("bypass off kept the lock: %+v", l)
	}
}

// Strict mode: unlock with re-lock lifts its lock for a while; it stays on.
func TestStrictRelock(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	if err := runWith(t, 0, []string{"pw"}, "strict", "on", "--password-stdin", "--relock", "15m", "--yes"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"pw"}, "strict", "unlock"); err != nil {
		t.Fatal(err)
	}
	trusted, _ := d.TrustedNow(time.Now())
	l, _ := d.GetSettingLock(enforce.StrictLockName)
	if v, _ := d.GetSetting(enforce.SettingStrict); v != "on" || !l.PendingRelock(trusted) || l.LiftedUntil != trusted+900 && l.LiftedUntil != trusted+899 {
		t.Errorf("strict %q, lock %+v", v, l)
	}
}
