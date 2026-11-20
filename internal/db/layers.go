// layers.go — layered sequential locks on a block (§3.4, R-LOCK-6/7).
//
// A block's locks are unlocked one at a time in the user's order
// (LockExtras.Order): `unlock` only acts on the first lock still on. A timer
// lock given as a duration behind other locks waits (TimerAfter) and counts
// from the moment the locks before it are gone; a restart lock behind
// others waits (RestartPending) and needs a boot after they are gone. A
// dated timer lock simply holds until its date. New locks go last (R-LOCK-7).
// Each kind has one place (the block's columns); a lock of a kind already
// there waits in LockExtras.Queue, after all the others, and takes its
// kind's place once every lock before it is gone (§3.4, several locks of one
// kind). A text or password lock re-lock has lifted that a later one
// replaces waits in LockExtras.Relifted and comes back with the others.
package db

import (
	"slices"
	"strings"
	"time"
)

// Lock layer kinds.
const (
	LayerTimer    = "timer"
	LayerText     = "text"
	LayerPassword = "password"
	LayerRestart  = "restart"
	LayerDelay    = "delay"
	LayerRange    = "range"
)

// RestartPending in LockExtras.RestartBoot marks a restart lock waiting for
// the locks before it; it gets the boot then current once they are gone.
const RestartPending = "pending"

// legacyOrder is the order of blocks locked before layers were stored: the
// timer and restart locks had to run out first, then unlock asked the
// password, then the challenge, then started the delay countdown.
var legacyOrder = []string{LayerTimer, LayerRestart, LayerPassword, LayerText, LayerDelay, LayerRange}

// ParseLayerOrder reads a stored order ("text,timer").
func ParseLayerOrder(s string) []string {
	var out []string
	for _, k := range strings.Split(s, ",") {
		if k = strings.TrimSpace(k); k != "" && !hasKind(out, k) {
			out = append(out, k)
		}
	}
	return out
}

func hasKind(list []string, k string) bool {
	for _, x := range list {
		if x == k {
			return true
		}
	}
	return false
}

// AppendLayers adds kinds not yet in the order at its end (R-LOCK-7).
func AppendLayers(order []string, kinds ...string) []string {
	for _, k := range kinds {
		if !hasKind(order, k) {
			order = append(order, k)
		}
	}
	return order
}

// textLifted reports whether a text or password lock is lifted by re-lock
// for now: kinds unlocked while other text/password locks still held stay
// lifted until those are unlocked too (LiftedUntil 0); then all come back
// together LiftedUntil (the re-lock time after the last unlock). Before
// LiftedKinds existed, a lift covered both.
func (e LockExtras) textLifted(kind string, trusted int64) bool {
	kinds := ParseLayerOrder(e.LiftedKinds)
	if len(kinds) == 0 {
		return e.LiftedUntil > trusted
	}
	return hasKind(kinds, kind) && (e.LiftedUntil == 0 || e.LiftedUntil > trusted)
}

// LayerOn reports whether a kind of lock holds the block now, or waits
// behind others (a pending timer or restart lock).
func LayerOn(kind string, detail *BlockDetail, e LockExtras, trusted int64, bootID string) bool {
	switch kind {
	case LayerTimer:
		return detail.LockedUntil > trusted || e.TimerAfter > 0
	case LayerText:
		return detail.Locked && !e.textLifted(LayerText, trusted)
	case LayerPassword:
		return e.PasswordHash != "" && !e.textLifted(LayerPassword, trusted)
	case LayerRestart:
		return e.RestartBoot == RestartPending || (e.RestartBoot != "" && e.RestartBoot == bootID)
	case LayerDelay:
		return e.Delay > 0
	case LayerRange:
		return e.RangeDays != 0 && e.InRange(time.Unix(trusted, 0))
	}
	return false
}

// InRange reports whether a range lock is on at t: within its times on its
// days (an overnight range belongs to the day it starts), or outside them
// when inverted.
func (e LockExtras) InRange(t time.Time) bool {
	if e.RangeDays == 0 {
		return false
	}
	day := func(d time.Weekday) bool { return e.RangeDays&(1<<uint(d)) != 0 }
	minute := t.Hour()*60 + t.Minute()
	var in bool
	if e.RangeFrom < e.RangeTo {
		in = day(t.Weekday()) && minute >= e.RangeFrom && minute < e.RangeTo
	} else {
		in = (day(t.Weekday()) && minute >= e.RangeFrom) || (day(t.AddDate(0, 0, -1).Weekday()) && minute < e.RangeTo)
	}
	return in != e.RangeInvert
}

// Layers returns the block's locks in unlock order: the stored order, then
// any kind set but missing from it, in the legacy order, then the queue.
func Layers(detail *BlockDetail, e LockExtras) []string {
	return append(placedLayers(detail, e), queuedKinds(e)...)
}

