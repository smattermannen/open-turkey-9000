// clusters.go — list and phrase set presets (§3.2; "clusters" in the code
// and DB, old command name kept as an alias, §4.12): lists kept outside any
// block, to copy into blocks.
//
//	kc9k list-preset list
//	kc9k list-preset show <preset>
//	kc9k list-preset create <preset> [--url] [--title] [--text] [--phrases]
//	kc9k list-preset rename <preset> <new name>
//	kc9k list-preset delete <preset>
//	kc9k list-preset add <preset> <entries...>
//	kc9k list-preset remove <preset> <entries...>
//	kc9k list-preset scope <preset> url|title|text on|off
//	kc9k list-preset export <preset>
//	kc9k list-preset import <preset> <file> [--url] [--title] [--text]
//	kc9k list-preset copy <preset> <block> [--into LIST | --as NAME] [--exceptions]
//	kc9k list-preset save <block> <list> <preset>
//
// Nothing enforces a cluster, so editing one is always allowed and never
// changes a block. Copying one into a block is an edit to that block like
// any other: into a blacklist it strengthens the block, into an exception
// list it loosens it and is gated (see gateEdit).
package cli

import (
	"fmt"
	"strings"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

// whereFlags reads --url, --title and --text; none means the address.
func whereFlags(cmd *cobra.Command) db.Where {
	var where db.Where
	where.URL, _ = cmd.Flags().GetBool("url")
	where.Title, _ = cmd.Flags().GetBool("title")
	where.Text, _ = cmd.Flags().GetBool("text")
	where.Ads, _ = cmd.Flags().GetBool("ads")
	if !where.Any() {
		where.URL = true
	}
	return where
}

func addWhereFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("url", false, "Look at the address (the default)")
	cmd.Flags().Bool("title", false, "Look at the page title")
	cmd.Flags().Bool("text", false, "Look at the page text")
	cmd.Flags().Bool("ads", false, "Look at the ad networks a page loads (ad network domains or industries)")
}

func describeCluster(c db.Cluster) string {
	if c.Phrases {
		return fmt.Sprintf("%s (phrases) looks at: %s; %d phrases", c.Name, c.Where, len(c.Entries))
	}
	return fmt.Sprintf("%s looks at: %s; %d entries", c.Name, c.Where, len(c.Entries))
}

// typedPhrases parses phrases and writes them back in their stored form.
func typedPhrases(texts []string) ([]string, error) {
	var out []string
	for _, text := range texts {
		p, err := db.ParsePhrase(text)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", text, err)
		}
		out = append(out, p.String())
	}
	return out, nil
}

// clusterAdd checks entries against where the cluster looks (or, in a
// phrase cluster, that they are phrases) and adds them.
func clusterAdd(database *db.DB, c db.Cluster, entries []string) (int, error) {
	var err error
	if c.Phrases {
		entries, err = typedPhrases(entries)
	} else if err = refuseCombinations(entries); err == nil {
		entries, err = prepareEntries(c.Where, entries)
	}
	if err != nil {
		return 0, err
	}
	return database.AddClusterEntries(c.ID, entries)
}

// phraseClusterCopy copies a phrase cluster into a block: into the phrase
// set named into, or else into a new one (named as, or after the cluster)
// that looks where the cluster does and blocks what it matches. There are
// no phrase exceptions (§3.2).
func phraseClusterCopy(database *db.DB, c db.Cluster, block, into, as string, exceptions bool) error {
	if exceptions {
		return fmt.Errorf("'%s' holds phrases, and there are no phrase exceptions", c.Name)
	}
	if _, err := database.GetBlock(block); err != nil {
		return err
	}
	target := into
	if into != "" {
		if as != "" {
			return fmt.Errorf("--as is for a new phrase set; --into adds to a phrase set the block has")
		}
	} else {
		target = as
		if target == "" {
			target = c.Name
		}
		if _, err := database.FindPhraseSet(block, target); err == nil {
			return fmt.Errorf("block '%s' already has a phrase set named '%s'; add to it with --into, or pick a name with --as", block, target)
		}
		if err := ensureEditable(database, block, true); err != nil {
			return err
		}
		if _, err := database.CreatePhraseSet(block, target, c.Where); err != nil {
			return err
		}
	}
	if err := phraseAdd(database, block, target, c.Entries); err != nil {
		if into == "" {
			if s, findErr := database.FindPhraseSet(block, target); findErr == nil {
				_ = database.DeletePhraseSet(s.ID)
			}
		}
		return err
	}
	return nil
}

