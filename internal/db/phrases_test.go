package db

import "testing"

func TestParsePhrase(t *testing.T) {
	p, err := ParsePhrase("[Eat, nice] [hello,  grandpa] [today]? :3")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Sets) != 3 || p.Sets[0].Terms[0] != "eat" || p.Sets[2].Necessary || p.Length != 3 {
		t.Fatalf("parsed = %+v", p)
	}
	if got := p.String(); got != "[eat, nice] [hello, grandpa] [today]? :3" {
		t.Errorf("String = %q", got)
	}
	for _, bad := range []string{"[a]", "eat grandpa", "[a] [b", "[-gorilla] [x]", "[-a-] [b]", "[a] [b] :5", "[a] [] "} {
		if _, err := ParsePhrase(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
	if _, err := ParsePhrase("[big-, strong] [-gorilla, cat]"); err != nil {
		t.Errorf("a bridge was refused: %v", err)
	}
}

func TestPhraseSets(t *testing.T) {
	d, _ := openTest(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	id, err := d.CreatePhraseSet("b", "Grandpa", Where{Text: true})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := ParsePhrase("[eat] [grandpa]")
	if added, err := d.AddPhrase(id, p); err != nil || !added {
		t.Fatalf("add: %v %v", added, err)
	}
	if added, _ := d.AddPhrase(id, p); added {
		t.Error("the same phrase was added twice")
	}
	if err := d.SetPhraseSetThreshold(id, ActionBlock, 2); err != nil {
		t.Fatal(err)
	}
	s, err := d.FindPhraseSet("b", "grandpa")
	if err != nil || len(s.Phrases) != 1 || len(s.Actions) != 1 || s.Actions[0].Distinct != 2 || !s.Where.Text {
		t.Fatalf("set = %+v, %v", s, err)
	}
	if removed, _ := d.RemovePhrase(id, p); !removed {
		t.Error("the phrase wasn't removed")
	}
	if err := d.RemoveBlock("b"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.PhraseSetByID(id); err == nil {
		t.Error("the phrase set outlived its block")
	}
}

// A phrase stored with its default length written out ("[a] [b] :2", from
// before phrases were canonical) reads the same as "[a] [b]": adding
// "[a] [b]" finds it, removing "[a] [b]" removes it, and it is listed once.
// So does a stored phrase today's rules refuse (a one-ended bridge).
func TestStoredPhrasesFoundByCanonicalForm(t *testing.T) {
	d, _ := openTest(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	id, err := d.CreatePhraseSet("b", "P", Where{Text: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range []string{
		`{"sets":[{"terms":["eat"],"necessary":true},{"terms":["grandpa"],"necessary":true}],"length":2}`,
		`{"sets":[{"terms":["eat"],"necessary":true},{"terms":["grandpa"],"necessary":true}]}`,
		`{"sets":[{"terms":["-gorilla"],"necessary":true},{"terms":["x"],"necessary":true}]}`,
	} {
		if _, err := d.conn.Exec("INSERT INTO phrases (set_id, spec) VALUES (?, ?)", id, spec); err != nil {
			t.Fatal(err)
		}
	}
	s, _ := d.FindPhraseSet("b", "P")
	if len(s.Phrases) != 2 || s.Phrases[0].String() != "[eat] [grandpa]" {
		t.Fatalf("listed: %v; want the two forms of [eat] [grandpa] once, and the bridge", s.Phrases)
	}
	p, _ := ParsePhrase("[Eat] [grandpa]")
	if added, err := d.AddPhrase(id, p); err != nil || added {
		t.Errorf("adding it again: added %v, %v", added, err)
	}
	if removed, err := d.RemovePhrase(id, p); err != nil || !removed {
		t.Errorf("removing [eat] [grandpa]: %v, %v", removed, err)
	}
	broken, err := ParsePhraseLoose("[-gorilla] [x]")
	if err != nil {
		t.Fatal(err)
	}
	if removed, err := d.RemovePhrase(id, broken); err != nil || !removed {
		t.Errorf("removing a phrase today's rules refuse: %v, %v", removed, err)
	}
	if s, _ := d.FindPhraseSet("b", "P"); len(s.Phrases) != 0 {
		t.Errorf("left: %v", s.Phrases)
	}
	// Canonical: the default length is 0, another one stays.
	q, _ := ParsePhrase("[a] [b] [c]? :2")
	r, _ := ParsePhrase("[a] [b] [c]? :3")
	if q.Canonical().Length != 0 || r.Canonical().Length != 3 || q.Same(r) {
		t.Errorf("canonical lengths: %+v %+v", q.Canonical(), r.Canonical())
	}
}