func queuedKinds(e LockExtras) []string {
	var out []string
	for _, q := range e.Queue {
		out = append(out, q.Kind)
	}
	return out
}

// PlacedLayers is Layers without the queue: the kinds in their places,
// whose order LockExtras.Order stores.
func PlacedLayers(detail *BlockDetail, e LockExtras) []string { return placedLayers(detail, e) }

func placedLayers(detail *BlockDetail, e LockExtras) []string {
	order := ParseLayerOrder(e.Order)
	if len(order) == 0 {
		order = append(order, legacyOrder...)
	}
	order = AppendLayers(order, legacyOrder...)
	var out []string
	for _, k := range order {
		if hasKind(legacyOrder, k) && layerSet(k, detail, e) {
			out = append(out, k)
		}
	}
	return out
}

// layerSet reports whether a kind of lock exists at all, held or not.
func layerSet(kind string, detail *BlockDetail, e LockExtras) bool {
	switch kind {
	case LayerTimer:
		return detail.LockedUntil != 0 || e.TimerAfter > 0
	case LayerText:
		return detail.Locked
	case LayerPassword:
		return e.PasswordHash != ""
	case LayerRestart:
		return e.RestartBoot != ""
	case LayerDelay:
		return e.Delay > 0
	case LayerRange:
		return e.RangeDays != 0
	}
	return false
}

// LayersOn returns the locks still on, in unlock order; the first is the
// one to unlock next.
// Queued locks are all on: they wait behind the others.
func LayersOn(detail *BlockDetail, e LockExtras, trusted int64, bootID string) []string {
	return append(placedOn(detail, e, trusted, bootID), queuedKinds(e)...)
}

func placedOn(detail *BlockDetail, e LockExtras, trusted int64, bootID string) []string {
	var out []string
	for _, k := range placedLayers(detail, e) {
		if LayerOn(k, detail, e, trusted, bootID) {
			out = append(out, k)
		}
	}
	return out
}

// promote puts the first queued lock in its kind's place, now that no other
// lock is on, and makes it last in the stored order.
func promote(detail *BlockDetail, e *LockExtras, trusted int64, bootID string) {
	q := e.Queue[0]
	e.Queue = e.Queue[1:]
	if (q.Kind == LayerText || q.Kind == LayerPassword) && layerSet(q.Kind, detail, *e) {
		// Its place holds a lock re-lock has lifted: that one waits to come
		// back with the others, and the new one is on.
		e.Relifted = append(e.Relifted, placed(q.Kind, detail, *e))
		lifted := ParseLayerOrder(e.LiftedKinds)
		if len(lifted) == 0 && e.LiftedUntil > trusted {
			lifted = []string{LayerText, LayerPassword}
		}
		e.LiftedKinds = strings.Join(slices.DeleteFunc(lifted, func(k string) bool { return k == q.Kind }), ",")
	}
	switch q.Kind {
	case LayerText:
		detail.Locked, detail.LockChars, e.Style = true, q.Chars, q.Style
	case LayerPassword:
		e.PasswordHash = q.PasswordHash
	case LayerRange:
		e.RangeDays, e.RangeFrom, e.RangeTo, e.RangeInvert = q.RangeDays, q.RangeFrom, q.RangeTo, q.RangeInvert
	case LayerTimer:
		e.TimerAfter = 0
		if q.For > 0 {
			detail.LockedUntil = trusted + q.For
		} else {
			detail.LockedUntil = q.Until
		}
	case LayerRestart:
		e.RestartBoot = bootID
	case LayerDelay:
		e.Delay, e.DelayUntil = q.Delay, 0
	}
	var order []string
	for _, k := range ParseLayerOrder(e.Order) {
		if k != q.Kind {
			order = append(order, k)
		}
	}
	e.Order = strings.Join(append(order, q.Kind), ",")
}

// AdvanceLayers starts the first lock still on if it was waiting: a
// pending timer lock starts counting, a pending restart lock takes the
// current boot. Lifted text/password locks whose re-lock time is over come
// back. It reports whether detail or e changed (the caller saves them).
func AdvanceLayers(detail *BlockDetail, e *LockExtras, trusted int64, bootID string) bool {
	changed := false
	if e.LiftedKinds != "" && e.LiftedUntil != 0 && e.LiftedUntil <= trusted {
		relockAll(detail, e)
		e.LiftedKinds, e.LiftedUntil = "", 0
		changed = true
	}
	// Queued locks take their kind's place once nothing before them is on
	// (a dated timer lock already past lets the next one through at once).
	for len(e.Queue) > 0 && len(placedOn(detail, *e, trusted, bootID)) == 0 {
		promote(detail, e, trusted, bootID)
		changed = true
	}
	on := placedOn(detail, *e, trusted, bootID)
	if len(on) == 0 {
		return changed
	}
	switch on[0] {
	case LayerTimer:
		if e.TimerAfter > 0 {
			if until := trusted + e.TimerAfter; until > detail.LockedUntil {
				detail.LockedUntil = until
			}
			e.TimerAfter = 0
			changed = true
		}
	case LayerRestart:
		if e.RestartBoot == RestartPending {
			e.RestartBoot = bootID
			changed = true
		}
	}
	return changed
}

