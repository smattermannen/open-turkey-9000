// lists.go — the commands from before website lists, kept as shortcuts to a
// block's default lists: "Blocked" (addresses), "Exceptions" (addresses) and
// "Page words" (page text). See weblists.go for the lists themselves.
//
//	kc9k block add <block> reddit.com '*casino*'       # to Blocked
//	kc9k block add-exception <block> reddit.com/r/x   # to Exceptions
//	kc9k block export-list <block> blocked|exceptions|apps|titles|<list>
//	kc9k block import-list <block> blocked|exceptions|apps|titles|<list> <file>
package cli

import (
	"fmt"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

const (
	listBlocked    = "blocked"
	listExceptions = "exceptions"
	listApps       = "apps"
	listTitles     = "titles"
)

// defaultList returns the name of one of a block's default lists, creating
// it if the block doesn't have it (it was deleted or renamed).
func defaultList(database *db.DB, block, name string) (string, error) {
	detail, err := database.GetBlock(block)
	if err != nil {
		return "", err
	}
	if _, ok := detail.Lists.Find(name); ok {
		return name, nil
	}
	kind, where := db.ListBlacklist, db.Where{URL: true}
	switch name {
	case db.DefaultExceptionsList:
		kind = db.ListExceptions
	case db.DefaultWordsList:
		where = db.Where{Text: true}
	}
	_, err = database.CreateList(block, kind, name, where)
	return name, err
}

// onDefaultList runs fn on one of the block's default lists.
func onDefaultList(database *db.DB, block, name string, fn func(list string) error) error {
	list, err := defaultList(database, block, name)
	if err != nil {
		return err
	}
	return fn(list)
}

func addBlockedEntries(cmd *cobra.Command, database *db.DB, name string, entries []string) error {
	return onDefaultList(database, name, db.DefaultBlockedList, func(list string) error {
		return listAdd(cmd, database, name, list, entries)
	})
}

func addExceptions(cmd *cobra.Command, database *db.DB, name string, entries []string) error {
	return onDefaultList(database, name, db.DefaultExceptionsList, func(list string) error {
		return listAdd(cmd, database, name, list, entries)
	})
}

func withDB(run func(cmd *cobra.Command, database *db.DB, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()
		return run(cmd, database, args)
	}
}

// shortcut makes a command that runs fn on a default list with the block
// name and the remaining arguments.
func shortcut(use, short, list string, add bool) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.MinimumNArgs(2),
		RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
			return onDefaultList(database, args[0], list, func(name string) error {
				if add {
					return listAdd(cmd, database, args[0], name, args[1:])
				}
				return listRemove(cmd, database, args[0], name, args[1:])
			})
		}),
	}
}

var (
	blockAddEntryCmd        = shortcut("add [block] [entries...]", "Block domains, URLs or wildcard patterns (the Blocked list)", db.DefaultBlockedList, true)
	blockRemoveEntryCmd     = shortcut("remove-entry [block] [entries...]", "Remove entries from the Blocked list", db.DefaultBlockedList, false)
	blockAddSiteCmd         = shortcut("add-site [block] [domains...]", "Add sites to the Blocked list", db.DefaultBlockedList, true)
	blockRemoveSiteCmd      = shortcut("remove-site [block] [domains...]", "Remove sites from the Blocked list", db.DefaultBlockedList, false)
	blockAddPatternCmd      = shortcut("add-pattern [block] [patterns...]", "Add URL patterns (wildcards) to the Blocked list", db.DefaultBlockedList, true)
	blockRemovePatternCmd   = shortcut("remove-pattern [block] [patterns...]", "Remove URL patterns from the Blocked list", db.DefaultBlockedList, false)
	blockAddExceptionCmd    = shortcut("add-exception [block] [entries...]", "Allow domains, URLs or patterns despite the block (the Exceptions list)", db.DefaultExceptionsList, true)
	blockRemoveExceptionCmd = shortcut("remove-exception [block] [entries...]", "Remove exceptions (always allowed: it makes the block stricter)", db.DefaultExceptionsList, false)
)

// listFor maps export-list/import-list's names to a list: blocked and
// exceptions are the default lists, anything else names a list.
func listFor(database *db.DB, block, kind string) (string, error) {
	switch kind {
	case listBlocked:
		return defaultList(database, block, db.DefaultBlockedList)
	case listExceptions:
		return defaultList(database, block, db.DefaultExceptionsList)
	}
	return kind, nil
}

var blockExportListCmd = &cobra.Command{
	Use:   "export-list [block] [blocked|exceptions|apps|titles|<list name>]",
	Short: "Print one list, one entry per line",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		detail, err := database.GetBlock(args[0])
		if err != nil {
			return err
		}
		var lines []string
		switch args[1] {
		case listApps:
			lines = detail.Apps
		case listTitles:
			lines = detail.Titles
		default:
			name, err := listFor(database, args[0], args[1])
			if err != nil {
				return err
			}
			_, l, err := findList(database, args[0], name)
			if err != nil {
				return err
			}
			lines = l.Entries
		}
		for _, line := range lines {
			fmt.Println(line)
		}
		return nil
	}),
}

// addApps adds apps to a block. Apps are always blocked, so adding one only
// strengthens it.
func addApps(database *db.DB, name string, apps []string) error {
	for _, a := range apps {
		if err := blocker.ValidAppEntry(a); err != nil {
			return err
		}
	}
	if err := database.AddApps(name, apps); err != nil {
		return err
	}
	return reapplyIfActive(database, name)
}

// addTitles adds window-title keywords, which only strengthens a block.
func addTitles(database *db.DB, name string, titles []string) error {
	if err := checkTitleKeywords(titles); err != nil {
		return err
	}
	if err := database.AddTitles(name, titles); err != nil {
		return err
	}
	return reapplyIfActive(database, name)
}

var blockImportListCmd = &cobra.Command{
	Use:   "import-list [block] [blocked|exceptions|apps|titles|<list name>] [file]",
	Short: "Add entries from a text file with one entry per line",
	Args:  cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		entries, err := readEntryFile(args[2])
		if err != nil {
			return err
		}
		switch args[1] {
		case listApps:
			err = addApps(database, args[0], entries)
		case listTitles:
			err = addTitles(database, args[0], entries)
		default:
			var name string
			if name, err = listFor(database, args[0], args[1]); err == nil {
				err = listAdd(cmd, database, args[0], name, entries)
			}
		}
		if err != nil {
			return err
		}
		fmt.Printf("Imported %d entries into %s of '%s'.\n", len(entries), args[1], args[0])
		return nil
	}),
}

func init() {
	addNowFlag(blockAddEntryCmd, blockRemoveEntryCmd, blockAddSiteCmd, blockRemoveSiteCmd, blockRemovePatternCmd,
		blockAddExceptionCmd, blockImportListCmd)
	blockCmd.AddCommand(blockAddEntryCmd, blockRemoveEntryCmd, blockAddSiteCmd, blockRemoveSiteCmd,
		blockAddPatternCmd, blockRemovePatternCmd, blockAddExceptionCmd,
		blockRemoveExceptionCmd, blockExportListCmd, blockImportListCmd)
}
