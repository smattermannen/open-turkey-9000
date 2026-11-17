// desktopprefs.go — the desktop settings the circadian rhythm changes, in
// each desktop user's own settings (gsettings, run as that user): the
// wallpaper, dark mode, and the desktop's own night light, which would
// fight KC9K's colour temperature.
package blocker

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/circadian"
)

// gsettingsTimeout bounds one gsettings run.
const gsettingsTimeout = 5 * time.Second

// desktopPrefsUsers is the desktop users to change (replaceable in tests).
var desktopPrefsUsers = desktopUsers

// gsettings runs gsettings as u, in u's session.
func gsettings(u *user.User, args ...string) (string, error) {
	path, err := exec.LookPath("gsettings")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), gsettingsTimeout)
	defer cancel()
	session := asUser(u, path)
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = append(session.Env, "HOME="+u.HomeDir, "XDG_RUNTIME_DIR=/run/user/"+u.Uid)
	cmd.SysProcAttr = session.SysProcAttr
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// hasKey reports whether u's settings have schema with key.
func hasKey(u *user.User, schema, key string) bool {
	out, err := gsettings(u, "list-keys", schema)
	if err != nil {
		return false
	}
	for _, k := range strings.Fields(out) {
		if k == key {
			return true
		}
	}
	return false
}

