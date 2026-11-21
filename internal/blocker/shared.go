// shared.go — types and helpers the layers share on every platform: what
// enforce hands a layer and what a layer reports back.

package blocker

import (
	"math"
	"time"
)

// BrowserSetup is what stays enforced in Firefox-family browsers whether or not
// a block runs.
type BrowserSetup struct {
	ExtensionID  string // force-installed when set
	ExtensionXPI string
	ForceSearch  bool
	// DisableDevTools blocks the developer tools, which can switch the
	// extension off from its own pages or about:debugging.
	DisableDevTools bool
	// DisablePrivate turns off private windows (no-private).
	DisablePrivate bool
	// RemoveExtensionID is set when uninstalling: browsers remove that
	// extension at their next start. Without it, a policy-installed
	// extension would stay behind and keep enforcing its last rules forever,
	// with no daemon to update them.
	RemoveExtensionID string
}

// ChromeSetup is what stays enforced in Google Chrome and Chromium whether or
// not a block runs, written as a policy file of its own beside the blocklist
// (kc9k.json, see browser.go), so the two never touch each other's settings.
type ChromeSetup struct {
	// ExtensionID is force-installed from UpdateURL (see webext.PackChrome).
	ExtensionID string
	UpdateURL   string
	// DisableDevTools blocks the developer tools everywhere (block-devtools).
	// They are always blocked on the extension itself.
	DisableDevTools bool
	ForceSearch     bool
}

// Reach says which desktop users a wallpaper or dark mode change goes to:
// everyone signed in but those in Skip (user IDs already given it), and
// with SignedOut also the people who aren't signed in, so their desktop
// starts with it.
type Reach struct {
	Skip      []string
	SignedOut bool
}

// DesktopLook is what a circadian preview may change on the desktop, per
// user and "schema key", as gsettings prints it, so it can be put back
// exactly: wallpaper, colour scheme and themes.
type DesktopLook map[string]map[string]string

// PercentToRaw turns a brightness in percent into a level of max, never
// fully off: a black screen at night would look broken.
func PercentToRaw(percent float64, max int) int {
	return int(math.Max(1, math.Round(percent/100*float64(max))))
}

// RawToPercent turns a level of max into percent.
func RawToPercent(level, max int) float64 {
	if max <= 0 {
		return 0
	}
	return float64(level) * 100 / float64(max)
}

// ExternalScreen is a screen whose brightness DDC/CI sets.
type ExternalScreen struct {
	Display string // ddcutil's display number
	Name    string
}

// GammaReading is what the screens' gamma ramps show.
type GammaReading int

const (
	GammaUnknown  GammaReading = iota // no X session to read
	GammaOurs                         // the ramps KC9K would set now
	GammaIdentity                     // plain ramps: nothing changes the colours
	GammaOther                        // ramps someone else set (a night light, Redshift)
)

func (g GammaReading) String() string {
	return [...]string{"unknown", "ours", "identity", "other"}[g]
}

// RampFunc makes a ramp of a size: the circadian package's Ramp with the
// colour temperature and brightness wanted.
type RampFunc func(size int) (red, green, blue []uint16)

// TitleTargets are what the title guard looks for in one user's windows.
type TitleTargets struct {
	Close   []string // window title entries whose windows close
	Watch   []string // window title entries only reported (owner, 2026-11-16)
	Folders []string // folder entries (§3.1): file manager windows showing them close
}

func (t TitleTargets) Empty() bool {
	return len(t.Close) == 0 && len(t.Watch) == 0 && len(t.Folders) == 0
}

// TitlesByUser are the title guard's targets per user (§3.2 affected
// users): All for every account, Users for the accounts (by user ID) a
// block names.
type TitlesByUser struct {
	All   TitleTargets
	Users map[uint64]TitleTargets
}

// For returns the targets in the windows of the user uid: every account's
// and the user's own.
func (t TitlesByUser) For(uid uint64) TitleTargets {
	own, ok := t.Users[uid]
	if !ok {
		return t.All
	}
	return TitleTargets{
		Close:   append(append([]string{}, t.All.Close...), own.Close...),
		Watch:   append(append([]string{}, t.All.Watch...), own.Watch...),
		Folders: append(append([]string{}, t.All.Folders...), own.Folders...),
	}
}

// Empty reports whether nobody's windows are looked at.
func (t TitlesByUser) Empty() bool {
	if !t.All.Empty() {
		return false
	}
	for _, own := range t.Users {
		if !own.Empty() {
			return false
		}
	}
	return true
}

