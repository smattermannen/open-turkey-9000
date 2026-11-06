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
var BrowserSettings = []string{"extension", "images", "greyscale", "web-only", "safesearch", "force-search", "force-librewolf"}

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

// Rules is what the extension enforces.
type Rules struct {
	Block      []string `json:"block"`
	Exceptions []string `json:"exceptions"`
	Images     bool     `json:"images"`
	Greyscale  bool     `json:"greyscale"`
	WebOnly    bool     `json:"web_only"`
}

// BuildRules collects the rules for the extension right now.
func BuildRules(d *db.DB) (Rules, error) {
	rules := Rules{
		Block: []string{}, Exceptions: []string{},
		Images: BrowserOn(d, "images"), Greyscale: BrowserOn(d, "greyscale"), WebOnly: BrowserOn(d, "web-only"),
	}
	active, err := d.GetActiveBlocks()
	if err != nil {
		return rules, err
	}
	trusted, err := d.TrustedNow(time.Now())
	if err != nil {
		return rules, err
	}
	for _, b := range withoutBreaks(d, active, trusted) {
		rules.Block = append(rules.Block, b.Patterns...)
		rules.Exceptions = append(rules.Exceptions, b.Exceptions...)
		if b.Mode == db.ModeAllowlist {
			rules.Block = append(rules.Block, "*.*")
			rules.Exceptions = append(rules.Exceptions, b.Sites...)
		} else {
			rules.Block = append(rules.Block, b.Sites...)
		}
	}
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
	if BrowserOn(d, "force-librewolf") {
		return blocker.ForcedBrowserKillList
	}
	return nil
}