// unquote reads a GVariant string as gsettings prints it.
func unquote(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		value = value[1 : len(value)-1]
		value = strings.NewReplacer(`\'`, `'`, `\\`, `\`).Replace(value)
	}
	return value
}

// signedOutPrefsUsers are the people with desktop settings who aren't
// signed in now (replaceable in tests).
var signedOutPrefsUsers = signedOutUsers

// signedOutUsers returns the regular users without a session bus who have
// a desktop settings database: their desktop reads it when they sign in.
func signedOutUsers() []*user.User {
	in := map[string]bool{}
	for _, u := range desktopPrefsUsers() {
		in[u.Uid] = true
	}
	var out []*user.User
	for _, name := range RegularUsers() {
		u, err := user.Lookup(name)
		if err != nil || in[u.Uid] {
			continue
		}
		if info, err := os.Stat(filepath.Join(u.HomeDir, ".config", "dconf", "user")); err != nil || !info.Mode().IsRegular() {
			continue
		}
		out = append(out, u)
	}
	return out
}

// gsettingsSignedOut runs gsettings as u, who isn't signed in, on a
// private session bus of its own (dbus-run-session), so a change reaches
// u's settings database before u's desktop starts and reads it.
func gsettingsSignedOut(u *user.User, args ...string) (string, error) {
	runner, err := exec.LookPath("dbus-run-session")
	if err != nil {
		return "", err
	}
	path, err := exec.LookPath("gsettings")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), gsettingsTimeout)
	defer cancel()
	session := asUser(u, path)
	cmd := exec.CommandContext(ctx, runner, append([]string{"--", path}, args...)...)
	cmd.Env = []string{"HOME=" + u.HomeDir, "USER=" + u.Username, "LOGNAME=" + u.Username, "PATH=/usr/bin:/bin"}
	cmd.Dir = u.HomeDir
	cmd.SysProcAttr = session.SysProcAttr
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// prefsTarget is one user a desktop change goes to, signed in or not.
type prefsTarget struct {
	*user.User
	signedOut bool
}

func (t prefsTarget) run(args ...string) (string, error) {
	if t.signedOut {
		return gsettingsSignedOut(t.User, args...)
	}
	return gsettings(t.User, args...)
}

func (t prefsTarget) has(schema, key string) bool { return hasKey(t.User, schema, key) }

// Reach says which desktop users a wallpaper or dark mode change goes to:
// everyone signed in but those in Skip (user IDs already given it), and
// with SignedOut also the people who aren't signed in, so their desktop
// starts with it.
type Reach struct {
	Skip      []string
	SignedOut bool
}

func (r Reach) targets() []prefsTarget {
	skip := map[string]bool{}
	for _, uid := range r.Skip {
		skip[uid] = true
	}
	var out []prefsTarget
	for _, u := range desktopPrefsUsers() {
		if !skip[u.Uid] {
			out = append(out, prefsTarget{User: u})
		}
	}
	if r.SignedOut {
		for _, u := range signedOutPrefsUsers() {
			if !skip[u.Uid] {
				out = append(out, prefsTarget{User: u, signedOut: true})
			}
		}
	}
	return out
}

// SignedInUserIDs returns the user IDs of everyone signed in to a desktop.
func SignedInUserIDs() []string {
	var out []string
	for _, u := range desktopPrefsUsers() {
		out = append(out, u.Uid)
	}
	return out
}

// SetWallpaper shows the picture at path as the wallpaper of the desktop
// users r reaches. It returns the user IDs it reached.
func SetWallpaper(path string, r Reach) ([]string, error) {
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	var reached []string
	var firstErr error
	for _, u := range r.targets() {
		done := false
		for _, s := range []struct{ schema, key string }{
			{"org.cinnamon.desktop.background", "picture-uri"},
			{"org.gnome.desktop.background", "picture-uri"},
			{"org.gnome.desktop.background", "picture-uri-dark"},
			{"org.mate.background", "picture-filename"},
		} {
			if !u.has(s.schema, s.key) {
				continue
			}
			value := uri
			if s.key == "picture-filename" {
				value = path
			}
			if _, err := u.run("set", s.schema, s.key, gvariantString(value)); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			done = true
		}
		if done {
			reached = append(reached, u.Uid)
		}
	}
	return reached, firstErr
}

// themeDirs are where GTK and Cinnamon themes live for u.
func themeDirs(u *user.User) []string {
	return []string{"/usr/share/themes", filepath.Join(u.HomeDir, ".themes"),
		filepath.Join(u.HomeDir, ".local/share/themes")}
}

// themeInstalled reports whether a theme with part (a subdirectory such as
// gtk-3.0 or cinnamon) is installed for u.
func themeInstalled(u *user.User, part string) func(string) bool {
	return func(name string) bool {
		if name == "Adwaita" || name == "Adwaita-dark" {
			if part == "gtk-3.0" {
				return true // built into GTK
			}
		}
		if strings.ContainsAny(name, "/\x00") || name == "." || name == ".." {
			return false
		}
		for _, dir := range themeDirs(u) {
			if info, err := os.Stat(filepath.Join(dir, name, part)); err == nil && info.IsDir() {
				return true
			}
		}
		return false
	}
}

// SetDarkMode turns every desktop user's dark mode on or off: the colour
// scheme apps read (prefer-dark) and, where the light and dark looks are
// separate themes (Mint-Y-Aqua and Mint-Y-Dark-Aqua), the theme of apps and
// of the desktop itself, for the desktop users r reaches. It returns the
// user IDs it reached.
func SetDarkMode(dark bool, r Reach) ([]string, error) {
	var reached []string
	var firstErr error
	for _, u := range r.targets() {
		changed := false
		note := func(err error) {
			if err == nil {
				changed = true
			} else if firstErr == nil {
				firstErr = err
			}
		}
		scheme := map[bool]string{true: "prefer-dark", false: "prefer-light"}[dark]
		if u.has("org.x.apps.portal", "color-scheme") {
			_, err := u.run("set", "org.x.apps.portal", "color-scheme", gvariantString(scheme))
			note(err)
		}
		if u.has("org.gnome.desktop.interface", "color-scheme") {
			gnome := map[bool]string{true: "prefer-dark", false: "default"}[dark]
			_, err := u.run("set", "org.gnome.desktop.interface", "color-scheme", gvariantString(gnome))
			note(err)
		}
		for _, theme := range []struct{ schema, key, part string }{
			{"org.cinnamon.desktop.interface", "gtk-theme", "gtk-3.0"},
			{"org.gnome.desktop.interface", "gtk-theme", "gtk-3.0"},
			{"org.cinnamon.theme", "name", "cinnamon"},
		} {
			current, err := u.run("get", theme.schema, theme.key)
			if err != nil {
				continue
			}
			current = unquote(current)
			var want string
			if dark {
				want = circadian.DarkTheme(current, themeInstalled(u.User, theme.part))
			} else {
				want = circadian.LightTheme(current, themeInstalled(u.User, theme.part))
			}
			if want == "" || want == current {
				continue
			}
			_, err = u.run("set", theme.schema, theme.key, gvariantString(want))
			note(err)
		}
		if changed {
			reached = append(reached, u.Uid)
		}
	}
	return reached, firstErr
}

// nightLightKeys are the desktops' own night light switches.
var nightLightKeys = []string{"org.cinnamon.settings-daemon.plugins.color", "org.gnome.settings-daemon.plugins.color"}

// NightLightOn reports whether a desktop user has the desktop's own night
// light on. It changes the same gamma ramps as KC9K's colour temperature,
// and the two would take turns.
func NightLightOn() bool {
	for _, u := range desktopPrefsUsers() {
		for _, schema := range nightLightKeys {
			if out, err := gsettings(u, "get", schema, "night-light-enabled"); err == nil && out == "true" {
				return true
			}
		}
	}
	return false
}

// TurnOffNightLight switches the desktop's own night light off for every
// desktop user, so KC9K's colour temperature has the screen to itself.
func TurnOffNightLight() error {
	var firstErr error
	for _, u := range desktopPrefsUsers() {
		for _, schema := range nightLightKeys {
			if !hasKey(u, schema, "night-light-enabled") {
				continue
			}
			if _, err := gsettings(u, "set", schema, "night-light-enabled", "false"); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// DesktopLook is what a circadian preview may change on the desktop, per
// user and "schema key", as gsettings prints it, so it can be put back
// exactly: wallpaper, colour scheme and themes.
type DesktopLook map[string]map[string]string

// lookKeys are the keys a DesktopLook holds.
var lookKeys = []struct{ schema, key string }{
	{"org.cinnamon.desktop.background", "picture-uri"},
	{"org.gnome.desktop.background", "picture-uri"},
	{"org.gnome.desktop.background", "picture-uri-dark"},
	{"org.mate.background", "picture-filename"},
	{"org.x.apps.portal", "color-scheme"},
	{"org.gnome.desktop.interface", "color-scheme"},
	{"org.cinnamon.desktop.interface", "gtk-theme"},
	{"org.gnome.desktop.interface", "gtk-theme"},
	{"org.cinnamon.theme", "name"},
}

// ReadDesktopLook reads every desktop user's DesktopLook.
func ReadDesktopLook() DesktopLook {
	look := DesktopLook{}
	for _, u := range desktopPrefsUsers() {
		values := map[string]string{}
		for _, k := range lookKeys {
			if !hasKey(u, k.schema, k.key) {
				continue
			}
			if value, err := gsettings(u, "get", k.schema, k.key); err == nil {
				values[k.schema+" "+k.key] = value
			}
		}
		look[u.Username] = values
	}
	return look
}

// RestoreDesktopLook puts back what ReadDesktopLook read, where it differs
// now.
func RestoreDesktopLook(look DesktopLook) error {
	var firstErr error
	for _, u := range desktopPrefsUsers() {
		for key, value := range look[u.Username] {
			schema, name, _ := strings.Cut(key, " ")
			if now, err := gsettings(u, "get", schema, name); err == nil && now == value {
				continue
			}
			if _, err := gsettings(u, "set", schema, name, value); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}
