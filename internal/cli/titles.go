// titles.go — window-title keywords for a block.
//
//	kc9k block add-title <block> YouTube "Twitch -"
//	kc9k block remove-title <block> YouTube
//
// While the block is on, any app whose window title contains a keyword
// (case-insensitive) is asked to close, then closed; in the browser, the
// extension sends just that tab to the block page.
package cli

import (
	"fmt"
	"strings"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

const maxTitleKeyword = 100

func checkTitleKeywords(keywords []string) error {
	for _, k := range keywords {
		k = strings.TrimSpace(k)
		if k == "" {
			return fmt.Errorf("a window title keyword can't be empty")
		}
		if len(k) > maxTitleKeyword || strings.ContainsAny(k, "\n\r\t") {
			return fmt.Errorf("invalid window title keyword %q (one line, at most %d characters)", k, maxTitleKeyword)
		}
	}
	return nil
}

var blockAddTitleCmd = &cobra.Command{
	Use:   "add-title [block] [keywords...]",
	Short: "Close apps whose window title contains these words",
	Long: `While the block is on, any app whose window title contains one of the
keywords (case-insensitive) is asked to close, and closed if it stays open.
In LibreWolf, the extension sends just the matching tab to the block page.
Programs the desktop needs are never closed. Adding keywords only makes a
block stricter, so it is always allowed.`,
	Args: cobra.MinimumNArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		name, keywords := args[0], args[1:]
		if err := checkTitleKeywords(keywords); err != nil {
			return err
		}
		if err := database.AddTitles(name, keywords); err != nil {
			return err
		}
		if err := reapplyIfActive(database, name); err != nil {
			return err
		}
		fmt.Printf("Window titles added to block '%s'.\n", name)
		return nil
	}),
}

var blockRemoveTitleCmd = &cobra.Command{
	Use:   "remove-title [block] [keywords...]",
	Short: "Stop closing apps by these window-title words",
	Args:  cobra.MinimumNArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		name, keywords := args[0], args[1:]
		detail, err := database.GetBlock(name)
		if err != nil {
			return err
		}
		if err := requireAll("window title", name, detail.Titles, keywords); err != nil {
			return err
		}
		proceed, err := gateEdit(cmd, database, name, false, db.Edit{Action: db.EditRemoveTitles, Values: keywords})
		if err != nil || !proceed {
			return err
		}
		if err := database.RemoveTitles(name, keywords); err != nil {
			return err
		}
		if err := reapplyIfActive(database, name); err != nil {
			return err
		}
		fmt.Printf("Window titles removed from block '%s'.\n", name)
		return nil
	}),
}

func init() {
	addNowFlag(blockRemoveTitleCmd)
	blockCmd.AddCommand(blockAddTitleCmd, blockRemoveTitleCmd)
}
