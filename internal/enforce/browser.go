package enforce

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
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
var BrowserSettings = []string{"web-only", "safesearch-google", "safesearch-bing", "safesearch-duckduckgo",
	"safesearch-youtube", "force-search", "block-devtools", "no-private"}

// SafeSearchGroup is the name that switches every engine's SafeSearch at
// once (and the one setting there was before 2026-11-18).
const SafeSearchGroup = "safesearch"

// SafeSearchEngines are the engines with their own SafeSearch browser
// setting, "safesearch-<engine>" (§4.10, §6 2026-11-18).
var SafeSearchEngines = []string{"google", "bing", "duckduckgo", "youtube"}

// SafeSearchSetting is the browser setting of one engine's SafeSearch.
func SafeSearchSetting(engine string) string { return SafeSearchGroup + "-" + engine }

// SafeSearchOn returns the engines whose SafeSearch is on.
func SafeSearchOn(d *db.DB) []string {
	out := []string{}
	for _, e := range SafeSearchEngines {
		if BrowserOn(d, SafeSearchSetting(e)) {
			out = append(out, e)
		}
	}
	return out
}

// defaultOn are options that are on until turned off. LibreWolf's developer
// tools can switch the extension off from its own pages or about:debugging,
// so they are blocked unless you turn that off (to develop websites, say).
var defaultOn = map[string]bool{"block-devtools": true}

// RetiredBrowserSettings were switches before the extension became
// mandatory.
var RetiredBrowserSettings = []string{"extension", "require-extension"}

// extensionGuard closes browsers whose extension doesn't check in. It lives
// in the daemon; the rules server feeds it.
var extensionGuard = blocker.NewExtensionGuard(rulesPort)

// GuardsBlocks reports whether a setting helps enforce blocks: without
// blocking unsupported browsers, another browser can be used while no
// website block is on, and without blocking developer tools the extension
// can be switched off. These also can't be turned off while a block is
// locked. The others only answer to their own locks (see db.SettingLock).
func GuardsBlocks(name string) bool {
	return name == db.UnsupportedBrowsers || name == "block-devtools"
}

// UnsupportedBrowsersOn reports whether the tamper protection setting block
// unsupported browsers (§4.10) is on.
func UnsupportedBrowsersOn(d *db.DB) bool {
	v, err := d.GetSetting(db.SettingUnsupportedBrowsers)
	return err == nil && v == "on"
}

// IsLockableSetting reports whether name is an on/off setting with its own
// setting lock: a browser setting, or block unsupported browsers.
func IsLockableSetting(name string) bool {
	return IsBrowserSetting(name) || name == db.UnsupportedBrowsers
}

// SettingKey is where an on/off setting with its own lock is stored.
func SettingKey(name string) string {
	if name == db.UnsupportedBrowsers {
		return db.SettingUnsupportedBrowsers
	}
	return "browser." + name
}

// SettingOn reports whether a lockable setting (see IsLockableSetting) is on.
func SettingOn(d *db.DB, name string) bool {
	if name == db.UnsupportedBrowsers {
		return UnsupportedBrowsersOn(d)
	}
	return BrowserOn(d, name)
}

// SettingExtensionCheckIn holds when the extension last checked in (unix
// seconds), written at most every checkInRecordEvery.
const SettingExtensionCheckIn = "extension.last_checkin"

const checkInRecordEvery = 30 * time.Second

// ExtensionRequired reports whether browsers must run the extension now:
// while an active block, or one in a quota session, has a blacklist that
// does something, or an option
// only the extension enforces (web-only, force-search) is on. A block of
// apps alone doesn't need it.
func ExtensionRequired(d *db.DB, active []db.ActiveBlockDetail) bool {
	required, _ := extensionUsers(d, active)
	return required
}

// needsExtension reports whether a block has something only the extension
// enforces.
func needsExtension(lists db.Lists, social []string, phrases bool) bool {
	return lists.Enforces() || len(social) > 0 || phrases
}

