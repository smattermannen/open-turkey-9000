package db

import (
	"encoding/json"
	"os"
	"testing"
)

// TestSharedPhraseCases runs the phrase grammar cases the Android app's
// PhrasesTest runs too (testdata/phrase_cases.json, R-ANDROID-3).
func TestSharedPhraseCases(t *testing.T) {
	raw, err := os.ReadFile("testdata/phrase_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases struct {
		Parse []struct {
			Text  string `json:"text"`
			Key   string `json:"key"`
			Error bool   `json:"error"`
		} `json:"parse"`
		Widens [][3]any `json:"widens"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases.Parse {
		p, err := ParsePhrase(c.Text)
		if c.Error {
			if err == nil {
				t.Errorf("%q: parsed as %s, want an error", c.Text, p.Key())
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", c.Text, err)
		} else if p.Key() != c.Key {
			t.Errorf("%q: key %q, want %q", c.Text, p.Key(), c.Key)
		}
	}
	for _, c := range cases.Widens {
		old, err1 := ParsePhrase(c[0].(string))
		p, err2 := ParsePhrase(c[1].(string))
		if err1 != nil || err2 != nil {
			t.Fatalf("%v: %v %v", c, err1, err2)
		}
		if got := p.Widens(old); got != c[2].(bool) {
			t.Errorf("%s widens %s: %v, want %v", c[1], c[0], got, c[2])
		}
	}
}
