// add.go — adding the site in the tab to a block, from the extension's
// toolbar button. Adding to a blacklist only ever makes a block stricter,
// so it needs no gate; it is refused when the Blocks page has an access
// password, since then not everyone at the browser may edit blocks.
package enforce

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
)

// blocksGuarded reports whether an access password guards editing blocks.
func blocksGuarded(d *db.DB) bool {
	for _, area := range []string{"app", "blocks"} {
		if hash, err := d.GetSetting(db.AccessPasswordSetting + area); err != nil || hash != "" {
			return true
		}
	}
	return false
}

// addableBlocks returns the blocks a site can be added to from the toolbar.
func addableBlocks(d *db.DB) []string {
	out := []string{}
	if blocksGuarded(d) {
		return out
	}
	blocks, err := d.ListBlocks()
	if err != nil {
		return out
	}
	for _, b := range blocks {
		if _, err := d.ListID(b.Name, db.DefaultBlockedList); err == nil {
			out = append(out, b.Name)
		}
	}
	return out
}

// siteOf returns the site of a web address, without "www.".
func siteOf(address string) (string, bool) {
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	site := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if len(blocker.ValidDomains([]string{site})) != 1 {
		return "", false
	}
	return site, true
}

func addHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Host != RulesAddress || r.Method != http.MethodPost || r.Header.Get(ExtensionHeader) != "1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var add struct {
			Block string `json:"block"`
			URL   string `json:"url"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&add); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		site, ok := siteOf(add.URL)
		if !ok || blocksGuarded(d) {
			http.Error(w, "refused", http.StatusForbidden)
			return
		}
		id, err := d.ListID(add.Block, db.DefaultBlockedList)
		if err != nil {
			http.Error(w, "no such block", http.StatusNotFound)
			return
		}
		if err := d.AddListEntries(id, []string{site}); err != nil {
			http.Error(w, "failed", http.StatusInternalServerError)
			return
		}
		log.Printf("add: %s added to block '%s' from the browser", site, add.Block)
		if detail, err := d.GetBlock(add.Block); err == nil && detail.Active {
			if err := ApplyFromDB(d); err != nil {
				log.Printf("add: %v", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"site": site})
	}
}

// darkGuarded reports whether the circadian settings sit behind an access
// password (the app's or the settings area's): then the browser can't
// change the dark pages' exemptions either.
func darkGuarded(d *db.DB) bool {
	for _, area := range []string{"app", "settings"} {
		if hash, err := d.GetSetting(db.AccessPasswordSetting + area); err != nil || hash != "" {
			return true
		}
	}
	return false
}

// darkExemptHandler takes a site out of dark pages, or back in, from the
// toolbar button (owner, 2026-11-16: "Exempt from dark pages"). Dark pages
// are never locked (§4.10), so it is always allowed but behind an access
// password.
func darkExemptHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Host != RulesAddress || r.Method != http.MethodPost || r.Header.Get(ExtensionHeader) != "1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var ask struct {
			URL    string `json:"url"`
			Exempt bool   `json:"exempt"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&ask); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		site, ok := siteOf(ask.URL)
		if !ok || darkGuarded(d) {
			http.Error(w, "refused", http.StatusForbidden)
			return
		}
		c := CircadianConfig(d)
		has := slices.Contains(c.DarkExempt, site)
		switch {
		case ask.Exempt && !has:
			c.DarkExempt = append(c.DarkExempt, site)
			slices.Sort(c.DarkExempt)
		case !ask.Exempt && has:
			c.DarkExempt = slices.DeleteFunc(c.DarkExempt, func(s string) bool { return s == site })
		}
		if err := SetCircadianConfig(d, c); err != nil {
			http.Error(w, "failed", http.StatusInternalServerError)
			return
		}
		log.Printf("dark pages: %s %s from the browser", site, map[bool]string{true: "exempt", false: "no longer exempt"}[ask.Exempt])
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"site": site, "exempt": ask.Exempt})
	}
}
