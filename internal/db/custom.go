package db

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CustomItem is one thing a custom action does (§3.9 Custom action,
// R-CUSTOM-1; owner 2026-11-17): open a site, file or folder, or run a
// program or command, as the signed-in user in their desktop session.
type CustomItem struct {
	Open string `json:"open,omitempty"` // xdg-open: an address, a file or a folder
	Run  string `json:"run,omitempty"`  // sh -c: a program or a command
}

// MaxCustomItems bounds what one custom action does.
const MaxCustomItems = 20

func (c CustomItem) String() string {
	if c.Open != "" {
		return "open " + c.Open
	}
	return "run " + c.Run
}

// ParseCustomItem reads "open TARGET" or "run COMMAND".
func ParseCustomItem(text string) (CustomItem, error) {
	verb, rest, _ := strings.Cut(strings.TrimSpace(text), " ")
	rest = strings.TrimSpace(rest)
	if rest == "" || strings.ContainsAny(rest, "\x00\n\r") || len(rest) > 2000 {
		return CustomItem{}, fmt.Errorf("invalid custom action %q: write \"open ADDRESS-FILE-OR-FOLDER\" or \"run COMMAND\"", text)
	}
	switch strings.ToLower(verb) {
	case "open":
		return CustomItem{Open: rest}, nil
	case "run":
		return CustomItem{Run: rest}, nil
	}
	return CustomItem{}, fmt.Errorf("invalid custom action %q: it starts with open or run", text)
}

// CustomParam is how a custom action's items are kept: as a JSON list.
func CustomParam(items []CustomItem) string {
	raw, _ := json.Marshal(items)
	return string(raw)
}

// CustomItems reads a custom action's items.
func CustomItems(param string) ([]CustomItem, error) {
	var items []CustomItem
	if err := json.Unmarshal([]byte(param), &items); err != nil {
		return nil, fmt.Errorf("unreadable custom action")
	}
	return items, nil
}

// CheckCustomParam checks a custom action's items.
func CheckCustomParam(param string) error {
	items, err := CustomItems(param)
	if err != nil {
		return err
	}
	if len(items) == 0 || len(items) > MaxCustomItems {
		return fmt.Errorf("a custom action does 1 to %d things", MaxCustomItems)
	}
	for _, it := range items {
		if (it.Open == "") == (it.Run == "") {
			return fmt.Errorf("each thing a custom action does is open or run")
		}
		if _, err := ParseCustomItem(it.String()); err != nil {
			return err
		}
	}
	return nil
}

// DescribeCustom says what a custom action does, briefly.
func DescribeCustom(param string) string {
	items, err := CustomItems(param)
	if err != nil {
		return "custom action"
	}
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = it.String()
	}
	return strings.Join(parts, "; ")
}

// CheckModeParam checks a Mode action's mode name (whether the mode exists
// is checked when it runs: a mode may be renamed or deleted meanwhile).
func CheckModeParam(name string) error {
	if strings.TrimSpace(name) == "" || len(name) > MaxListName || strings.ContainsAny(name, "\x00\n\r") {
		return fmt.Errorf("a Mode action names a mode")
	}
	return nil
}
