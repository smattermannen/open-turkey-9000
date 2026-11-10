// options.go — switches for how Killer Chicken 9000 behaves, as opposed to
// what it blocks. They lock nothing and are never held; each is stored as
// "on" or "off" in the setting "option.<name>".
package enforce

import (
	"fmt"
	"log"

	"github.com/smattermannen/kc9k/internal/db"
)

// Option is one switch.
type Option struct {
	Name    string
	Default bool
	Title   string // for the interface
	Detail  string
}

const (
	OptionPoliteClose = "polite-close"
	OptionPageDetails = "block-page-details"
)

// Options lists the switches in display order.
var Options = []Option{
	{OptionPoliteClose, true, "Ask blocked apps to close first",
		"A blocked app with a window is asked to close, so it can offer to save, and is killed a few seconds later if it is still open. Off: killed at once."},
	{OptionPageDetails, true, "Say why on the block page",
		"The block page names the block and list that stopped the page, and when the block ends. It changes nothing about what a list does: redirects, pictures and closed tabs work as before."},
}

// FindOption returns the option with that name.
func FindOption(name string) (Option, bool) {
	for _, o := range Options {
		if o.Name == name {
			return o, true
		}
	}
	return Option{}, false
}

// OptionOn reports whether an option is on.
func OptionOn(d *db.DB, name string) bool {
	o, ok := FindOption(name)
	if !ok {
		return false
	}
	value, err := d.GetSetting("option." + name)
	if err != nil {
		log.Printf("options: %v", err)
	}
	if value == "" {
		return o.Default
	}
	return value == "on"
}

// SetOption turns an option on or off.
func SetOption(d *db.DB, name string, on bool) error {
	if _, ok := FindOption(name); !ok {
		return fmt.Errorf("unknown option %q", name)
	}
	value := "off"
	if on {
		value = "on"
	}
	return d.SetSetting("option."+name, value)
}
