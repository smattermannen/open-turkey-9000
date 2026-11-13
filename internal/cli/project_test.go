package cli

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

// capture returns what a command prints.
func capture(t *testing.T, args ...string) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := runCLI(t, args...)
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if runErr != nil {
		t.Fatalf("%s: %v", strings.Join(args, " "), runErr)
	}
	return string(out)
}

func TestProjectExportImport(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "work")
	mustRun(t, "block", "add", "work", "reddit.com", "youtube.com")
	mustRun(t, "block", "add-app", "work", "discord")
	mustRun(t, "block", "add-title", "work", "YouTube")
	mustRun(t, "list", "create", "work", "Words", "--text")
	mustRun(t, "list", "add", "work", "Words", "casino")
	// Two days from now, so the window never holds the block while the test
	// runs (a held block refuses the looser edits below).
	day := strings.ToLower(time.Now().AddDate(0, 0, 2).Weekday().String()[:3])
	mustRun(t, "schedule", "add", "work", "--days", day, "--from", "09:00", "--to", "17:00")
	mustRun(t, "pomodoro", "set", "work", "25m", "5m", "--long", "15m", "--every", "4")
	mustRun(t, "break", "allowance", "work", "10m", "--per", "1h")
	mustRun(t, "break", "relock", "work", "5m")
	mustRun(t, "quota", "set", "work", "30m")
	mustRun(t, "nepsis", "set", "--block", "work", "--step", "3 in 10m: warning")
	mustRun(t, "cluster", "create", "papers")
	mustRun(t, "cluster", "add", "papers", "arxiv.org", "jstor.org")
	mustRun(t, "browser", "safesearch", "on")
	mustRun(t, "option", "polite-close", "off")
	file := filepath.Join(t.TempDir(), "project.json")
	if err := os.WriteFile(file, []byte(capture(t, "project", "export")), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := d.GetBlock("work")

	// Into the same setup: the block that exists is untouched, and the
	// imported one gets a new name (§3.13).
	out := capture(t, "project", "import", file)
	if !strings.Contains(out, "add block 'work' as 'work-2'") || !strings.Contains(out, "Cluster 'papers' exists already") {
		t.Errorf("import over itself said: %s", out)
	}
	if after, _ := d.GetBlock("work"); !reflect.DeepEqual(before.Lists, after.Lists) {
		t.Error("importing over an existing block changed it")
	}
	if copy, err := d.GetBlock("work-2"); err != nil || len(copy.Lists) != len(before.Lists) || copy.Active {
		t.Errorf("the renamed import: %+v, %v", copy, err)
	}

	// Into an empty setup: everything comes back, and nothing is started.
	t.Setenv("KC9K_DB", filepath.Join(t.TempDir(), "fresh.db"))
	fresh, err := openDB()
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	dry := capture(t, "project", "import", file, "--dry-run")
	if _, err := fresh.GetBlock("work"); err == nil || !strings.Contains(dry, "Would: add block 'work'") {
		t.Errorf("--dry-run wrote something or said: %s", dry)
	}
	capture(t, "project", "import", file)
	got, err := fresh.GetBlock("work")
	if err != nil {
		t.Fatal(err)
	}
	if got.Active || !reflect.DeepEqual(got.Apps, before.Apps) || !reflect.DeepEqual(got.Titles, before.Titles) || len(got.Lists) != len(before.Lists) {
		t.Errorf("imported block: %+v", got)
	}
	for i, l := range before.Lists {
		if g := got.Lists[i]; g.Name != l.Name || g.Kind != l.Kind || g.Where != l.Where || !reflect.DeepEqual(g.Entries, l.Entries) {
			t.Errorf("list %s came back as %+v", l.Name, g)
		}
	}
	if s, _ := fresh.ListSchedules("work"); len(s) != 1 || s[0].Window().Start != 9*60 || s[0].Window().End != 17*60 {
		t.Errorf("schedules = %+v", s)
	}
	if p, _ := fresh.GetPomodoro("work"); p == nil || p.On != 1500 || p.Off != 300 || p.Long != 900 || p.Every != 4 {
		t.Errorf("pomodoro = %+v", p)
	}
	if k, _ := fresh.GetBreaks("work"); k.Allowance != 600 || k.Per != 3600 || k.Relock != 300 || k.BalanceMs != 0 {
		t.Errorf("breaks = %+v", k)
	}
	if q, _ := fresh.GetQuota("work"); q == nil || q.Limit != 1800 {
		t.Errorf("quota = %+v", q)
	}
	if n, _ := fresh.BlockNepsis("work"); len(n.Steps) != 1 {
		t.Errorf("bypass detection = %+v", n)
	}
	if c, err := fresh.GetCluster("papers"); err != nil || len(c.Entries) != 2 {
		t.Errorf("cluster = %+v, %v", c, err)
	}
	if !enforce.BrowserOn(fresh, "safesearch") || enforce.OptionOn(fresh, enforce.OptionPoliteClose) {
		t.Error("browser option or behaviour option didn't come along")
	}

	// A file that isn't a project, or is broken, changes nothing.
	bad := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(bad, []byte(`{"kc9k_project":1,"blocks":[{"name":"x","apps":["python3"]}]}`), 0o644)
	if err := runCLI(t, "project", "import", bad); err == nil {
		t.Error("a project with a forbidden app was imported")
	}
	if _, err := fresh.GetBlock("x"); err == nil {
		t.Error("a broken project left a block behind")
	}
	os.WriteFile(bad, []byte(`{"name":"x","apps":[]}`), 0o644)
	if err := runCLI(t, "project", "import", bad); err == nil {
		t.Error("a single block's export was taken for a project")
	}
}

