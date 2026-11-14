// phrases.go — kc9k phrase-set: phrase sets (§3.1, §3.2, R-PHRASE-1 to 10).
//
//	kc9k phrase-set show <block> [<set>]
//	kc9k phrase-set create <block> <set> [--url --title --text]
//	kc9k phrase-set add <block> <set> "[eat, nice] [hello, grandpa]" ...
//	kc9k phrase-set remove <block> <set> "<phrase>"
//	kc9k phrase-set scope <block> <set> url|title|text on|off
//	kc9k phrase-set action <block> <set> <action> on|off [--url U --image F --for D]
//	kc9k phrase-set threshold <block> <set> <action> <n>
//	kc9k phrase-set test <block> <set> on|off
//	kc9k phrase-set rename <block> <set> <new name>
//	kc9k phrase-set delete <block> <set>
//	kc9k phrase-set export <block> <set>          (one phrase per line)
//	kc9k phrase-set import <block> <set> <file>
//
// Strengthening is always allowed (adding a phrase, ticking a place or an
// action, a lower threshold, test mode off); everything else weakens the
// block and is refused while it is locked (R-LOCK-1).
package cli

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var phraseSetCmd = &cobra.Command{
	Use:   "phrase-set",
	Short: "Phrase sets: words that count only together, such as [eat, nice] [hello, grandpa]",
	Long: `A phrase set is a named set of phrases in a block, with a scope (where it
looks: url, title, text) and its own actions, like a blacklist. A phrase is
two or more word sets in brackets; a match takes one term from each, directly
after each other in any order, in one sentence:

  [eat, nice] [hello, grandpa]       matches "Let's eat grandpa", not "Let's eat, grandpa!"
  [long cat, neko] [running, sitting] matches "long cat running" and "long-cat sitting"
  [big-, strong] [-gorilla, cat]     a bridge: up to 20 words between big and gorilla
  [red] [car] [big]?                 a ? makes a word set (third or later) optional
  [red] [car] [big, small]? :3       :N is the phrase length: how many terms a match needs

In a term, * matches inside a word (porn*). Each action has a threshold: how
many different phrases of the set must match (default 1). The block's
exceptions spare pages from its phrase sets too.`,
}

func phraseSetArgs(database *db.DB, block, name string) (db.PhraseSet, error) {
	if _, err := database.GetBlock(block); err != nil {
		return db.PhraseSet{}, err
	}
	return database.FindPhraseSet(block, name)
}

var phraseSetShowCmd = &cobra.Command{
	Use:   "show <block> [<set>]",
	Short: "Show a block's phrase sets, or one with its phrases",
	Args:  cobra.RangeArgs(1, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		sets, err := database.PhraseSets(args[0])
		if err != nil {
			return err
		}
		if len(sets) == 0 {
			fmt.Printf("Block '%s' has no phrase sets.\n", args[0])
		}
		if len(args) == 2 {
			if _, err := database.FindPhraseSet(args[0], args[1]); err != nil {
				return err
			}
		}
		for _, s := range sets {
			if len(args) == 2 && !strings.EqualFold(s.Name, args[1]) {
				continue
			}
			var acts []string
			for _, a := range s.Actions {
				acts = append(acts, fmt.Sprintf("%s (%d)", a.Name, a.Distinct))
			}
			test := ""
			if s.Test {
				test = "; test mode"
			}
			fmt.Printf("%s looks at: %s; actions: %s%s; %d phrases\n", s.Name, s.Where, strings.Join(acts, ", "), test, len(s.Phrases))
			if len(args) == 2 {
				for _, p := range s.Phrases {
					fmt.Printf("  %s\n", p)
				}
			}
		}
		return nil
	}),
}

var phraseSetCreateCmd = &cobra.Command{
	Use:   "create <block> <set> [--url --title --text]",
	Short: "Make a phrase set (it blocks what it matches; looks at the text unless told otherwise)",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		var where db.Where
		for _, place := range []string{db.PlaceURL, db.PlaceTitle, db.PlaceText} {
			if on, _ := cmd.Flags().GetBool(place); on {
				where = where.With(place, true)
			}
		}
		if !where.Any() {
			where.Text = true
		}
		if err := ensureEditable(database, args[0], true); err != nil {
			return err
		}
		if _, err := database.CreatePhraseSet(args[0], args[1], where); err != nil {
			return err
		}
		fmt.Printf("Phrase set '%s' added to block '%s' (looks at: %s).\n", args[1], args[0], where)
		return nil
	}),
}

var phraseSetAddCmd = &cobra.Command{
	Use:   "add <block> <set> <phrase>...",
	Short: "Add phrases (always allowed: it makes the block stricter)",
	Args:  cobra.MinimumNArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		return phraseAdd(database, args[0], args[1], args[2:])
	}),
}

