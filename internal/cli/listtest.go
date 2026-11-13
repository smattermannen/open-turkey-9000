// listtest.go — kc9k list test: test mode, per list (§3.2).
package cli

import (
	"fmt"
	"slices"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

var listTestCmd = &cobra.Command{
	Use:   "test [block] [list] [on|off]",
	Short: "Test mode: show what a list finds on each page instead of acting",
	Long: `A list in test mode that reads a page's title or text does nothing to the
page. Instead a panel in the corner of each page shows which entries were
found, how often each, how many different ones, and for every action
whether its thresholds (different entries, and times in all) are met. Use
it to see whether a list would be too harsh before relying on it.

  kc9k list test work Words on     # one list
  kc9k list test work on           # every list of the block that reads pages
  kc9k list test                   # the lists in test mode

Lists that only look at addresses keep working as usual. Turning test mode
on weakens the block, so it is refused while the block is locked; turning it
off is always allowed.`,
	Args: cobra.RangeArgs(0, 3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		testing := database.TestLists()
		if len(args) == 0 {
			blocks, err := database.ListBlocks()
			if err != nil {
				return err
			}
			any := false
			for _, b := range blocks {
				detail, err := database.GetBlock(b.Name)
				if err != nil {
					return err
				}
				for _, l := range detail.Lists {
					if slices.Contains(testing, l.ID) {
						fmt.Printf("%s: %s\n", b.Name, l.Name)
						any = true
					}
				}
			}
			if !any {
				fmt.Println("No list is in test mode.")
			}
			return nil
		}
		name := args[0]
		detail, err := database.GetBlock(name)
		if err != nil {
			return err
		}
		var lists []db.WebList
		state := ""
		switch {
		case len(args) == 3 || (len(args) == 2 && args[1] != "on" && args[1] != "off"):
			l, ok := detail.Lists.Find(args[1])
			if !ok {
				return fmt.Errorf("block '%s' has no list '%s'", name, args[1])
			}
			lists = []db.WebList{l}
			if len(args) == 3 {
				state = args[2]
			}
		default:
			for _, l := range detail.Lists {
				if l.Where.Title || l.Where.Text {
					lists = append(lists, l)
				}
			}
			if len(args) == 2 {
				state = args[1]
			}
		}
		if state == "" {
			for _, l := range lists {
				fmt.Printf("Test mode for '%s' of '%s': %s\n", l.Name, name, map[bool]string{true: "on", false: "off"}[slices.Contains(testing, l.ID)])
			}
			return nil
		}
		if state != "on" && state != "off" {
			return fmt.Errorf("usage: kc9k list test <block> [<list>] on|off")
		}
		on := state == "on"
		if len(lists) == 0 {
			return fmt.Errorf("block '%s' has no list that reads pages", name)
		}
		if err := ensureEditable(database, name, !on); err != nil {
			return err
		}
		for _, l := range lists {
			if err := database.SetTestList(l.ID, on); err != nil {
				return err
			}
			fmt.Printf("Test mode for '%s' of '%s': %s\n", l.Name, name, state)
		}
		return nil
	}),
}

func init() {
	listCmd.AddCommand(listTestCmd)
}