// clusterCopy copies a cluster's entries into a block: into the list named
// into, or else into a new list (named as, or after the cluster) that looks
// where the cluster does; a blacklist unless exceptions.
func clusterCopy(cmd *cobra.Command, database *db.DB, cluster, block, into, as string, exceptions bool) error {
	c, err := database.GetCluster(cluster)
	if err != nil {
		return err
	}
	if len(c.Entries) == 0 {
		return fmt.Errorf("list preset '%s' is empty", cluster)
	}
	if c.Phrases {
		return phraseClusterCopy(database, c, block, into, as, exceptions)
	}
	detail, err := database.GetBlock(block)
	if err != nil {
		return err
	}
	target, created := into, false
	if into != "" {
		if as != "" || exceptions {
			return fmt.Errorf("--as and --exceptions are for a new list; --into adds to a list the block has")
		}
	} else {
		target = as
		if target == "" {
			target = c.Name
		}
		if _, exists := detail.Lists.Find(target); exists {
			return fmt.Errorf("block '%s' already has a list named '%s'; add to it with --into, or pick a name with --as", block, target)
		}
		kind := db.ListBlacklist
		if exceptions {
			kind = db.ListExceptions
		}
		if _, err := database.CreateList(block, kind, target, c.Where); err != nil {
			return err
		}
		created = true
	}
	if err := listAdd(cmd, database, block, target, c.Entries); err != nil {
		if created {
			// Refused: don't leave the empty list behind.
			if _, l, findErr := findList(database, block, target); findErr == nil {
				_ = database.DeleteList(l.ID)
			}
		}
		return err
	}
	_, l, err := findList(database, block, target)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, e := range l.Entries {
		have[strings.ToLower(e)] = true
	}
	if !have[strings.ToLower(c.Entries[0])] {
		return nil // queued: the gate said so
	}
	if created {
		fmt.Printf("List preset '%s' added to block '%s' as the list '%s' (%d entries).\n", cluster, block, target, len(c.Entries))
	} else {
		fmt.Printf("List preset '%s' copied into '%s' of block '%s' (%d entries).\n", cluster, target, block, len(c.Entries))
	}
	return nil
}

var clusterCmd = &cobra.Command{
	Use:     "list-preset",
	Aliases: []string{"cluster", "phrase-set-preset"},
	Short:   "Manage list presets: website lists kept outside blocks, to copy into them",
	Long: `A list preset is a website list of its own: a name, where it looks (the
address, the page title, the page text) and entries, as in a block's list.
Nothing enforces a list preset; it is a stock to build blocks from. Copy one
into a block as a new blacklist, as a new exception list (--exceptions), or
into a list the block already has (--into):

  kc9k list-preset create academia
  kc9k list-preset add academia arxiv.org scholar.google.com jstor.org
  kc9k list-preset copy academia study --exceptions     # spare them in 'study'
  kc9k list-preset copy anime focus                     # block them in 'focus'
  kc9k list-preset copy anime focus --into Blocked      # ... in its Blocked list

A copy is a copy: later changes to the list preset don't reach the block, and
editing a list preset is always allowed. Copying into a block follows the
block's rules: into a blacklist is always allowed, into an exception list
weakens the block and is refused while it is locked, like other edits.`,
}

var clusterListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show the list presets",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		clusters, err := database.ListClusters()
		if err != nil {
			return err
		}
		if len(clusters) == 0 {
			fmt.Println("No list presets. Make one with 'kc9k list-preset create <name>'.")
		}
		for _, c := range clusters {
			fmt.Println(describeCluster(c))
		}
		return nil
	}),
}

var clusterShowCmd = &cobra.Command{
	Use:   "show [preset]",
	Short: "Show a list preset's entries",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, err := database.GetCluster(args[0])
		if err != nil {
			return err
		}
		fmt.Println(describeCluster(c))
		for _, e := range c.Entries {
			fmt.Println("  " + e)
		}
		return nil
	}),
}