// phraseAdd adds typed phrases to a set: all are checked before any is
// added. Adding is always allowed (it makes the block stricter).
func phraseAdd(database *db.DB, block, set string, texts []string) error {
	s, err := phraseSetArgs(database, block, set)
	if err != nil {
		return err
	}
	var phrases []db.Phrase
	for _, text := range texts {
		p, err := db.ParsePhrase(text)
		if err != nil {
			return fmt.Errorf("%q: %w", text, err)
		}
		phrases = append(phrases, p)
	}
	if err := ensureEditable(database, block, true); err != nil {
		return err
	}
	added := 0
	for _, p := range phrases {
		ok, err := database.AddPhrase(s.ID, p)
		if err != nil {
			return err
		}
		if ok {
			added++
		}
	}
	if err := reapplyIfActive(database, block); err != nil {
		return err
	}
	fmt.Printf("%d phrases added to '%s' of '%s'.\n", added, s.Name, block)
	return nil
}

var phraseSetExportCmd = &cobra.Command{
	Use:   "export <block> <set>",
	Short: "Print a phrase set's phrases, one per line (redirect to a .txt file to save them)",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		s, err := phraseSetArgs(database, args[0], args[1])
		if err != nil {
			return err
		}
		for _, p := range s.Phrases {
			fmt.Println(p)
		}
		return nil
	}),
}

var phraseSetImportCmd = &cobra.Command{
	Use:   "import <block> <set> <file>",
	Short: "Add phrases from a text file with one phrase per line",
	Args:  cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		texts, err := readEntryFile(args[2])
		if err != nil {
			return err
		}
		return phraseAdd(database, args[0], args[1], texts)
	}),
}

var phraseSetRemoveCmd = &cobra.Command{
	Use:   "remove <block> <set> <phrase>...",
	Short: "Remove phrases (weakens the block)",
	Args:  cobra.MinimumNArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		s, err := phraseSetArgs(database, args[0], args[1])
		if err != nil {
			return err
		}
		// Every phrase must be there before any is removed.
		var phrases []db.Phrase
		for _, text := range args[2:] {
			p, err := db.ParsePhrase(text)
			if err != nil {
				return fmt.Errorf("%q: %w", text, err)
			}
			if !slices.ContainsFunc(s.Phrases, func(q db.Phrase) bool { return q.String() == p.String() }) {
				return fmt.Errorf("'%s' has no phrase %s", s.Name, p)
			}
			phrases = append(phrases, p)
		}
		if err := ensureEditable(database, args[0], false); err != nil {
			return err
		}
		for _, p := range phrases {
			if _, err := database.RemovePhrase(s.ID, p); err != nil {
				return err
			}
		}
		fmt.Printf("Removed from '%s' of '%s'.\n", s.Name, args[0])
		return settle(database, args[0])
	}),
}

var phraseSetScopeCmd = &cobra.Command{
	Use:   "scope <block> <set> url|title|text on|off",
	Short: "Tick or untick where a phrase set looks (unticking weakens the block)",
	Args:  cobra.ExactArgs(4),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		s, err := phraseSetArgs(database, args[0], args[1])
		if err != nil {
			return err
		}
		if !db.IsPlace(args[2]) || (args[3] != "on" && args[3] != "off") {
			return fmt.Errorf("usage: kc9k phrase-set scope <block> <set> url|title|text on|off")
		}
		on := args[3] == "on"
		if err := ensureEditable(database, args[0], on); err != nil {
			return err
		}
		if err := database.SetPhraseSetWhere(s.ID, s.Where.With(args[2], on)); err != nil {
			return err
		}
		fmt.Printf("'%s' of '%s' looks at: %s\n", s.Name, args[0], s.Where.With(args[2], on))
		return reapplyIfActive(database, args[0])
	}),
}

var phraseSetActionCmd = &cobra.Command{
	Use:   "action <block> <set> <action> on|off",
	Short: "Turn one of a phrase set's actions on or off (off weakens the block)",
	Args:  cobra.ExactArgs(4),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		s, err := phraseSetArgs(database, args[0], args[1])
		if err != nil {
			return err
		}
		action := strings.ToLower(args[2])
		if !db.IsAction(action) {
			return fmt.Errorf("unknown action %q (use %s)", action, strings.Join(db.ActionNames, ", "))
		}
		if args[3] != "on" && args[3] != "off" {
			return fmt.Errorf("use 'on' or 'off'")
		}
		on := args[3] == "on"
		if on && action == db.ActionMonk && args[0] == enforce.MonkBlock {
			return fmt.Errorf("monk mode's own block can't start monk mode (R-MONK-1)")
		}
		if !on && (cmd.Flags().Changed("url") || cmd.Flags().Changed("image") || cmd.Flags().Changed("for")) {
			return fmt.Errorf("--url, --image and --for only go with 'on'")
		}
		if !on && len(s.Actions) == 1 && s.Actions[0].Name == action {
			return fmt.Errorf("a phrase set keeps at least one action")
		}
		var set actionSettings
		if on {
			if set, err = readActionSettings(cmd, action); err != nil {
				return err
			}
		}
		// Turning on is stricter, unless it changes the settings of an
		// action the set has (another address).
		cur, has := s.Action(action)
		strengthens := on && (!has || db.ParamsStricter(action, cur.Param, set.param, cur.Minutes, set.minutes))
		if err := ensureEditable(database, args[0], strengthens); err != nil {
			return err
		}
		if err := database.SetPhraseSetAction(s.ID, action, on, set.param, set.minutes); err != nil {
			return err
		}
		fmt.Printf("'%s' of '%s': %s %s\n", s.Name, args[0], action, args[3])
		return settle(database, args[0])
	}),
}

