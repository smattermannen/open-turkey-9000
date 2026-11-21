package blocker

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
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
