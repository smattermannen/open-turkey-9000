// browsers.go — the supported browsers (§3.13, §4.10 Browsers): which are
// installed, whether the extension checks in from each, one-step install.
//
//	kc9k browsers                       # list
//	kc9k browsers install chromium      # librewolf, chrome or chromium
package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

// guiBrowser is a supported browser for 'kc9k browsers' and the interface.
type guiBrowser struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	// Found: one of its executables is on the computer. Unlike Installed,
	// it doesn't ask whether KC9K can give it the extension: on Windows,
	// Chrome and Edge are found but not installed as supported browsers
	// until the extension's Chrome Web Store listing exists (the GUI says
	// so, W30). On Linux the two are the same.
	Found bool `json:"found"`
	// CheckIn is how many seconds ago its extension last checked in; -1
	// never (since this was recorded).
	CheckIn int64 `json:"checkin_seconds"`
}

// supportedBrowsers lists the supported browsers' state.
func supportedBrowsers(database *db.DB) []guiBrowser {
	var out []guiBrowser
	for _, b := range blocker.SupportedBrowsers {
		g := guiBrowser{ID: b.ID, Name: b.Name, Installed: b.Installed(), CheckIn: -1}
		g.Found = g.Installed || browserFound(b.Exes)
		if last, ok := enforce.LastBrowserCheckIn(database, b.ID); ok {
			g.CheckIn = max(0, int64(time.Since(last).Seconds()))
		}
		out = append(out, g)
	}
	return out
}

// browserFileStat is os.Stat, replaced in tests.
var browserFileStat = os.Stat

// browserFound reports whether any of a browser's executables exists.
func browserFound(exes []string) bool {
	for _, exe := range exes {
		if _, err := browserFileStat(exe); err == nil {
			return true
		}
	}
	return false
}

// extensionText describes a browser's extension for people.
func (g guiBrowser) extensionText() string {
	switch {
	case !g.Installed:
		return "not installed"
	case g.CheckIn < 0:
		return "installed; extension not seen yet (open it once)"
	case g.CheckIn < 120:
		return "installed; extension working"
	}
	return fmt.Sprintf("installed; extension last seen %s ago", fmtAgo(time.Duration(g.CheckIn)*time.Second))
}

var browsersCmd = &cobra.Command{
	Use:   "browsers [install librewolf|chrome|chromium]",
	Short: "Supported browsers: installed, extension status, install",
	Long: `The supported browsers run the Killer Chicken extension: LibreWolf, Google
Chrome and Chromium. Without arguments, lists each: installed or not, and
whether its extension has checked in.

  install librewolf   from LibreWolf's repository (via extrepo)
  install chromium    from Linux Mint's repository
  install chrome      from Google's repository (added, with its signing key)

The extension is added the next time the browser starts. With 'kc9k tamper
unsupported-browsers on', every other browser is closed.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || (len(args) == 2 && args[0] == "install") {
			return nil
		}
		return fmt.Errorf("give no arguments to list, or install and librewolf, chrome or chromium")
	},
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 2 {
			return installBrowser(args[1])
		}
		for _, b := range supportedBrowsers(database) {
			fmt.Printf("%-10s %s\n", b.ID, b.extensionText())
		}
		return nil
	}),
}

func installBrowser(id string) error {
	b, ok := blocker.SupportedBrowserByID(id)
	if !ok {
		return fmt.Errorf("unknown browser %q (use librewolf, chrome or chromium)", id)
	}
	if b.Installed() {
		fmt.Printf("%s is already installed.\n", b.Name)
		return nil
	}
	fmt.Printf("Installing %s, this can take a few minutes…\n", b.Name)
	if err := blocker.InstallBrowser(id); err != nil {
		return err
	}
	fmt.Printf("%s installed. The extension is added the next time it starts.\n", b.Name)
	return nil
}

func init() {
	rootCmd.AddCommand(browsersCmd)
}
