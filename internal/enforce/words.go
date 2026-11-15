// words.go — word recording in the daemon (§3.12). While the rules say
// "words", the extension reports each page's rendered visible words once:
//
//	POST /words {"mode": "record"|"baseline", "words": {"word": times}}
//
// "record" pages go to the word recording, "baseline" pages (normal
// browsing, no private windows) to the baseline; nothing else is kept, not
// even the address. Only the extension's requests count.
package enforce

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"unicode"
	"unicode/utf8"

	"github.com/smattermannen/kc9k/internal/db"
)

// recordableWord: 2 to 40 characters, letters and digits (at least one
// letter), apostrophes and hyphens inside; lower case.
func recordableWord(w string) bool {
	n := utf8.RuneCountInString(w)
	if n < 2 || n > 40 {
		return false
	}
	letter := false
	for i, r := range w {
		switch {
		case unicode.IsLetter(r):
			letter = true
			if unicode.IsUpper(r) {
				return false
			}
		case unicode.IsDigit(r):
		case (r == '-' || r == '\'') && i > 0 && i < len(w)-1:
		default:
			return false
		}
	}
	return letter
}

func wordsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Host != RulesAddress || r.Method != http.MethodPost || r.Header.Get(ExtensionHeader) != "1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var report struct {
			Mode  string         `json:"mode"`
			Words map[string]int `json:"words"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&report); err != nil {
			http.Error(w, "bad report", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		words := map[string]int{}
		for word, n := range report.Words {
			if len(words) >= db.MaxPageWords {
				break
			}
			if recordableWord(word) && n > 0 {
				words[word] = min(n, 10000)
			}
		}
		if len(words) == 0 {
			return
		}
		var err error
		switch report.Mode {
		case "record":
			if rec, e := d.GetRecording(); e != nil || rec == nil || !rec.Words {
				return
			}
			err = d.AddRecordedPage(words)
		case "baseline":
			if rec, _ := d.GetRecording(); (rec != nil && rec.Words) || !d.WordBaselineOn() {
				return
			}
			err = d.AddBaselinePage(words)
		}
		if err != nil {
			log.Printf("words: %v", err)
		}
	}
}
