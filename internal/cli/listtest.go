// listtest.go — kc9k list test: keyword test mode.
package cli

import (
	"fmt"
	"slices"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

var listTestCmd = &cobra.Command{
	Use:   "test [block] [on|off]",
	Short: "Keyword test mode: show what a block's lists find on each page instead of acting",
	Long: `In keyword test mode the block's lists that read a page's title or text do
nothing to the page. Instead a panel in the corner of each page shows which
entries were found, how often each, how many different ones, and for every
action whether its thresholds (different entries, and times in all) are
met. Use it to see whether a rule would be too harsh before relying on it.

Lists that only look at addresses keep working as usual. Turning test mode
on weakens the block, so it is refused while the block is locked; turning it
off is always allowed. Without on or off, shows the blocks in test mode.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 0 {
			names := database.TestBlocks()
			if len(names) == 0 {
				fmt.Println("No block is in keyword test mode.")
			}
			for _, n := range names {
				fmt.Println(n)
			}
			return nil
		}
		name := args[0]
		if _, err := database.GetBlock(name); err != nil {
			return err
		}
		if len(args) == 1 {
			state := "off"
			if slices.Contains(database.TestBlocks(), name) {
				state = "on"
			}
			fmt.Printf("Keyword test mode for '%s': %s\n", name, state)
			return nil
		}
		if args[1] != "on" && args[1] != "off" {
			return fmt.Errorf("usage: kc9k list test <block> on|off")
		}
		on := args[1] == "on"
		if err := ensureEditable(database, name, !on); err != nil {
			return err
		}
		if err := database.SetTestBlock(name, on); err != nil {
			return err
		}
		fmt.Printf("Keyword test mode for '%s': %s\n", name, args[1])
		return nil
	}),
}

func init() {
	listCmd.AddCommand(listTestCmd)
}