// extensionUsers is ExtensionRequired, and whose browsers need it (§3.2
// affected users): nil for every account, else the user IDs of the accounts
// the blocks needing it name. Options, locks and blocks for everyone need
// it in every account.
func extensionUsers(d *db.DB, active []db.ActiveBlockDetail) (bool, []uint64) {
	// Close browsers if the extension stops (§4.10) off: only a lock
	// requires it.
	if !TamperOn(d, SettingExtensionStop) {
		now := time.Now()
		trusted, err := d.TrustedNow(now)
		if err != nil {
			return false, nil
		}
		locked, err := AnyLock(d, now, trusted)
		return err == nil && locked, nil
	}
	var some []uint64
	partial := false
	add := func(b db.ActiveBlockDetail) bool {
		uids := blockUIDs(b)
		if uids == nil {
			return true
		}
		partial = true
		some = append(some, uids...)
		return false
	}
	for _, b := range active {
		if needsExtension(b.Lists, b.Social, b.HasPhrases) && add(b) {
			return true, nil
		}
	}
	// An open quota session counts the time on its websites, which only the
	// extension sees: without it, another browser would be free.
	for _, b := range openQuotaBlocks(d) {
		if !b.Lists.Enforces() {
			continue
		}
		users, _ := d.BlockUsers(b.Name)
		if add(db.ActiveBlockDetail{BlockName: b.Name, Lists: b.Lists, Users: users}) {
			return true, nil
		}
	}
	if BrowserOn(d, "web-only") || BrowserOn(d, "force-search") {
		return true, nil
	}
	// R-LOCK-9: while anything is locked, the extension is required.
	now := time.Now()
	trusted, err := d.TrustedNow(now)
	if err == nil {
		if locked, err := AnyLock(d, now, trusted); err == nil && locked {
			return true, nil
		}
	}
	if partial {
		return true, dedupUIDs(some)
	}
	return false, nil
}

// dedupUIDs returns uids without repeats, never nil.
func dedupUIDs(uids []uint64) []uint64 {
	out := []uint64{}
	for _, u := range uids {
		if !slices.Contains(out, u) {
			out = append(out, u)
		}
	}
	return out
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
	return ExtensionRequired(d, withMonk(d, enabledBlocks(d, active, trusted), trusted)), nil
}

// LastExtensionCheckIn returns when the extension last checked in, if ever.
func LastExtensionCheckIn(d *db.DB) (time.Time, bool) {
	return lastCheckIn(d, SettingExtensionCheckIn)
}

// LastBrowserCheckIn returns when the extension in a supported browser
// (blocker.SupportedBrowsers ID) last checked in, if ever.
func LastBrowserCheckIn(d *db.DB, id string) (time.Time, bool) {
	return lastCheckIn(d, SettingExtensionCheckIn+"."+id)
}

