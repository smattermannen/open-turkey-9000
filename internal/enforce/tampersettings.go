// tampersettings.go — the tamper protection settings of §4.10 that change
// what the guard, the extension and the installer do (SPEC §4.10, decided
// 2026-11-21 By Opus):
//
//   - guest: guest browsing off in every browser and no guest session at
//     the desktop's login screen (off by default, like private);
//   - extension-stop: close browsers if the extension stops; on by default
//     (what KC9K always did). Off, browsers without it are closed only
//     while anything is locked (§3.4, R-LOCK-9), not for an active block;
//   - installer: while anything is locked, reinstalling or going back to an
//     older version is refused too (removal always is, R-LOCK-9); off by
//     default;
//   - inactive-tabs: background tabs are blocked at once; on by default.
//     Off, a tab open in the background is judged when it is switched to;
//   - embedded: a blocked site inside another page gets the block page; on
//     by default. Off, the extension leaves frames alone (hosts and the
//     firewall still block whole domains).
package enforce

import (
	"github.com/smattermannen/kc9k/internal/db"
)

// The settings, stored "on"/"off".
const (
	SettingTamperGuest     = "tamper.guest"
	SettingExtensionStop   = "tamper.extension-stop"
	SettingTamperInstaller = "tamper.installer"
	SettingInactiveTabs    = "tamper.inactive-tabs"
	SettingEmbedded        = "tamper.embedded"
)

// tamperDefaultOn are the settings that were always on before they were
// settings: unset means on (principle 7: behaviour changes are settings,
// not new defaults).
var tamperDefaultOn = map[string]bool{SettingExtensionStop: true, SettingInactiveTabs: true, SettingEmbedded: true}

// TamperOn reports whether a tamper protection setting stored as on/off is
// on, its default when unset.
func TamperOn(d *db.DB, key string) bool {
	v, err := d.GetSetting(key)
	if err != nil {
		return tamperDefaultOn[key]
	}
	switch v {
	case "on":
		return true
	case "off":
		return false
	}
	return tamperDefaultOn[key]
}

// TamperDefaultOn reports whether a setting is on when never set.
func TamperDefaultOn(key string) bool { return tamperDefaultOn[key] }
