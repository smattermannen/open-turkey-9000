package circadian

import "strings"

// Desktop themes come as separate light and dark themes (Mint-Y-Aqua and
// Mint-Y-Dark-Aqua, Adwaita and Adwaita-dark). These find one from the
// other; exists says whether a theme of that name is installed.

// IsDarkTheme reports whether a theme's name marks it as dark.
func IsDarkTheme(name string) bool {
	for _, part := range strings.Split(name, "-") {
		if strings.EqualFold(part, "dark") || strings.EqualFold(part, "darker") {
			return true
		}
	}
	return false
}

// DarkTheme is the dark theme that goes with name, or "" when none is
// installed. A dark theme is its own.
func DarkTheme(name string, exists func(string) bool) string {
	if name == "" {
		return ""
	}
	if IsDarkTheme(name) {
		return name
	}
	parts := strings.Split(name, "-")
	for _, word := range []string{"Dark", "dark"} {
		// Mint-Y-Aqua: try Mint-Y-Aqua-Dark, then Mint-Y-Dark-Aqua, …
		for i := len(parts); i >= 1; i-- {
			candidate := strings.Join(append(append(append([]string{}, parts[:i]...), word), parts[i:]...), "-")
			if exists(candidate) {
				return candidate
			}
		}
	}
	return ""
}

// LightTheme is the light theme that goes with name, or "" when none is
// installed. A light theme is its own.
func LightTheme(name string, exists func(string) bool) string {
	if name == "" || !IsDarkTheme(name) {
		return name
	}
	var kept []string
	for _, part := range strings.Split(name, "-") {
		if !strings.EqualFold(part, "dark") && !strings.EqualFold(part, "darker") {
			kept = append(kept, part)
		}
	}
	candidate := strings.Join(kept, "-")
	if candidate != "" && exists(candidate) {
		return candidate
	}
	return ""
}
