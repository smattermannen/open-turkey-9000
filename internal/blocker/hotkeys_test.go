package blocker

import (
	"reflect"
	"testing"
)

func TestParseAccel(t *testing.T) {
	good := map[string]string{
		"ctrl+alt+s":       "<Primary><Alt>s",
		"Alt+Ctrl+S":       "<Primary><Alt>s",
		"super+F9":         "<Super>F9",
		"f9":               "F9",
		"shift+super+1":    "<Shift><Super>1",
		"ctrl+shift+Home":  "<Primary><Shift>Home",
		"ctrl+ctrl+k":      "<Primary>k",
		" ctrl + alt + b ": "<Primary><Alt>b",
	}
	for in, want := range good {
		got, err := ParseAccel(in)
		if err != nil || got != want {
			t.Errorf("ParseAccel(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "s", "ctrl+", "hyper+s", "ctrl+'s", "ctrl+a b", "ctrl+<Alt>s"} {
		if got, err := ParseAccel(bad); err == nil {
			t.Errorf("ParseAccel(%q) = %q; want an error", bad, got)
		}
	}
}

func TestHotkeyList(t *testing.T) {
	if got := parseStringArray("['custom0', '__dummy__', 'kc9k3']\n"); !reflect.DeepEqual(got, []string{"custom0", "__dummy__", "kc9k3"}) {
		t.Errorf("parse = %q", got)
	}
	if got := parseStringArray("@as []"); len(got) != 0 {
		t.Errorf("empty list parsed as %q", got)
	}
	if got := parseStringArray(""); len(got) != 0 {
		t.Errorf("unset list parsed as %q", got)
	}
	cinnamon, gnome := hotkeyDesktops[0], hotkeyDesktops[1]
	list, removed := mergeHotkeyList(cinnamon, []string{"custom0", "kc9k3", "kc9k5"}, []int{5, 7})
	if !reflect.DeepEqual(list, []string{"custom0", "kc9k5", "kc9k7"}) || !reflect.DeepEqual(removed, []string{"kc9k3"}) {
		t.Errorf("merge = %q, removed %q", list, removed)
	}
	// GNOME's list holds folder paths.
	dir := "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/"
	list, removed = mergeHotkeyList(gnome, []string{dir + "custom0/", dir + "kc9k3/"}, []int{7})
	if !reflect.DeepEqual(list, []string{dir + "custom0/", dir + "kc9k7/"}) || !reflect.DeepEqual(removed, []string{"kc9k3"}) {
		t.Errorf("GNOME merge = %q, removed %q", list, removed)
	}
	if got := stringArray(nil); got != "@as []" {
		t.Errorf("empty stringArray = %s", got)
	}
	if got := stringArray([]string{"a'b", `c\d`}); got != `['a\'b', 'c\\d']` {
		t.Errorf("stringArray = %s", got)
	}
}