// LiftTextLayer unlocks a text or password lock (kind). Without re-lock it
// is gone. With re-lock it is lifted: while another text or password lock
// still holds or waits, until those are unlocked too; then all of them for
// the re-lock time.
// The next layer is started (AdvanceLayers).
func LiftTextLayer(detail *BlockDetail, e *LockExtras, kind string, trusted int64, bootID string) {
	if e.Relock > 0 {
		lifted := ParseLayerOrder(e.LiftedKinds)
		if len(lifted) == 0 && e.LiftedUntil > trusted {
			lifted = []string{LayerText, LayerPassword}
		}
		lifted = AppendLayers(lifted, kind)
		e.LiftedKinds = strings.Join(lifted, ",")
		e.LiftedUntil = 0
		// The re-lock time starts once no text or password lock is on,
		// placed or waiting.
		if !LayerOn(LayerText, detail, *e, trusted, bootID) && !LayerOn(LayerPassword, detail, *e, trusted, bootID) &&
			!slices.ContainsFunc(e.Queue, isTextOrPassword) {
			e.LiftedUntil = trusted + e.Relock
		}
	} else if kind == LayerText {
		detail.Locked, detail.LockChars, e.Style = false, 0, ""
	} else {
		e.PasswordHash = ""
	}
	AdvanceLayers(detail, e, trusted, bootID)
}

// SaveLayers stores what AdvanceLayers changed on an active block.
func (d *DB) SaveLayers(detail *BlockDetail, e LockExtras) error {
	if err := d.StrengthenLocks(detail.Name, detail.Locked, detail.LockChars, detail.LockedUntil); err != nil {
		return err
	}
	return d.SetLockExtras(detail.Name, e)
}

// AdvanceBlockLayers loads an active block's locks, advances its layers
// and saves them if anything changed.
func (d *DB) AdvanceBlockLayers(name string, trusted int64, bootID string) error {
	detail, err := d.GetBlock(name)
	if err != nil || !detail.Active {
		return err
	}
	e, err := d.GetLockExtras(name)
	if err != nil {
		return err
	}
	if AdvanceLayers(detail, &e, trusted, bootID) {
		return d.SaveLayers(detail, e)
	}
	return nil
}

// LayerName is a lock kind as the user reads it.
func LayerName(kind string) string {
	switch kind {
	case LayerTimer:
		return "timer lock"
	case LayerText:
		return "text lock"
	case LayerPassword:
		return "password lock"
	case LayerRestart:
		return "restart lock"
	case LayerDelay:
		return "delay lock"
	case LayerRange:
		return "range lock"
	}
	return kind
}

func isTextOrPassword(q QueuedLock) bool { return q.Kind == LayerText || q.Kind == LayerPassword }

// placed returns the lock in a text or password kind's place.
func placed(kind string, detail *BlockDetail, e LockExtras) QueuedLock {
	if kind == LayerText {
		return QueuedLock{Kind: kind, Chars: detail.LockChars, Style: e.Style}
	}
	return QueuedLock{Kind: kind, PasswordHash: e.PasswordHash}
}

// relockAll puts back the text and password locks a later one of their
// kind replaced while re-lock had them lifted (Relifted), in the order they
// were given: each kind's first takes its place again, the rest wait first
// in the queue. The caller then ends the lift.
func relockAll(detail *BlockDetail, e *LockExtras) {
	if len(e.Relifted) == 0 {
		return
	}
	all := e.Relifted
	var order []string
	for _, k := range placedLayers(detail, *e) {
		if k == LayerText || k == LayerPassword {
			all = append(all, placed(k, detail, *e))
		} else {
			order = append(order, k)
		}
	}
	var back []QueuedLock
	for _, q := range all {
		if hasKind(order, q.Kind) {
			back = append(back, q)
			continue
		}
		order = append(order, q.Kind)
		if q.Kind == LayerText {
			detail.Locked, detail.LockChars, e.Style = true, q.Chars, q.Style
		} else {
			e.PasswordHash = q.PasswordHash
		}
	}
	e.Relifted = nil
	e.Queue = append(back, e.Queue...)
	e.Order = strings.Join(order, ",")
}
