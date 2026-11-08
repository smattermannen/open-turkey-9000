package enforce

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/webext"
)

// RulesAddress is where the extension reads its rules.
const RulesAddress = "127.0.0.1:47613"

// rulesPort is the port in RulesAddress.
const rulesPort = 47613

// BrowserSettings lists the browser options, each stored as "on" or "off".
// Greyscale and image blocking are not here: each block chooses them for its
// own sites (see db.Actions). Neither is the extension: it is always
// installed, and browsers without it are closed while it is needed (see
// ExtensionRequired).
var BrowserSettings = []string{"web-only", "safesearch", "force-search", "force-librewolf"}

// RetiredBrowserSettings were switches before the extension became
// mandatory.
var RetiredBrowserSettings = []string{"extension", "require-extension"}

// extensionGuard closes browsers whose extension doesn't check in. It lives
// in the daemon; the rules server feeds it.
var extensionGuard = blocker.NewExtensionGuard(rulesPort)

// GuardsBlocks reports whether a setting helps enforce blocks: without forcing
// LibreWolf, another browser can be used while no website block is on. It
// also can't be turned off while a block is locked. The others only answer
// to their own locks (see db.SettingLock).
func GuardsBlocks(name string) bool {
	return name == "force-librewolf"
}

// SettingExtensionCheckIn holds when the extension last checked in (unix
// seconds), written at most every checkInRecordEvery.
const SettingExtensionCheckIn = "extension.last_checkin"

const checkInRecordEvery = 30 * time.Second

// ExtensionRequired reports whether browsers must run the extension now:
// while an active block has websites (sites, patterns, or allowlist mode)
// or an option only the extension enforces (web-only, force-search) is on.
// A block of apps alone doesn't need it.
func ExtensionRequired(d *db.DB, active []db.ActiveBlockDetail) bool {
	for _, b := range active {
		if b.Mode == db.ModeAllowlist || len(b.Sites) > 0 || len(b.Patterns) > 0 {
			return true
		}
	}
	return BrowserOn(d, "web-only") || BrowserOn(d, "force-search")
}

// ExtensionRequiredNow is ExtensionRequired for the blocks in force now,
// leaving out blocks on a Pomodoro break.
func ExtensionRequiredNow(d *db.DB) (bool, error) {
	active, err := d.GetActiveBlocks()
	if err != nil {
		return false, err
	}
	trusted, err := d.TrustedNow(time.Now())
	if err != nil {
		return false, err
	}
	return ExtensionRequired(d, withoutBreaks(d, active, trusted)), nil
}

// LastExtensionCheckIn returns when the extension last checked in, if ever.
func LastExtensionCheckIn(d *db.DB) (time.Time, bool) {
	value, err := d.GetSetting(SettingExtensionCheckIn)
	if err != nil || value == "" {
		return time.Time{}, false
	}
	unix, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(unix, 0), true
}

// checkInRecorder passes check-ins to the guard and notes the latest one
// from a real browser in the database, at most every checkInRecordEvery, so
// 'kc9k status' and the interface can show the extension is working.
func checkInRecorder(d *db.DB) func(remoteAddr string) {
	var mu sync.Mutex
	var last time.Time
	return func(remoteAddr string) {
		if !extensionGuard.CheckIn(remoteAddr) {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		now := time.Now()
		if now.Sub(last) < checkInRecordEvery {
			return
		}
		last = now
		if err := d.SetSetting(SettingExtensionCheckIn, strconv.FormatInt(now.Unix(), 10)); err != nil {
			log.Printf("rules: %v", err)
		}
	}
}

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
	mux.HandleFunc("/rules", rulesHandler(d, checkInRecorder(d)))
	mux.HandleFunc("/usage", usageHandler(d))
	return http.Serve(listener, mux)
}

// ExtensionHeader marks a request as coming from the KC9K extension. Only
// requests carrying it count as the extension checking in: a web page can
// make any browser fetch /rules, but it can't add a custom header to a
// request to another address without the browser asking first (a CORS
// preflight), which the daemon never answers. Without this, a page left
// open in Firefox could keep Firefox alive without the extension.
const ExtensionHeader = "X-KC9K-Extension"

// rulesHandler serves the rules, and passes requests from the extension to
// checkIn. Older extensions without the header still get the rules, so they
// keep enforcing until the browser installs the update.
func rulesHandler(d *db.DB, checkIn func(remoteAddr string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Host != RulesAddress || r.Method != http.MethodGet {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.Header.Get(ExtensionHeader) == "1" {
			checkIn(r.RemoteAddr)
		}
		rules, err := BuildRules(d)
		if err != nil {
			log.Printf("rules: %v", err)
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rules)
	}
}

func enforceBrowser(d *db.DB) {
	setup := blocker.BrowserSetup{ForceSearch: BrowserOn(d, "force-search")}
	xpi, err := webext.Pack()
	if err == nil {
		err = blocker.EnsureFile(webext.Path(), xpi)
	}
	if err != nil {
		log.Printf("browser: failed to install the extension file: %v", err)
	} else {
		setup.ExtensionID, setup.ExtensionXPI = webext.ID, webext.Path()
		for _, old := range webext.StaleFiles() {
			if err := os.Remove(old); err != nil && !os.IsNotExist(err) {
				log.Printf("browser: removing old extension file: %v", err)
			}
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

// enforceExtension closes browsers without a working extension while it is
// required (see ExtensionRequired), as long as LibreWolf is there to use
// instead: without it, every browser would be closed.
func enforceExtension(d *db.DB, now time.Time, active []db.ActiveBlockDetail) {
	if !ExtensionRequired(d, active) || blocker.LibreWolfVersion() == "" {
		return
	}
	extensionGuard.Enforce(now)
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
