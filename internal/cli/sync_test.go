package cli

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/circadian"
	"github.com/smattermannen/kc9k/internal/cloud/cloudtest"
	"github.com/smattermannen/kc9k/internal/db"
)

// §4.14 (2.10): an account made and signed in to; Upload all sends the
// setup less the device's own items; the session is kept and refreshed.
func TestSyncUpload(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	f := cloudtest.New()
	defer f.Close()
	old := accounts
	accounts = func() []string { return []string{"anna", "ben"} }
	defer func() { accounts = old }()

	if err := runCLI(t, "sync", "upload"); err == nil || !strings.Contains(err.Error(), "isn't set up") {
		t.Errorf("upload without a cloud: %v", err)
	}
	mustRun(t, "account", "server", f.URL, cloudtest.AnonKey)
	if out := capture(t, "account", "status", "--json"); !strings.Contains(out, `"available":true`) || strings.Contains(out, "email") {
		t.Errorf("status before signing in: %s", out)
	}
	if err := runCLI(t, "sync", "upload"); err == nil || !strings.Contains(err.Error(), "not signed in") {
		t.Errorf("upload signed out: %v", err)
	}
	if err := runWith(t, 0, []string{"secret123"}, "account", "signup", "me@example.org"); err != nil {
		t.Fatal(err)
	}
	if v, _ := d.GetSetting(settingCloudSession); !strings.Contains(v, "me@example.org") {
		t.Fatalf("session not kept: %q", v)
	}

	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "block", "add-app", "b", "steam", "/opt/game/game.bin")
	mustRun(t, "block", "users", "b", "anna")
	f.Expire() // the next call refreshes the session first
	if s := loadSession(d); s != nil {
		s.ExpiresAt = 0
		_ = saveSession(d, s)
	}
	mustRun(t, "sync", "upload")
	s := loadSession(d)
	if s == nil || s.ExpiresAt == 0 {
		t.Fatalf("refreshed session not kept: %+v", s)
	}
	raw, err := gunzipped(f.Object(s.UserID))
	if err != nil {
		t.Fatal(err)
	}
	var p projectExport
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if p.Kind != "sync" || len(p.Blocks) != 1 || p.Blocks[0].Name != "b" || p.Backup == nil {
		t.Fatalf("uploaded: %s", raw)
	}
	b := p.Blocks[0]
	if len(b.Users) != 0 || strings.Join(b.Apps, ",") != "steam" {
		t.Errorf("device's own items uploaded: users %v apps %v", b.Users, b.Apps)
	}
	out := capture(t, "account", "status")
	if !strings.Contains(out, "Signed in as me@example.org") || !strings.Contains(out, "Last upload") {
		t.Errorf("status: %s", out)
	}
	mustRun(t, "account", "logout")
	if loadSession(d) != nil {
		t.Error("logout kept the session")
	}
	if err := runWith(t, 0, []string{"wrong"}, "account", "login", "me@example.org"); err == nil {
		t.Error("a wrong password signed in")
	}
	if err := runWith(t, 0, []string{"secret123"}, "account", "login", "me@example.org"); err != nil {
		t.Error(err)
	}
}

func TestSyncSetupLeavesDeviceItems(t *testing.T) {
	p := projectExport{
		Places:    []placeExport{{Name: "home"}},
		Wallpaper: &wallpaperExport{},
		Circadian: &circadianExport{Config: circadian.Config{Wallpapers: []circadian.Wallpaper{{Image: "x.png"}}}, Wallpapers: map[string]string{"x.png": "AA"}},
		Modes:     []modeExport{{Name: "m", Places: []string{"home"}, Wallpapers: []circadian.Wallpaper{{Image: "x.png"}}}},
		Backup: &backupExtras{Statistics: []db.UsageRow{{}},
			SettingLocks: []db.SettingLock{{Name: "strict"}, {Name: "no-escape"}, {Name: "firmware-lock"}, {Name: "safesearch-google"}}},
	}
	var b projectBlock
	b.Name = "b"
	b.Apps = []string{"steam", "/usr/bin/x", `C:\Games\x.exe`, "sha256:ab:1:/usr/bin/x", "Discord.exe"}
	b.Places, b.Users = []string{"home"}, []string{"anna"}
	p.Blocks = []projectBlock{b}
	syncSetup(&p)
	if p.Places != nil || p.Wallpaper != nil || p.Circadian.Wallpapers != nil || p.Circadian.Config.Wallpapers != nil ||
		p.Modes[0].Places != nil || p.Modes[0].Wallpapers != nil || p.Backup.Statistics != nil {
		t.Errorf("device items left: %+v", p)
	}
	if len(p.Backup.SettingLocks) != 1 || p.Backup.SettingLocks[0].Name != "safesearch-google" {
		t.Errorf("setting locks: %+v", p.Backup.SettingLocks)
	}
	if got := strings.Join(p.Blocks[0].Apps, ","); got != "steam,Discord.exe" || p.Blocks[0].Users != nil || p.Blocks[0].Places != nil {
		t.Errorf("block: apps %s users %v places %v", got, p.Blocks[0].Users, p.Blocks[0].Places)
	}
}

