// hotkeys.go — KC9K's key combinations as Cinnamon custom keyboard
// shortcuts. Cinnamon keeps them in dconf: a list of names under
// /org/cinnamon/desktop/keybindings/custom-list, and for each name a folder
// with its name, command and binding. KC9K's entries are named kc9k<id>, so
// syncing replaces only those and leaves the user's own shortcuts alone.
package blocker

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	cinnamonKeysDir   = "/org/cinnamon/desktop/keybindings/"
	cinnamonKeyList   = cinnamonKeysDir + "custom-list"
	cinnamonKeyFolder = cinnamonKeysDir + "custom-keybindings/"
	hotkeyPrefix      = "kc9k"
)

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

func stringArray(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = gvariantString(s)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// mergeHotkeyList returns Cinnamon's custom list with KC9K's old entries
// replaced by these IDs, and the KC9K entries that are gone.
func mergeHotkeyList(current []string, ids []int) (list, removed []string) {
	want := map[string]bool{}
	for _, id := range ids {
		want[hotkeyPrefix+strconv.Itoa(id)] = true
	}
	for _, name := range current {
		if strings.HasPrefix(name, hotkeyPrefix) {
			if !want[name] {
				removed = append(removed, name)
			}
			continue
		}
		list = append(list, name)
	}
	for _, id := range ids {
		list = append(list, hotkeyPrefix+strconv.Itoa(id))
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
		out, err := asUser(u, dconf, "read", cinnamonKeyList).Output()
		if err != nil {
			failures = append(failures, u.Username)
			continue
		}
		list, removed := mergeHotkeyList(parseStringArray(string(out)), ids)
		run := func(args ...string) {
			if err == nil {
				err = asUser(u, dconf, args...).Run()
			}
		}
		for _, name := range removed {
			run("reset", "-f", cinnamonKeyFolder+name+"/")
		}
		for _, k := range keys {
			folder := cinnamonKeyFolder + hotkeyPrefix + strconv.Itoa(k.ID) + "/"
			run("write", folder+"name", gvariantString(k.Name))
			run("write", folder+"command", gvariantString(k.Command))
			run("write", folder+"binding", stringArray([]string{k.Accel}))
		}
		// The list goes last: Cinnamon picks the shortcuts up when it changes.
		run("write", cinnamonKeyList, stringArray(list))
		if err != nil {
			failures = append(failures, u.Username)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("couldn't set hotkeys up for %s", strings.Join(failures, ", "))
	}
	return nil
}
