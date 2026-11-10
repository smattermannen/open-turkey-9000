package enforce

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
)

// A list that only greys out or strips images leaves its sites reachable:
// they stay out of /etc/hosts, the firewall and the browser policy, while its
// block's apps are still blocked. Lists that don't look at addresses stay out
// too, and so does a block with exceptions by title or text.
func TestInputsOnlyBlockingAddressLists(t *testing.T) {
	url := db.Where{URL: true}
	grey := withActions(blacklist("Grey", url, "reddit.com", "*casino*"), db.ListAction{Name: db.ActionGreyscale, Distinct: 1, Hits: 1})
	active := []db.ActiveBlockDetail{
		{BlockName: "grey", Lists: db.Lists{grey}, Apps: []string{"steam"}},
		{BlockName: "words", Lists: db.Lists{blacklist("Words", db.Where{Text: true, Title: true}, "youtube.com")}},
		{BlockName: "spared", Lists: db.Lists{blacklist("Blocked", url, "youtube.com"),
			exceptionList("Lectures", db.Where{Title: true}, "lecture")}},
		{BlockName: "hard", Lists: db.Lists{withActions(blacklist("Blocked", url, "x.com", "y.com"),
			db.ListAction{Name: db.ActionBlock, Distinct: 1, Hits: 1}, db.ListAction{Name: db.ActionGreyscale, Distinct: 1, Hits: 1}),
			exceptionList("Exceptions", url, "y.com/home")}},
	}
	hosts, apps, bp := Inputs(active)
	if !reflect.DeepEqual(hosts, []string{"x.com"}) {
		t.Errorf("hosts = %v; want [x.com] (y.com has an exception under it)", hosts)
	}
	if !reflect.DeepEqual(apps, []string{"steam"}) {
		t.Errorf("apps = %v; want [steam]", apps)
	}
	if len(bp.BlockPatterns) != 0 || !reflect.DeepEqual(bp.BlockDomains, []string{"x.com", "y.com"}) ||
		!reflect.DeepEqual(bp.Exceptions, []string{"y.com/home"}) {
		t.Errorf("policy = %+v; want x.com and y.com blocked, y.com/home excepted", bp)
	}
}

func TestBuildRulesLists(t *testing.T) {
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, d, "b", db.DefaultBlockedList, "facebook.com", "nsfw")
	mustAdd(t, d, "b", db.DefaultExceptionsList, "facebook.com/help")
	id, err := d.CreateList("b", db.ListBlacklist, "Words", db.Where{Title: true, Text: true})
	if err != nil {
		t.Fatal(err)
	}
	mustAdd(t, d, "b", "Words", "nsfw", "18+")
	if err := d.SetListAction(id, db.ActionGreyscale, true); err != nil {
		t.Fatal(err)
	}
	if err := d.SetListThreshold(id, db.ActionBlock, 2); err != nil {
		t.Fatal(err)
	}
	// An empty list sends nothing.
	if _, err := d.CreateList("b", db.ListBlacklist, "Empty", db.Where{URL: true}); err != nil {
		t.Fatal(err)
	}
	if err := d.ActivateBlock("b", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}

	rules, err := BuildRules(d)
	if err != nil {
		t.Fatal(err)
	}
	except := RuleExcept{URL: []string{"facebook.com/help"}, Title: []string{}, Text: []string{}}
	want := []ListRule{
		{Block: "b", Name: "Blocked", URL: []string{"facebook.com"}, Title: []string{}, Text: []string{},
			Actions: []RuleAction{{Name: "block", Distinct: 1, Hits: 1}}, Except: except},
		{Block: "b", Name: "Words", URL: []string{}, Title: []string{"nsfw", "18+"}, Text: []string{"nsfw", "18+"},
			Actions: []RuleAction{{Name: "block", Distinct: 2, Hits: 1}, {Name: "greyscale", Distinct: 1, Hits: 1}}, Except: except},
	}
	if !reflect.DeepEqual(rules.Lists, want) {
		t.Errorf("lists:\n got %+v\nwant %+v", rules.Lists, want)
	}
	if !reflect.DeepEqual(rules.Block, []string{"facebook.com"}) || !reflect.DeepEqual(rules.Exceptions, []string{"facebook.com/help"}) {
		t.Errorf("old-extension rules: block %v, exceptions %v", rules.Block, rules.Exceptions)
	}
	active, err := d.GetActiveBlocks()
	if err != nil {
		t.Fatal(err)
	}
	if !ExtensionRequired(d, active) {
		t.Error("a block with websites doesn't require the extension")
	}
}
