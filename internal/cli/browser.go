// browser.go — settings for the extension and forced browser behaviour.
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var browserCmd = &cobra.Command{
	Use:   "browser [setting on|off | install-librewolf]",
	Short: "Set browser options: " + strings.Join(enforce.BrowserSettings, ", "),
	Long: `Options are always allowed to be turned on. While a block is locked or
Frozen Chicken runs, they can't be turned off. Without arguments, lists
every option and whether it is on, and the installed LibreWolf version.

force-librewolf needs LibreWolf installed; 'kc9k browser install-librewolf'
installs it from its official repository (via extrepo).`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 && args[0] == "install-librewolf" {
			return nil
		}
		if len(args) != 0 && len(args) != 2 {
			return fmt.Errorf("give no arguments to list options, a setting and on|off, or install-librewolf")
		}
		return nil
	},
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 0 {
			for _, name := range enforce.BrowserSettings {
				state := "off"
				if enforce.BrowserOn(database, name) {
					state = "on"
				}
				fmt.Printf("%s %s\n", name, state)
			}
			version := blocker.LibreWolfVersion()
			if version == "" {
				version = "not-installed"
			}
			fmt.Printf("librewolf %s\n", version)
			return nil
		}
		if len(args) == 1 {
			if version := blocker.LibreWolfVersion(); version != "" {
				fmt.Printf("LibreWolf %s is already installed.\n", version)
				return nil
			}
			fmt.Println("Installing LibreWolf, this can take a few minutes…")
			if err := blocker.InstallLibreWolf(); err != nil {
				return err
			}
			fmt.Printf("LibreWolf %s installed. The extension is added the next time it starts.\n", blocker.LibreWolfVersion())
			return nil
		}
		name, value := args[0], args[1]
		if !enforce.IsBrowserSetting(name) {
			return fmt.Errorf("unknown setting %q (use %s)", name, strings.Join(enforce.BrowserSettings, ", "))
		}
		if value != "on" && value != "off" {
			return fmt.Errorf("use 'on' or 'off'")
		}
		if value == "off" {
			now := time.Now()
			trusted, err := database.TrustedNow(now)
			if err != nil {
				return err
			}
			locked, err := enforce.AnyLock(database, now, trusted)
			if err != nil {
				return err
			}
			if locked {
				return fmt.Errorf("%s can't be turned off during a lock", name)
			}
		}
		if name == "force-librewolf" && value == "on" && blocker.LibreWolfVersion() == "" {
			return fmt.Errorf("LibreWolf isn't installed, so every browser would be closed; run 'kc9k browser install-librewolf' first")
		}
		return database.SetSetting("browser."+name, value)
	}),
}

func init() {
	rootCmd.AddCommand(browserCmd)
}
