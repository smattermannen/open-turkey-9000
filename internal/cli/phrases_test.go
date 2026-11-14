package cli

import (
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

func phraseSet(t *testing.T, d *db.DB, block, name string) db.PhraseSet {
	t.Helper()
	s, err := d.FindPhraseSet(block, name)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPhraseSetCommands(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "phrase-set", "create", "b", "Grandpa")
	if err := runCLI(t, "phrase-set", "create", "b", "grandpa"); err == nil {
		t.Error("two phrase sets got the same name")
	}
	s := phraseSet(t, d, "b", "Grandpa")
	if s.Where != (db.Where{Text: true}) || len(s.Actions) != 1 || s.Actions[0].Name != db.ActionBlock || s.Actions[0].Distinct != 1 {
		t.Errorf("a new phrase set = %+v; want text, block at 1", s)
	}
	mustRun(t, "phrase-set", "add", "b", "Grandpa", "[Eat, nice] [hello, grandpa]", "[long cat, neko] [running, sitting] [fast]? :3")
	// Adding the same phrase again changes nothing; a bad one adds nothing.
	mustRun(t, "phrase-set", "add", "b", "Grandpa", "[eat, nice]  [hello, grandpa]")
	if err := runCLI(t, "phrase-set", "add", "b", "Grandpa", "[red] [car]", "eat grandpa"); err == nil {
		t.Error("a phrase without brackets was accepted")
	}
	s = phraseSet(t, d, "b", "Grandpa")
	got := []string{}
	for _, p := range s.Phrases {
		got = append(got, p.String())
	}
	if strings.Join(got, " | ") != "[eat, nice] [hello, grandpa] | [long cat, neko] [running, sitting] [fast]? :3" {
		t.Errorf("phrases = %q", got)
	}

	out, err := captureStdout(func() error { return runCLI(t, "phrase-set", "show", "b", "Grandpa") })
	if err != nil || !strings.Contains(out, "  [eat, nice] [hello, grandpa]\n") || !strings.Contains(out, "block (1)") {
		t.Errorf("show = %q, %v", out, err)
	}
	if err := runCLI(t, "phrase-set", "show", "b", "nothere"); err == nil {
		t.Error("show of a missing set succeeded")
	}

	// Removing checks every phrase first: nothing goes if one is missing.
	if err := runCLI(t, "phrase-set", "remove", "b", "Grandpa", "[eat, nice] [hello, grandpa]", "[x] [y]"); err == nil {
		t.Error("removing a missing phrase succeeded")
	}
	if n := len(phraseSet(t, d, "b", "Grandpa").Phrases); n != 2 {
		t.Errorf("a failed remove left %d phrases", n)
	}
	mustRun(t, "phrase-set", "remove", "b", "Grandpa", "[Eat, nice] [hello, grandpa]")

	mustRun(t, "phrase-set", "scope", "b", "Grandpa", "url", "on")
	mustRun(t, "phrase-set", "scope", "b", "Grandpa", "text", "off")
	if err := runCLI(t, "phrase-set", "scope", "b", "Grandpa", "url", "off"); err == nil {
		t.Error("a phrase set was left looking nowhere")
	}
	mustRun(t, "phrase-set", "test", "b", "Grandpa", "on")
	mustRun(t, "phrase-set", "rename", "b", "Grandpa", "Cats")
	if s := phraseSet(t, d, "b", "Cats"); s.Where != (db.Where{URL: true}) || !s.Test || len(s.Phrases) != 1 {
		t.Errorf("after edits: %+v", s)
	}
	mustRun(t, "phrase-set", "delete", "b", "Cats")
	if sets, _ := d.PhraseSets("b"); len(sets) != 0 {
		t.Errorf("sets after delete: %+v", sets)
	}
}

// R-PHRASE-1: a phrase has at least two word sets, and the first two are
// necessary; later ones may be optional.
func TestRPhrase1(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "phrase-set", "create", "b", "P")
	if err := runCLI(t, "phrase-set", "add", "b", "P", "[eat, nice]"); err == nil {
		t.Error("a phrase of one word set was accepted")
	}
	mustRun(t, "phrase-set", "add", "b", "P", "[a] [b]? [c]?")
	p := phraseSet(t, d, "b", "P").Phrases[0]
	if !p.Sets[0].Necessary || !p.Sets[1].Necessary || p.Sets[2].Necessary {
		t.Errorf("word sets = %+v; want the first two necessary, the third optional", p.Sets)
	}
	// R-PHRASE-2: the length is from the necessary sets to all of them.
	if err := runCLI(t, "phrase-set", "add", "b", "P", "[a] [b] [c]? :4"); err == nil {
		t.Error("a length beyond the word sets was accepted")
	}
	if err := runCLI(t, "phrase-set", "add", "b", "P", "[a] [b] [c] :2"); err == nil {
		t.Error("a length below the necessary word sets was accepted")
	}
}

