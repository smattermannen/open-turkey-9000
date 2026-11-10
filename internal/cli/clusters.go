// clusters.go — clusters: website lists kept outside any block, to copy
// into blocks.
//
//	kc9k cluster list
//	kc9k cluster show <cluster>
//	kc9k cluster create <cluster> [--url] [--title] [--text]
//	kc9k cluster rename <cluster> <new name>
//	kc9k cluster delete <cluster>
//	kc9k cluster add <cluster> <entries...>
//	kc9k cluster remove <cluster> <entries...>
//	kc9k cluster where <cluster> url|title|text on|off
//	kc9k cluster export <cluster>
//	kc9k cluster import <cluster> <file> [--url] [--title] [--text]
//	kc9k cluster copy <cluster> <block> [--into LIST | --as NAME] [--exceptions]
//	kc9k cluster save <block> <list> <cluster>
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
	if !where.Any() {
		where.URL = true
	}
	return where
}

func addWhereFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("url", false, "Look at the address (the default)")
	cmd.Flags().Bool("title", false, "Look at the page title")
	cmd.Flags().Bool("text", false, "Look at the page text")
}

func describeCluster(c db.Cluster) string {
	return fmt.Sprintf("%s looks at: %s; %d entries", c.Name, c.Where, len(c.Entries))
}

// clusterAdd checks entries against where the cluster looks and adds them.
func clusterAdd(database *db.DB, c db.Cluster, entries []string) (int, error) {
	entries, err := prepareEntries(c.Where, entries)
	if err != nil {
		return 0, err
	}
	return database.AddClusterEntries(c.ID, entries)
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
		return fmt.Errorf("cluster '%s' is empty", cluster)
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
		fmt.Printf("Cluster '%s' added to block '%s' as the list '%s' (%d entries).\n", cluster, block, target, len(c.Entries))
	} else {
		fmt.Printf("Cluster '%s' copied into '%s' of block '%s' (%d entries).\n", cluster, target, block, len(c.Entries))
	}
	return nil
}

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Manage clusters: website lists kept outside blocks, to copy into them",
	Long: `A cluster is a website list of its own: a name, where it looks (the
address, the page title, the page text) and entries, as in a block's list.
Nothing enforces a cluster; it is a stock to build blocks from. Copy one
into a block as a new blacklist, as a new exception list (--exceptions), or
into a list the block already has (--into):

  kc9k cluster create academia
  kc9k cluster add academia arxiv.org scholar.google.com jstor.org
  kc9k cluster copy academia study --exceptions     # spare them in 'study'
  kc9k cluster copy anime focus                     # block them in 'focus'
  kc9k cluster copy anime focus --into Blocked      # ... in its Blocked list

A copy is a copy: later changes to the cluster don't reach the block, and
editing a cluster is always allowed. Copying into a block follows the
block's rules: into a blacklist is always allowed, into an exception list
loosens the block and is refused, queued or challenged like other edits.`,
}

var clusterListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show the clusters",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		clusters, err := database.ListClusters()
		if err != nil {
			return err
		}
		if len(clusters) == 0 {
			fmt.Println("No clusters. Make one with 'kc9k cluster create <name>'.")
		}
		for _, c := range clusters {
			fmt.Println(describeCluster(c))
		}
		return nil
	}),
}

var clusterShowCmd = &cobra.Command{
	Use:   "show [cluster]",
	Short: "Show a cluster's entries",
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
	Use:   "create [cluster]",
	Short: "Make an empty cluster",
	Long: `Makes an empty cluster. It looks at addresses unless --title or --text is
given (combine them freely).`,
	Args: cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		where := whereFlags(cmd)
		if err := database.CreateCluster(args[0], where); err != nil {
			return err
		}
		fmt.Printf("Cluster '%s' created (looks at: %s).\n", args[0], where)
		return nil
	}),
}

var clusterRenameCmd = &cobra.Command{
	Use:   "rename [cluster] [new name]",
	Short: "Rename a cluster",
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
	Use:   "delete [cluster]",
	Short: "Remove a cluster with its entries (lists copied from it stay)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, err := database.GetCluster(args[0])
		if err != nil {
			return err
		}
		if err := database.DeleteCluster(c.ID); err != nil {
			return err
		}
		fmt.Printf("Cluster '%s' removed.\n", args[0])
		return nil
	}),
}

var clusterAddCmd = &cobra.Command{
	Use:   "add [cluster] [entries...]",
	Short: "Add entries to a cluster",
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
	Use:   "remove [cluster] [entries...]",
	Short: "Remove entries from a cluster",
	Args:  cobra.MinimumNArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, err := database.GetCluster(args[0])
		if err != nil {
			return err
		}
		entries := args[1:]
		// Domains are kept normalized, so look them up the same way.
		for i, e := range entries {
			if !containsString(c.Entries, e) && c.Where.URL {
				entries[i] = normalizeURLEntry(e)
			}
		}
		for _, e := range entries {
			if !containsString(c.Entries, e) {
				return fmt.Errorf("entry '%s' not found in cluster '%s'", e, c.Name)
			}
		}
		return database.RemoveClusterEntries(c.ID, entries)
	}),
}

