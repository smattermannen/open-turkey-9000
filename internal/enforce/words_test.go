package enforce

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
)

// §3.12: page words reach the word recording or the baseline only as the
// rules say, only from the extension, and only real words.
func TestWordsReported(t *testing.T) {
	d := testDB(t)
	handler := wordsHandler(d)
	post := func(header bool, body string) int {
		r := httptest.NewRequest(http.MethodPost, "http://"+RulesAddress+"/words", strings.NewReader(body))
		r.Host = RulesAddress
		if header {
			r.Header.Set(ExtensionHeader, "1")
		}
		w := httptest.NewRecorder()
		handler(w, r)
		return w.Code
	}
	page := `{"mode": "%s", "words": {"calculus": 3, "the": 9, "Bad": 1, "x": 1, "12": 1, "<b>": 1, "don't": 1}}`
	post(true, strings.Replace(page, "%s", "baseline", 1))
	if d.BaselinePages() != 0 {
		t.Error("baseline kept while off")
	}
	if rules, _ := BuildRules(d); rules.Words != "" {
		t.Errorf("rules.words = %q with nothing on", rules.Words)
	}
	_ = d.SetWordBaseline(true)
	if rules, _ := BuildRules(d); rules.Words != "baseline" {
		t.Errorf("rules.words = %q with the baseline on", rules.Words)
	}
	post(true, strings.Replace(page, "%s", "baseline", 1))
	if d.BaselinePages() != 1 {
		t.Error("baseline page not kept")
	}
	post(true, strings.Replace(page, "%s", "record", 1))
	if words, _ := d.RecordedWords(); len(words) != 0 {
		t.Error("recorded while not recording")
	}
	if err := d.StartRecording(db.Recording{Cluster: "c", Words: true}); err != nil {
		t.Fatal(err)
	}
	if rules, _ := BuildRules(d); rules.Words != "record" || rules.Recording {
		t.Errorf("rules while recording words: words %q, links %v", rules.Words, rules.Recording)
	}
	if post(false, strings.Replace(page, "%s", "record", 1)) != http.StatusForbidden {
		t.Error("a report without the extension's header was accepted")
	}
	post(true, strings.Replace(page, "%s", "record", 1))
	post(true, strings.Replace(page, "%s", "baseline", 1)) // not while recording words
	words, _ := d.RecordedWords()
	got := map[string]bool{}
	for _, w := range words {
		got[w.Word] = true
	}
	if len(words) != 3 || !got["calculus"] || !got["the"] || !got["don't"] || d.BaselinePages() != 1 {
		t.Errorf("recorded %+v, baseline pages %d", words, d.BaselinePages())
	}
}

// newUserBlock makes an active block with one blocked site and app.
func newUserBlock(t *testing.T, d *db.DB, name, site, app string, users ...string) {
	t.Helper()
	if err := d.CreateBlock(name); err != nil {
		t.Fatal(err)
	}
	b, _ := d.GetBlock(name)
	if err := d.AddListEntries(b.Lists.Blacklists()[0].ID, []string{site}); err != nil {
		t.Fatal(err)
	}
	if err := d.AddApps(name, []string{app}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetBlockUsers(name, users); err != nil {
		t.Fatal(err)
	}
	if err := d.ActivateBlock(name, false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
}

// §3.2: a block for some users stays out of the shared layers and reaches
// only their extensions.
func TestAffectedUsers(t *testing.T) {
	old := regularUsers
	regularUsers = func() []string { return []string{"anna", "ben"} }
	defer func() { regularUsers = old }()
	d := testDB(t)
	newUserBlock(t, d, "all", "reddit.com", "steam")
	newUserBlock(t, d, "some", "youtube.com", "minecraft", "anna")
	newUserBlock(t, d, "every", "tiktok.com", "roblox", "anna", "ben")
	active, err := d.GetActiveBlocks()
	if err != nil {
		t.Fatal(err)
	}
	hosts, apps, _ := Inputs(active)
	if !slices.Contains(hosts, "reddit.com") || slices.Contains(hosts, "youtube.com") || slices.Contains(apps, "minecraft") ||
		!slices.Contains(hosts, "tiktok.com") || !slices.Contains(apps, "roblox") {
		t.Errorf("shared layers: hosts %v apps %v", hosts, apps)
	}
	for _, b := range active {
		if (blockUIDs(b) == nil) != (b.BlockName != "some") {
			t.Errorf("blockUIDs(%s) = %v", b.BlockName, blockUIDs(b))
		}
	}
	has := func(user string) bool {
		rules, err := BuildRulesFor(d, user)
		if err != nil {
			t.Fatal(err)
		}
		return slices.Contains(rules.Block, "youtube.com")
	}
	if !has("anna") || has("ben") || !has("") {
		t.Errorf("rules per user: anna %v ben %v unknown %v", has("anna"), has("ben"), has(""))
	}
}