// TitleHit is a window the guard newly asked to close, or newly saw with a
// watched entry: the entry (or folder entry) and the user whose window it was.
type TitleHit struct {
	Keyword string
	UID     uint64
	Watched bool
}

// DefaultExtensionGrace is how long a browser may run without the extension
// checking in before ExtensionGuard closes it, unless the user chose
// otherwise (§4.10 tamper protection). It gives a fresh browser time to
// start and load its extensions.
const DefaultExtensionGrace = 60 * time.Second

// UserApp is an app in the foreground of a desktop user's session.
type UserApp struct{ User, App string }

// FirmwareSetting is one firmware setting the lock looks after.
type FirmwareSetting struct {
	Driver  string `json:"driver"`
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
	Current string `json:"current"`
	Want    string `json:"want"` // "" when no possible value fits
	Done    bool   `json:"done"`
}

// FirmwareStatus is what the firmware lets KC9K see.
type FirmwareStatus struct {
	Drivers       []string          `json:"drivers"`
	Password      bool              `json:"password"` // an admin (setup) password is set
	Settings      []FirmwareSetting `json:"settings"`
	PendingReboot bool              `json:"pending_reboot"`
}

// Available reports whether the computer's firmware can be changed from Linux.
func (s FirmwareStatus) Available() bool { return len(s.Drivers) > 0 }

// Locked reports whether the firmware lock is complete: a password, and every
// setting found as the lock wants it.
func (s FirmwareStatus) Locked() bool {
	if !s.Password || !s.Available() {
		return false
	}
	for _, set := range s.Settings {
		if set.Want != "" && !set.Done {
			return false
		}
	}
	return true
}

// Missing lists the purposes no setting was found for: they are done by hand
// in the firmware's own setup.
func (s FirmwareStatus) Missing() []string {
	var out []string
	for _, p := range firmwarePurposes {
		found := false
		for _, set := range s.Settings {
			if set.Purpose == p.Name && set.Want != "" {
				found = true
			}
		}
		if !found {
			out = append(out, p.Name)
		}
	}
	return out
}

// SupportedBrowser is one browser that runs the extension.
type SupportedBrowser struct {
	ID   string // librewolf, chrome, chromium
	Name string
	// Exes are the executables its package installs, as /proc/<pid>/exe
	// shows them for the main process.
	Exes []string
}

// FirmwarePurpose is what one firmware setting is changed for.
type FirmwarePurpose struct {
	Name string   // shown to the user, e.g. "USB boot"
	On   bool     // the lock wants it on (else off)
	Keys []string // matched against the setting's name, lower case, letters and digits only
	Skip []string // names containing one of these don't match
}

// firmwarePurposes are the settings the lock changes. Vendors name them
// differently (Lenovo USBBootSupport, HP "USB Storage Boot", Dell
// UefiNwStack), so a setting matches when its folded name has every word of
// one key.
var firmwarePurposes = []FirmwarePurpose{
	{Name: "USB boot", Keys: []string{"usb boot", "usbstorage boot", "removable boot", "external boot", "bootfromusb"}, Skip: []string{"order", "legacy"}},
	{Name: "network boot", Keys: []string{"pxe", "network boot", "netboot", "lanboot", "networkstack", "nwstack", "httpboot", "wakeonlanboot"}, Skip: []string{"order"}},
	{Name: "one-time boot menu", Keys: []string{"bootmenu", "f12", "bootdevicelist", "onetimeboot"}, Skip: []string{"timeout", "delay"}},
	{Name: "boot order lock", On: true, Keys: []string{"bootorderlock", "lockbootorder", "fixedbootorder"}},
}

// rampTolerance is how far a read ramp may be from the one expected: the
// driver may round what it was given to the hardware's precision.
const rampTolerance = 1024

func rampsMatch(want RampFunc, red, green, blue []uint16) bool {
	wr, wg, wb := want(len(red))
	for i := range red {
		for _, pair := range [][2]uint16{{wr[i], red[i]}, {wg[i], green[i]}, {wb[i], blue[i]}} {
			if math.Abs(float64(pair[0])-float64(pair[1])) > rampTolerance {
				return false
			}
		}
	}
	return true
}

func allZero(channels ...[]uint16) bool {
	for _, channel := range channels {
		for _, v := range channel {
			if v != 0 {
				return false
			}
		}
	}
	return true
}

func identityRamp(size int) (red, green, blue []uint16) {
	red = make([]uint16, size)
	for i := range red {
		if size > 1 {
			red[i] = uint16(math.Round(65535 * float64(i) / float64(size-1)))
		}
	}
	return red, append([]uint16{}, red...), append([]uint16{}, red...)
}
