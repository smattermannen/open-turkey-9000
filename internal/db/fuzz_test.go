package db

import "testing"

// Phrases never panic; a phrase that is accepted writes back to the same
// phrase (what the GUI and remove/change rely on), loosely and strictly.
func FuzzParsePhrase(f *testing.F) {
	for _, seed := range []string{"[eat, nice] [hello, grandpa]", "[big-] [-gorilla] [x]? :3", "[a] [b] :2",
		"[] []", "[a]", "[a] [b", "[re:zero] [x]", "[a,,b] [c]?", ":3", "[a] [b] :0", "[-a-] [b] [c-] [-d]"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		loose, err := ParsePhraseLoose(text)
		if err != nil {
			return
		}
		again, err := ParsePhraseLoose(loose.String())
		if err != nil || !again.Same(loose) {
			t.Errorf("%q -> %q -> %q, %v", text, loose.String(), again.String(), err)
		}
		if strict, err := ParsePhrase(text); err == nil {
			back, err := ParsePhrase(strict.String())
			if err != nil || !back.Same(strict) {
				t.Errorf("strict %q -> %q -> %v", text, strict.String(), err)
			}
		}
	})
}