var clusterWhereCmd = &cobra.Command{
	Use:   "where [cluster] [url|title|text] [on|off]",
	Short: "Choose where a cluster looks: the address, the page title, the page text",
	Args:  cobra.ExactArgs(3),
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
			return fmt.Errorf("cluster '%s' needs to look somewhere; tick another place first", c.Name)
		}
		return database.SetClusterWhere(c.ID, next)
	}),
}

var clusterExportCmd = &cobra.Command{
	Use:   "export [cluster]",
	Short: "Print a cluster's entries, one per line",
	Long: `Prints the cluster as a text file: a comment line saying where it looks,
then one entry per line. 'kc9k cluster import' reads it back, and so does
'kc9k list import'.`,
	Args: cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, err := database.GetCluster(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("%s%s\n", clusterHeader, strings.Join(c.Where.List(), ", "))
		for _, e := range c.Entries {
			fmt.Println(e)
		}
		return nil
	}),
}

// clusterHeader starts the comment line of an exported cluster that says
// where it looks, e.g. "# kc9k cluster, looks at: url, title".
const clusterHeader = "# kc9k cluster, looks at: "

// exportedWhere reads where an exported cluster looks from its header line.
func exportedWhere(data string) (db.Where, bool) {
	var where db.Where
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, clusterHeader) {
			continue
		}
		for _, place := range strings.Split(strings.TrimPrefix(line, clusterHeader), ",") {
			if place = strings.TrimSpace(place); db.IsPlace(place) {
				where = where.With(place, true)
			}
		}
		return where, where.Any()
	}
	return where, false
}

var clusterImportCmd = &cobra.Command{
	Use:   "import [cluster] [file]",
	Short: "Add entries from a text file with one entry per line",
	Long: `Adds the file's entries (one per line; # starts a comment) to the cluster,
making it if there is none by that name. A new cluster looks where --url,
--title and --text say, or else where the file says (a file written by
'kc9k cluster export'), or else at addresses.`,
	Args: cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		entries, err := readEntryFile(args[1])
		if err != nil {
			return err
		}
		exists, err := database.ClusterExists(args[0])
		if err != nil {
			return err
		}
		flagged := cmd.Flags().Changed("url") || cmd.Flags().Changed("title") || cmd.Flags().Changed("text")
		if exists && flagged {
			return fmt.Errorf("cluster '%s' exists; change where it looks with 'kc9k cluster where'", args[0])
		}
		where := whereFlags(cmd)
		if !exists && !flagged {
			if data, err := readAsCaller(args[1]); err == nil {
				if w, ok := exportedWhere(string(data)); ok {
					where = w
				}
			}
		}
		if !exists {
			// Check the entries before making the cluster, so a bad file
			// leaves nothing behind.
			if _, err := prepareEntries(where, entries); err != nil {
				return err
			}
			if err := database.CreateCluster(args[0], where); err != nil {
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
		fmt.Printf("Imported %d new entries into cluster '%s'.\n", added, args[0])
		return nil
	}),
}

var clusterCopyCmd = &cobra.Command{
	Use:   "copy [cluster] [block]",
	Short: "Copy a cluster into a block, as a new list or into one of its lists",
	Long: `Without --into, adds the cluster to the block as a new list that looks
where the cluster does: a blacklist that blocks what it matches, or with
--exceptions an exception list. The list is named after the cluster unless
--as gives another name.

With --into, adds the cluster's entries to that list of the block; every
entry must fit where the list looks.

Into a blacklist this strengthens the block and is always allowed. Into an
exception list it loosens the block, like 'kc9k list add' to that list.`,
	Args: cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		into, _ := cmd.Flags().GetString("into")
		as, _ := cmd.Flags().GetString("as")
		exceptions, _ := cmd.Flags().GetBool("exceptions")
		return clusterCopy(cmd, database, args[0], args[1], into, as, exceptions)
	}),
}

var clusterSaveCmd = &cobra.Command{
	Use:   "save [block] [list] [cluster]",
	Short: "Save a block's list as a cluster (adds to the cluster if it exists)",
	Args:  cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		_, l, err := findList(database, args[0], args[1])
		if err != nil {
			return err
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
		added, err := clusterAdd(database, c, l.Entries)
		if err != nil {
			return err
		}
		fmt.Printf("Saved %d new entries from '%s' of '%s' to cluster '%s'.\n", added, args[1], args[0], args[2])
		return nil
	}),
}

func init() {
	addWhereFlags(clusterCreateCmd)
	addWhereFlags(clusterImportCmd)
	clusterCopyCmd.Flags().String("into", "", "Add the entries to this list of the block")
	clusterCopyCmd.Flags().String("as", "", "Name of the new list (default: the cluster's name)")
	clusterCopyCmd.Flags().Bool("exceptions", false, "Make the new list an exception list")
	addNowFlag(clusterCopyCmd)
	clusterCmd.AddCommand(clusterListCmd, clusterShowCmd, clusterCreateCmd, clusterRenameCmd, clusterDeleteCmd,
		clusterAddCmd, clusterRemoveCmd, clusterWhereCmd, clusterExportCmd, clusterImportCmd, clusterCopyCmd, clusterSaveCmd)
	rootCmd.AddCommand(clusterCmd)
}