var clusterCreateCmd = &cobra.Command{
	Use:   "create [preset]",
	Short: "Make an empty list preset",
	Long: `Makes an empty list preset. It looks at addresses unless --title or --text is
given (combine them freely).

With --phrases it is a phrase set preset: it holds phrases such as
"[eat, nice] [hello, grandpa]", is copied into a block as a phrase set, and
looks at the text unless --url, --title or --text say otherwise.`,
	Args: cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		where := whereFlags(cmd)
		create := database.CreateCluster
		if phrases, _ := cmd.Flags().GetBool("phrases"); phrases {
			create = database.CreatePhraseCluster
			if !cmd.Flags().Changed("url") {
				where.URL = false
			}
			if !where.Any() {
				where.Text = true
			}
		}
		if err := create(args[0], where); err != nil {
			return err
		}
		fmt.Printf("List preset '%s' created (looks at: %s).\n", args[0], where)
		return nil
	}),
}

var clusterRenameCmd = &cobra.Command{
	Use:   "rename [preset] [new name]",
	Short: "Rename a list preset",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, err := database.GetCluster(args[0])
		if err != nil {
			return err
		}
		return database.RenameCluster(c.ID, args[1])
	}),
}

var clusterDeleteCmd = &cobra.Command{
	Use:   "delete [preset]",
	Short: "Remove a list preset with its entries (lists copied from it stay)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, err := database.GetCluster(args[0])
		if err != nil {
			return err
		}
		if err := database.DeleteCluster(c.ID); err != nil {
			return err
		}
		fmt.Printf("List preset '%s' removed.\n", args[0])
		return nil
	}),
}

var clusterAddCmd = &cobra.Command{
	Use:   "add [preset] [entries...]",
	Short: "Add entries to a list preset",
	Args:  cobra.MinimumNArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, err := database.GetCluster(args[0])
		if err != nil {
			return err
		}
		_, err = clusterAdd(database, c, args[1:])
		return err
	}),
}

var clusterRemoveCmd = &cobra.Command{
	Use:   "remove [preset] [entries...]",
	Short: "Remove entries from a list preset",
	Args:  cobra.MinimumNArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, err := database.GetCluster(args[0])
		if err != nil {
			return err
		}
		entries := args[1:]
		if c.Phrases {
			if entries, err = typedPhrases(entries); err != nil {
				return err
			}
		}
		if !c.Phrases {
			entries = storedEntries(c.Entries, entries)
		}
		for _, e := range entries {
			if !containsString(c.Entries, e) {
				return fmt.Errorf("entry '%s' not found in list preset '%s'", e, c.Name)
			}
		}
		return database.RemoveClusterEntries(c.ID, entries)
	}),
}

var clusterWhereCmd = &cobra.Command{
	Use:     "scope [preset] [url|title|text] [on|off]",
	Aliases: []string{"where"},
	Short:   "Choose where a list preset looks: the address, the page title, the page text",
	Args:    cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		place := strings.ToLower(args[1])
		if !db.IsPlace(place) {
			return fmt.Errorf("unknown place %q (use %s)", args[1], strings.Join(db.Places, ", "))
		}
		on, err := parseOnOff(args[2])
		if err != nil {
			return err
		}
		c, err := database.GetCluster(args[0])
		if err != nil {
			return err
		}
		next := c.Where.With(place, on)
		if !next.Any() {
			return fmt.Errorf("list preset '%s' needs to look somewhere; tick another place first", c.Name)
		}
		return database.SetClusterWhere(c.ID, next)
	}),
}

var clusterExportCmd = &cobra.Command{
	Use:   "export [preset]",
	Short: "Print a list preset's entries, one per line",
	Long: `Prints the list preset as a text file: a comment line saying where it looks,
then one entry per line. 'kc9k list-preset import' reads it back, and so does
'kc9k list import'.`,
	Args: cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, err := database.GetCluster(args[0])
		if err != nil {
			return err
		}
		header := clusterHeader
		if c.Phrases {
			header = phraseClusterHeader
		}
		fmt.Printf("%s%s\n", header, strings.Join(c.Where.List(), ", "))
		for _, e := range c.Entries {
			fmt.Println(e)
		}
		return nil
	}),
}

// clusterHeader starts the comment line of an exported list preset that
// says where it looks, e.g. "# kc9k list preset, looks at: url, title".
const clusterHeader = "# kc9k list preset, looks at: "