// R-PHRASE-8: a bridge needs both ends, each in another word set, and a
// phrase with a one-ended bridge is refused when entered.
func TestRPhrase8(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "phrase-set", "create", "b", "P")
	refused := []string{
		"[-gorilla, cat] [jumping, river]", // SPEC's example: no big-
		"[big-, strong] [cat, river]",      // no -gorilla
		"[big-, -gorilla] [cat]",           // both ends in one word set
		"[-strong-] [big] [cat]",           // a middle link without ends
		"[big-] [-strong-]",                // a middle link needs three sets
		"[big-, -gorilla] [-strong-] [x]",  // the ends share a set
	}
	for _, text := range refused {
		err := runCLI(t, "phrase-set", "add", "b", "P", text)
		if err == nil || !strings.Contains(err.Error(), "bridge") && !strings.Contains(err.Error(), "middle link") {
			t.Errorf("%s: %v; want refused as a one-ended bridge", text, err)
		}
	}
	if n := len(phraseSet(t, d, "b", "P").Phrases); n != 0 {
		t.Errorf("%d refused phrases were stored", n)
	}
	for _, text := range []string{
		"[big-, strong] [-gorilla, cat]", // R-PHRASE-7's example
		"[big-] [-strong-] [-gorilla]",   // a middle link
		"[a-, b] [-c, d-] [-e]",          // bridges both ways
	} {
		if err := runCLI(t, "phrase-set", "add", "b", "P", text); err != nil {
			t.Errorf("%s was refused: %v", text, err)
		}
	}
}

// R-PHRASE-9: each action has its own threshold of different phrases
// (default 1); phrase sets have no hit count and no rate.
func TestRPhrase9(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "phrase-set", "create", "b", "P", "--title", "--text")
	mustRun(t, "phrase-set", "add", "b", "P", "[eat] [grandpa]", "[long cat] [running]", "[red] [car]")
	mustRun(t, "phrase-set", "action", "b", "P", "Greyscale", "on")
	mustRun(t, "phrase-set", "threshold", "b", "P", "block", "3")
	s := phraseSet(t, d, "b", "P")
	if a, _ := s.Action(db.ActionBlock); a.Distinct != 3 {
		t.Errorf("block threshold = %d, want 3", a.Distinct)
	}
	if a, _ := s.Action(db.ActionGreyscale); a.Distinct != 1 {
		t.Errorf("greyscale threshold = %d, want the default 1", a.Distinct)
	}
	if err := runCLI(t, "phrase-set", "threshold", "b", "P", "no-images", "2"); err == nil {
		t.Error("a threshold for an action the set doesn't have was accepted")
	}
	if err := runCLI(t, "phrase-set", "threshold", "b", "P", "block", "0"); err == nil {
		t.Error("threshold 0 was accepted")
	}
	for _, c := range phraseSetCmd.Commands() {
		if c.Name() == "hits" || c.Name() == "rate" {
			t.Errorf("phrase sets have a %s command", c.Name())
		}
	}
	if err := runCLI(t, "list", "hits", "b", "P", "block", "2"); err == nil {
		t.Error("list hits reached a phrase set")
	}

	// The extension gets each action with its own threshold, and its scope.
	mustRun(t, "start", "b")
	rules, err := enforce.BuildRules(d)
	if err != nil {
		t.Fatal(err)
	}
	var rule *enforce.ListRule
	for i := range rules.Lists {
		if rules.Lists[i].ID == -s.ID {
			rule = &rules.Lists[i]
		}
	}
	if rule == nil {
		t.Fatalf("no rule for the phrase set in %+v", rules.Lists)
	}
	want := map[string]int{db.ActionBlock: 3, db.ActionGreyscale: 1}
	for _, a := range rule.Actions {
		if want[a.Name] != a.Distinct || a.Hits != 1 {
			t.Errorf("rule action %+v", a)
		}
		delete(want, a.Name)
	}
	if len(want) > 0 || len(rule.Phrases) != 3 || strings.Join(rule.PhraseScope, ",") != "title,text" {
		t.Errorf("rule = %+v", rule)
	}
	// A set in test mode reports instead of acting.
	mustRun(t, "stop", "b")
	mustRun(t, "phrase-set", "test", "b", "P", "on")
	mustRun(t, "start", "b")
	rules, _ = enforce.BuildRules(d)
	for _, r := range rules.Lists {
		if r.ID == -s.ID && !r.Test {
			t.Error("a phrase set in test mode went out without test")
		}
	}
	mustRun(t, "stop", "b")
}

