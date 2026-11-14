package db

import (
	"strings"
	"testing"
)

func phraseTexts(ps []Phrase) string {
	var out []string
	for _, p := range ps {
		out = append(out, p.String())
	}
	return strings.Join(out, " | ")
}

// Word combinations become phrases: a box needing one word is a word set;
// a box needing N different words is N one-term word sets, for every choice
// of N of its words (R-PHRASE-3).
func TestCombinationPhrases(t *testing.T) {
	cases := map[string]string{
		"cars(red, big)":                     "[cars] [red, big]",
		"(Car, lambo*) (big, small, nice):2": "[car, lambo*] [big] [small] | [car, lambo*] [big] [nice] | [car, lambo*] [small] [nice]",
		"(very cheap) (watch, rolex)":        "[very cheap] [watch, rolex]",
		"(a, b):2":                           "[a] [b]",
		"(a, b) (c):2":                       "", // never matched anything
		"(x, y):2 (p, q):2":                  "[x] [y] [p] [q]",
	}
	for combo, want := range cases {
		boxes, ok := ParseCombination(combo)
		if !ok {
			t.Fatalf("%q isn't a combination", combo)
		}
		ps, err := CombinationPhrases(boxes)
		if err != nil || phraseTexts(ps) != want {
			t.Errorf("%q: %q, %v; want %q", combo, phraseTexts(ps), err, want)
		}
	}
	boxes, _ := ParseCombination("(big-, x) (y)")
	if _, err := CombinationPhrases(boxes); err == nil {
		t.Error("a word ending in - was turned into a bridge")
	}
	boxes, _ = ParseCombination("(a, b, c, d, e, f, g, h, i):4 (j, k, l, m, n, o, p, q, r):4")
	if _, err := CombinationPhrases(boxes); err == nil {
		t.Error("a combination of thousands of phrases was moved")
	}
}

// The one-time move: a blacklist's combinations go into a new phrase set
// with its scope, test mode and actions (threshold 1); a term cluster's
// into a phrase cluster; exception lists and plain terms stay.
func TestCombinationMigration(t *testing.T) {
	d, path := openTest(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	words, err := d.CreateList("b", ListBlacklist, "Words", Where{Title: true, Text: true})
	if err != nil {
		t.Fatal(err)
	}
	spare, err := d.CreateList("b", ListExceptions, "Spare", Where{Text: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.CreateCluster("combos", Where{Text: true}); err != nil {
		t.Fatal(err)
	}
	c, _ := d.GetCluster("combos")
	if err := d.AddListEntries(words, []string{"nsfw", "(car, lambo*) (big, small, nice):2", "cars(red, big)", "(a) (b):2"}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetListAction(words, ActionGreyscale, true); err != nil {
		t.Fatal(err)
	}
	if err := d.SetListThreshold(words, ActionBlock, 3); err != nil {
		t.Fatal(err)
	}
	if err := d.SetTestList(words, true); err != nil {
		t.Fatal(err)
	}
	if err := d.AddListEntries(spare, []string{"(lecture) (notes, slides)"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.AddClusterEntries(c.ID, []string{"(eat) (grandpa)", "plain"}); err != nil {
		t.Fatal(err)
	}
	// An existing phrase set has the name the new one would take.
	if _, err := d.CreatePhraseSet("b", "words phrases", Where{Text: true}); err != nil {
		t.Fatal(err)
	}
	d.Close()
	for i := 0; i < 2; i++ { // the second open finds nothing more to move
		if d, err = OpenDB(path); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			d.Close()
		}
	}
	defer d.Close()
	b, _ := d.GetBlock("b")
	l, _ := b.Lists.Find("Words")
	if strings.Join(l.Entries, "|") != "nsfw" {
		t.Errorf("Words = %q; want only the plain term", l.Entries)
	}
	if l, _ := b.Lists.Find("Spare"); len(l.Entries) != 1 {
		t.Errorf("an exception list's combination moved: %q", l.Entries)
	}
	sets, _ := d.PhraseSets("b")
	if len(sets) != 2 {
		t.Fatalf("phrase sets = %+v", sets)
	}
	s := sets[1]
	block, _ := s.Action(ActionBlock)
	grey, _ := s.Action(ActionGreyscale)
	if s.Name != "Words phrases 2" || s.Where != (Where{Title: true, Text: true}) || !s.Test || len(s.Actions) != 2 ||
		block.Distinct != 1 || grey.Distinct != 1 {
		t.Errorf("phrase set = %+v", s)
	}
	if got := phraseTexts(s.Phrases); got != "[car, lambo*] [big] [small] | [car, lambo*] [big] [nice] | [car, lambo*] [small] [nice] | [cars] [red, big]" {
		t.Errorf("phrases = %q", got)
	}
	if c, _ := d.GetCluster("combos"); strings.Join(c.Entries, "|") != "plain" {
		t.Errorf("cluster = %+v", c)
	}
	if p, err := d.GetCluster("combos phrases"); err != nil || !p.Phrases || p.Where != (Where{Text: true}) ||
		strings.Join(p.Entries, "|") != "[eat] [grandpa]" {
		t.Errorf("phrase cluster = %+v, %v", p, err)
	}
}
