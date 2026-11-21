package enforce

import (
	"slices"
	"testing"
)

// §3.1: with tamper "allow file:// blocking" on, the extension judges local
// files too: each list gets its file entries and the block's file
// exceptions (file://*, R-LIST-3). Off, nothing about files is sent.
func TestRulesFilePages(t *testing.T) {
	d := testDB(t)
	newUserBlock(t, d, "files", "file:///home/u/games/*", "steam")
	ruleOf := func() ListRule {
		rules, err := BuildRules(d)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rules.Lists {
			if r.Block == "files" && r.Phrases == nil {
				return r
			}
		}
		t.Fatal("no rule for the block")
		return ListRule{}
	}
	if r := ruleOf(); r.FilePages || len(r.Files) > 0 || len(r.Except.Files) > 0 || slices.Contains(r.URL, "file:///home/u/games/*") {
		t.Errorf("file blocking off: %+v", r)
	}
	if err := d.SetSetting(SettingFileURLs, "on"); err != nil {
		t.Fatal(err)
	}
	r := ruleOf()
	if !r.FilePages || !slices.Equal(r.Files, []string{"file:///home/u/games/*"}) || !slices.Contains(r.Except.Files, "file:///*") {
		t.Errorf("file blocking on: pages %v files %v except %v", r.FilePages, r.Files, r.Except.Files)
	}
	if slices.Contains(r.URL, "file:///home/u/games/*") {
		t.Error("a file entry among the web addresses")
	}
}