// R-LOCK-1: what strengthens a phrase set is always allowed; what weakens
// it is refused while the block is locked.
func TestPhraseSetGating(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "phrase-set", "create", "b", "P")
	mustRun(t, "phrase-set", "add", "b", "P", "[eat] [grandpa]")
	mustRun(t, "phrase-set", "action", "b", "P", "greyscale", "on")
	mustRun(t, "phrase-set", "threshold", "b", "P", "block", "3")
	mustRun(t, "phrase-set", "test", "b", "P", "on")
	if err := d.ActivateBlock("b", true, 300, 0, false); err != nil {
		t.Fatal(err)
	}
	allowed := [][]string{
		{"phrase-set", "add", "b", "P", "[red] [car]"},
		{"phrase-set", "create", "b", "Q"},
		{"phrase-set", "scope", "b", "P", "title", "on"},
		{"phrase-set", "action", "b", "P", "no-images", "on"},
		{"phrase-set", "threshold", "b", "P", "block", "2"},
		{"phrase-set", "test", "b", "P", "off"},
		{"phrase-set", "delete", "b", "Q"}, // empty: nothing is lost
	}
	for _, args := range allowed {
		if err := runCLI(t, args...); err != nil {
			t.Errorf("%s on a locked block: %v; want allowed", strings.Join(args, " "), err)
		}
	}
	refused := [][]string{
		{"phrase-set", "remove", "b", "P", "[eat] [grandpa]"},
		{"phrase-set", "scope", "b", "P", "text", "off"},
		{"phrase-set", "action", "b", "P", "greyscale", "off"},
		{"phrase-set", "threshold", "b", "P", "block", "3"},
		{"phrase-set", "test", "b", "P", "on"},
		{"phrase-set", "delete", "b", "P"},
	}
	for _, args := range refused {
		if err := runCLI(t, args...); err == nil || !strings.Contains(err.Error(), "locked") {
			t.Errorf("%s on a locked block: %v; want refused as locked", strings.Join(args, " "), err)
		}
	}
	s := phraseSet(t, d, "b", "P")
	if a, _ := s.Action(db.ActionBlock); len(s.Phrases) != 2 || s.Test || !s.Where.Text || a.Distinct != 2 || len(s.Actions) != 3 {
		t.Errorf("after refused edits: %+v", s)
	}
}

// R-MONK-1: monk mode's own block can't start monk mode, from a phrase set
// either.
func TestPhraseSetMonk(t *testing.T) {
	testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "phrase-set", "create", "b", "P")
	if err := runCLI(t, "phrase-set", "action", "b", "P", "greyscale", "off", "--for", "1h"); err == nil {
		t.Error("--for was taken with off")
	}
	if err := runCLI(t, "phrase-set", "action", "b", "P", "block", "off"); err == nil {
		t.Error("a phrase set was left without actions")
	}
	if err := runCLI(t, "phrase-set", "action", "b", "P", "launch", "on"); err == nil {
		t.Error("an unknown action was accepted")
	}
}

// A block whose last phrase goes is left with nothing to enforce and stops,
// as with lists.
func TestPhraseSetSettle(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	if err := runCLI(t, "list", "delete", "b", "Blocked"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "phrase-set", "create", "b", "P")
	mustRun(t, "phrase-set", "add", "b", "P", "[eat] [grandpa]")
	mustRun(t, "start", "b")
	if on, _ := d.IsBlockActive("b"); !on {
		t.Fatal("a block with only a phrase set didn't start")
	}
	mustRun(t, "phrase-set", "remove", "b", "P", "[eat] [grandpa]")
	if on, _ := d.IsBlockActive("b"); on {
		t.Error("a block with nothing left to enforce stayed on")
	}
}
