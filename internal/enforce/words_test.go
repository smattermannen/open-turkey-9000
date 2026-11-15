package enforce

import (
	"net/http"
	"net/http/httptest"
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
