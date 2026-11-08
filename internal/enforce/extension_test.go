package enforce

import (
	"path/filepath"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
)

func TestExtensionRequired(t *testing.T) {
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	appsOnly := db.ActiveBlockDetail{BlockName: "games", Mode: db.ModeBlocklist, Apps: []string{"steam"}}
	sites := db.ActiveBlockDetail{BlockName: "social", Mode: db.ModeBlocklist, Sites: []string{"reddit.com"}}
	patterns := db.ActiveBlockDetail{BlockName: "casino", Mode: db.ModeBlocklist, Patterns: []string{"*casino*"}}
	everything := db.ActiveBlockDetail{BlockName: "focus", Mode: db.ModeAllowlist}

	cases := []struct {
		name   string
		active []db.ActiveBlockDetail
		want   bool
	}{
		{"nothing active", nil, false},
		{"a block of apps only", []db.ActiveBlockDetail{appsOnly}, false},
		{"a block with sites", []db.ActiveBlockDetail{appsOnly, sites}, true},
		{"a block with patterns", []db.ActiveBlockDetail{patterns}, true},
		{"an allowlist with nothing allowed", []db.ActiveBlockDetail{everything}, true},
	}
	for _, c := range cases {
		if got := ExtensionRequired(d, c.active); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}

	// Options only the extension enforces need it even without a block.
	for _, option := range []string{"web-only", "force-search"} {
		if err := d.SetSetting("browser."+option, "on"); err != nil {
			t.Fatal(err)
		}
		if !ExtensionRequired(d, nil) {
			t.Errorf("%s on: extension not required", option)
		}
		if err := d.SetSetting("browser."+option, "off"); err != nil {
			t.Fatal(err)
		}
	}
	// SafeSearch works through /etc/hosts in every browser.
	if err := d.SetSetting("browser.safesearch", "on"); err != nil {
		t.Fatal(err)
	}
	if ExtensionRequired(d, nil) {
		t.Error("SafeSearch alone required the extension")
	}
}
