// actions.go — what a blacklist does to the pages it matches.
//
// Each blacklist ticks any of its actions. Page actions change or replace the
// page: block it (the block page), grey it, hide its images, send it to
// another address, show an image instead, or close the tab. Device actions
// close the browser and, for some, go further: log out, shut down, or start
// monk mode. Ticking a box makes the block stricter; unticking one loosens
// it, so the CLI gates it like any other loosening edit.
package db

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Website actions, as named on the command line and in exports.
const (
	ActionBlock        = "block"
	ActionGreyscale    = "greyscale"
	ActionNoImages     = "no-images"
	ActionMute         = "mute"          // the tab's sound is switched off
	ActionNoVideo      = "no-video"      // videos and players are stopped and hidden
	ActionDeclutter    = "declutter"     // sidebars, banners and the like are hidden
	ActionDark         = "dark"          // a light page is shown dark
	ActionRedirect     = "redirect"      // Param: the address to open instead
	ActionImage        = "image"         // Param: a stored image (see ImageName)
	ActionCloseTab     = "close-tab"     //
	ActionCloseBrowser = "close-browser" //
	ActionLogout       = "logout"        // Minutes: stay logged out that long (0: once)
	ActionShutdown     = "shutdown"      // Minutes: stay off that long (0: once)
	ActionMonk         = "monk"          // Minutes: how long monk mode lasts
)

// ActionNames lists the website actions in display order.
var ActionNames = []string{ActionBlock, ActionGreyscale, ActionNoImages, ActionMute, ActionNoVideo,
	ActionDeclutter, ActionDark, ActionRedirect, ActionImage,
	ActionCloseTab, ActionCloseBrowser, ActionLogout, ActionShutdown, ActionMonk}

// IsDeviceAction reports whether an action reaches past the page: it closes
// the browser, and the daemon carries out the rest.
func IsDeviceAction(name string) bool {
	switch name {
	case ActionCloseBrowser, ActionLogout, ActionShutdown, ActionMonk:
		return true
	}
	return false
}

// IsBlockingAction reports whether an action takes the page away (rather than
// only changing how it looks). Bypass prevention counts these.
func IsBlockingAction(name string) bool {
	switch name {
	case ActionGreyscale, ActionNoImages, ActionMute, ActionNoVideo, ActionDeclutter, ActionDark:
		return false
	}
	return IsAction(name)
}

// Limits on an action's minutes. Logging out and shutting down for a while
// go through Frozen Chicken, which lasts at most a day.
const (
	MaxFreezeMinutes = 24 * 60
	MaxMonkMinutes   = 30 * 24 * 60
)

var imageName = regexp.MustCompile(`^[0-9a-f]{64}\.(png|jpg|gif|webp)$`)

// IsImageName reports whether name is a stored image's name: its SHA-256 and
// type. Only such names are stored or served.
func IsImageName(name string) bool { return imageName.MatchString(name) }

// CheckActionParams checks what an action needs: a web address for
// redirect, a stored image for image, minutes for monk (and optionally for
// logout and shutdown). Other actions take nothing.
func CheckActionParams(action, param string, minutes int) error {
	switch action {
	case ActionRedirect:
		u, err := url.Parse(param)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || strings.ContainsAny(param, " \n\r\t") {
			return fmt.Errorf("redirect needs a web address starting with http:// or https://")
		}
	case ActionImage:
		if !IsImageName(param) {
			return fmt.Errorf("image needs a picture (PNG, JPEG, GIF or WebP)")
		}
	default:
		if param != "" {
			return fmt.Errorf("%s takes no address or image", action)
		}
	}
	switch action {
	case ActionMonk:
		if minutes < 1 || minutes > MaxMonkMinutes {
			return fmt.Errorf("monk mode lasts from 1 minute to %s", time.Duration(MaxMonkMinutes)*time.Minute)
		}
	case ActionLogout, ActionShutdown:
		if minutes < 0 || minutes > MaxFreezeMinutes {
			return fmt.Errorf("%s lasts at most %s (0: just once)", action, time.Duration(MaxFreezeMinutes)*time.Minute)
		}
	default:
		if minutes != 0 {
			return fmt.Errorf("%s takes no duration", action)
		}
	}
	return nil
}

// ParamsStricter reports whether changing an action's settings from (param,
// minutes) to (param2, minutes2) can only make it stricter: a longer time
// does, a different address doesn't (it could open anything). A different
// image is neither, so it is allowed.
func ParamsStricter(action, param, param2 string, minutes, minutes2 int) bool {
	switch action {
	case ActionRedirect:
		return param == param2
	case ActionImage:
		return true
	}
	return minutes2 >= minutes
}

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

// ImagesDir is where images for the image action are kept, beside the
// database.
func ImagesDir() string { return filepath.Join(filepath.Dir(ResolveDBPath()), "images") }