// phraseClusterHeader starts that line for a phrase set preset. The old
// headers (from while presets were "clusters", and term clusters while
// they had types) are still read.
const (
	phraseClusterHeader    = "# kc9k phrase set preset, looks at: "
	oldClusterHeader       = "# kc9k cluster, looks at: "
	oldPhraseClusterHeader = "# kc9k phrase cluster, looks at: "
	oldTermClusterHeader   = "# kc9k term cluster, looks at: "
)

// exportedWhere reads where an exported cluster looks from its header line.
func exportedWhere(data string) (db.Where, bool) {
	where, _, ok := exportedCluster(data)
	return where, ok
}

// exportedCluster reads an exported cluster's header line: where it looks,
// and whether it holds "phrases" ("" for entries).
func exportedCluster(data string) (db.Where, string, bool) {
	var where db.Where
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		header, what := "", ""
		for _, h := range []struct{ prefix, what string }{
			{phraseClusterHeader, "phrases"}, {clusterHeader, ""}, {oldPhraseClusterHeader, "phrases"},
			{oldTermClusterHeader, ""}, {oldClusterHeader, ""}} {
			if header == "" && strings.HasPrefix(line, h.prefix) {
				header, what = h.prefix, h.what
			}
		}
		if header == "" {
			continue
		}
		for _, place := range strings.Split(strings.TrimPrefix(line, header), ",") {
			if place = strings.TrimSpace(place); db.IsPlace(place) {
				where = where.With(place, true)
			}
		}
		return where, what, where.Any()
	}
	return where, "", false
}

var clusterImportCmd = &cobra.Command{
	Use:   "import [preset] [file]",
	Short: "Add entries from a text file with one entry per line",
	Long: `Adds the file's entries (one per line; # starts a comment) to the list preset,
making it if there is none by that name. A new list preset looks where --url,
--title and --text say, or else where the file says (a file written by
'kc9k list-preset export'), or else at addresses.`,
	Args: cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		f, err := readEntries(args[1])
		if err != nil {
			return err
		}
		exists, err := database.ClusterExists(args[0])
		if err != nil {
			return err
		}
		flagged := cmd.Flags().Changed("url") || cmd.Flags().Changed("title") || cmd.Flags().Changed("text") || cmd.Flags().Changed("ads")
		if exists && flagged {
			return fmt.Errorf("list preset '%s' exists; change where it looks with 'kc9k list-preset scope'", args[0])
		}
		where := whereFlags(cmd)
		phrases, _ := cmd.Flags().GetBool("phrases")
		if phrases && !cmd.Flags().Changed("url") {
			where.URL = false
		}
		if !exists && !flagged {
			if data, err := readAsCaller(args[1]); err == nil {
				if w, what, ok := exportedCluster(string(data)); ok {
					where, phrases = w, phrases || what == "phrases"
				}
			}
		}
		if !exists && !flagged && len(f.Entries) == 0 && len(f.Phrases) > 0 {
			phrases = true // a file of phrases makes a phrase cluster
		}
		entries := f.Entries
		if phrases {
			entries = f.Phrases
		}
		if len(entries) == 0 {
			return fmt.Errorf("%s holds %s, none for this list preset", args[1], describeEntryFile(f))
		}
		defer noteLeftOut(f, phrases)
		if phrases && !where.Any() {
			where.Text = true
		}
		if !exists {
			// Check the entries before making the cluster, so a bad file
			// leaves nothing behind.
			create := database.CreateCluster
			if phrases {
				create = database.CreatePhraseCluster
				if _, err := typedPhrases(entries); err != nil {
					return err
				}
			} else if _, err := prepareEntries(where, entries); err != nil {
				return err
			}
			if err := create(args[0], where); err != nil {
				return err
			}
		}
		c, err := database.GetCluster(args[0])
		if err != nil {
			return err
		}
		added, err := clusterAdd(database, c, entries)
		if err != nil {
			return err
		}
		fmt.Printf("Imported %d new entries into list preset '%s'.\n", added, args[0])
		return nil
	}),
}

