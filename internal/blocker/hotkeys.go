// hotkeys.go — KC9K's key combinations as custom keyboard shortcuts of
// Cinnamon and GNOME. Both keep them in dconf: a list, and for each entry a
// folder with its name, command and binding. Cinnamon's list holds names
// (under /org/cinnamon/desktop/keybindings/custom-list) and a binding is an
// array; GNOME's holds folder paths (media-keys/custom-keybindings) and a
// binding is a string. KC9K's entries are named kc9k<id>, so syncing
// replaces only those and leaves the user's own shortcuts alone. Both are
// written in every session: a desktop ignores the other's keys, and
// hotkeys then survive switching desktops.
package blocker

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const hotkeyPrefix = "kc9k"

// hotkeyDesktop is where one desktop keeps custom shortcuts.
type hotkeyDesktop struct {
	name      string
	list      string // the dconf key listing the shortcuts
	folder    string // the folder holding one folder per shortcut
	listPaths bool   // the list holds folder paths rather than names
	arrayKey  bool   // a binding is an array of accelerators
}

var hotkeyDesktops = []hotkeyDesktop{
	{name: "Cinnamon", list: "/org/cinnamon/desktop/keybindings/custom-list",
		folder: "/org/cinnamon/desktop/keybindings/custom-keybindings/", arrayKey: true},
	{name: "GNOME", list: "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings",
		folder: "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/", listPaths: true},
}

// entry is how the desktop's list names a shortcut.
func (d hotkeyDesktop) entry(name string) string {
	if d.listPaths {
		return d.folder + name + "/"
	}
	return name
}

// nameOf is the shortcut's name for a list entry.
func (d hotkeyDesktop) nameOf(entry string) string {
	if d.listPaths {
		return strings.TrimSuffix(strings.TrimPrefix(entry, d.folder), "/")
	}
	return entry
}

// DesktopHotkey is one shortcut to put on the desktop.
type DesktopHotkey struct {
	ID      int
	Accel   string // GTK accelerator, e.g. <Primary><Alt>s
	Name    string // shown in Cinnamon's keyboard settings
	Command string
}

var modifierNames = map[string]string{
	"ctrl": "<Primary>", "control": "<Primary>", "primary": "<Primary>",
	"alt": "<Alt>", "shift": "<Shift>",
	"super": "<Super>", "win": "<Super>", "meta": "<Super>",
}

var keyName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
var functionKey = regexp.MustCompile(`^F([1-9]|1[0-9]|2[0-4])$`)

// ParseAccel turns "ctrl+alt+s" (or "Super+F9") into a GTK accelerator
// ("<Primary><Alt>s"). A key needs at least one modifier, except the
// function keys F1 to F24, so a hotkey doesn't swallow ordinary typing.
func ParseAccel(text string) (string, error) {
	parts := strings.Split(strings.TrimSpace(text), "+")
	if len(parts) == 0 || strings.TrimSpace(parts[len(parts)-1]) == "" {
		return "", fmt.Errorf("give keys like ctrl+alt+s or super+F9")
	}
	var mods []string
	seen := map[string]bool{}
	for _, p := range parts[:len(parts)-1] {
		m, ok := modifierNames[strings.ToLower(strings.TrimSpace(p))]
		if !ok {
			return "", fmt.Errorf("unknown modifier %q (use ctrl, alt, shift or super)", strings.TrimSpace(p))
		}
		if !seen[m] {
			seen[m] = true
			mods = append(mods, m)
		}
	}
	sort.SliceStable(mods, func(i, j int) bool { return modifierOrder(mods[i]) < modifierOrder(mods[j]) })
	key := strings.TrimSpace(parts[len(parts)-1])
	if !keyName.MatchString(key) {
		return "", fmt.Errorf("invalid key %q", key)
	}
	if len(key) == 1 {
		key = strings.ToLower(key)
	} else if upper := strings.ToUpper(key); functionKey.MatchString(upper) {
		key = upper
	}
	if len(mods) == 0 && !functionKey.MatchString(key) {
		return "", fmt.Errorf("a hotkey needs ctrl, alt, shift or super (only F1 to F24 work alone)")
	}
	return strings.Join(mods, "") + key, nil
}

