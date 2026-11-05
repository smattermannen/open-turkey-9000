// browser.go — settings for the extension and forced browser behaviour.
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var browserCmd = &cobra.Command{
	Use:   "browser [setting] [on|off]",
	Short: "Set browser options: " + strings.Join(enforce.BrowserSettings, ", "),
	Long: `Options are always allowed to be turned on. While a block is locked or
Frozen Chicken runs, they can't be turned off.`,
	Args: cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
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
		return database.SetSetting("browser."+name, value)
	}),
}

func init() {
	rootCmd.AddCommand(browserCmd)
}
