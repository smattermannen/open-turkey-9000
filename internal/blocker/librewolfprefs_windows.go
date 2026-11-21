package blocker

import (
	"net/url"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/smattermannen/kc9k/internal/platform"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// LibreWolfOverridesDir holds the cleaned overrides files, one per user;
// in the program folder, which LibreWolf (running as the user) may read
// and only SYSTEM may change.
var LibreWolfOverridesDir = filepath.Join(platform.ShareDir, "librewolf")

// overridesLines point LibreWolf at the user's cleaned overrides: the
// file named after the account, URL-escaped (account names may hold
// spaces).
func overridesLines() []string {
	base := "file:///" + strings.ReplaceAll(filepath.ToSlash(LibreWolfOverridesDir), " ", "%20") + "/"
	return []string{
		`let kc9kUser = getenv("USERNAME") || "";`,
		`lockPref("autoadmin.global_config_url", /^[A-Za-z0-9 ._-]{1,64}$/.test(kc9kUser) ? "` + base + `" + encodeURIComponent(kc9kUser) + ".cfg" : "");`,
	}
}

// overridesName is a user's cleaned overrides file's name, as the URL
// above names it once unescaped.
func overridesName(name string) string {
	return strings.ReplaceAll(url.PathEscape(name), "%20", " ") + ".cfg"
}

// librewolfUsers are the computer's people: every profile of a local or
// Microsoft account (ProfileList), with its home folder.
func librewolfUsers() []*user.User {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList`, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil
	}
	defer k.Close()
	sids, _ := k.ReadSubKeyNames(0)
	var out []*user.User
	for _, s := range sids {
		if !strings.HasPrefix(s, "S-1-5-21-") {
			continue // service accounts
		}
		sub, err := registry.OpenKey(k, s, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		home, _, err := sub.GetStringValue("ProfileImagePath")
		sub.Close()
		if err != nil {
			continue
		}
		sid, err := windows.StringToSid(s)
		if err != nil {
			continue
		}
		out = append(out, &user.User{Uid: s, Username: platform.AccountName(sid), HomeDir: home})
	}
	return out
}
