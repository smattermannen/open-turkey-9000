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
