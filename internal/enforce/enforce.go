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
	"log"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// Inputs derives the layer inputs from the active blocks:
//   - hostsDomains: the domains of blacklists that look at addresses and
//     block, for /etc/hosts and the firewall, minus those with an address
//     exception under them (those layers block whole hosts).
//   - apps: the union of process names to kill.
//   - bp: the browser policy: those blacklists' domains and patterns, with
//     the block's address exceptions.
//
// Everything is unioned across active blocks. A block with exceptions by
// title or text leaves its websites to the extension alone: only it can
// read a page before deciding, so those layers would block pages the
// exceptions spare. Greyscale and no-images only go to the extension (see
// BuildRules). Apps are blocked whatever the lists do.
func Inputs(active []db.ActiveBlockDetail) (hostsDomains, apps []string, bp blocker.BrowserPolicy) {
	var blockDomains, hostDomains, patterns, exceptions, appList []string

	for _, b := range active {
		appList = append(appList, b.Apps...)
		if pageExceptions(b.Lists) {
			continue
		}
		except := urlEntries(b.Lists.ExceptionEntries(db.PlaceURL))
		used := false
		for _, l := range b.Lists.Blacklists() {
			if !l.Enforces() || !l.Where.URL || !l.HasAction(db.ActionBlock) {
				continue
			}
			used = true
			domains, pats := blocker.SplitURLEntries(l.Entries)
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
	}
	return blocker.ValidDomains(hostDomains), dedup(appList), bp
}

// pageExceptions reports whether a block has exceptions by title or text.
func pageExceptions(lists db.Lists) bool {
	return len(lists.ExceptionEntries(db.PlaceTitle))+len(lists.ExceptionEntries(db.PlaceText)) > 0
}

// titleGuard closes apps whose window titles contain blocked keywords.
var titleGuard = blocker.NewTitleGuard()

// titleGuardBusy is set while the title guard runs. It talks to X servers,
// and anyone can start one, so it runs beside the daemon's cycle, once at a
// time, rather than in it: a slow server delays only the next title check.
var titleGuardBusy atomic.Bool

// enforceTitles starts a title guard cycle unless one is still running.
// Each window asked to close is logged as an event of every active block
// listing its keyword.
func enforceTitles(d *db.DB, active []db.ActiveBlockDetail, now time.Time) {
	if !titleGuardBusy.CompareAndSwap(false, true) {
		return
	}
	keywords := activeTitles(active)
	owners := titleOwners(active)
	go func() {
		defer titleGuardBusy.Store(false)
		_, asked := titleGuard.Enforce(keywords, now)
		for _, keyword := range asked {
			for _, block := range owners[keyword] {
				logEvent(d, db.EventTitle, block, keyword)
			}
		}
	}()
}

// titleOwners maps each window-title keyword to the active blocks listing it.
func titleOwners(active []db.ActiveBlockDetail) map[string][]string {
	owners := map[string][]string{}
	for _, b := range active {
		for _, k := range b.Titles {
			owners[k] = append(owners[k], b.BlockName)
		}
	}
	return owners
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

// activeTitles collects the window-title keywords of the active blocks.
func activeTitles(active []db.ActiveBlockDetail) []string {
	var out []string
	for _, b := range active {
		out = append(out, b.Titles...)
	}
	return dedup(out)
}

// deferKills makes Apply leave blocked apps to the daemon (see ApplyFromDB).
// Only set while the apply lock is held.
var deferKills bool

// Apply forces every layer to match the active blocks. Used by the CLI right
// after the desired state changes (start, stop, unlock, edit), where the domain
// set may have changed and we want the effect immediately.
func Apply(active []db.ActiveBlockDetail) error {
	hosts, apps, bp := Inputs(active)

	var firstErr error
	note := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	note(blocker.ApplyHosts(hosts))        // empty list => removes our section
	note(blocker.ApplyFirewall(hosts))     // empty list => removes the chain
	note(blocker.ApplyBrowserPolicies(bp)) // empty policy => removes files
	if len(apps) > 0 && !deferKills {
		blocker.KillBlocked(apps)
	}
	return firstErr
}

// ApplyFromDB reads the active blocks and applies them. Convenience for the CLI.
func ApplyFromDB(d *db.DB) error {
	defer applyLock()()
	active, err := d.GetActiveBlocks()
	if err != nil {
		return err
	}
	if trusted, err := d.TrustedNow(time.Now()); err == nil {
		active = withoutBreaks(d, active, trusted)
	}
	// With polite closing on, the daemon's next cycle asks the apps to
	// close instead of this killing them outright.
	deferKills = OptionOn(d, OptionPoliteClose)
	defer func() { deferKills = false }()
	return Apply(active)
}

// Tick runs one daemon cycle: apply due queued edits, reconcile schedules,
// then re-apply drifted layers.
func Tick(d *db.DB, now time.Time) error {
	defer applyLock()()
	if err := reconcileFreezeSchedules(d, now); err != nil {
		log.Printf("daemon: freeze schedule error: %v", err)
	}
	enforceFreeze(d, now)
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
	if err := reconcileSchedulesAt(d, now, trusted); err != nil {
		log.Printf("daemon: schedule reconciliation error: %v", err)
	}
	if err := reconcileQuotas(d, now, trusted); err != nil {
		log.Printf("daemon: quota error: %v", err)
	}
	sendNotices(d, now)

	active, err := d.GetActiveBlocks()
	if err != nil {
		return err
	}
	active = withoutBreaks(d, active, trusted)
	hosts, _, bp := Inputs(active)
	enforceExtension(d, now, active)
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
		key := strings.Join(hosts, ",")
		if !blocker.IsFirewallApplied() || key != firewallDomains {
			log.Println("daemon: firewall drifted or domains changed, reapplying")
			if err := blocker.ApplyFirewall(hosts); err != nil {
				log.Printf("daemon: error applying firewall: %v", err)
			} else {
				firewallDomains = key
			}
		}
	} else {
		firewallDomains = ""
		if blocker.HostsSectionPresent() {
			if err := blocker.RemoveHosts(); err != nil {
				log.Printf("daemon: error removing /etc/hosts section: %v", err)
			}
		}
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
		killApps(d, b.Apps, db.EventApp, b.BlockName)
	}
	killApps(d, forcedKills(d), db.EventBrowser, websiteBlocks(active)...)
	enforceNepsis(d, now, active)
	return nil
}

// killApps closes apps and logs an event when any were running.
func killApps(d *db.DB, apps []string, kind string, blocks ...string) {
	if len(apps) == 0 {
		return
	}
	var killed int
	var err error
	if kind == db.EventApp && OptionOn(d, OptionPoliteClose) {
		killed, err = blocker.KillBlockedPolitely(apps, time.Now())
	} else {
		killed, err = blocker.KillBlocked(apps)
	}
	if err != nil {
		log.Printf("daemon: error killing blocked processes: %v", err)
	} else if killed > 0 {
		log.Printf("daemon: %d blocked processes terminated", killed)
		logEventFor(d, kind, blocks, "")
	}
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
		if detail.Active && (detail.Locked || detail.LockedUntil > trusted) {
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
	all, err := d.AllSchedules()
	if err != nil {
		return err
	}
	active, err := d.GetActiveBlocks()
	if err != nil {
		return err
	}

	// Group windows by block; scheduled blocks with no schedules left get an
	// empty list, which schedule.Enforced treats as "off".
	windows := map[string][]schedule.Window{}
	for _, s := range all {
		windows[s.BlockName] = append(windows[s.BlockName], s.Window())
	}
	for _, b := range active {
		if _, ok := windows[b.BlockName]; !ok && b.Scheduled {
			windows[b.BlockName] = nil
		}
	}

	// A quota keeps its block on outside its sessions (reconcileQuotas), as
	// a scheduled block: leave turning those off to the quota.
	quotas, err := d.QuotaBlocks()
	if err != nil {
		return err
	}

	for name, ws := range windows {
		within := schedule.Enforced(ws, now)

		detail, err := d.GetBlock(name)
		if err != nil {
			log.Printf("daemon: schedule: %v", err)
			continue
		}

		switch {
		case within && !detail.Active:
			// Activate as a scheduled block (not char-locked, but the CLI
			// refuses to stop it while the schedule enforces it).
			if err := d.ActivateBlock(name, false, 0, 0, true); err != nil {
				log.Printf("daemon: schedule: failed to activate %s: %v", name, err)
			} else {
				log.Printf("daemon: schedule: activated %s", name)
			}
		case !within && detail.Active && detail.Scheduled && !detail.Locked && detail.LockedUntil <= trusted &&
			!slices.Contains(quotas, name):
			if err := d.DeactivateBlock(name); err != nil {
				log.Printf("daemon: schedule: failed to deactivate %s: %v", name, err)
			} else {
				log.Printf("daemon: schedule: deactivated %s", name)
			}
		}
	}
	return nil
}

// ScheduleActiveNow reports whether a block's schedule enforces it now (see
// schedule.Enforced). The CLI uses it to refuse stopping/unlocking a scheduled
// block while that holds.
func ScheduleActiveNow(d *db.DB, blockName string, now time.Time) (bool, error) {
	schedules, err := d.ListSchedules(blockName)
	if err != nil {
		return false, err
	}
	return schedule.Enforced(db.Windows(schedules), now), nil
}

// ScheduleFrozen reports whether a block's schedule enforces it now or at any
// point within schedule.EditFreeze. The CLI refuses edits that could weaken the
// block while this holds, whether or not the block is active yet.
func ScheduleFrozen(d *db.DB, blockName string, now time.Time) (bool, error) {
	schedules, err := d.ListSchedules(blockName)
	if err != nil {
		return false, err
	}
	return schedule.EnforcedWithin(db.Windows(schedules), now, schedule.EditFreeze), nil
}

// firewallDomains is the domain set the daemon last applied to the firewall.
var firewallDomains string

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