var clusterCopyCmd = &cobra.Command{
	Use:   "copy [preset] [block]",
	Short: "Copy a list preset into a block, as a new list or into one of its lists",
	Long: `Without --into, adds the list preset to the block as a new list that looks
where the list preset does: a blacklist that blocks what it matches, or with
--exceptions an exception list. The list is named after the list preset unless
--as gives another name.

With --into, adds the list preset's entries to that list of the block; every
entry must fit where the list looks.

Into a blacklist this strengthens the block and is always allowed. Into an
exception list it weakens the block, like 'kc9k list add' to that list.`,
	Args: cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		into, _ := cmd.Flags().GetString("into")
		as, _ := cmd.Flags().GetString("as")
		exceptions, _ := cmd.Flags().GetBool("exceptions")
		return clusterCopy(cmd, database, args[0], args[1], into, as, exceptions)
	}),
}

var clusterSaveCmd = &cobra.Command{
	Use:   "save [block] [list or phrase set] [preset]",
	Short: "Save a block's list or phrase set as a list preset (adds to the list preset if it exists)",
	Args:  cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		_, l, err := findList(database, args[0], args[1])
		if err != nil {
			s, setErr := database.FindPhraseSet(args[0], args[1])
			if setErr != nil {
				return err
			}
			return savePhraseCluster(database, args[0], s, args[2])
		}
		if len(l.Entries) == 0 {
			return fmt.Errorf("list '%s' of block '%s' is empty", args[1], args[0])
		}
		exists, err := database.ClusterExists(args[2])
		if err != nil {
			return err
		}
		if !exists {
			if err := database.CreateCluster(args[2], l.Where); err != nil {
				return err
			}
		}
		c, err := database.GetCluster(args[2])
		if err != nil {
			return err
		}
		if c.Phrases {
			return fmt.Errorf("list preset '%s' holds phrases, not a list's entries", args[2])
		}
		added, err := clusterAdd(database, c, l.Entries)
		if err != nil {
			return err
		}
		fmt.Printf("Saved %d new entries from '%s' of '%s' to list preset '%s'.\n", added, args[1], args[0], args[2])
		return nil
	}),
}

// savePhraseCluster saves a block's phrase set as a phrase cluster.
func savePhraseCluster(database *db.DB, block string, s db.PhraseSet, name string) error {
	if len(s.Phrases) == 0 {
		return fmt.Errorf("phrase set '%s' of block '%s' is empty", s.Name, block)
	}
	exists, err := database.ClusterExists(name)
	if err != nil {
		return err
	}
	if !exists {
		if err := database.CreatePhraseCluster(name, s.Where); err != nil {
			return err
		}
	}
	c, err := database.GetCluster(name)
	if err != nil {
		return err
	}
	if !c.Phrases {
		return fmt.Errorf("list preset '%s' holds entries, not phrases", name)
	}
	var texts []string
	for _, p := range s.Phrases {
		texts = append(texts, p.String())
	}
	added, err := clusterAdd(database, c, texts)
	if err != nil {
		return err
	}
	fmt.Printf("Saved %d new phrases from '%s' of '%s' to list preset '%s'.\n", added, s.Name, block, name)
	return nil
}

func init() {
	clusterCreateCmd.Flags().Bool("phrases", false, "Make a phrase set preset (copied into blocks as a phrase set)")
	clusterImportCmd.Flags().Bool("phrases", false, "A new list preset holds phrases")
	clusterCreateCmd.Flags().Bool("terms", false, "Old alias; changes nothing (list presets have no types)")
	clusterImportCmd.Flags().Bool("terms", false, "Old alias; changes nothing (list presets have no types)")
	addWhereFlags(clusterCreateCmd)
	addWhereFlags(clusterImportCmd)
	clusterCopyCmd.Flags().String("into", "", "Add the entries to this list of the block")
	clusterCopyCmd.Flags().String("as", "", "Name of the new list (default: the list preset's name)")
	clusterCopyCmd.Flags().Bool("exceptions", false, "Make the new list an exception list")
	addNowFlag(clusterCopyCmd)
	clusterCmd.AddCommand(clusterListCmd, clusterShowCmd, clusterCreateCmd, clusterRenameCmd, clusterDeleteCmd,
		clusterAddCmd, clusterRemoveCmd, clusterWhereCmd, clusterExportCmd, clusterImportCmd, clusterCopyCmd, clusterSaveCmd)
	rootCmd.AddCommand(clusterCmd)
}
