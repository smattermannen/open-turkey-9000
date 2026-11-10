// actions.go — what a block does to the websites on its list.
//
// Each block has three boxes to tick, in any combination: block the sites,
// show them in greyscale, and block their images. Apps are always blocked.
// Ticking a box makes the block stricter; unticking one loosens it, so the
// CLI gates it like any other loosening edit.
package db

import (
	"fmt"
	"strings"
)

// Website actions, as named on the command line and in exports.
const (
	ActionBlock     = "block"
	ActionGreyscale = "greyscale"
	ActionNoImages  = "no-images"
)

// ActionNames lists the website actions in display order.
var ActionNames = []string{ActionBlock, ActionGreyscale, ActionNoImages}

// Actions holds which website actions a block uses. The zero value has none;
// DefaultActions is what a new block gets.
type Actions struct {
	Block     bool
	Greyscale bool
	NoImages  bool
}

// DefaultActions only blocks, as every block did before actions existed.
var DefaultActions = Actions{Block: true}

// IsAction reports whether name is a website action.
func IsAction(name string) bool {
	for _, a := range ActionNames {
		if a == name {
			return true
		}
	}
	return false
}

// Has reports whether the named action is on.
func (a Actions) Has(name string) bool {
	switch name {
	case ActionBlock:
		return a.Block
	case ActionGreyscale:
		return a.Greyscale
	case ActionNoImages:
		return a.NoImages
	}
	return false
}

// With returns a copy with the named action turned on or off.
func (a Actions) With(name string, on bool) Actions {
	switch name {
	case ActionBlock:
		a.Block = on
	case ActionGreyscale:
		a.Greyscale = on
	case ActionNoImages:
		a.NoImages = on
	}
	return a
}

// Any reports whether at least one action is on.
func (a Actions) Any() bool { return a.Block || a.Greyscale || a.NoImages }

// List returns the names of the actions that are on, in display order.
func (a Actions) List() []string {
	out := []string{}
	for _, name := range ActionNames {
		if a.Has(name) {
			out = append(out, name)
		}
	}
	return out
}

// String describes the actions, e.g. "block, greyscale".
func (a Actions) String() string { return strings.Join(a.List(), ", ") }

// ParseActions turns a list of action names into Actions. An empty list gives
// DefaultActions, so exports made before actions existed import unchanged.
func ParseActions(names []string) (Actions, error) {
	if len(names) == 0 {
		return DefaultActions, nil
	}
	var a Actions
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if !IsAction(name) {
			return a, fmt.Errorf("unknown action %q (use %s)", name, strings.Join(ActionNames, ", "))
		}
		a = a.With(name, true)
	}
	return a, nil
}
