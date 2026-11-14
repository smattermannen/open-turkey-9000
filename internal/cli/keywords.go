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
// inside words ("*porn*" hits "pornhub"). A word combination such as
// cars(red, big) or (car, lambo*) (big, small, nice):2 counts where its words
// follow one another directly. When a page contains at least the
// threshold's number of the list's different keywords, that action applies.
package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

const (
	maxPageKeyword     = 100
	maxWordCombination = 300
)

// comboBox is one box of a word combination, the old form of phrases
// (db/combinations.go), still read from old files.
type comboBox = db.ComboBox

// pageCombination returns a word combination's boxes, or false when k isn't
// a well-formed one.
func pageCombination(k string) ([]comboBox, bool) {
	return db.ParseCombination(k)
}

// comboString writes boxes the one way they are stored:
// "cars (red, big)", "(car, lambo*) (big, small, nice):2".
func comboString(boxes []comboBox) string {
	parts := make([]string, len(boxes))
	for i, b := range boxes {
		if b.Bare && b.Min == 1 && len(b.Terms) == 1 {
			parts[i] = b.Terms[0]
			continue
		}
		parts[i] = "(" + strings.Join(b.Terms, ", ") + ")"
		if b.Min > 1 {
			parts[i] += ":" + strconv.Itoa(b.Min)
		}
	}
	return strings.Join(parts, " ")
}

// normalizeCombination writes a word combination the way it is stored, and
// leaves anything else as it is.
func normalizeCombination(k string) string {
	if boxes, ok := pageCombination(k); ok {
		return comboString(boxes)
	}
	return k
}

func checkPageKeywords(keywords []string) error {
	for _, k := range keywords {
		k = strings.TrimSpace(k)
		if strings.Trim(k, "*") == "" {
			return fmt.Errorf("a page keyword needs more than wildcards")
		}
		limit := maxPageKeyword
		if strings.ContainsAny(k, "()") {
			if _, ok := pageCombination(k); !ok {
				return fmt.Errorf("invalid word combination %q: write boxes side by side, each a list in brackets with an optional :N for how many of its words are needed, or one word, as in cars(red, big) or (car, lambo*) (big, small, nice):2 (sport, hello)", k)
			}
			limit = maxWordCombination
		}
		if len(k) > limit || strings.ContainsAny(k, "\n\r\t") {
			return fmt.Errorf("invalid page keyword %q (one line, at most %d characters)", k, limit)
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
raising one weakens the block. With only the block name, prints them.`,
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
