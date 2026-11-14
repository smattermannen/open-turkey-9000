// phraseexport.go — phrase sets in block and project files (R-PHRASE-10).
// Phrases are written as they are typed ("[eat, nice] [hello, grandpa]"),
// so a file stays readable and goes through ParsePhrase again on import.
package cli

import (
	"fmt"

	"github.com/smattermannen/kc9k/internal/db"
)

// phraseSetExport is one phrase set in an export.
type phraseSetExport struct {
	Name    string          `json:"name"`
	Where   []string        `json:"where"`
	Test    bool            `json:"test,omitempty"`
	Actions []db.ListAction `json:"actions,omitempty"`
	Phrases []string        `json:"phrases"`
}

// exportPhraseSets collects a block's phrase sets for a file.
func exportPhraseSets(database *db.DB, block string) ([]phraseSetExport, error) {
	sets, err := database.PhraseSets(block)
	if err != nil {
		return nil, err
	}
	var out []phraseSetExport
	for _, s := range sets {
		e := phraseSetExport{Name: s.Name, Where: s.Where.List(), Test: s.Test, Actions: s.Actions, Phrases: []string{}}
		for _, p := range s.Phrases {
			e.Phrases = append(e.Phrases, p.String())
		}
		out = append(out, e)
	}
	return out, nil
}

// toPhraseSets validates phrase sets from a file, without writing anything.
func toPhraseSets(in []phraseSetExport) ([]db.PhraseSet, error) {
	var out []db.PhraseSet
	for _, e := range in {
		if err := db.CheckListName(e.Name); err != nil {
			return nil, err
		}
		s := db.PhraseSet{Name: e.Name, Test: e.Test}
		for _, place := range e.Where {
			if !db.IsPlace(place) {
				return nil, fmt.Errorf("phrase set '%s' looks at an unknown place %q", e.Name, place)
			}
			s.Where = s.Where.With(place, true)
		}
		if !s.Where.Any() {
			s.Where.Text = true
		}
		for _, a := range e.Actions {
			if !db.IsAction(a.Name) {
				return nil, fmt.Errorf("phrase set '%s' has an unknown action %q", e.Name, a.Name)
			}
			if err := db.CheckActionParams(a.Name, a.Param, a.Minutes); err != nil {
				return nil, fmt.Errorf("phrase set '%s': %w", e.Name, err)
			}
			if a.Distinct < 1 || a.Distinct > db.MaxThreshold {
				a.Distinct = 1
			}
			a.Hits = 1
			s.Actions = append(s.Actions, a)
		}
		for _, text := range e.Phrases {
			p, err := db.ParsePhrase(text)
			if err != nil {
				return nil, fmt.Errorf("phrase set '%s', %q: %w", e.Name, text, err)
			}
			s.Phrases = append(s.Phrases, p)
		}
		out = append(out, s)
	}
	return out, nil
}

// addPhraseSets writes validated phrase sets into a new block.
func addPhraseSets(database *db.DB, block string, sets []db.PhraseSet) error {
	for _, s := range sets {
		if err := database.AddPhraseSet(block, s); err != nil {
			return fmt.Errorf("phrase set '%s': %w", s.Name, err)
		}
	}
	return nil
}