func modifierOrder(m string) int {
	return strings.Index("<Primary><Alt><Shift><Super>", m)
}

// gvariantString quotes s as a GVariant string for dconf write.
func gvariantString(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// parseStringArray reads a GVariant string array as dconf prints it:
// ['a', 'b'], @as [] or nothing.
func parseStringArray(text string) []string {
	text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "@as"))
	text = strings.TrimSuffix(strings.TrimPrefix(text, "["), "]")
	var out []string
	for _, item := range strings.Split(text, ",") {
		item = strings.TrimSpace(item)
		if len(item) >= 2 && (item[0] == '\'' || item[0] == '"') && item[len(item)-1] == item[0] {
			out = append(out, item[1:len(item)-1])
		}
	}
	return out
}

// stringArray writes items as a GVariant string array. An empty one needs
// its type spelled out, or dconf can't tell what "[]" is.
func stringArray(items []string) string {
	if len(items) == 0 {
		return "@as []"
	}
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = gvariantString(s)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// mergeHotkeyList returns a desktop's custom list with KC9K's old entries
// replaced by these IDs, and the names of the KC9K entries that are gone.
func mergeHotkeyList(d hotkeyDesktop, current []string, ids []int) (list, removed []string) {
	want := map[string]bool{}
	for _, id := range ids {
		want[hotkeyPrefix+strconv.Itoa(id)] = true
	}
	for _, entry := range current {
		if name := d.nameOf(entry); strings.HasPrefix(name, hotkeyPrefix) {
			if !want[name] {
				removed = append(removed, name)
			}
			continue
		}
		list = append(list, entry)
	}
	for _, id := range ids {
		list = append(list, d.entry(hotkeyPrefix+strconv.Itoa(id)))
	}
	return list, removed
}

// SyncHotkeys makes KC9K's shortcuts in every desktop session match keys.
// It reports the sessions it couldn't update.
func SyncHotkeys(keys []DesktopHotkey) error {
	dconf, err := exec.LookPath("dconf")
	if err != nil {
		return fmt.Errorf("dconf isn't installed, so hotkeys can't be set up")
	}
	users := desktopUsers()
	if len(users) == 0 {
		return fmt.Errorf("no desktop session to set hotkeys up in; they are added the next time you change one")
	}
	ids := make([]int, len(keys))
	for i, k := range keys {
		ids[i] = k.ID
	}
	var failures []string
	for _, u := range users {
		ok := false
		for _, d := range hotkeyDesktops {
			out, err := asUser(u, dconf, "read", d.list).Output()
			if err != nil {
				continue
			}
			list, removed := mergeHotkeyList(d, parseStringArray(string(out)), ids)
			run := func(args ...string) {
				if err == nil {
					err = asUser(u, dconf, args...).Run()
				}
			}
			for _, name := range removed {
				run("reset", "-f", d.folder+name+"/")
			}
			for _, k := range keys {
				folder := d.folder + hotkeyPrefix + strconv.Itoa(k.ID) + "/"
				binding := gvariantString(k.Accel)
				if d.arrayKey {
					binding = stringArray([]string{k.Accel})
				}
				run("write", folder+"name", gvariantString(k.Name))
				run("write", folder+"command", gvariantString(k.Command))
				run("write", folder+"binding", binding)
			}
			// The list goes last: the desktop picks the shortcuts up when it changes.
			run("write", d.list, stringArray(list))
			ok = ok || err == nil
		}
		if !ok {
			failures = append(failures, u.Username)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("couldn't set hotkeys up for %s", strings.Join(failures, ", "))
	}
	return nil
}
