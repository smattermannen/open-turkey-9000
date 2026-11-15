// toggle.go — kc9k toggle: switch a block off if it is on, else on
// (§4.12, Cold Turkey's -toggle). It runs stop or start with all their
// checks: a locked block isn't switched off, and toggle sets no locks.
package cli

import (
	"github.com/spf13/cobra"
)

var toggleCmd = &cobra.Command{
	Use:   "toggle [block]",
	Short: "Switch a block off if it is on, or on (without locks) if it is off",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		active, err := database.IsBlockActive(args[0])
		database.Close()
		if err != nil {
			return err
		}
		if active {
			return stopCmd.RunE(stopCmd, args)
		}
		// Not parsed for this command, so start's lock flags are unset.
		return startCmd.RunE(startCmd, args)
	},
}

func init() {
	rootCmd.AddCommand(toggleCmd)
}
