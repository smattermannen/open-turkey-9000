// hits.go — what the extension reports when a blacklist takes a page away,
// and the images it shows instead of pages.
//
// POST /hit {"list": id, "actions": [...], "url": "...", "from": "..."} logs
// the event for bypass detection (as a search event when the page was reached
// from a search engine's results: see fromSearch), judges it at once, and carries out the list's logout,
// shutdown and monk actions (the extension closes the tab and the browser
// itself). It answers {"close_tab": true} when a bypass detection step that
// fired closes the tab of the page. Only
// requests with the extension's header count: a web page can't add it (see
// ExtensionHeader). A request can only trigger what an active blacklist
// already does, so the worst a local program can do is what the user chose.
//
// GET /image/<name> serves a picture stored for the image action.
package enforce

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
)

// deviceDelay gives the extension time to close the browser cleanly before
// logging out or shutting down.
const deviceDelay = 3 * time.Second

type hitReport struct {
	List    int      `json:"list"`
	Actions []string `json:"actions"`
	URL     string   `json:"url"`
	From    string   `json:"from"` // the page whose link led here, if any
	// Via says the site was reached another way: "proxy" (inside a proxy or
	// archive address) or "mirror" (another site showing it).
	Via string `json:"via"`
}

// searchHosts are search engines by host name: the name itself, or a
// subdomain of it. Google, Yandex and Yahoo are matched by label, for their
// many country domains.
var searchHosts = []string{"bing.com", "duckduckgo.com", "search.brave.com", "startpage.com",
	"ecosia.org", "qwant.com", "kagi.com", "baidu.com", "mojeek.com", "swisscows.com",
	"metager.org", "metager.de", "presearch.com", "you.com", "perplexity.ai", "ask.com", "aol.com", "naver.com"}

// searchLabels are search engines with many country domains (google.se,
// google.co.uk, yandex.ru): any host with this label counts.
var searchLabels = []string{"google", "yandex", "yahoo"}

// fromSearch reports whether an address is a search engine's: a blocked
// page opened from there was looked for, not stumbled upon. Besides the
// engines known by name, a results page of a self-hosted engine (SearXNG and
// the like) counts by its shape: a /search path with a q parameter.
func fromSearch(address string) bool {
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range searchHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	for _, label := range strings.Split(host, ".") {
		if slices.Contains(searchLabels, label) {
			return true
		}
	}
	return strings.TrimSuffix(u.Path, "/") == "/search" && u.Query().Has("q")
}

// findActiveList returns the active block (not on a break) and the
// blacklist with this id, if it does something.
func findActiveList(d *db.DB, listID int) (string, db.WebList, bool) {
	active, err := d.GetActiveBlocks()
	if err != nil {
		log.Printf("hit: %v", err)
		return "", db.WebList{}, false
	}
	trusted, err := d.TrustedNow(time.Now())
	if err != nil {
		log.Printf("hit: %v", err)
		return "", db.WebList{}, false
	}
	for _, b := range withoutBreaks(d, active, trusted) {
		for _, l := range b.Lists.Blacklists() {
			if l.ID == listID && l.Enforces() {
				return b.BlockName, l, true
			}
		}
	}
	return "", db.WebList{}, false
}

func hitHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Host != RulesAddress || r.Method != http.MethodPost || r.Header.Get(ExtensionHeader) != "1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var hit hitReport
		if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&hit); err != nil {
			http.Error(w, "bad report", http.StatusBadRequest)
			return
		}
		closeTab := false
		defer func() {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]bool{"close_tab": closeTab})
		}()
		block, list, ok := findActiveList(d, hit.List)
		if !ok {
			return
		}
		blocking := false
		var device []db.ListAction
		for _, name := range hit.Actions {
			a, taken := list.Action(name)
			if !taken {
				continue
			}
			blocking = blocking || db.IsBlockingAction(name)
			if name == db.ActionLogout || name == db.ActionShutdown || name == db.ActionMonk {
				device = append(device, a)
			}
		}
		if !blocking {
			return
		}
		kind := db.EventPage
		if fromSearch(hit.From) {
			kind = db.EventSearch
		}
		if hit.Via != "" {
			kind = db.EventWorkaround
		}
		logEvent(d, kind, block, hit.URL)
		closeTab = checkScopes(d, time.Now(), []string{"", block})
		for _, a := range device {
			c := Consequence{Kind: map[string]string{db.ActionLogout: db.ConsequenceLogout,
				db.ActionShutdown: db.ConsequenceShutdown, db.ActionMonk: db.ConsequenceMonk}[a.Name], Minutes: a.Minutes}
			why := "A page matched the list '" + list.Name + "' of block '" + block + "'."
			time.AfterFunc(deviceDelay, func() {
				if err := Run(d, c, why); err != nil {
					log.Printf("hit: %v", err)
				}
			})
		}
	}
}

func imageHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/image/")
	if r.Host != RulesAddress || r.Method != http.MethodGet || !db.IsImageName(name) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	data, err := os.ReadFile(filepath.Join(db.ImagesDir(), name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	types := map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp"}
	w.Header().Set("Content-Type", types[filepath.Ext(name)])
	w.Header().Set("Cache-Control", "max-age=86400")
	_, _ = w.Write(data)
}