// §4.14 Download all: device A's setup replaces device B's; B keeps its own
// items (affected users, apps' paths); locks come along; a lock set on B
// stops the download, one that came by sync doesn't (ROADMAP §5 DECIDED
// 2026-11-23).
func TestSyncDownload(t *testing.T) {
	notTerminal(t)
	f := cloudtest.New()
	defer f.Close()
	old := accounts
	accounts = func() []string { return []string{"anna", "ben"} }
	defer func() { accounts = old }()
	device := func() (*db.DB, string) {
		path := filepath.Join(t.TempDir(), "t.db")
		t.Setenv("KC9K_DB", path)
		d, err := db.OpenDB(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { d.Close() })
		mustRun(t, "account", "server", f.URL, cloudtest.AnonKey)
		return d, path
	}
	a, _ := device()
	if err := runWith(t, 0, []string{"secret123"}, "account", "signup", "me@example.org"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "block", "create", "work")
	mustRun(t, "block", "add", "work", "reddit.com")
	mustRun(t, "block", "create", "games")
	mustRun(t, "block", "add-app", "games", "steam")
	mustRun(t, "start", "work", "--for", "2h", "--yes")
	mustRun(t, "sync", "upload")

	b, _ := device()
	if err := runWith(t, 0, []string{"secret123"}, "account", "login", "me@example.org"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "block", "create", "old")
	mustRun(t, "block", "create", "games")
	mustRun(t, "block", "add-app", "games", "/opt/x/run.bin")
	mustRun(t, "block", "users", "games", "anna")
	if out := capture(t, "sync", "download", "--dry-run"); !strings.Contains(out, "Would: add block 'work'") ||
		!strings.Contains(out, "Would: set a lock") {
		t.Errorf("dry run: %s", out)
	}
	if _, err := b.GetBlock("old"); err != nil {
		t.Fatal("the dry run changed something")
	}
	if err := runCLI(t, "sync", "download"); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("download without confirming: %v", err)
	}
	mustRun(t, "sync", "download", "--yes")
	if _, err := b.GetBlock("old"); err == nil {
		t.Error("a block of this device stayed")
	}
	work, err := b.GetBlock("work")
	if err != nil || !work.Active || work.LockedUntil == 0 {
		t.Errorf("the locked block came without its lock: %+v %v", work, err)
	}
	games, _ := b.GetBlock("games")
	if !slices.Contains(games.Apps, "steam") || !slices.Contains(games.Apps, "/opt/x/run.bin") {
		t.Errorf("games' apps: %v", games.Apps)
	}
	if u, _ := b.BlockUsers("games"); !slices.Equal(u, []string{"anna"}) {
		t.Errorf("the device's affected users: %v", u)
	}
	// The lock came by sync: downloading again is allowed.
	mustRun(t, "sync", "download", "--yes")
	// A lock set here stops it, and nothing changes.
	mustRun(t, "start", "games", "--for", "1h", "--yes")
	err = runCLI(t, "sync", "download", "--yes")
	if err == nil || !strings.Contains(err.Error(), "block 'games'") || strings.Contains(err.Error(), "'work'") {
		t.Fatalf("download over a lock set here: %v", err)
	}
	if g, _ := b.GetBlock("games"); !g.Active {
		t.Error("the refused download changed the block")
	}

	// Signed in to another account, the lock that came from the first one
	// counts as set here: its empty upload lifts nothing.
	if err := runWith(t, 0, []string{"secret456"}, "account", "signup", "other@example.org"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "sync", "upload")
	err = runCLI(t, "sync", "download", "--yes")
	if err == nil || !strings.Contains(err.Error(), "block 'work'") {
		t.Errorf("another account's download over a synced lock: %v", err)
	}
	_ = a
}
