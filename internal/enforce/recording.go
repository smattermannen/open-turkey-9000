// recording.go — link recording in the daemon. While a recording is on (see
// db/recording.go), the rules tell the extension, which reports the address
// of every page and frame the browser opens:
//
//	POST /visit {"url": "https://..."}
//
// The site (the address's domain) is kept until 'kc9k record stop'. Only
// requests with the extension's header count, and nothing here changes a
// block: the sites reach their list or cluster when the recording stops,
// through the CLI's gate.
package enforce

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
)

// recordedSite is the site of an address worth recording: the domain of a
// web page, without a leading www. (a domain entry covers its subdomains). Addresses without a domain
// (this computer, an IP address) aren't.
func recordedSite(address string) (string, bool) {
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	site := strings.TrimPrefix(blocker.NormalizeDomain(u.Hostname()), "www.")
	return site, blocker.IsValidDomain(site)
}

func visitHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Host != RulesAddress || r.Method != http.MethodPost || r.Header.Get(ExtensionHeader) != "1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var visit struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&visit); err != nil {
			http.Error(w, "bad report", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		recording, err := d.GetRecording()
		if err != nil || recording == nil {
			return
		}
		if site, ok := recordedSite(visit.URL); ok {
			if err := d.AddRecordedSite(site); err != nil {
				log.Printf("recording: %v", err)
			}
		}
	}
}
