// resetguard.go — factory reset protection's installer media guard (§3.8,
// ROADMAP 2.6). While the setting is on and anything is locked, a built-in
// block, made here and never stored (like monk mode's), blocks the pages
// that hand out operating system images and USB image writers, and the
// image writers themselves are closed. It makes a reinstall slow and
// deliberate; it can't stop media made elsewhere. The firmware lock part
// of factory reset protection is not built.
package enforce

import (
	"time"

	"github.com/smattermannen/kc9k/internal/db"
)

const (
	// SettingResetGuard is "on" while the guard is on.
	SettingResetGuard = "device.reset-guard"
	// ResetGuardName names its block on block pages and in events.
	ResetGuardName = "Factory reset protection"
	// ResetGuardListID is its list's ID, apart from stored lists.
	ResetGuardListID = MonkListID + 16
)

// ResetGuardSites are OS image download pages and USB image writers' sites.
// Package mirrors stay open: updates must keep working.
var ResetGuardSites = []string{
	"ventoy.net", "etcher.balena.io", "balena.io/etcher*", "rufus.ie", "unetbootin.github.io",
	"rpi.org/software*", "raspberrypi.com/software*",
	"linuxmint.com/download*", "linuxmint.com/edition.php*", "ubuntu.com/download*",
	"releases.ubuntu.com", "cdimage.ubuntu.com", "cdimage.debian.org", "debian.org/distrib*",
	"fedoraproject.org/*download*", "getfedora.org", "archlinux.org/download*", "manjaro.org/download*",
	"pop.system76.com", "elementary.io", "zorin.com/os/download*", "mxlinux.org/download*",
	"opensuse.org/*download*", "get.opensuse.org", "kali.org/get-kali*", "endeavouros.com",
	"microsoft.com/*software-download*", "distrowatch.com",
}

// ResetGuardApps are USB image writers and disk restore tools.
var ResetGuardApps = []string{
	"mintstick", "balena-etcher", "balenaEtcher", "balena-etcher-electron", "VentoyGUI.x86_64",
	"popsicle-gtk", "com.system76.Popsicle", "usb-creator-gtk", "usb-creator-kde", "gnome-disks",
	"org.gnome.DiskUtility", "mediawriter", "org.fedoraproject.MediaWriter", "rpi-imager",
	"org.raspberrypi.rpi-imager", "unetbootin", "woeusb", "woeusbgui", "impression",
	"io.gitlab.adhami3310.Impression", "gnome-multi-writer", "isoimagewriter", "caligula",
}

// ResetGuardOn reports whether the setting is on.
func ResetGuardOn(d *db.DB) bool {
	v, err := d.GetSetting(SettingResetGuard)
	return err == nil && v == "on"
}

// resetGuardActive: on, and something locked.
func resetGuardActive(d *db.DB, trusted int64) bool {
	if !ResetGuardOn(d) {
		return false
	}
	locked, err := AnyLock(d, time.Now(), trusted)
	return err == nil && locked
}

// resetGuardBlock is what the guard blocks, as an active block.
func resetGuardBlock() db.ActiveBlockDetail {
	return db.ActiveBlockDetail{BlockName: ResetGuardName, Apps: ResetGuardApps, Lists: db.Lists{
		{ID: ResetGuardListID, Kind: db.ListBlacklist, Name: ResetGuardName, Where: db.Where{URL: true},
			Entries: ResetGuardSites, Actions: []db.ListAction{{Name: db.ActionBlock, Distinct: 1, Hits: 1}}},
		{ID: ResetGuardListID + 1, Kind: db.ListExceptions, Name: db.DefaultExceptionsList, Where: db.Where{URL: true},
			Entries: []string{db.FileException}},
	}}
}
