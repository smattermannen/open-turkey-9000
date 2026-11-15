package db

import "testing"

// §3.12: recorded words ranked by specificity against the baseline; cut.
func TestRecordedWords(t *testing.T) {
	d, _ := openTest(t)
	if err := d.SetWordBaseline(true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if err := d.AddBaselinePage(map[string]int{"the": 30, "news": 2}); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.StartRecording(Recording{Cluster: "c", Words: true}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := d.AddRecordedPage(map[string]int{"the": 40, "calculus": 3, "integral": 1}); err != nil {
			t.Fatal(err)
		}
	}
	words, err := d.RecordedWords()
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 3 || words[0].Word != "calculus" || words[1].Word != "integral" || words[2].Word != "the" {
		t.Fatalf("ranking: %+v", words)
	}
	if words[0].Count != 9 || words[0].Pages != 3 || d.RecordedPages() != 3 || d.BaselinePages() != 10 {
		t.Errorf("counts: %+v pages %d/%d", words[0], d.RecordedPages(), d.BaselinePages())
	}
	if words[2].Specificity > 1 {
		t.Errorf("a word on every page is not specific: %v", words[2].Specificity)
	}
	if got := CutWords(words, 34); len(got) != 2 {
		t.Errorf("cut 34%% of 3 keeps %d", len(got))
	}
	if err := d.EndRecording(); err != nil {
		t.Fatal(err)
	}
	if words, _ := d.RecordedWords(); len(words) != 0 || d.RecordedPages() != 0 {
		t.Error("the recording's words stay after it ends")
	}
	if err := d.SetWordBaseline(false); err != nil || d.BaselinePages() != 0 || d.WordBaselineOn() {
		t.Error("baseline off keeps it")
	}
}
