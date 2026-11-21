package blocker

import (
	"sync/atomic"
	"time"
)

// extensionGrace is the grace in force (nanoseconds); SetExtensionGrace.
var extensionGrace atomic.Int64

func init() { extensionGrace.Store(int64(DefaultExtensionGrace)) }

// SetExtensionGrace sets the grace (10 s to 10 min).
func SetExtensionGrace(d time.Duration) {
	extensionGrace.Store(int64(min(max(d, 10*time.Second), 10*time.Minute)))
}

// ExtensionGrace returns the grace in force.
func ExtensionGrace() time.Duration { return time.Duration(extensionGrace.Load()) }

// guardedProcess is what the guard knows of a running browser.
type guardedProcess struct {
	remote    bool // started for remote control, which isn't allowed
	supported bool // an installed supported browser (SupportedBrowsers)
	age       time.Duration
	ageKnown  bool
	lastSeen  time.Time // its extension's last check-in
	seen      bool
}

// closeReason says why the guard closes p, or "" to leave it open.
func closeReason(p guardedProcess, now, settingsChanged time.Time, unsupported bool) string {
	if p.remote {
		return "started for remote control"
	}
	if unsupported && !p.supported {
		return "not a supported browser (block unsupported browsers is on)"
	}
	if !settingsChanged.IsZero() && now.Sub(settingsChanged) >= ExtensionGrace() &&
		p.ageKnown && now.Add(-p.age).Before(settingsChanged) {
		return "started before KC9K changed its browser settings"
	}
	if !p.ageKnown || p.age < ExtensionGrace() {
		return ""
	}
	if p.seen && now.Sub(p.lastSeen) < ExtensionGrace() {
		return ""
	}
	return "no Killer Chicken extension"
}
