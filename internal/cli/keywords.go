// keywords.go — shortcuts to a block's "Page words" list, a blacklist that
// looks at page text (created when first needed).
//
//	kc9k block add-keyword <block> nsfw 18+ "*porn*"
//	kc9k block remove-keyword <block> nsfw
//	kc9k block threshold <block> greyscale 1
//	kc9k block threshold <block> block 3
//
// The extension reads each page's text. A plain keyword counts only as a
// whole word ("porn" misses "pornhub", "18+" misses "1118+"); a * matches
// inside words ("*porn*" hits "pornhub"). When a page contains at least the
// threshold's number of the list's different keywords, that action applies.
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

var blockAddKeywordCmd = shortcut("add-keyword [block] [keywords...]", "Look for these words in page text (the Page words list)", db.DefaultWordsList, true)

var blockRemoveKeywordCmd = shortcut("remove-keyword [block] [keywords...]", "Remove words from the Page words list", db.DefaultWordsList, false)

var blockThresholdCmd = &cobra.Command{
	Use:   "threshold [block] [" + strings.Join(db.ActionNames, "|") + "] [count]",
	Short: "How many different page words trigger each action (the Page words list)",
	Long: `A shortcut for 'kc9k list threshold' on the block's Page words list: how many
of its different words a page must contain before that action applies.
Repeats of the same word count once. Lowering a threshold is always allowed;
raising one loosens the block. With only the block name, prints them.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 && len(args) != 3 {
			return fmt.Errorf("give a block, or a block, an action and a count")
		}
		return nil
	},
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		name := args[0]
		list, err := defaultList(database, name, db.DefaultWordsList)
		if err != nil {
			return err
		}
		if len(args) == 1 {
			_, l, err := findList(database, name, list)
			if err != nil {
				return err
			}
			for _, a := range l.Actions {
				fmt.Printf("%s %d\n", a.Name, a.Distinct)
			}
			return nil
		}
		n, err := strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("a threshold is a number from 1 to %d", db.MaxThreshold)
		}
		return listThreshold(cmd, database, name, list, strings.ToLower(args[1]), n)
	}),
}

func init() {
	addNowFlag(blockRemoveKeywordCmd, blockThresholdCmd)
	blockCmd.AddCommand(blockAddKeywordCmd, blockRemoveKeywordCmd, blockThresholdCmd)
}
