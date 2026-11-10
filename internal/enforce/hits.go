// hits.go — what the extension reports when a blacklist takes a page away,
// and the images it shows instead of pages.
//
// POST /hit {"list": id, "actions": [...], "url": "..."} logs the event for
// bypass prevention, and carries out the list's logout, shutdown and monk
// actions (the extension closes the tab and the browser itself). Only
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
	"os"
	"path/filepath"
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
		w.WriteHeader(http.StatusNoContent)
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
		logEvent(d, db.EventPage, block, hit.URL)
		for _, a := range device {
			c := Consequence{Kind: map[string]string{db.ActionLogout: db.ConsequenceLogout,
				db.ActionShutdown: db.ConsequenceShutdown, db.ActionMonk: db.ConsequenceMonk}[a.Name], Minutes: a.Minutes}
			why := "A page matched the list '" + list.Name + "' of block '" + block + "'."
			time.AfterFunc(deviceDelay, func() {
				if err := Run(d, c, why, false); err != nil {
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
