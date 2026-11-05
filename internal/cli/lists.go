// lists.go — one list per block for what is blocked and one for exceptions,
// with separate export and import. Entries work as in Cold Turkey: a plain
// domain, a URL, or a wildcard pattern.
package cli

import (
	"fmt"
	"strings"

	"github.com/brunodcdo/open-turkey/internal/blocker"
	"github.com/brunodcdo/open-turkey/internal/db"
	"github.com/spf13/cobra"
)

const (
	listBlocked    = "blocked"
	listExceptions = "exceptions"
)

// splitEntries separates plain domains from URLs and wildcard patterns.
func splitEntries(entries []string) (domains, patterns []string, err error) {
	var plain []string
	for _, e := range entries {
		if strings.ContainsAny(e, "/*") {
			if !blocker.IsValidPattern(e) {
				return nil, nil, fmt.Errorf("invalid entry %q", e)
			}
			patterns = append(patterns, e)
		} else {
			plain = append(plain, e)
		}
	}
	domains, err = normalizeDomains(plain)
	return domains, patterns, err
}

func missing(have, want []string) []string {
	set := make(map[string]bool, len(have))
	for _, h := range have {
		set[h] = true
	}
	var out []string
	for _, w := range want {
		if !set[w] {
			set[w] = true
			out = append(out, w)
		}
	}
	return out
}

func addBlockedEntries(cmd *cobra.Command, database *db.DB, name string, entries []string) error {
	domains, patterns, err := splitEntries(entries)
	if err != nil {
		return err
	}
	detail, err := database.GetBlock(name)
	if err != nil {
		return err
	}
	domains, patterns = missing(detail.Sites, domains), missing(detail.Patterns, patterns)
	if len(patterns) > 0 {
		if err := database.AddPatterns(name, patterns); err != nil {
			return err
		}
	}
	if len(domains) > 0 {
		proceed, err := gateEdit(cmd, database, name, siteEditStrengthens(detail, domains, true), db.Edit{Action: db.EditAddSites, Values: domains})
		if err != nil {
			return err
		}
		if proceed {
			if err := database.AddSites(name, domains); err != nil {
				return err
			}
		}
	}
	return reapplyIfActive(database, name)
}

func addExceptions(cmd *cobra.Command, database *db.DB, name string, entries []string) error {
	for _, e := range entries {
		if !blocker.IsValidPattern(e) {
			return fmt.Errorf("invalid entry %q", e)
		}
	}
	proceed, err := gateEdit(cmd, database, name, false, db.Edit{Action: db.EditAddExceptions, Values: entries})
	if err != nil || !proceed {
		return err
	}
	if err := database.AddExceptions(name, entries); err != nil {
		return err
	}
	return reapplyIfActive(database, name)
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

var blockAddEntryCmd = &cobra.Command{
	Use:   "add [block] [entries...]",
	Short: "Block domains, URLs or wildcard patterns",
	Args:  cobra.MinimumNArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		return addBlockedEntries(cmd, database, args[0], args[1:])
	}),
}

var blockRemoveEntryCmd = &cobra.Command{
	Use:   "remove-entry [block] [entries...]",
	Short: "Remove domains, URLs or wildcard patterns from a block",
	Args:  cobra.MinimumNArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		name := args[0]
		domains, patterns, err := splitEntries(args[1:])
		if err != nil {
			return err
		}
		detail, err := database.GetBlock(name)
		if err != nil {
			return err
		}
		if len(domains) > 0 {
			if err := requireAll("site", name, detail.Sites, domains); err != nil {
				return err
			}
			proceed, err := gateEdit(cmd, database, name, siteEditStrengthens(detail, domains, false), db.Edit{Action: db.EditRemoveSites, Values: domains})
			if err != nil {
				return err
			}
			if proceed {
				if err := database.RemoveSites(name, domains); err != nil {
					return err
				}
			}
		}
		if len(patterns) > 0 {
			if err := requireAll("pattern", name, detail.Patterns, patterns); err != nil {
				return err
			}
			proceed, err := gateEdit(cmd, database, name, false, db.Edit{Action: db.EditRemovePatterns, Values: patterns})
			if err != nil {
				return err
			}
			if proceed {
				if err := database.RemovePatterns(name, patterns); err != nil {
					return err
				}
			}
		}
		return reapplyIfActive(database, name)
	}),
}

var blockAddExceptionCmd = &cobra.Command{
	Use:   "add-exception [block] [entries...]",
	Short: "Allow domains, URLs or wildcard patterns despite the block",
	Args:  cobra.MinimumNArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		return addExceptions(cmd, database, args[0], args[1:])
	}),
}

var blockRemoveExceptionCmd = &cobra.Command{
	Use:   "remove-exception [block] [entries...]",
	Short: "Remove exceptions from a block (always allowed: it makes the block stricter)",
	Args:  cobra.MinimumNArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if err := database.RemoveExceptions(args[0], args[1:]); err != nil {
			return err
		}
		return reapplyIfActive(database, args[0])
	}),
}

func checkListKind(kind string) error {
	if kind != listBlocked && kind != listExceptions {
		return fmt.Errorf("unknown list %q (use '%s' or '%s')", kind, listBlocked, listExceptions)
	}
	return nil
}

var blockExportListCmd = &cobra.Command{
	Use:   "export-list [block] [blocked|exceptions]",
	Short: "Print one list, one entry per line",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if err := checkListKind(args[1]); err != nil {
			return err
		}
		detail, err := database.GetBlock(args[0])
		if err != nil {
			return err
		}
		lines := detail.Exceptions
		if args[1] == listBlocked {
			lines = append(append([]string{}, detail.Sites...), detail.Patterns...)
			if detail.Mode == db.ModeAllowlist {
				lines = detail.Patterns
			}
		} else if detail.Mode == db.ModeAllowlist {
			lines = append(append([]string{}, detail.Sites...), detail.Exceptions...)
		}
		for _, line := range lines {
			fmt.Println(line)
		}
		return nil
	}),
}

var blockImportListCmd = &cobra.Command{
	Use:   "import-list [block] [blocked|exceptions] [file]",
	Short: "Add entries from a text file with one entry per line",
	Args:  cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if err := checkListKind(args[1]); err != nil {
			return err
		}
		data, err := readAsCaller(args[2])
		if err != nil {
			return err
		}
		var entries []string
		for _, line := range strings.Split(string(data), "\n") {
			if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
				entries = append(entries, line)
			}
		}
		if len(entries) == 0 {
			return fmt.Errorf("%s has no entries", args[2])
		}
		if args[1] == listBlocked {
			err = addBlockedEntries(cmd, database, args[0], entries)
		} else {
			err = addExceptions(cmd, database, args[0], entries)
		}
		if err != nil {
			return err
		}
		fmt.Printf("Imported %d entries into %s of '%s'.\n", len(entries), args[1], args[0])
		return nil
	}),
}

func init() {
	addNowFlag(blockAddEntryCmd, blockRemoveEntryCmd, blockAddExceptionCmd, blockImportListCmd)
	blockCmd.AddCommand(blockAddEntryCmd, blockRemoveEntryCmd, blockAddExceptionCmd,
		blockRemoveExceptionCmd, blockExportListCmd, blockImportListCmd)
}
