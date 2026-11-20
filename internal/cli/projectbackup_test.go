package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
)

// §3.13: a backup holds running locks, passwords, hotkeys and statistics; a
// share doesn't. Import adds them (locks confirmed, R-LOCK-12) and never
// replaces what is here.
func TestBackupHoldsEverything(t *testing.T) {
	notTerminal(t)
	old := syncDesktopHotkeys
	syncDesktopHotkeys = func([]blocker.DesktopHotkey) error { return nil }
	t.Cleanup(func() { syncDesktopHotkeys = old })

	d := testDB(t)
	mustRun(t, "block", "create", "work")
	mustRun(t, "block", "add", "work", "reddit.com")
	if err := runWith(t, 0, []string{"pw"}, "start", "work", "--password-stdin", "--for", "2h", "--range", "mon-sun 00:00-23:59", "--yes"); err != nil {
		t.Fatal(err)
	}
	// A second password lock waits behind the others (§4.6).
	if err := runWith(t, 0, []string{"pw2"}, "start", "work", "--password-stdin", "--yes"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "browser", "safesearch", "on", "--for", "3h", "--yes")
	mustRun(t, "hotkey", "add", "ctrl+alt+w", "work", "--yes")
	if err := d.AddUsage("2026-11-01", db.UsageSite, "reddit.com", 600); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"secret1"}, "password", "set"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"view1"}, "block", "password", "work", "set"); err != nil {
		t.Fatal(err)
	}
	backup := capture(t, "project", "export")
	share := capture(t, "project", "export", "--share")
	if strings.Contains(share, "\"backup\"") || !strings.Contains(backup, "settings_password_hash") {
		t.Fatalf("share has backup parts, or backup lacks them")
	}
	file := filepath.Join(t.TempDir(), "backup.json")
	if err := os.WriteFile(file, []byte(backup), 0o644); err != nil {
		t.Fatal(err)
	}

	// A fresh computer.
	d2 := testDB(t)
	if err := runCLI(t, "project", "import", file); err == nil {
		t.Error("imported locks without confirmation")
	}
	capture(t, "project", "import", file, "--yes")
	detail, _ := d2.GetBlock("work")
	e, _ := d2.GetLockExtras("work")
	if !detail.Active || e.PasswordHash == "" || e.TimerAfter != 7200 || e.Order != "password,timer,range" || e.RangeDays == 0 ||
		len(e.Queue) != 1 || !db.CheckPassword(e.Queue[0].PasswordHash, "pw2") {
		t.Errorf("block lock not restored: active %v, %+v", detail.Active, e)
	}
	if l, _ := d2.GetSettingLock("safesearch"); l.LockedUntil == 0 {
		t.Error("setting lock not restored")
	}
	if v, _ := d2.GetSetting(SettingSettingsPassword); v == "" {
		t.Error("settings password not restored")
	}
	if h, _ := d2.Hotkeys(); len(h) != 1 || h[0].Block != "work" {
		t.Errorf("hotkeys = %+v", h)
	}
	if rows, _ := d2.UsageSince(""); len(rows) != 1 || rows[0].Seconds != 600 {
		t.Errorf("statistics = %+v", rows)
	}
	if h, _ := d2.BlockViewPassword("work"); h == "" {
		t.Error("block password not restored")
	}
	// Again: nothing doubled, the existing password kept.
	if err := runWith(t, 0, []string{"secret1"}, "project", "import", file, "--yes"); err != nil {
		t.Fatal(err)
	}
	if rows, _ := d2.UsageSince(""); len(rows) != 1 || rows[0].Seconds != 600 {
		t.Errorf("statistics doubled: %+v", rows)
	}
	if h, _ := d2.Hotkeys(); len(h) != 1 {
		t.Errorf("hotkey doubled: %+v", h)
	}
}
