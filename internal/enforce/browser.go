package enforce

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/webext"
)

// RulesAddress is where the extension reads its rules.
const RulesAddress = "127.0.0.1:47613"

// BrowserSettings lists the browser options, each stored as "on" or "off".
// "extension" counts as on until it is turned off.
// Greyscale and image blocking are not here: each block chooses them for its
// own sites (see db.Actions).
var BrowserSettings = []string{"extension", "web-only", "safesearch", "force-search", "force-librewolf"}

// IsBrowserSetting reports whether name is a browser option.
func IsBrowserSetting(name string) bool {
	for _, s := range BrowserSettings {
		if s == name {
			return true
		}
	}
	return false
}

// BrowserOn reports whether a browser option is on.
func BrowserOn(d *db.DB, name string) bool {
	value, err := d.GetSetting("browser." + name)
	if err != nil {
		log.Printf("browser settings: %v", err)
	}
	if value == "" {
		return name == "extension"
	}
	return value == "on"
}

var otherSearchEngines = []string{
	"bing.com", "duckduckgo.com", "search.yahoo.com", "ecosia.org", "startpage.com", "search.brave.com",
	"yandex.com", "yandex.ru", "baidu.com", "qwant.com", "kagi.com", "you.com", "perplexity.ai",
	"mojeek.com", "metager.org",
}

// Filter is a set of sites a block action applies to: an entry in Match that
// no entry in Except matches.
type Filter struct {
	Match  []string `json:"match"`
	Except []string `json:"except"`
}

// add adds a block's sites to the filter: its sites and patterns, or in
// allowlist mode every site except its listed ones.
func (f *Filter) add(b db.ActiveBlockDetail) {
	f.Match = append(f.Match, b.Patterns...)
	f.Except = append(f.Except, b.Exceptions...)
	if b.Mode == db.ModeAllowlist {
		f.Match = append(f.Match, "*.*")
		f.Except = append(f.Except, b.Sites...)
	} else {
		f.Match = append(f.Match, b.Sites...)
	}
}

// Rules is what the extension enforces. Block and Exceptions come from blocks
// that block their sites; Greyscale and NoImages from blocks with those
// actions. The keys differ from the old global greyscale/images switches, so
// an extension from before per-block actions ignores them instead of
// misreading them.
type Rules struct {
	Block      []string `json:"block"`
	Exceptions []string `json:"exceptions"`
	Greyscale  Filter   `json:"greyscale_sites"`
	NoImages   Filter   `json:"no_image_sites"`
	WebOnly    bool     `json:"web_only"`
}

// BuildRules collects the rules for the extension right now.
func BuildRules(d *db.DB) (Rules, error) {
	rules := Rules{
		Block: []string{}, Exceptions: []string{},
		Greyscale: Filter{Match: []string{}, Except: []string{}},
		NoImages:  Filter{Match: []string{}, Except: []string{}},
		WebOnly:   BrowserOn(d, "web-only"),
	}
	active, err := d.GetActiveBlocks()
	if err != nil {
		return rules, err
	}
	trusted, err := d.TrustedNow(time.Now())
	if err != nil {
		return rules, err
	}
	blocking := Filter{Match: rules.Block, Except: rules.Exceptions}
	for _, b := range withoutBreaks(d, active, trusted) {
		if b.Actions.Block {
			blocking.add(b)
		}
		if b.Actions.Greyscale {
			rules.Greyscale.add(b)
		}
		if b.Actions.NoImages {
			rules.NoImages.add(b)
		}
	}
	rules.Block, rules.Exceptions = blocking.Match, blocking.Except
	if BrowserOn(d, "force-search") {
		rules.Block = append(rules.Block, otherSearchEngines...)
	}
	return rules, nil
}

// ServeRules serves the rules to the extension on the loopback address. The
// Host check stops a web page from reaching it through DNS rebinding.
func ServeRules(d *db.DB) error {
	listener, err := net.Listen("tcp", RulesAddress)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/rules", func(w http.ResponseWriter, r *http.Request) {
		if r.Host != RulesAddress || r.Method != http.MethodGet {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		rules, err := BuildRules(d)
		if err != nil {
			log.Printf("rules: %v", err)
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rules)
	})
	mux.HandleFunc("/usage", usageHandler(d))
	return http.Serve(listener, mux)
}

func enforceBrowser(d *db.DB) {
	setup := blocker.BrowserSetup{ForceSearch: BrowserOn(d, "force-search")}
	if BrowserOn(d, "extension") {
		xpi, err := webext.Pack()
		if err == nil {
			err = blocker.EnsureFile(webext.Path, xpi)
		}
		if err != nil {
			log.Printf("browser: failed to install the extension file: %v", err)
		} else {
			setup.ExtensionID, setup.ExtensionXPI = webext.ID, webext.Path
		}
	}
	if err := blocker.SyncLibreWolfSigning(setup.ExtensionID != ""); err != nil {
		log.Printf("browser: %v", err)
	}
	if err := blocker.EnsureBrowserSetup(setup); err != nil {
		log.Printf("browser: %v", err)
	}
	if err := blocker.SyncSafeSearch(BrowserOn(d, "safesearch")); err != nil {
		log.Printf("browser: safesearch: %v", err)
	}
}

func forcedKills(d *db.DB) []string {
	if !BrowserOn(d, "force-librewolf") {
		return nil
	}
	// Never leave the computer without any browser: if LibreWolf is gone, the
	// others stay open until it is installed again.
	if blocker.LibreWolfVersion() == "" {
		log.Printf("browser: force-librewolf is on but LibreWolf isn't installed; not closing other browsers")
		return nil
	}
	return blocker.ForcedBrowserKillList
}
