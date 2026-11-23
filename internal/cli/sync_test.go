package cli

import (
	"encoding/json"
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
