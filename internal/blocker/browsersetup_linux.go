//go:build linux

package blocker

// otherPolicyDirs are installation folders that read a distribution policy
// file. The first target list is written always; these only if installed.
var (
	alwaysPolicyFiles = []string{firefoxPolicyPath, "/etc/librewolf/policies/policies.json"}
	installDirs       = []string{"/usr/lib/librewolf", "/usr/share/librewolf", "/usr/lib/firefox", "/usr/lib/firefox-esr"}
)

// otherBrowserNames names browsers besides LibreWolf, for BrowserNames.
var otherBrowserNames = []string{
	"firefox", "firefox-bin", "firefox-esr", "chrome", "google-chrome", "chromium", "chromium-browser",
	"brave", "brave-browser", "opera", "vivaldi-bin", "msedge", "microsoft-edge", "epiphany",
	"falkon", "konqueror", "midori", "qutebrowser", "palemoon", "waterfox", "floorp", "zen", "tor", "torbrowser",
}

var librewolfConfigs = []string{"/usr/share/librewolf/librewolf.cfg", "/usr/lib/librewolf/librewolf.cfg"}

// firefoxInstallDirs are the Firefox-family installation folders, written
// only where they exist.
func firefoxInstallDirs() []string { return installDirs }
