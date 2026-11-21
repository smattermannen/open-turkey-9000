// owners.go — which processes KC9K may kill, and which apps it may block.
//
// Blocking works on programs people run. A block must never take down the
// system or the desktop it runs on: blocking "python3" must not kill a system
// update half-way, and blocking "cinnamon" must not close the desktop over
// and over for as long as the block is locked. Two rules keep it that way:
//
//   - Only processes of regular users are killed. System programs run as root
//     or as service accounts, and under strict mode a user can't start
//     anything as those, so sparing them opens no way around a block.
//   - Programs the desktop session needs can't be added as apps, and are
//     skipped even if a block already lists them.
package blocker

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// regularUser reports whether uid belongs to a person rather than the
// system: 1000 and up, but not nobody (65534) or systemd's dynamic service
// accounts (61184-65519). The subordinate IDs of rootless containers (100000
// and up) are the user's own, so they count as regular.
func regularUser(uid uint64) bool {
	if uid < 1000 || uid == 65534 || uid == 4294967295 {
		return false
	}
	return uid < 61184 || uid > 65519
}

// personUser reports whether uid belongs to a person: a regular user, or a
// guest account. LightDM's guest session (guest-account, Mint's "Guest
// session") logs a person in as a system account (uid below 1000) named
// guest-XXXXXX with its home in /tmp/guest-XXXXXX, so the uid alone would
// leave the guest's programs, windows and logins out of every block (B3).
func personUser(uid uint64) bool {
	return regularUser(uid) || guestUIDs()[uid]
}

// passwdPath is read for guest accounts (replaced in tests).
var passwdPath = "/etc/passwd"

var guestCache struct {
	sync.Mutex
	stamp string
	uids  map[uint64]bool
}

// guestUIDs returns the user IDs of LightDM guest accounts, read from
// /etc/passwd again only when it changes.
func guestUIDs() map[uint64]bool {
	info, err := os.Stat(passwdPath)
	if err != nil {
		return nil
	}
	stamp := info.ModTime().String() + "/" + strconv.FormatInt(info.Size(), 10)
	guestCache.Lock()
	defer guestCache.Unlock()
	if guestCache.uids != nil && guestCache.stamp == stamp {
		return guestCache.uids
	}
	data, err := os.ReadFile(passwdPath)
	if err != nil {
		return nil
	}
	guestCache.stamp, guestCache.uids = stamp, parseGuestUIDs(string(data))
	return guestCache.uids
}

// parseGuestUIDs picks the guest accounts out of /etc/passwd content: named
// guest-* with their home in /tmp/guest-*, as guest-account makes them.
func parseGuestUIDs(passwd string) map[uint64]bool {
	out := map[uint64]bool{}
	for _, line := range strings.Split(passwd, "\n") {
		f := strings.Split(line, ":")
		if len(f) < 7 || !strings.HasPrefix(f[0], "guest-") || f[5] != "/tmp/"+f[0] {
			continue
		}
		if uid, err := strconv.ParseUint(f[2], 10, 32); err == nil && uid != 0 {
			out[uid] = true
		}
	}
	return out
}

// parseStatusUIDs reads the real and effective user IDs from
// /proc/<pid>/status content.
func parseStatusUIDs(status string) (real, effective uint64, ok bool) {
	for _, line := range strings.Split(status, "\n") {
		if !strings.HasPrefix(line, "Uid:") {
			continue
		}
		fields := strings.Fields(line[len("Uid:"):])
		if len(fields) < 2 {
			return 0, 0, false
		}
		r, err1 := strconv.ParseUint(fields[0], 10, 32)
		e, err2 := strconv.ParseUint(fields[1], 10, 32)
		if err1 != nil || err2 != nil {
			return 0, 0, false
		}
		return r, e, true
	}
	return 0, 0, false
}

// processUIDs returns a process's real and effective user IDs.
func processUIDs(pid int) (real, effective uint64, ok bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil {
		return 0, 0, false
	}
	return parseStatusUIDs(string(data))
}

// ownedByUser reports whether a process runs as a person (personUser), by
// both its real and effective user ID: a setuid program a user started
// (sudo, passwd) is running with system rights and is left alone.
func ownedByUser(pid int) bool {
	real, effective, ok := processUIDs(pid)
	return ok && personUser(real) && personUser(effective)
}

// ownedBy reports whether a process runs as the person uid, by both its
// real and effective user ID.
func ownedBy(pid int, uid uint64) bool {
	real, effective, ok := processUIDs(pid)
	return ok && personUser(uid) && real == uid && effective == uid
}

// essentialApps are programs the desktop session can't do without (names in
// lower case). Desktop helpers, and KC9K's own interface, run on python3.
var essentialApps = map[string]bool{
	"x": true, "xorg": true, "xwayland": true,
	"cinnamon": true, "cinnamon-session": true, "cinnamon-session-binary": true,
	"cinnamon-launcher": true, "cinnamon-screensaver": true, "cinnamon-killer-daemon": true,
	"muffin": true, "gnome-shell": true, "gnome-session": true, "gnome-session-binary": true,
	"plasmashell": true, "kwin_x11": true, "kwin_wayland": true, "ksmserver": true,
	"xfce4-session": true, "xfwm4": true, "mate-session": true, "marco": true,
	"systemd": true, "init": true, "dbus-daemon": true, "dbus-broker": true, "dbus-broker-launch": true,
	"pipewire": true, "pipewire-pulse": true, "wireplumber": true, "pulseaudio": true,
	"gnome-keyring-daemon": true, "at-spi-bus-launcher": true, "at-spi2-registryd": true,
	"xdg-desktop-portal": true, "xdg-document-portal": true, "xdg-permission-store": true,
	"lightdm": true, "gdm": true, "gdm3": true, "sddm": true, "login": true, "agetty": true,
	"polkitd": true, "kc9k": true, "kc9k-bin": true, "kc9k-gui": true,
	"python": true, "python3": true,
}

// essentialPrefixes cover families of desktop helpers: Cinnamon's and
// GNOME's settings daemons, portal backends, and versioned Python.
var essentialPrefixes = []string{"csd-", "gsd-", "xdg-desktop-portal-", "python3."}

// isEssentialApp reports whether an app entry (a name, or a path whose last
// element is checked) names a program the desktop needs.
func isEssentialApp(entry string) bool {
	name := strings.ToLower(baseName(strings.TrimRight(entry, "/")))
	if essentialApps[name] {
		return true
	}
	for _, prefix := range essentialPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// withoutEssentialApps drops essential programs from a list of app entries.
func withoutEssentialApps(names []string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if !isEssentialApp(n) {
			out = append(out, n)
		}
	}
	return out
}

// checkEssentialApp refuses an entry naming a program the desktop needs.
func checkEssentialApp(entry string) error {
	if strings.HasSuffix(entry, "/") || !isEssentialApp(entry) {
		return nil
	}
	return fmt.Errorf("%s can't be blocked: the desktop needs it, and blocking it would close your session over and over", entry)
}
