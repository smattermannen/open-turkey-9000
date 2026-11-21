// Package enforce turns the set of active blocks into concrete layer actions.
//
// It is the single place that knows how to combine several active blocks (with
// different modes, patterns and schedules) into the inputs each blocking layer
// needs, so the CLI (on state changes) and the daemon (on every tick) behave
// identically.
//
//   - Apply forces every layer to match the active blocks right now. The CLI
//     uses it after start/stop/edit, when the desired set has just changed.
//   - Tick is the daemon's per-cycle routine: it applies queued edits whose
//     delay has passed, reconciles schedules (auto-activating/deactivating
//     scheduled blocks), then re-applies only the layers that have drifted, so
//     a steady state does no work.
package enforce

import (
	"fmt"
	"log"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// Inputs derives the layer inputs from the active blocks:
//   - hostsDomains: the sites of blacklists that look at the url and block,
//     for /etc/hosts and the firewall, minus those with an address
//     exception under them (those layers block whole hosts). A list's
//     rendered terms never reach these layers.
//   - apps: the union of process names to kill.
//   - bp: the browser policy: those blacklists' domains and patterns
//     (patterns only with block embedded content on), with the block's
//     address exceptions.
//
// Everything is unioned across active blocks. A block with exceptions by
// title or text leaves its websites to the extension alone: only it can
// read a page before deciding, so those layers would block pages the
// exceptions spare. Greyscale and no-images only go to the extension (see
// BuildRules). Apps are blocked whatever the lists do.
//
// Results are kept for the same blocks (inputsKey): the daemon asks every
// cycle, for all blocks and for each alone, and working it out from 100,000
// sites took 150 ms each time (principle 9; 2026-11-21, desktop).
func Inputs(active []db.ActiveBlockDetail) (hostsDomains, apps []string, bp blocker.BrowserPolicy) {
	key := inputsKey(active)
	keptInputs.Lock()
	defer keptInputs.Unlock()
	r, ok := keptInputs.byKey[key]
	if !ok {
		r.hosts, r.apps, r.bp = inputs(active)
		if keptInputs.byKey == nil || len(keptInputs.byKey) >= keptInputsMax {
			keptInputs.byKey = map[uint64]inputsResult{}
		}
		keptInputs.byKey[key] = r
	}
	bp = r.bp
	bp.BlockDomains, bp.BlockPatterns = slices.Clone(bp.BlockDomains), slices.Clone(bp.BlockPatterns)
	bp.AllowDomains, bp.Exceptions = slices.Clone(bp.AllowDomains), slices.Clone(bp.Exceptions)
	bp.BlockFiles = slices.Clone(bp.BlockFiles)
	return slices.Clone(r.hosts), slices.Clone(r.apps), bp
}

// inputsResult is one result of inputs.
type inputsResult struct {
	hosts, apps []string
	bp          blocker.BrowserPolicy
}

// keptInputs holds Inputs' results by inputsKey, emptied when it reaches
// keptInputsMax (a cycle asks for one set per active block and one for
// them all).
var keptInputs struct {
	sync.Mutex
	byKey map[uint64]inputsResult
}

const keptInputsMax = 32

// inputsKey sums up everything inputs reads: the file URL and embedded
// content settings, and per block whether it is for everyone, its apps and
// its lists (ID, kind, places, actions, entries).
func inputsKey(active []db.ActiveBlockDetail) uint64 {
	lists := [][]string{{"embedded-off", strconv.FormatBool(embeddedOff.Load())}}
	for _, b := range active {
		lists = append(lists, []string{strconv.FormatBool(forEveryone(b))}, b.Apps)
		for _, l := range b.Lists {
			lists = append(lists, []string{fmt.Sprintf("%d %s %+v %+v", l.ID, l.Kind, l.Where, l.Actions)}, l.Entries)
		}
	}
	return blocker.ListsKey(fileURLsOn.Load(), lists...)
}

// inputs is Inputs, worked out.
func inputs(active []db.ActiveBlockDetail) (hostsDomains, apps []string, bp blocker.BrowserPolicy) {
	var blockDomains, hostDomains, patterns, exceptions, appList, files []string

	for _, b := range active {
		if !forEveryone(b) {
			// §3.2: a block for some users only stays out of the layers
			// every account shares; its sites go to those users'
			// extensions and its apps are closed per user (killApps).
			continue
		}
		appList = append(appList, b.Apps...)
		if pageExceptions(b.Lists) {
			continue
		}
		except := urlEntries(b.Lists.ExceptionAddresses())
		used := false
		for _, l := range b.Lists.Blacklists() {
			// Only a list's sites and URL entries feed the layers; its
			// rendered terms (in the address, title or text) are the
			// extension's (§3.2).
			addresses := l.Addresses()
			if !l.Enforces() || len(addresses) == 0 || !l.HasAction(db.ActionBlock) {
				continue
			}
			used = true
			if fileURLsOn.Load() {
				// Local files (§3.1): only those the block's own file
				// exceptions (file://* by default, R-LIST-3) don't spare.
				for _, e := range addresses {
					if !blocker.WebPattern(e) && !blocker.FileCovered(e, b.Lists.ExceptionAddresses()) {
						files = append(files, e)
					}
				}
			}
			domains, pats := blocker.SplitURLEntries(addresses)
			if embeddedOff.Load() && l.ID < MonkListID {
				// Block embedded content off (§4.10): a browser policy
				// blocks frames too (Chrome's URLBlocklist, Firefox's
				// WebsiteFilter), so only whole sites go there, which
				// hosts and the firewall block in frames anyway; the
				// extension blocks paths and patterns in pages, not in
				// frames. KC9K's own lists (monk mode, factory reset
				// protection) keep theirs.
				pats = nil
			}
			blockDomains = append(blockDomains, domains...)
			patterns = append(patterns, pats...)
			for _, site := range domains {
				if !blocker.HasExceptionUnder(site, except) {
					hostDomains = append(hostDomains, site)
				}
			}
		}
		if used {
			exceptions = append(exceptions, except...)
		}
	}

	bp = blocker.BrowserPolicy{
		BlockDomains:  blockDomains,
		BlockPatterns: patterns,
		Exceptions:    exceptions,
		BlockFiles:    files,
	}
	return blocker.ValidDomains(hostDomains), dedup(appList), bp
}

// regularUsers lists the computer's people accounts; replaceable in tests.
var regularUsers = blocker.RegularUsers

// forEveryone reports whether a block applies to every account (§3.2): no
// users chosen, or all of them.
func forEveryone(b db.ActiveBlockDetail) bool {
	if len(b.Users) == 0 {
		return true
	}
	for _, u := range regularUsers() {
		if !slices.Contains(b.Users, u) {
			return false
		}
	}
	return true
}

// blockUIDs is the users whose processes a block's apps are closed for:
// nil for everyone.
func blockUIDs(b db.ActiveBlockDetail) []uint64 {
	if forEveryone(b) {
		return nil
	}
	uids := blocker.UserIDs(b.Users)
	if uids == nil {
		uids = []uint64{}
	}
	return uids
}

// blockDomains are the sites a block's lists block by address, whoever it
// applies to (Inputs leaves a block for some users out).
func blockDomains(b db.ActiveBlockDetail) []string {
	b.Users = nil
	_, _, policy := Inputs([]db.ActiveBlockDetail{b})
	return policy.BlockDomains
}

// pageExceptions reports whether a block has exceptions by title or text.
func pageExceptions(lists db.Lists) bool {
	return len(lists.ExceptionTerms(db.PlaceURL))+len(lists.ExceptionTerms(db.PlaceTitle))+len(lists.ExceptionTerms(db.PlaceText)) > 0
}

// titleGuard closes apps whose window titles contain blocked keywords.
var titleGuard = blocker.NewTitleGuard()

// titleGuardBusy is set while the title guard runs. It talks to X servers,
// and anyone can start one, so it runs beside the daemon's cycle, once at a
// time, rather than in it: a slow server delays only the next title check.
var titleGuardBusy atomic.Bool

// enforceTitles starts a title guard cycle unless one is still running.
// Each window asked to close is logged as an event of every active block
// listing its keyword for the user whose window it was.
func enforceTitles(d *db.DB, active []db.ActiveBlockDetail, now time.Time) {
	if !titleGuardBusy.CompareAndSwap(false, true) {
		return
	}
	targets, titleActions := titleTargets(d, active)
	go func() {
		defer titleGuardBusy.Store(false)
		_, hits := titleGuard.EnforceUsers(targets, now)
		for _, h := range hits {
			for _, block := range titleHitBlocks(active, h) {
				logEvent(d, db.EventTitle, block, h.Keyword)
				if actions, ok := titleActions[block]; ok {
					fireTrigger(d, block, db.TriggerTitles, actions,
						"A window of block '"+block+"' was open ("+h.Keyword+").", now)
				}
			}
		}
	}()
}

// titleTargets is what the title guard looks for, per user (§3.2 affected
// users): each block's window titles and folder entries go to the accounts
// it names, or to everyone. It also returns each block's title actions.
func titleTargets(d *db.DB, active []db.ActiveBlockDetail) (blocker.TitlesByUser, map[string][]db.TriggerAction) {
	targets := blocker.TitlesByUser{Users: map[uint64]blocker.TitleTargets{}}
	titleActions := map[string][]db.TriggerAction{}
	for _, b := range active {
		var own blocker.TitleTargets
		// Owner, 2026-11-16: a block's window titles close their windows
		// unless its title actions leave them open; then they are only
		// watched, for the other actions.
		if len(b.Titles) > 0 {
			actions := triggerActions(d, b.BlockName, db.TriggerTitles)
			titleActions[b.BlockName] = actions
			if hasTriggerAction(actions, db.TriggerClose) {
				own.Close = b.Titles
			} else if len(triggerConsequences(actions)) > 0 {
				own.Watch = b.Titles
			}
		}
		own.Folders = folderEntries(b.Apps)
		if own.Empty() {
			continue
		}
		uids := blockUIDs(b)
		if uids == nil {
			targets.All = joinTargets(targets.All, own)
			continue
		}
		for _, uid := range uids {
			targets.Users[uid] = joinTargets(targets.Users[uid], own)
		}
	}
	return targets, titleActions
}

// folderEntries are the folder entries among a block's apps (§3.1).
func folderEntries(apps []string) []string {
	var out []string
	for _, a := range apps {
		if strings.HasSuffix(a, "/") && strings.HasPrefix(a, "/") {
			out = append(out, a)
		}
	}
	return out
}

func joinTargets(a, b blocker.TitleTargets) blocker.TitleTargets {
	return blocker.TitleTargets{
		Close:   append(append([]string{}, a.Close...), b.Close...),
		Watch:   append(append([]string{}, a.Watch...), b.Watch...),
		Folders: append(append([]string{}, a.Folders...), b.Folders...),
	}
}

// titleHitBlocks names the active blocks a title guard hit is to blame on:
// those listing its keyword (a window title or folder entry) that apply to
// the user whose window it was.
func titleHitBlocks(active []db.ActiveBlockDetail, h blocker.TitleHit) []string {
	var out []string
	for _, b := range active {
		if !slices.Contains(b.Titles, h.Keyword) && !slices.Contains(folderEntries(b.Apps), h.Keyword) {
			continue
		}
		if uids := blockUIDs(b); uids != nil && !slices.Contains(uids, h.UID) {
			continue
		}
		out = append(out, b.BlockName)
	}
	return out
}

// logEvent records an event, logging a failure.
func logEvent(d *db.DB, kind, block, detail string) {
	if err := d.RecordEvent(kind, block, detail); err != nil {
		log.Printf("daemon: %v", err)
	}
}

// logEventFor records one event that several blocks are to blame for.
func logEventFor(d *db.DB, kind string, blocks []string, detail string) {
	if err := d.RecordEventFor(kind, blocks, detail); err != nil {
		log.Printf("daemon: %v", err)
	}
}

// websiteBlocks names the active blocks whose websites need the extension:
// the ones a browser without it, or another browser, gets around.
func websiteBlocks(active []db.ActiveBlockDetail) []string {
	var names []string
	for _, b := range active {
		if b.Lists.Enforces() {
			names = append(names, b.BlockName)
		}
	}
	return names
}

// activeNames names the active blocks: the ones tampering gets around.
func activeNames(d *db.DB) []string {
	active, err := d.GetActiveBlocks()
	if err != nil {
		log.Printf("daemon: %v", err)
		return nil
	}
	var names []string
	for _, b := range active {
		names = append(names, b.BlockName)
	}
	return names
}

// deferKills makes Apply leave blocked apps to the daemon (see ApplyFromDB).
// Only set while the apply lock is held.
var deferKills bool

// deferFirewall makes Apply leave filling the firewall to the daemon (see
// ApplyFromDB): it looks up every blocked site's addresses, which with a
// huge list takes long, and the daemon keeps the answers (principle 9).
// Hosts and browser policy, which decide, apply at once; removing the
// firewall needs no lookups and is done at once too.
var deferFirewall bool

// Apply forces every layer to match the active blocks. Used by the CLI right
// after the desired state changes (start, stop, unlock, edit), where the domain
// set may have changed and we want the effect immediately.
func Apply(active []db.ActiveBlockDetail) error { return apply(active, false) }

// apply is Apply; with cut (monk mode) the firewall keeps dropping
// everything except loopback.
func apply(active []db.ActiveBlockDetail, cut bool) error {
	// A database used through KC9K_DB is a test's: it must never drive the
	// real hosts file, firewall or browser policies.
	if os.Getenv("KC9K_DB") != "" {
		return nil
	}
	hosts, apps, bp := Inputs(active)

	var firstErr error
	note := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	note(blocker.ApplyHosts(hosts)) // empty list => removes our section
	if !deferFirewall || cut || len(hosts) == 0 {
		note(blocker.ApplyFirewallCut(hosts, cut)) // empty list and no cut => removes the chain
	}
	note(blocker.ApplyBrowserPolicies(bp)) // empty policy => removes files
	if len(apps) > 0 && !deferKills {
		blocker.KillBlocked(apps)
	}
	return firstErr
}

// SettingDNS is which DNS server the firewall layer asks for blocked sites'
// real addresses (§4.10): blocker.DNSNetwork (default), blocker.DNSQuad9, or
// the user's own addresses.
const SettingDNS = "dns"

// SettingFileURLs is tamper protection "allow file:// blocking" (§4.10):
// "on" lets blacklists' file:// entries block local files in browsers.
const SettingFileURLs = "tamper.file-urls"

// fileURLsOn is SettingFileURLs, loaded with the DNS choice before each
// apply (Inputs takes no database).
var fileURLsOn atomic.Bool

// embeddedOff is tamper protection "block embedded content" off, loaded
// with fileURLsOn.
var embeddedOff atomic.Bool

// FileURLsOn reports whether file:// blocking is allowed.
func FileURLsOn(d *db.DB) bool {
	v, err := d.GetSetting(SettingFileURLs)
	return err == nil && v == "on"
}

// loadDNSChoice hands the DNS setting to the blocker, and loads the file://
// blocking and block embedded content settings.
func loadDNSChoice(d *db.DB) {
	fileURLsOn.Store(FileURLsOn(d))
	embeddedOff.Store(!TamperOn(d, SettingEmbedded))
	blocker.SetExtensionGrace(time.Duration(ExtensionGraceSeconds(d)) * time.Second)
	choice, err := d.GetSetting(SettingDNS)
	if err != nil {
		log.Printf("daemon: dns setting: %v", err)
		return
	}
	blocker.SetDNSChoice(choice)
}

// ApplyFromDB reads the active blocks and applies them. Convenience for the CLI.
func ApplyFromDB(d *db.DB) error {
	defer applyLock()()
	loadDNSChoice(d)
	active, err := d.GetActiveBlocks()
	if err != nil {
		return err
	}
	trusted, err := d.TrustedNow(time.Now())
	if err == nil {
		active = enabledBlocks(d, active, trusted)
	} else {
		trusted = 0
	}
	// With polite closing on, the daemon's next cycle asks the apps to
	// close instead of this killing them outright.
	deferKills = OptionOn(d, OptionPoliteClose)
	deferFirewall = true // the daemon's next cycle fills it, within seconds
	defer func() { deferKills, deferFirewall = false, false }()
	return apply(withMonk(d, active, trusted), MonkOn(d, trusted))
}

// Tick runs one daemon cycle: apply due queued edits, reconcile schedules,
// then re-apply drifted layers.
func Tick(d *db.DB, now time.Time) error {
	defer applyLock()()
	if err := reconcileFreezeSchedules(d, now); err != nil {
		log.Printf("daemon: freeze schedule error: %v", err)
	}
	enforceFreeze(d, now)
	loadDNSChoice(d)
	enforceBrowser(d)
	watchClock(d, now)

	trusted, err := d.TrustedNow(now)
	if err != nil {
		// Zero means no time lock counts as expired during this cycle.
		log.Printf("daemon: trusted clock error: %v", err)
		trusted = 0
	}
	if err == nil {
		enforceLockdown(d, now, trusted)
		finishDelayedStops(d, trusted)
	}
	if err := applyDueEditsAt(d, now, trusted); err != nil {
		log.Printf("daemon: pending edits error: %v", err)
	}
	runAutostarts(d, now)             // §3.6: before the layers, so a started block applies this cycle
	restartPomodoros(d, now, trusted) // R-SCHED-3
	if err := reconcileSchedulesAt(d, now, trusted); err != nil {
		log.Printf("daemon: schedule reconciliation error: %v", err)
	}
	if err := reconcileQuotas(d, now, trusted); err != nil {
		log.Printf("daemon: allowance error: %v", err)
	}
	sendNotices(d, now)

	active, err := d.GetActiveBlocks()
	if err != nil {
		return err
	}
	active = enabledBlocks(d, active, trusted)
	if trusted != 0 {
		earnBreaks(d, active, trusted)
	}
	// Monk mode: its cuts are held or undone here, and what it blocks is
	// enforced beside the blocks. It is no block: apps, events and bypass
	// detection below go by the blocks alone.
	endModeRuns(d, trusted) // R-MODE-4: a Mode action's run whose time is over
	enabledNames := map[string]bool{}
	for _, b := range active {
		enabledNames[b.BlockName] = true
	}
	runModeCustoms(d, enabledNames, trusted) // §3.9: what a mode opens when it becomes enabled
	monk := enforceMonk(d, trusted)          // R-MONK-3: a run for a set time, or switched on as a mode
	enforced := active
	if monk {
		enforced = append(append([]db.ActiveBlockDetail{}, active...), monkBlock())
	}
	guard := resetGuardActive(d, trusted)
	if guard {
		enforced = append(append([]db.ActiveBlockDetail{}, enforced...), resetGuardBlock())
	}
	hosts, _, bp := Inputs(enforced)
	enforceExtension(d, now, enforced)
	enforceTitles(d, active, now)

	// Site layers, re-applied only on drift (so an idle system does no writes).
	if len(hosts) > 0 {
		if !blocker.IsHostsApplied(hosts) {
			log.Println("daemon: /etc/hosts drifted, reapplying")
			if err := blocker.ApplyHosts(hosts); err != nil {
				log.Printf("daemon: error applying /etc/hosts: %v", err)
			}
		}
		// The firewall holds resolved IPs, so also reapply when the domain set
		// changed (otherwise a stopped block's rules would linger).
	} else if blocker.HostsSectionPresent() {
		if err := blocker.RemoveHosts(); err != nil {
			log.Printf("daemon: error removing /etc/hosts section: %v", err)
		}
	}
	switch {
	case monk:
		// enforceMonk holds the firewall, which drops everything. Whatever
		// comes after starts afresh: the addresses in it weren't looked up.
		firewallDomains, firewallLookups = firewallCutKey, false
	case len(hosts) > 0:
		// The firewall holds resolved IPs, so also reapply when the domain set
		// changed (otherwise a stopped block's rules would linger).
		// Lookups run in the background (principle 9): applied with the
		// addresses known so far, and again once they end.
		key := strings.Join(hosts, ",")
		refresh := !firewallRefresh.IsZero() && now.After(firewallRefresh)
		looked := firewallLookups && !blocker.LookupsRunning()
		if !blocker.IsFirewallApplied() || key != firewallDomains || refresh || looked {
			if !looked {
				log.Println("daemon: firewall drifted or domains changed, reapplying")
			}
			if refresh {
				blocker.ForgetFailedLookups()
			}
			complete, err := blocker.ApplyFirewallKnown(hosts)
			if err != nil {
				log.Printf("daemon: error applying firewall: %v", err)
			} else {
				firewallDomains = key
				firewallLookups = !complete
				if refresh {
					firewallRefresh = time.Time{}
				}
			}
		}
	default:
		firewallDomains = ""
		firewallLookups = false
		firewallRefresh = time.Time{}
		if blocker.IsFirewallApplied() {
			if err := blocker.RemoveFirewall(); err != nil {
				log.Printf("daemon: error removing firewall: %v", err)
			}
		}
	}

	// Browser policies.
	if !bp.Empty() {
		if !blocker.IsBrowserPoliciesApplied(bp) {
			log.Println("daemon: browser policies drifted, reapplying")
			if err := blocker.ApplyBrowserPolicies(bp); err != nil {
				log.Printf("daemon: error applying browser policies: %v", err)
			}
		}
	} else if blocker.BrowserPoliciesPresent() {
		if err := blocker.RemoveBrowserPolicies(); err != nil {
			log.Printf("daemon: error removing browser policies: %v", err)
		}
	}

	// Kill blocked processes, block by block so each kill is logged as an
	// event of its block.
	for _, b := range active {
		appsTrigger(d, b, now) // closes them, unless the apps' actions say otherwise
	}
	if guard {
		killApps(d, ResetGuardApps, db.EventTamper) // installer media guard
	}
	// Download and streaming tools given a blocked site's address, in the
	// processes of the users the block applies to (§3.2).
	for _, b := range active {
		if tools := blocker.KillNamingFor(blockDomains(b), blockUIDs(b)); len(tools) > 0 {
			log.Printf("daemon: closed %s: given an address block '%s' stops", strings.Join(tools, ", "), b.BlockName)
			logEvent(d, db.EventWorkaround, b.BlockName, strings.Join(tools, ", "))
		}
	}
	enforceNepsis(d, now, active)
	closeTaskManagers(d, now, trusted)
	if v, _ := d.GetSetting(SettingTamperPrivate); v == "on" && os.Getenv("KC9K_DB") == "" {
		if n := blocker.CloseTor(); n > 0 {
			logEvent(d, db.EventTamper, "", "Tor")
		}
	}
	return nil
}

// killApps closes apps and logs an event when any were running.
func killApps(d *db.DB, apps []string, kind string, blocks ...string) {
	killAppsFor(d, apps, nil, kind, blocks...)
}

// killAppsFor is killApps for the processes of uids only (nil: everyone).
// It returns how many processes it closed.
func killAppsFor(d *db.DB, apps []string, uids []uint64, kind string, blocks ...string) int {
	if len(apps) == 0 {
		return 0
	}
	killed, err := blocker.KillBlockedFor(apps, uids, kind == db.EventApp && OptionOn(d, OptionPoliteClose), time.Now())
	if err != nil {
		log.Printf("daemon: error killing blocked processes: %v", err)
	} else if killed > 0 {
		log.Printf("daemon: %d blocked processes terminated", killed)
		logEventFor(d, kind, blocks, "")
		if kind == db.EventApp && LoadNoticeOptions(d).On(NoticeAppBlocked) {
			noticeOnce("app-blocked "+time.Now().Format("2006-01-02 15:04"), "A blocked app was closed",
				"Killer Chicken 9000 closed it: it is in a block that is on.")
		}
	}
	return killed
}

// applyDueEdits applies the queued edits (see db/pending.go) whose time has
// come. An edit to a block that is char-locked or time-locked stays queued
// until the lock lifts, so the queue can't get past a manual lock. An edit
// that fails (e.g. the app was already removed) is logged and dropped.
func applyDueEdits(d *db.DB, now time.Time) error {
	return applyDueEditsAt(d, now, now.Unix())
}

// applyDueEditsAt takes trusted time separately; see db.TrustedNow.
func applyDueEditsAt(d *db.DB, now time.Time, trusted int64) error {
	pending, err := d.ListPendingEdits()
	if err != nil {
		return err
	}
	for _, p := range pending {
		if p.ApplyAt > now.Unix() {
			break // soonest first
		}
		detail, err := d.GetBlock(p.BlockName)
		if err != nil {
			log.Printf("daemon: pending edit: %v", err)
			continue
		}
		if blockLockedNow(d, detail, trusted) {
			continue
		}
		if err := d.ApplyEdit(p.BlockName, p.Edit); err != nil {
			log.Printf("daemon: pending edit to %s (%s) failed, dropping it: %v", p.BlockName, p.Edit, err)
		} else {
			log.Printf("daemon: applied queued edit to %s: %s", p.BlockName, p.Edit)
		}
		if err := d.DeletePendingEdit(p.ID); err != nil {
			return err
		}
	}
	return nil
}

// reconcileSchedules activates blocks their schedule enforces right now (see
// schedule.Enforced: a block window is open, or no allow window is) and
// deactivates the scheduled ones it no longer enforces. Manually-activated
// blocks are never auto-deactivated, nor are scheduled ones that were locked
// while active ("start --lock/--for/--until" on them): a time-locked block is
// deactivated on the first tick after its time lock expires, and a
// char-locked one stays on until it is unlocked. A scheduled block whose
// schedules have all been removed is deactivated like one whose window closed.
func reconcileSchedules(d *db.DB, now time.Time) error {
	return reconcileSchedulesAt(d, now, now.Unix())
}

// reconcileSchedulesAt takes trusted time separately; see db.TrustedNow.
func reconcileSchedulesAt(d *db.DB, now time.Time, trusted int64) error {
	// Schedules and place rules no longer switch blocks on or off (R-SCHED-1,
	// §3.3): enabledBlocks lifts active blocks outside their windows. Places
	// still need Wi-Fi on to be seen.
	keepWifiOn(d)
	return nil
}

// blockLockedNow reports whether any lock of an active block holds it
// (R-LOCK-3): every kind and layer, not only text and timer locks. On a
// read error it says locked, so nothing is stopped or weakened by mistake.
func blockLockedNow(d *db.DB, detail *db.BlockDetail, trusted int64) bool {
	if !detail.Active {
		return false
	}
	extras, err := d.GetLockExtras(detail.Name)
	if err != nil {
		return true
	}
	bootID, err := db.BootID()
	if err != nil {
		return true
	}
	return BlockLocked(detail, extras, trusted, bootID)
}

// ScheduleActiveNow reports whether a block's automation has it enabled now
// (§3.6): its schedule lock holds then, and stopping or unlocking it is
// refused. False for a continuous block.
func ScheduleActiveNow(d *db.DB, blockName string, now time.Time) (bool, error) {
	a, err := AutomationOf(d, blockName)
	if err != nil || a.Continuous() {
		return false, err
	}
	var at map[int]bool
	unknown := false
	if len(a.Places) > 0 {
		places, err := d.Places()
		if err != nil {
			return false, err
		}
		at, unknown = placesHere(now, places)
	}
	return automationEnabled(a, now, at, unknown), nil
}

// ScheduleFrozen reports whether a block's schedule enforces it now or at any
// point within lead: while it has a schedule lock (§3.4), the CLI refuses
// edits that could weaken the block then, whether or not it is active yet.
func ScheduleFrozen(d *db.DB, blockName string, now time.Time, lead time.Duration) (bool, error) {
	schedules, err := d.ListSchedules(blockName)
	if err != nil {
		return false, err
	}
	return schedule.EnforcedWithin(db.Windows(schedules), now, lead), nil
}

// firewallDomains is the domain set the daemon last applied to the firewall.
var firewallDomains string

// firewallLookups is set while that apply left lookups to the background:
// the firewall is applied again when they end.
var firewallLookups bool

// firewallCutKey stands in firewallDomains while monk mode cuts the
// connection; no domain set equals it.
const firewallCutKey = "\x00cut"

func dedup(in []string) []string {
	set := make(map[string]struct{}, len(in))
	for _, s := range in {
		set[s] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// Tamper protection settings for the extension's grace and task managers
// (§4.10).
const (
	SettingExtensionGrace = "tamper.grace"         // seconds; "" = 60
	SettingTaskManagers   = "tamper.task-managers" // "on": closed while anything is locked
	// SettingTamperPrivate: "on" turns private windows off in every browser
	// and closes Tor Browser (§4.10 close private windows and Tor).
	SettingTamperPrivate = "tamper.private"
)

// ExtensionGraceSeconds is how long a browser may run without the
// extension checking in (10 to 600 s; default 60).
func ExtensionGraceSeconds(d *db.DB) int {
	v, _ := d.GetSetting(SettingExtensionGrace)
	n, err := strconv.Atoi(v)
	if err != nil || n < 10 || n > 600 {
		return int(blocker.DefaultExtensionGrace / time.Second)
	}
	return n
}

// TaskManagers are the programs "block task managers while locked" closes.
var TaskManagers = []string{"gnome-system-monitor", "mate-system-monitor", "xfce4-taskmanager", "lxtask",
	"plasma-systemmonitor", "ksysguard", "htop", "btop", "atop", "top", "glances", "stacer", "resources"}

// closeTaskManagers closes task managers while anything is locked, when
// the tamper protection setting is on.
func closeTaskManagers(d *db.DB, now time.Time, trusted int64) {
	if v, _ := d.GetSetting(SettingTaskManagers); v != "on" {
		return
	}
	if locked, err := AnyLock(d, now, trusted); err != nil || !locked {
		return
	}
	killApps(d, TaskManagers, db.EventTamper)
}
