// layers.go — layered sequential locks on a block (§3.4, R-LOCK-6/7).
//
// A block's locks are unlocked one at a time in the user's order
// (LockExtras.Order): `unlock` only acts on the first lock still on. A timer
// lock given as a duration behind other locks waits (TimerAfter) and counts
// from the moment the locks before it are gone; a restart lock behind
// others waits (RestartPending) and needs a boot after they are gone. A
// dated timer lock simply holds until its date. New locks go last (R-LOCK-7).
// Each kind appears once.
package db

import (
	"strings"
)

// Lock layer kinds.
const (
	LayerTimer    = "timer"
	LayerText     = "text"
	LayerPassword = "password"
	LayerRestart  = "restart"
	LayerDelay    = "delay"
)

// RestartPending in LockExtras.RestartBoot marks a restart lock waiting for
// the locks before it; it gets the boot then current once they are gone.
const RestartPending = "pending"

// legacyOrder is the order of blocks locked before layers were stored: the
// timer and restart locks had to run out first, then unlock asked the
// password, then the challenge, then started the delay countdown.
var legacyOrder = []string{LayerTimer, LayerRestart, LayerPassword, LayerText, LayerDelay}

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
	}
	return false
}

// Layers returns the block's locks in unlock order: the stored order, then
// any kind set but missing from it, in the legacy order.
func Layers(detail *BlockDetail, e LockExtras) []string {
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
	}
	return false
}

// LayersOn returns the locks still on, in unlock order; the first is the
// one to unlock next.
func LayersOn(detail *BlockDetail, e LockExtras, trusted int64, bootID string) []string {
	var out []string
	for _, k := range Layers(detail, e) {
		if LayerOn(k, detail, e, trusted, bootID) {
			out = append(out, k)
		}
	}
	return out
}

// AdvanceLayers starts the first lock still on if it was waiting: a
// pending timer lock starts counting, a pending restart lock takes the
// current boot. Lifted text/password locks whose re-lock time is over come
// back. It reports whether detail or e changed (the caller saves them).
func AdvanceLayers(detail *BlockDetail, e *LockExtras, trusted int64, bootID string) bool {
	changed := false
	if e.LiftedKinds != "" && e.LiftedUntil != 0 && e.LiftedUntil <= trusted {
		e.LiftedKinds, e.LiftedUntil = "", 0
		changed = true
	}
	on := LayersOn(detail, *e, trusted, bootID)
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
// is gone. With re-lock it is lifted: while the other of the two still
// holds, until that one is unlocked too; then both for the re-lock time.
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
		other := LayerPassword
		if kind == LayerPassword {
			other = LayerText
		}
		if !LayerOn(other, detail, *e, trusted, bootID) {
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
	}
	return kind
}
