package enforce

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
)

// A block that only greys out or strips images leaves its sites reachable:
// they stay out of /etc/hosts, the firewall and the browser policy, while its
// apps are still blocked.
func TestInputsSkipSitesWithoutBlockAction(t *testing.T) {
	active := []db.ActiveBlockDetail{
		{BlockName: "grey", Mode: db.ModeBlocklist, Actions: db.Actions{Greyscale: true},
			Sites: []string{"reddit.com"}, Apps: []string{"steam"}, Patterns: []string{"*casino*"}},
		{BlockName: "focus", Mode: db.ModeAllowlist, Actions: db.Actions{NoImages: true}, Sites: []string{"work.com"}},
		{BlockName: "hard", Mode: db.ModeBlocklist, Actions: db.Actions{Block: true, Greyscale: true}, Sites: []string{"x.com"}},
	}
	hosts, apps, bp := Inputs(active)
	if !reflect.DeepEqual(hosts, []string{"x.com"}) {
		t.Errorf("hosts = %v; want [x.com]", hosts)
	}
	if !reflect.DeepEqual(apps, []string{"steam"}) {
		t.Errorf("apps = %v; want [steam]", apps)
	}
	if bp.AllowMode || len(bp.BlockPatterns) != 0 || !reflect.DeepEqual(bp.BlockDomains, []string{"x.com"}) {
		t.Errorf("policy = %+v; want only x.com blocked", bp)
	}
}

func TestBuildRulesPerBlockActions(t *testing.T) {
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	add := func(name, mode string, a db.Actions, sites, exceptions []string) {
		t.Helper()
		if err := d.CreateBlock(name, mode); err != nil {
			t.Fatal(err)
		}
		if err := d.SetBlockActions(name, a); err != nil {
			t.Fatal(err)
		}
		if err := d.AddSites(name, sites); err != nil {
			t.Fatal(err)
		}
		if len(exceptions) > 0 {
			if err := d.AddExceptions(name, exceptions); err != nil {
				t.Fatal(err)
			}
		}
		if err := d.ActivateBlock(name, false, 0, 0, false); err != nil {
			t.Fatal(err)
		}
	}
	add("social", db.ModeBlocklist, db.Actions{Block: true}, []string{"facebook.com"}, nil)
	add("grey", db.ModeBlocklist, db.Actions{Greyscale: true, NoImages: true}, []string{"reddit.com"}, []string{"reddit.com/r/programming"})
	add("focus", db.ModeAllowlist, db.Actions{Greyscale: true}, []string{"work.com"}, nil)

	rules, err := BuildRules(d)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rules.Block, []string{"facebook.com"}) || len(rules.Exceptions) != 0 {
		t.Errorf("block = %v, exceptions = %v; want only facebook.com blocked", rules.Block, rules.Exceptions)
	}
	if !reflect.DeepEqual(rules.NoImages, Filter{Match: []string{"reddit.com"}, Except: []string{"reddit.com/r/programming"}}) {
		t.Errorf("no images = %+v", rules.NoImages)
	}
	grey := map[string]bool{}
	for _, m := range rules.Greyscale.Match {
		grey[m] = true
	}
	if !grey["reddit.com"] || !grey["*.*"] || len(rules.Greyscale.Match) != 2 {
		t.Errorf("greyscale match = %v; want reddit.com and *.* (allowlist)", rules.Greyscale.Match)
	}
	except := map[string]bool{}
	for _, e := range rules.Greyscale.Except {
		except[e] = true
	}
	if !except["work.com"] || !except["reddit.com/r/programming"] {
		t.Errorf("greyscale except = %v; want work.com and reddit.com/r/programming", rules.Greyscale.Except)
	}
}