var phraseSetThresholdCmd = &cobra.Command{
	Use:   "threshold <block> <set> <action> <n>",
	Short: "How many different phrases an action needs (a higher number weakens the block)",
	Args:  cobra.ExactArgs(4),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		s, err := phraseSetArgs(database, args[0], args[1])
		if err != nil {
			return err
		}
		n, err := strconv.Atoi(args[3])
		if err != nil {
			return fmt.Errorf("the threshold is a number")
		}
		cur, has := s.Action(args[2])
		if !has {
			return fmt.Errorf("'%s' doesn't have the action %s", s.Name, args[2])
		}
		if err := ensureEditable(database, args[0], n <= cur.Distinct); err != nil {
			return err
		}
		if err := database.SetPhraseSetThreshold(s.ID, args[2], n); err != nil {
			return err
		}
		fmt.Printf("'%s' of '%s': %s needs %d different phrases\n", s.Name, args[0], args[2], n)
		return reapplyIfActive(database, args[0])
	}),
}

var phraseSetTestCmd = &cobra.Command{
	Use:   "test <block> <set> on|off",
	Short: "Test mode: show what the set finds in pages instead of acting (on weakens the block)",
	Args:  cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		s, err := phraseSetArgs(database, args[0], args[1])
		if err != nil {
			return err
		}
		if args[2] != "on" && args[2] != "off" {
			return fmt.Errorf("use 'on' or 'off'")
		}
		on := args[2] == "on"
		if err := ensureEditable(database, args[0], !on); err != nil {
			return err
		}
		if err := database.SetPhraseSetTest(s.ID, on); err != nil {
			return err
		}
		fmt.Printf("Test mode for '%s' of '%s': %s\n", s.Name, args[0], args[2])
		return reapplyIfActive(database, args[0])
	}),
}

var phraseSetRenameCmd = &cobra.Command{
	Use:   "rename <block> <set> <new name>",
	Short: "Rename a phrase set",
	Args:  cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		s, err := phraseSetArgs(database, args[0], args[1])
		if err != nil {
			return err
		}
		if err := database.RenamePhraseSet(s.ID, args[2]); err != nil {
			return err
		}
		fmt.Printf("Phrase set '%s' of '%s' is now '%s'.\n", s.Name, args[0], args[2])
		return nil
	}),
}

var phraseSetDeleteCmd = &cobra.Command{
	Use:   "delete <block> <set>",
	Short: "Delete a phrase set (weakens the block)",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		s, err := phraseSetArgs(database, args[0], args[1])
		if err != nil {
			return err
		}
		if err := ensureEditable(database, args[0], len(s.Phrases) == 0); err != nil {
			return err
		}
		if err := database.DeletePhraseSet(s.ID); err != nil {
			return err
		}
		fmt.Printf("Phrase set '%s' removed from '%s'.\n", s.Name, args[0])
		return settle(database, args[0])
	}),
}

func init() {
	for _, place := range []string{db.PlaceURL, db.PlaceTitle, db.PlaceText} {
		phraseSetCreateCmd.Flags().Bool(place, false, "Look at the page's "+map[string]string{"url": "address", "title": "title", "text": "text"}[place])
	}
	phraseSetActionCmd.Flags().String("url", "", "For redirect: the address to open instead")
	phraseSetActionCmd.Flags().String("image", "", "For image: the picture to show instead")
	phraseSetActionCmd.Flags().String("for", "", "For monk, logout and shutdown: how long (e.g. 30m, 2h)")
	phraseSetCmd.AddCommand(phraseSetShowCmd, phraseSetCreateCmd, phraseSetAddCmd, phraseSetRemoveCmd, phraseSetScopeCmd,
		phraseSetActionCmd, phraseSetThresholdCmd, phraseSetTestCmd, phraseSetRenameCmd, phraseSetDeleteCmd,
		phraseSetExportCmd, phraseSetImportCmd)
	rootCmd.AddCommand(phraseSetCmd)
}