// §3.13: a backup holds places and place rules; a share leaves them out.
// Schedule locks and social media settings travel in both.
func TestProjectBackupAndShare(t *testing.T) {
	testDB(t)
	oldScan := enforce.ScanWifi
	enforce.ScanWifi = func(bool) ([]db.Network, error) {
		return []db.Network{{BSSID: "AA:AA:AA:AA:AA:01", SSID: "mine"}, {BSSID: "AA:AA:AA:AA:AA:02", SSID: "n"}}, nil
	}
	t.Cleanup(func() { enforce.ScanWifi = oldScan })
	mustRun(t, "place", "add", "home")
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "youtube.com")
	mustRun(t, "place", "rule", "b", "home", "on")
	mustRun(t, "schedule", "lock", "b", "on", "--lead", "30m")
	mustRun(t, "social", "b", "youtube:no-shorts", "on")
	backup, share := capture(t, "project", "export"), capture(t, "project", "export", "--share")
	if !strings.Contains(backup, "AA:AA:AA:AA:AA:01") || strings.Contains(share, "AA:AA:AA:AA:AA:01") || strings.Contains(share, `"home"`) {
		t.Fatalf("places: backup %v, share %v", strings.Contains(backup, "AA:AA"), strings.Contains(share, "AA:AA"))
	}

	for name, text := range map[string]string{"backup": backup, "share": share} {
		file := filepath.Join(t.TempDir(), name+".json")
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("KC9K_DB", filepath.Join(t.TempDir(), name+".db"))
		fresh, err := openDB()
		if err != nil {
			t.Fatal(err)
		}
		capture(t, "project", "import", file)
		if on, lead, _ := fresh.ScheduleLock("b"); !on || lead != 30 {
			t.Errorf("%s: schedule lock %v %d", name, on, lead)
		}
		if got, _ := fresh.GetBlock("b"); len(got.Social) != 1 {
			t.Errorf("%s: social = %v", name, got.Social)
		}
		p, err := fresh.GetPlace("home")
		if name == "backup" && (err != nil || len(p.Networks) != 2 || len(p.Blocks) != 1) {
			t.Errorf("backup: place = %+v, %v", p, err)
		}
		if name == "share" && err == nil {
			t.Error("share: a place came along")
		}
		fresh.Close()
	}
}
