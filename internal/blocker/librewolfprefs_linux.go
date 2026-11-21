//go:build linux

package blocker

import "os/user"

// LibreWolfOverridesDir holds the cleaned overrides files, one per user.
var LibreWolfOverridesDir = "/var/lib/kc9k/librewolf"

// overridesLines point LibreWolf at the user's cleaned overrides.
func overridesLines() []string {
	return []string{
		`let kc9kUser = getenv("USER") || "";`,
		`lockPref("autoadmin.global_config_url", /^[a-z_][a-z0-9_-]{0,31}$/.test(kc9kUser) ? "file://` + LibreWolfOverridesDir + `/" + kc9kUser + ".cfg" : "");`,
	}
}

// overridesName is a user's cleaned overrides file's name.
func overridesName(user string) string { return user + ".cfg" }

// librewolfUsers are the users whose overrides are cleaned: those signed
// in to a desktop.
func librewolfUsers() []*user.User { return desktopUsers() }
