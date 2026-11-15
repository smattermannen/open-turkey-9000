// options.go — kc9k behaviour: behaviour settings, how KC9K behaves (§3.13).
package cli

import (
	"fmt"
	"strings"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var optionCmd = &cobra.Command{
	Use:     "behaviour [<name> on|off]",
	Aliases: []string{"option"},
	Short:   "Behaviour settings: how Killer Chicken 9000 behaves",
	Long: `Behaviour settings change how Killer Chicken 9000 behaves, not what it
blocks; they are never locked:

  polite-close        ask a blocked app to close (so it can offer to save)
                      and kill it a few seconds later; off kills at once
  block-page-details  the block page names the block and list that stopped
                      the page and says when the block ends

Without arguments, shows them.`,
	Args: cobra.MaximumNArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 0 {
			for _, o := range enforce.Options {
				state := "off"
				if enforce.OptionOn(database, o.Name) {
					state = "on"
				}
				fmt.Printf("%-20s %s\n", o.Name, state)
			}
			return nil
		}
		if len(args) != 2 || (args[1] != "on" && args[1] != "off") {
			return fmt.Errorf("usage: kc9k behaviour <name> on|off")
		}
		name := strings.ToLower(args[0])
		if err := enforce.SetOption(database, name, args[1] == "on"); err != nil {
			return err
		}
		fmt.Printf("%s %s\n", name, args[1])
		return nil
	}),
}

func init() {
	rootCmd.AddCommand(optionCmd)
}
