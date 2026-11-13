package enforce

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
)

func TestRulesSoonTestAndDetails(t *testing.T) {
	d := testDB(t)
	for _, name := range []string{"on", "later", "words"} {
		if err := d.CreateBlock(name); err != nil {
			t.Fatal(err)
		}
	}
	mustAdd(t, d, "on", db.DefaultBlockedList, "reddit.com")
	mustAdd(t, d, "later", db.DefaultBlockedList, "youtube.com")
	if err := d.ActivateBlock("on", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	// "later" starts in about half an hour.
	now := time.Now()
	start := now.Add(30 * time.Minute)
	w := schedule.Window{Days: 1 << uint(start.Weekday()), Start: start.Hour()*60 + start.Minute(), End: 24 * 60}
	if w.Start >= w.End-1 || now.Weekday() != start.Weekday() {
		t.Skip("too close to midnight for this test")
	}
	if err := d.AddSchedule("later", w); err != nil {
		t.Fatal(err)
	}
	rules, err := BuildRules(d)
	if err != nil {
		t.Fatal(err)
	}
	if !rules.Details {
		t.Error("the block page details are off by default")
	}
	if len(rules.Soon) != 1 || rules.Soon[0].Block != "later" || rules.Soon[0].Kind != "schedule" ||
		rules.Soon[0].Seconds < 28*60 || rules.Soon[0].Seconds > 30*60 || len(rules.Soon[0].Lists) != 1 {
		t.Errorf("soon = %+v", rules.Soon)
	}
	if !slices.Contains(rules.Blocks, "later") || !slices.Contains(rules.Blocks, "on") {
		t.Errorf("blocks = %v", rules.Blocks)
	}

	// A pause lifts an active block and announces its return.
	trusted, _ := d.TrustedNow(now)
	if err := d.SetPause("on", trusted+300); err != nil {
		t.Fatal(err)
	}
	if rules, _ = BuildRules(d); len(rules.Lists) != 0 {
		t.Errorf("a paused block still has %d lists enforced", len(rules.Lists))
	}
	found := false
	for _, s := range rules.Soon {
		if s.Block == "on" && s.Kind == "break" && s.Seconds > 290 && s.Seconds <= 300 {
			found = true
		}
	}
	if !found {
		t.Errorf("the paused block isn't announced: %+v", rules.Soon)
	}
	if err := d.SetPause("on", 0); err != nil {
		t.Fatal(err)
	}

	// Test mode: a text list is marked and kept out of the browser policy;
	// an address list is not.
	if _, err := d.CreateList("on", db.ListBlacklist, "Words", db.Where{Text: true}); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, d, "on", "Words", "casino")
	id, _ := d.ListID("on", "Words")
	if err := d.SetListAction(id, db.ActionBlock, true); err != nil {
		t.Fatal(err)
	}
	blocked, _ := d.ListID("on", db.DefaultBlockedList)
	for _, l := range []int{id, blocked} {
		if err := d.SetTestList(l, true); err != nil {
			t.Fatal(err)
		}
	}
	rules, _ = BuildRules(d)
	for _, l := range rules.Lists {
		if l.Test != (l.Name == "Words") {
			t.Errorf("list %s: test = %v", l.Name, l.Test)
		}
	}
	if err := d.SetTestList(id, false); err != nil || len(d.TestLists()) != 1 {
		t.Errorf("test mode didn't turn off: %v %v", err, d.TestLists())
	}

	// Options and the Blocks password change what the extension is told.
	if err := SetOption(d, OptionPageDetails, false); err != nil {
		t.Fatal(err)
	}
	if err := d.SetSetting(db.AccessPasswordSetting+"blocks", "x:y"); err != nil {
		t.Fatal(err)
	}
	if rules, _ = BuildRules(d); rules.Details || len(rules.Blocks) != 0 {
		t.Errorf("details %v, blocks %v; want off and none", rules.Details, rules.Blocks)
	}
}

func TestAddHandler(t *testing.T) {
	d := testDB(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	post := func(body string, header bool) int {
		r := httptest.NewRequest(http.MethodPost, "http://"+RulesAddress+"/add", bytes.NewBufferString(body))
		if header {
			r.Header.Set(ExtensionHeader, "1")
		}
		w := httptest.NewRecorder()
		addHandler(d)(w, r)
		return w.Code
	}
	body, _ := json.Marshal(map[string]string{"block": "b", "url": "https://www.Example.com/some/page?x=1"})
	if code := post(string(body), false); code != http.StatusForbidden {
		t.Errorf("without the extension header: %d", code)
	}
	if code := post(string(body), true); code != http.StatusOK {
		t.Fatalf("add: %d", code)
	}
	detail, _ := d.GetBlock("b")
	if l, ok := detail.Lists.Find(db.DefaultBlockedList); !ok || !slices.Contains(l.Entries, "example.com") {
		t.Errorf("entries = %+v", detail.Lists)
	}
	for _, bad := range []string{`{"block":"b","url":"file:///etc/passwd"}`, `{"block":"b","url":"https://bad host/"}`} {
		if code := post(bad, true); code != http.StatusForbidden {
			t.Errorf("%s: %d", bad, code)
		}
	}
	if code := post(`{"block":"nope","url":"https://example.org/"}`, true); code != http.StatusNotFound {
		t.Errorf("unknown block: %d", code)
	}
	// With an access password on Blocks, the browser can't edit them.
	if err := d.SetSetting(db.AccessPasswordSetting+"blocks", "x:y"); err != nil {
		t.Fatal(err)
	}
	if code := post(`{"block":"b","url":"https://example.org/"}`, true); code != http.StatusForbidden {
		t.Errorf("guarded: %d", code)
	}
}