func lastCheckIn(d *db.DB, key string) (time.Time, bool) {
	value, err := d.GetSetting(key)
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
// from a real browser in the database, overall and per supported browser,
// each at most every checkInRecordEvery, so 'kc9k status' and the interface
// can show the extension is working, and in which browsers.
func checkInRecorder(d *db.DB) func(remoteAddr string) {
	var mu sync.Mutex
	last := map[string]time.Time{}
	record := func(key string, now time.Time) {
		if now.Sub(last[key]) < checkInRecordEvery {
			return
		}
		last[key] = now
		if err := d.SetSetting(key, strconv.FormatInt(now.Unix(), 10)); err != nil {
			log.Printf("rules: %v", err)
		}
	}
	return func(remoteAddr string) {
		found, id := extensionGuard.CheckIn(remoteAddr)
		if !found {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		now := time.Now()
		record(SettingExtensionCheckIn, now)
		if id != "" {
			record(SettingExtensionCheckIn+"."+id, now)
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
		return defaultOn[name]
	}
	return value == "on"
}

var otherSearchEngines = []string{
	"bing.com", "duckduckgo.com", "search.yahoo.com", "ecosia.org", "startpage.com", "search.brave.com",
	"yandex.com", "yandex.ru", "baidu.com", "qwant.com", "kagi.com", "you.com", "perplexity.ai",
	"mojeek.com", "metager.org",
}

// Rules is what the extension enforces. Lists holds every blacklist of the
// active blocks, each with its block's exceptions. Block and Exceptions are
// for extensions from before lists: the addresses blacklists block, so an
// extension that hasn't updated yet keeps blocking them until the browser
// restarts with the new one.
type Rules struct {
	Lists      []ListRule `json:"lists"`
	Quota      []ListRule `json:"quota"` // blocks in a quota session: only to count their time
	Block      []string   `json:"block"`
	Exceptions []string   `json:"exceptions"`
	WebOnly    bool       `json:"web_only"`
	// SafeSearch: the extension keeps Bing, DuckDuckGo, Brave, Yahoo and
	// SearXNG on strict, YouTube on Restricted Mode, and blocks engines it
	// can't hold (§4.10; also by hosts, blocker.SyncSafeSearch). On when
	// any engine's is; SafeSearchEngines says which (an extension without
	// it, or Android's rules, take every engine).
	SafeSearch        bool     `json:"safesearch"`
	SafeSearchEngines []string `json:"safesearch_engines"`
	// StrictUse: every open tab of an allowance session's sites counts, not
	// only the one in front (§3.7).
	StrictUse bool     `json:"strict_use"`
	Titles    []string `json:"titles"`    // always empty: window title entries never apply to browsers (R-TITLE-1)
	Recording bool     `json:"recording"` // report opened addresses to /visit (see recording.go)
	// Words: "record" (a word recording) or "baseline": report each page's
	// rendered words to /words (§3.12, see words.go); "" not.
	Words string `json:"words,omitempty"`
	// StatsSkipPrivate: leave private windows out of statistics (§4.9).
	StatsSkipPrivate bool `json:"stats_skip_private,omitempty"`
	// Details: the block page may say which block and list stopped a page.
	Details bool `json:"details"`
	// Ends: when each active block ends (unix seconds), where that is known.
	Ends map[string]int64 `json:"ends"`
	// Soon: blocks that will take their pages away, with the time left.
	Soon []SoonRule `json:"soon"`
	// Blocks: every block's name, for adding a site from the toolbar. Empty
	// when the Blocks page has an access password.
	Blocks []string `json:"blocks"`
	// Social: the social media settings of the blocks in force (db/social.go).
	Social []string `json:"social"`
	// ExtensionVersion is the extension's current version: Chrome's asks
	// for an update at once when it is older.
	ExtensionVersion string `json:"extension_version"`
	// Dark: the circadian rhythm's dark pages (§4.10), nil when off.
	Dark *DarkRule `json:"dark,omitempty"`
	// InactiveTabsOff: tamper protection "block inactive tabs" is off: a tab
	// in the background is judged when it is switched to (§4.10). Absent
	// (older daemons, Android): on.
	InactiveTabsOff bool `json:"inactive_tabs_off,omitempty"`
	// EmbeddedOff: "block embedded content" is off: frames are left alone.
	EmbeddedOff bool `json:"embedded_off,omitempty"`
}

// ListRule is one blacklist: the entries it looks for in a page's address,
// title and text, its actions, and the exceptions of its block. A match in
// the address or the title takes every action; in the text, an action needs
// Distinct different entries, occurring Hits times in all.
type ListRule struct {
	ID    int      `json:"id"`
	Block string   `json:"block"`
	Name  string   `json:"name"`
	URL   []string `json:"url"`
	Title []string `json:"title"`
	Text  []string `json:"text"`
	// URL holds the list's sites and URL entries; URLTerms its rendered
	// terms, looked for as words in the decoded address, separators as
	// blank space (R-PHRASE-6, §3.2).
	URLTerms []string `json:"url_terms,omitempty"`
	// Ads: ad network domains; a page loading one matches (§3.9 ad trigger).
	Ads     []string     `json:"ads,omitempty"`
	Actions []RuleAction `json:"actions"`
	Except  RuleExcept   `json:"except"`
	// Test: the list is in test mode (§3.2). The extension shows what it
	// found on each page instead of acting on it.
	Test bool `json:"test,omitempty"`
	// Phrases makes the rule a phrase set (its ID is minus the set's): an
	// action's Distinct is how many different phrases must match, in the
	// places PhraseScope names (url, title, text).
	Phrases     []db.Phrase `json:"phrases,omitempty"`
	PhraseScope []string    `json:"phrase_scope,omitempty"`
}

// phraseRules turns a block's phrase sets into rules, with the block's
// exceptions (R-LIST-1).
func phraseRules(d *db.DB, block string, except RuleExcept) []ListRule {
	sets, err := d.PhraseSets(block)
	if err != nil {
		log.Printf("rules: phrase sets of %s: %v", block, err)
		return nil
	}
	var out []ListRule
	for _, s := range sets {
		if len(s.Phrases) == 0 || len(s.Actions) == 0 || !s.Where.Any() {
			continue
		}
		rule := ListRule{ID: -s.ID, Block: block, Name: s.Name, URL: []string{}, Title: []string{}, Text: []string{},
			Actions: []RuleAction{}, Except: except, Test: s.Test, Phrases: s.Phrases, PhraseScope: s.Where.List()}
		for _, a := range s.Actions {
			rule.Actions = append(rule.Actions, RuleAction{Name: a.Name, Distinct: a.Distinct, Hits: 1, Param: a.Param})
		}
		out = append(out, rule)
	}
	return out
}

// RuleAction is one action of a ListRule. Param is the address for
// redirect and the image's name for image.
type RuleAction struct {
	Name     string `json:"action"`
	Distinct int    `json:"distinct"`
	Hits     int    `json:"hits"`
	Rate     int    `json:"rate,omitempty"` // matches per 1,000 words; 0: none
	Param    string `json:"param,omitempty"`
}

// RuleExcept holds a block's exceptions by place: a page any of them
// matches is spared from the block's blacklists.
type RuleExcept struct {
	URL      []string `json:"url"`
	Title    []string `json:"title"`
	Text     []string `json:"text"`
	URLTerms []string `json:"url_terms,omitempty"`
}

// blockExcept collects a block's exceptions for its rules (R-LIST-1).
func blockExcept(lists db.Lists) RuleExcept {
	return RuleExcept{URL: urlEntries(lists.ExceptionAddresses()),
		Title:    append([]string{}, lists.ExceptionTerms(db.PlaceTitle)...),
		Text:     append([]string{}, lists.ExceptionTerms(db.PlaceText)...),
		URLTerms: lists.ExceptionTerms(db.PlaceURL)}
}

// urlEntries keeps the entries that can match addresses.
func urlEntries(entries []string) []string {
	out := []string{}
	for _, e := range entries {
		// file://* (R-LIST-3) is for local files, which only the browser
		// itself opens: it never spares or blocks a web address.
		if blocker.IsURLEntry(e) && blocker.WebPattern(e) {
			out = append(out, e)
		}
	}
	return out
}

// listRules turns a block's blacklists into rules.
func listRules(b db.ActiveBlockDetail) []ListRule {
	except := blockExcept(b.Lists)
	var out []ListRule
	for _, l := range b.Lists.Blacklists() {
		if !l.Enforces() {
			continue
		}
		rule := ListRule{ID: l.ID, Block: b.BlockName, Name: l.Name, URL: []string{}, Title: []string{}, Text: []string{},
			Actions: []RuleAction{}, Except: except}
		// §3.2: with the url, sites and URL entries match the address and
		// rendered terms words in it; title and text read every entry as a
		// rendered term.
		rule.URL = urlEntries(l.Addresses())
		if terms := l.URLTerms(); len(terms) > 0 {
			rule.URLTerms = terms
		}
		if l.Where.Title {
			rule.Title = append(rule.Title, l.Entries...)
		}
		if l.Where.Text {
			rule.Text = append(rule.Text, l.Entries...)
		}
		rule.Ads = l.AdNetworks()
		for _, a := range l.Actions {
			rule.Actions = append(rule.Actions, RuleAction{Name: a.Name, Distinct: a.Distinct, Hits: a.Hits, Rate: a.Rate, Param: a.Param})
		}
		out = append(out, rule)
	}
	return out
}

// BuildRules collects the rules for the extension right now.
func BuildRules(d *db.DB) (Rules, error) { return BuildRulesFor(d, "") }

// BuildRulesFor is BuildRules for the extension of one user (§3.2): blocks
// for other users only are left out. "" (unknown): every block.
func BuildRulesFor(d *db.DB, user string) (Rules, error) {
	rules := Rules{
		Lists: []ListRule{}, Quota: []ListRule{}, Block: []string{}, Exceptions: []string{},
		WebOnly:           BrowserOn(d, "web-only"),
		SafeSearchEngines: SafeSearchOn(d),
		StrictUse:         StrictUseOn(d),
		Titles:            []string{},
		InactiveTabsOff:   !TamperOn(d, SettingInactiveTabs),
		EmbeddedOff:       !TamperOn(d, SettingEmbedded),

		ExtensionVersion: webext.Version(),
	}
	rules.SafeSearch = len(rules.SafeSearchEngines) > 0
	if recording, err := d.GetRecording(); err != nil {
		return rules, err
	} else if recording != nil && recording.Words {
		rules.Words = "record"
	} else if recording != nil {
		rules.Recording = true
	}
	if v, _ := d.GetSetting(SettingStatsPrivate); v == "skip" {
		rules.StatsSkipPrivate = true
	}
	if rules.Words == "" && d.WordBaselineOn() {
		rules.Words = "baseline"
	}
	active, err := d.GetActiveBlocks()
	if err != nil {
		return rules, err
	}
	trusted, err := d.TrustedNow(time.Now())
	if err != nil {
		return rules, err
	}
	now := time.Now()
	rules.Details = OptionOn(d, OptionPageDetails)
	rules.Dark = CircadianDarkRule(d, now)
	rules.Ends = blockEnds(d, active, now, trusted)
	rules.Soon = soonRules(d, active, now, trusted)
	if until := MonkEnd(d, trusted); until > 0 {
		rules.Ends[MonkName] = now.Unix() + until - trusted
	}
	rules.Blocks = addableBlocks(d)
	testing := d.TestLists()
	rules.Social = []string{}
	for _, b := range withMonk(d, enabledBlocks(d, active, trusted), trusted) {
		if !b.AppliesTo(user) {
			continue
		}
		for _, preset := range b.Social {
			if !slices.Contains(rules.Social, preset) {
				rules.Social = append(rules.Social, preset)
			}
		}
		except := blockExcept(b.Lists)
		blockRules := listRules(b)
		if len(b.Lists) == 0 || (b.Lists[0].ID != MonkListID && b.Lists[0].ID != ResetGuardListID) {
			blockRules = append(blockRules, phraseRules(d, b.BlockName, except)...)
		}
		for _, rule := range blockRules {
			// Test mode is for tuning what a list finds in pages: lists that
			// read the title or text are shown, not run.
			if len(rule.Phrases) == 0 {
				rule.Test = slices.Contains(testing, rule.ID) && len(rule.Title)+len(rule.Text)+len(rule.URLTerms) > 0
			}
			rules.Lists = append(rules.Lists, rule)
			if rule.Test {
				continue
			}
			if len(rule.URL) > 0 && len(rule.Except.Title)+len(rule.Except.Text)+len(rule.Except.URLTerms) == 0 {
				for _, a := range rule.Actions {
					if a.Name == db.ActionBlock {
						rules.Block = append(rules.Block, rule.URL...)
						rules.Exceptions = append(rules.Exceptions, rule.Except.URL...)
					}
				}
			}
		}
	}
	for _, b := range openQuotaBlocks(d) {
		rules.Quota = append(rules.Quota, listRules(db.ActiveBlockDetail{BlockName: b.Name, Lists: b.Lists})...)
	}
	if BrowserOn(d, "force-search") {
		rules.Lists = append(rules.Lists, ListRule{Block: "force-search", Name: "Other search engines",
			URL: otherSearchEngines, Title: []string{}, Text: []string{},
			Actions: []RuleAction{{Name: db.ActionBlock, Distinct: 1, Hits: 1}},
			Except:  RuleExcept{URL: []string{}, Title: []string{}, Text: []string{}}})
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
	mux.HandleFunc("/hit", hitHandler(d))
	mux.HandleFunc("/visit", visitHandler(d))
	mux.HandleFunc("/words", wordsHandler(d))
	mux.HandleFunc("/add", addHandler(d))
	mux.HandleFunc("/dark-exempt", darkExemptHandler(d))
	mux.HandleFunc("/image/", imageHandler)
	mux.HandleFunc(chromeUpdatePath, chromeUpdateHandler)
	mux.HandleFunc("/chrome/", chromeExtensionHandler)
	return http.Serve(listener, mux)
}

// Chrome installs the extension from the daemon: it reads chromeUpdatePath
// for the current version and where to download it (see webext.PackChrome).
const chromeUpdatePath = "/chrome/update.xml"

// ChromeUpdateURL is the update URL in Chrome's policy.
const ChromeUpdateURL = "http://" + RulesAddress + chromeUpdatePath

// chromeCRXPath is where this version's .crx is served. Its name carries
// the version, like the .xpi's.
func chromeCRXPath() string {
	return "/chrome/kc9k-" + webext.Version() + ".crx"
}

// chromeExt is the packed Chrome extension, made once per daemon run (the
// files are built in, so they can't change while it runs).
var chromeExt struct {
	sync.Mutex
	id  string
	crx []byte
}

// chromeExtension returns the Chrome extension's ID and .crx, signed with
// this computer's key (made the first time).
func chromeExtension() (string, []byte, error) {
	chromeExt.Lock()
	defer chromeExt.Unlock()
	if chromeExt.crx != nil {
		return chromeExt.id, chromeExt.crx, nil
	}
	key, err := webext.ChromeKey(webext.ChromeKeyPath)
	if err != nil {
		return "", nil, err
	}
	id, err := webext.ChromeID(&key.PublicKey)
	if err != nil {
		return "", nil, err
	}
	crx, err := webext.PackChrome(key)
	if err != nil {
		return "", nil, err
	}
	chromeExt.id, chromeExt.crx = id, crx
	return id, crx, nil
}

// chromeUpdateHandler answers Chrome's update check. Like /rules, only for
// requests addressed to the daemon's own address (no DNS rebinding).
func chromeUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Host != RulesAddress || r.Method != http.MethodGet {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	id, _, err := chromeExtension()
	if err != nil {
		log.Printf("chrome: %v", err)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write(webext.ChromeUpdateXML(id, "http://"+RulesAddress+chromeCRXPath()))
}

// chromeExtensionHandler serves the .crx.
func chromeExtensionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Host != RulesAddress || r.Method != http.MethodGet || r.URL.Path != chromeCRXPath() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	_, crx, err := chromeExtension()
	if err != nil {
		log.Printf("chrome: %v", err)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/x-chrome-extension")
	_, _ = w.Write(crx)
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
		rules, err := BuildRulesFor(d, clientUser(r.RemoteAddr))
		if err != nil {
			log.Printf("rules: %v", err)
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rules)
	}
}

// Settings that remember when KC9K last changed something in the browsers
// that protects the extension (see noteSettingsChange).
const (
	settingProtectionFingerprint = "browser.protection_fingerprint"
	settingProtectionChanged     = "browser.protection_changed"
)

// noteSettingsChange records when the browser settings that protect the
// extension last changed: its version, blocked developer tools, or KC9K's
// locks in LibreWolf. Browsers only read their settings when they start, so
// the extension guard closes browsers older than that (see Enforce).
func noteSettingsChange(d *db.DB, setup blocker.BrowserSetup) {
	fingerprint := strings.Join([]string{setup.ExtensionXPI,
		"devtools-blocked=" + strconv.FormatBool(setup.DisableDevTools),
		"locks=" + blocker.LibreWolfLocksVersion()}, "|")
	if setup.DisablePrivate {
		// Open private windows only close when the browser restarts.
		fingerprint += "|no-private"
	}
	old, err := d.GetSetting(settingProtectionFingerprint)
	if err != nil || old == fingerprint {
		return
	}
	if err := d.SetSetting(settingProtectionFingerprint, fingerprint); err != nil {
		log.Printf("browser: %v", err)
		return
	}
	if err := d.SetSetting(settingProtectionChanged, strconv.FormatInt(time.Now().Unix(), 10)); err != nil {
		log.Printf("browser: %v", err)
	}
}

// protectionChanged returns when noteSettingsChange last saw a change.
func protectionChanged(d *db.DB) time.Time {
	value, err := d.GetSetting(settingProtectionChanged)
	if err != nil || value == "" {
		return time.Time{}
	}
	unix, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(unix, 0)
}

// PrivateTamperOn reports tamper protection "close private windows and
// Tor" (§4.10).
func PrivateTamperOn(d *db.DB) bool {
	v, err := d.GetSetting(SettingTamperPrivate)
	return err == nil && v == "on"
}

func enforceBrowser(d *db.DB) {
	setup := blocker.BrowserSetup{ForceSearch: BrowserOn(d, "force-search"),
		DisableDevTools: BrowserOn(d, "block-devtools") && !blocker.RemoteControlAllowed(),
		DisablePrivate:  BrowserOn(d, "no-private") || PrivateTamperOn(d)}
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
	if err := blocker.SyncLibreWolfLocks(true); err != nil {
		log.Printf("browser: locking LibreWolf settings: %v", err)
	}
	noteSettingsChange(d, setup)
	if err := blocker.EnsureBrowserSetup(setup); err != nil {
		log.Printf("browser: %v", err)
	}
	chrome := blocker.ChromeSetup{DisableDevTools: setup.DisableDevTools, ForceSearch: setup.ForceSearch}
	if id, _, err := chromeExtension(); err != nil {
		log.Printf("browser: preparing the Chrome extension: %v", err)
	} else {
		chrome.ExtensionID, chrome.UpdateURL = id, ChromeUpdateURL
	}
	if err := blocker.EnsureChromeSetup(chrome); err != nil {
		log.Printf("browser: %v", err)
	}
	if err := blocker.SyncSafeSearch(SafeSearchOn(d)); err != nil {
		log.Printf("browser: safesearch: %v", err)
	}
	if err := blocker.SyncGuestOff(TamperOn(d, SettingTamperGuest)); err != nil {
		log.Printf("browser: guest: %v", err)
	}
}

// enforceExtension closes browsers without a working extension while it is
// required (see ExtensionRequired), and with block unsupported browsers on
// also every browser that isn't a supported browser running it, at any time.
// Only as long as a supported browser (LibreWolf, Google Chrome or Chromium)
// is there to use instead: without one, every browser would be closed.
func enforceExtension(d *db.DB, now time.Time, active []db.ActiveBlockDetail) {
	unsupported := UnsupportedBrowsersOn(d)
	required, uids := extensionUsers(d, active)
	if unsupported {
		uids = nil // every account: no other browser may run
	} else if !required {
		return
	}
	if !blocker.ExtensionBrowserInstalled() {
		return // 'kc9k status' says so
	}
	if extensionGuard.EnforceFor(now, protectionChanged(d), unsupported, uids) > 0 {
		logEventFor(d, db.EventBrowser, websiteBlocks(active), "")
	}
}
