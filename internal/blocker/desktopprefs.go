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

// SetWallpaper shows the picture at path as every desktop user's wallpaper.
// It reports how many users it reached.
func SetWallpaper(path string) (int, error) {
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	reached := 0
	var firstErr error
	for _, u := range desktopPrefsUsers() {
		done := false
		for _, s := range []struct{ schema, key string }{
			{"org.cinnamon.desktop.background", "picture-uri"},
			{"org.gnome.desktop.background", "picture-uri"},
			{"org.gnome.desktop.background", "picture-uri-dark"},
			{"org.mate.background", "picture-filename"},
		} {
			if !hasKey(u, s.schema, s.key) {
				continue
			}
			value := uri
			if s.key == "picture-filename" {
				value = path
			}
			if _, err := gsettings(u, "set", s.schema, s.key, gvariantString(value)); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			done = true
		}
		if done {
			reached++
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
// of the desktop itself. It reports how many users it reached.
func SetDarkMode(dark bool) (int, error) {
	reached := 0
	var firstErr error
	for _, u := range desktopPrefsUsers() {
		changed := false
		note := func(err error) {
			if err == nil {
				changed = true
			} else if firstErr == nil {
				firstErr = err
			}
		}
		scheme := map[bool]string{true: "prefer-dark", false: "prefer-light"}[dark]
		if hasKey(u, "org.x.apps.portal", "color-scheme") {
			_, err := gsettings(u, "set", "org.x.apps.portal", "color-scheme", gvariantString(scheme))
			note(err)
		}
		if hasKey(u, "org.gnome.desktop.interface", "color-scheme") {
			gnome := map[bool]string{true: "prefer-dark", false: "default"}[dark]
			_, err := gsettings(u, "set", "org.gnome.desktop.interface", "color-scheme", gvariantString(gnome))
			note(err)
		}
		for _, theme := range []struct{ schema, key, part string }{
			{"org.cinnamon.desktop.interface", "gtk-theme", "gtk-3.0"},
			{"org.gnome.desktop.interface", "gtk-theme", "gtk-3.0"},
			{"org.cinnamon.theme", "name", "cinnamon"},
		} {
			current, err := gsettings(u, "get", theme.schema, theme.key)
			if err != nil {
				continue
			}
			current = unquote(current)
			var want string
			if dark {
				want = circadian.DarkTheme(current, themeInstalled(u, theme.part))
			} else {
				want = circadian.LightTheme(current, themeInstalled(u, theme.part))
			}
			if want == "" || want == current {
				continue
			}
			_, err = gsettings(u, "set", theme.schema, theme.key, gvariantString(want))
			note(err)
		}
		if changed {
			reached++
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
