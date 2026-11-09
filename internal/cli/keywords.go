// keywords.go — page keywords and their thresholds for a block.
//
//	kc9k block add-keyword <block> nsfw 18+ "*porn*"
//	kc9k block remove-keyword <block> nsfw
//	kc9k block threshold <block> greyscale 1
//	kc9k block threshold <block> block 3
//
// The extension reads each page's text. A plain keyword counts only as a
// whole word ("porn" misses "pornhub", "18+" misses "1118+"); a * matches
// inside words ("*porn*" hits "pornhub"). When a page contains at least the
// threshold's number of the block's distinct keywords, the block's ticked
// actions apply to it: the block page, greyscale, no images.
package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

const maxPageKeyword = 100

func checkPageKeywords(keywords []string) error {
	for _, k := range keywords {
		k = strings.TrimSpace(k)
		if strings.Trim(k, "*") == "" {
			return fmt.Errorf("a page keyword needs more than wildcards")
		}
		if len(k) > maxPageKeyword || strings.ContainsAny(k, "\n\r\t") {
			return fmt.Errorf("invalid page keyword %q (one line, at most %d characters)", k, maxPageKeyword)
		}
	}
	return nil
}

var blockAddKeywordCmd = &cobra.Command{
	Use:   "add-keyword [block] [keywords...]",
	Short: "Act on pages whose text contains these words",
	Long: `While the block is on, the extension looks for its keywords in the text of
every page (case-insensitive). A plain keyword counts only as a whole word:
"porn" doesn't count in "pornhub", and "18+" doesn't count in "1118+". A *
matches inside words: "*porn*" counts in "pornhub", "porn*" in "pornstar".

Once a page contains at least the threshold's number of different keywords
('kc9k block threshold'; 1 unless changed), the block's actions apply to it:
the block page, greyscale or no images, as ticked with 'kc9k block action'.
Adding keywords only makes a block stricter, so it is always allowed.`,
	Args: cobra.MinimumNArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		name, keywords := args[0], args[1:]
		if err := checkPageKeywords(keywords); err != nil {
			return err
		}
		if err := database.AddPageKeywords(name, keywords); err != nil {
			return err
		}
		if err := reapplyIfActive(database, name); err != nil {
			return err
		}
		fmt.Printf("Page keywords added to block '%s'.\n", name)
		return nil
	}),
}

var blockRemoveKeywordCmd = &cobra.Command{
	Use:   "remove-keyword [block] [keywords...]",
	Short: "Stop looking for these words in pages",
	Args:  cobra.MinimumNArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		name, keywords := args[0], args[1:]
		detail, err := database.GetBlock(name)
		if err != nil {
			return err
		}
		if err := requireAll("page keyword", name, detail.Keywords, keywords); err != nil {
			return err
		}
		proceed, err := gateEdit(cmd, database, name, false, db.Edit{Action: db.EditRemoveKeywords, Values: keywords})
		if err != nil || !proceed {
			return err
		}
		if err := database.RemovePageKeywords(name, keywords); err != nil {
			return err
		}
		if err := reapplyIfActive(database, name); err != nil {
			return err
		}
		fmt.Printf("Page keywords removed from block '%s'.\n", name)
		return nil
	}),
}

var blockThresholdCmd = &cobra.Command{
	Use:   "threshold [block] [" + strings.Join(db.ActionNames, "|") + "] [count]",
	Short: "How many different page keywords trigger each action",
	Long: `Sets, for one of the block's actions, how many of its different page keywords
a page must contain before that action applies. Repeats of the same keyword
count once. For example, greyscale at 1 and block at 3: one keyword greys the
page, three different ones block it.

Lowering a threshold makes the block stricter and is always allowed. Raising
one loosens it, so it is gated like other loosening edits (refused while the
block is held; queued for a block with allow windows, or applied with --now
after the typing challenge). With only the block name, prints the thresholds.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 && len(args) != 3 {
			return fmt.Errorf("give a block, or a block, an action and a count")
		}
		return nil
	},
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		name := args[0]
		detail, err := database.GetBlock(name)
		if err != nil {
			return err
		}
		if len(args) == 1 {
			for _, action := range db.ActionNames {
				fmt.Printf("%s %d\n", action, detail.Thresholds.Get(action))
			}
			return nil
		}
		action := strings.ToLower(args[1])
		if !db.IsAction(action) {
			return fmt.Errorf("unknown action %q (use %s)", args[1], strings.Join(db.ActionNames, ", "))
		}
		n, err := strconv.Atoi(args[2])
		if err != nil || n < 1 || n > db.MaxThreshold {
			return fmt.Errorf("a threshold is a number from 1 to %d", db.MaxThreshold)
		}
		current := detail.Thresholds.Get(action)
		if n == current {
			fmt.Printf("Block '%s' already needs %d keyword(s) for %s.\n", name, n, action)
			return nil
		}
		if n > current {
			proceed, err := gateEdit(cmd, database, name, false,
				db.Edit{Action: db.EditSetThreshold, Values: []string{action, strconv.Itoa(n)}})
			if err != nil || !proceed {
				return err
			}
		}
		if err := database.SetThreshold(name, action, n); err != nil {
			return err
		}
		if err := reapplyIfActive(database, name); err != nil {
			return err
		}
		fmt.Printf("Block '%s' now needs %d different keyword(s) for %s.\n", name, n, action)
		return nil
	}),
}

func init() {
	addNowFlag(blockRemoveKeywordCmd, blockThresholdCmd)
	blockCmd.AddCommand(blockAddKeywordCmd, blockRemoveKeywordCmd, blockThresholdCmd)
}
