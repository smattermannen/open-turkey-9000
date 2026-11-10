// weblists.go — a block's website lists: blacklists and exceptions.
//
//	kc9k list show <block> [list]
//	kc9k list create <block> <list> [--exceptions] [--url] [--title] [--text]
//	kc9k list rename <block> <list> <new name>
//	kc9k list delete <block> <list>
//	kc9k list add <block> <list> <entries...>
//	kc9k list remove <block> <list> <entries...>
//	kc9k list where <block> <list> url|title|text on|off
//	kc9k list action <block> <list> block|greyscale|no-images on|off
//	kc9k list threshold <block> <list> <action> <count>
//	kc9k list export <block> <list>
//	kc9k list import <block> <list> <file>
//
// Each list ticks where it looks: the address (url), the page title (the
// tab title) and the page text. A blacklist's actions apply to the pages it
// matches; an exception list spares the pages it matches from every
// blacklist of its block. What strengthens a block is always allowed: adding
// to a blacklist, ticking its places and actions, lowering its thresholds,
// and trimming exceptions. Everything else loosens it and is gated like any
// loosening edit (see gateEdit).
package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

// findList returns a block and its named list.
func findList(database *db.DB, block, list string) (*db.BlockDetail, db.WebList, error) {
	detail, err := database.GetBlock(block)
	if err != nil {
		return nil, db.WebList{}, err
	}
	l, ok := detail.Lists.Find(list)
	if !ok {
		var names []string
		for _, have := range detail.Lists {
			names = append(names, "'"+have.Name+"'")
		}
		return nil, db.WebList{}, fmt.Errorf("block '%s' has no list named '%s' (it has %s)", block, list, strings.Join(names, ", "))
	}
	return detail, l, nil
}

// prepareEntries checks that each entry works in at least one place the list
// looks, and normalizes domains for address matching.
func prepareEntries(where db.Where, entries []string) ([]string, error) {
	if !where.Any() {
		return nil, fmt.Errorf("the list doesn't look anywhere; tick url, title or text first")
	}
	out := make([]string, 0, len(entries))
	for _, raw := range entries {
		e := strings.TrimSpace(raw)
		if e == "" {
			continue
		}
		word := (where.Title || where.Text) && checkPageKeywords([]string{e}) == nil
		if where.URL && urlEntry(e) {
			e = normalizeURLEntry(e)
		} else if !word {
			return nil, fmt.Errorf("invalid entry %q: %s", raw, entryHelp(where))
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no entries given")
	}
	return out, nil
}

// urlEntry reports whether an entry can match addresses. A pattern needs
// more than wildcards, dots and slashes, except * and *.* (every site).
func urlEntry(e string) bool {
	if e != "*" && e != "*.*" && strings.Trim(e, "*./") == "" {
		return false
	}
	return blocker.IsURLEntry(normalizeURLEntry(e))
}

// normalizeURLEntry keeps a lone domain normalized (lower case, no scheme,
// www. or trailing slash), so it reaches /etc/hosts and the firewall and is
// found again when removed.
func normalizeURLEntry(e string) string {
	bare := e
	if i := strings.Index(bare, "://"); i > 0 {
		bare = bare[i+3:]
	}
	bare = strings.TrimSuffix(bare, "/")
	if bare != "" && !strings.ContainsAny(bare, "/*") {
		if host := blocker.NormalizeDomain(bare); blocker.IsValidDomain(host) {
			return host
		}
	}
	return e
}

func entryHelp(where db.Where) string {
	var parts []string
	if where.URL {
		parts = append(parts, "an address needs a domain (example.com), a path (example.com/x) or a pattern with * (*.xxx, *casino*)")
	}
	if where.Title || where.Text {
		parts = append(parts, fmt.Sprintf("a word or phrase is one line of at most %d characters, not only *", maxPageKeyword))
	}
	return strings.Join(parts, "; ")
}

// settle re-applies the layers after an edit, and deactivates a block the
// edit left with nothing to enforce.
func settle(database *db.DB, name string) error {
	detail, err := database.GetBlock(name)
	if err != nil {
		return err
	}
	if detail.Active && detail.Empty() {
		if err := database.DeactivateBlock(name); err != nil {
			return err
		}
	}
	return reapplyIfActive(database, name)
}

func listEdit(action string, l db.WebList, values ...string) db.Edit {
	return db.Edit{Action: action, List: l.ID, ListName: l.Name, Values: values}
}

// listAdd adds entries to a list: stricter for a blacklist, looser for an
// exception list.
func listAdd(cmd *cobra.Command, database *db.DB, block, list string, entries []string) error {
	_, l, err := findList(database, block, list)
	if err != nil {
		return err
	}
	entries, err = prepareEntries(l.Where, entries)
	if err != nil {
		return err
	}
	proceed, err := gateEdit(cmd, database, block, !l.IsExceptions(), listEdit(db.EditListAdd, l, entries...))
	if err != nil || !proceed {
		return err
	}
	if err := database.AddListEntries(l.ID, entries); err != nil {
		return err
	}
	return settle(database, block)
}

// listRemove removes entries from a list: looser for a blacklist, stricter
// for an exception list.
func listRemove(cmd *cobra.Command, database *db.DB, block, list string, entries []string) error {
	_, l, err := findList(database, block, list)
	if err != nil {
		return err
	}
	// Domains are kept normalized, so look them up the same way.
	for i, e := range entries {
		if !containsString(l.Entries, e) && l.Where.URL {
			entries[i] = normalizeURLEntry(e)
		}
	}
	if err := requireAll("entry", block, l.Entries, entries); err != nil {
		return err
	}
	proceed, err := gateEdit(cmd, database, block, l.IsExceptions(), listEdit(db.EditListRemove, l, entries...))
	if err != nil || !proceed {
		return err
	}
	if err := database.RemoveListEntries(l.ID, entries); err != nil {
		return err
	}
	return settle(database, block)
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// listWhere ticks or unticks a place.
func listWhere(cmd *cobra.Command, database *db.DB, block, list, place string, on bool) error {
	if !db.IsPlace(place) {
		return fmt.Errorf("unknown place %q (use %s)", place, strings.Join(db.Places, ", "))
	}
	_, l, err := findList(database, block, list)
	if err != nil {
		return err
	}
	if l.Where.Has(place) == on {
		return nil
	}
	next := l.Where.With(place, on)
	if !next.Any() {
		return fmt.Errorf("list '%s' needs to look somewhere; tick another place first", list)
	}
	value := "off"
	if on {
		value = "on"
	}
	// Looking in more places makes a blacklist stricter and an exception
	// list looser.
	proceed, err := gateEdit(cmd, database, block, on != l.IsExceptions(), listEdit(db.EditListWhere, l, place, value))
	if err != nil || !proceed {
		return err
	}
	if err := database.SetListWhere(l.ID, next); err != nil {
		return err
	}
	return settle(database, block)
}

// listAction turns a blacklist's action on or off.
func listAction(cmd *cobra.Command, database *db.DB, block, list, action string, on bool) error {
	if !db.IsAction(action) {
		return fmt.Errorf("unknown action %q (use %s)", action, strings.Join(db.ActionNames, ", "))
	}
	_, l, err := findList(database, block, list)
	if err != nil {
		return err
	}
	if l.IsExceptions() {
		return fmt.Errorf("'%s' is an exception list; only blacklists have actions", list)
	}
	if l.HasAction(action) == on {
		return nil
	}
	if !on && len(l.Actions) == 1 {
		return fmt.Errorf("list '%s' needs at least one action; turn another on first", list)
	}
	if !on {
		proceed, err := gateEdit(cmd, database, block, false, listEdit(db.EditListUnsetAction, l, action))
		if err != nil || !proceed {
			return err
		}
	}
	if err := database.SetListAction(l.ID, action, on); err != nil {
		return err
	}
	return settle(database, block)
}

// listThreshold sets how many different entries a page's text needs for an
// action. Raising it loosens the list.
func listThreshold(cmd *cobra.Command, database *db.DB, block, list, action string, n int) error {
	if !db.IsAction(action) {
		return fmt.Errorf("unknown action %q (use %s)", action, strings.Join(db.ActionNames, ", "))
	}
	if n < 1 || n > db.MaxThreshold {
		return fmt.Errorf("a threshold is a number from 1 to %d", db.MaxThreshold)
	}
	_, l, err := findList(database, block, list)
	if err != nil {
		return err
	}
	a, ok := l.Action(action)
	if !ok {
		return fmt.Errorf("list '%s' doesn't take the %s action; turn it on first", list, action)
	}
	if n == a.Distinct {
		return nil
	}
	proceed, err := gateEdit(cmd, database, block, n < a.Distinct, listEdit(db.EditListSetThreshold, l, action, strconv.Itoa(n)))
	if err != nil || !proceed {
		return err
	}
	if err := database.SetListThreshold(l.ID, action, n); err != nil {
		return err
	}
	return settle(database, block)
}

// listDelete removes a list: looser for a blacklist with entries.
func listDelete(cmd *cobra.Command, database *db.DB, block, list string) error {
	_, l, err := findList(database, block, list)
	if err != nil {
		return err
	}
	strengthens := l.IsExceptions() || len(l.Entries) == 0
	proceed, err := gateEdit(cmd, database, block, strengthens, listEdit(db.EditListDelete, l))
	if err != nil || !proceed {
		return err
	}
	if err := database.DeleteList(l.ID); err != nil {
		return err
	}
	return settle(database, block)
}

// readEntryFile reads a file with one entry per line; # starts a comment.
func readEntryFile(path string) ([]string, error) {
	data, err := readAsCaller(path)
	if err != nil {
		return nil, err
	}
	var entries []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			entries = append(entries, line)
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("%s has no entries", path)
	}
	return entries, nil
}

func parseOnOff(value string) (bool, error) {
	switch strings.ToLower(value) {
	case "on":
		return true, nil
	case "off":
		return false, nil
	}
	return false, fmt.Errorf("use 'on' or 'off'")
}

func describeList(l db.WebList) string {
	kind := "blacklist"
	if l.IsExceptions() {
		kind = "exceptions"
	}
	s := fmt.Sprintf("%s (%s) looks at: %s", l.Name, kind, l.Where)
	if !l.IsExceptions() {
		var acts []string
		for _, a := range l.Actions {
			if l.Where.Text {
				acts = append(acts, fmt.Sprintf("%s at %d", a.Name, a.Distinct))
			} else {
				acts = append(acts, a.Name)
			}
		}
		s += "; actions: " + strings.Join(acts, ", ")
	}
	return s
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Manage a block's website lists (blacklists and exceptions)",
	Long: `A block has blacklists and exception lists. Each list ticks where it looks:
the page's address (url), its title (the tab title) and its text.

Address entries are domains (reddit.com), paths (reddit.com/r/funny) or
patterns (*.xxx, *casino*, *.* for every site). Title and text entries are
words or phrases: a plain word counts only on its own ("porn" misses
"pornhub"); a * matches inside words ("*porn*" hits "pornhub").

A blacklist's actions (block, greyscale, no-images) apply to the pages it
matches. A match in the address or the title takes every action; in the
text, each action needs its threshold of different entries. An exception
list spares the pages it matches from all of its block's blacklists.

A block with exceptions by title or text is enforced by the extension only,
since only it reads a page before deciding.`,
}

var listShowCmd = &cobra.Command{
	Use:   "show [block] [list]",
	Short: "Show a block's lists, or one list's entries",
	Args:  cobra.RangeArgs(1, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 2 {
			_, l, err := findList(database, args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Println(describeList(l))
			for _, e := range l.Entries {
				fmt.Println("  " + e)
			}
			return nil
		}
		detail, err := database.GetBlock(args[0])
		if err != nil {
			return err
		}
		for _, l := range detail.Lists {
			fmt.Printf("%s; %d entries\n", describeList(l), len(l.Entries))
		}
		return nil
	}),
}

var listCreateCmd = &cobra.Command{
	Use:   "create [block] [list]",
	Short: "Add a blacklist (or, with --exceptions, an exception list) to a block",
	Long: `Adds an empty list. It looks at addresses unless --title or --text is
given (combine them freely). A new blacklist blocks what it matches; choose
other actions with 'kc9k list action'.`,
	Args: cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		exceptions, _ := cmd.Flags().GetBool("exceptions")
		var where db.Where
		where.URL, _ = cmd.Flags().GetBool("url")
		where.Title, _ = cmd.Flags().GetBool("title")
		where.Text, _ = cmd.Flags().GetBool("text")
		if !where.Any() {
			where.URL = true
		}
		kind := db.ListBlacklist
		if exceptions {
			kind = db.ListExceptions
		}
		if _, err := database.CreateList(args[0], kind, args[1], where); err != nil {
			return err
		}
		fmt.Printf("List '%s' added to block '%s' (looks at: %s).\n", args[1], args[0], where)
		return nil
	}),
}

var listRenameCmd = &cobra.Command{
	Use:   "rename [block] [list] [new name]",
	Short: "Rename a list",
	Args:  cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		_, l, err := findList(database, args[0], args[1])
		if err != nil {
			return err
		}
		return database.RenameList(l.ID, args[2])
	}),
}

var listDeleteCmd = &cobra.Command{
	Use:   "delete [block] [list]",
	Short: "Remove a list with its entries",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		return listDelete(cmd, database, args[0], args[1])
	}),
}

var listAddCmd = &cobra.Command{
	Use:   "add [block] [list] [entries...]",
	Short: "Add entries to a list",
	Args:  cobra.MinimumNArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		return listAdd(cmd, database, args[0], args[1], args[2:])
	}),
}

var listRemoveCmd = &cobra.Command{
	Use:   "remove [block] [list] [entries...]",
	Short: "Remove entries from a list",
	Args:  cobra.MinimumNArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		return listRemove(cmd, database, args[0], args[1], args[2:])
	}),
}

var listWhereCmd = &cobra.Command{
	Use:   "where [block] [list] [url|title|text] [on|off]",
	Short: "Choose where a list looks: the address, the page title, the page text",
	Args:  cobra.ExactArgs(4),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		on, err := parseOnOff(args[3])
		if err != nil {
			return err
		}
		return listWhere(cmd, database, args[0], args[1], strings.ToLower(args[2]), on)
	}),
}

var listActionCmd = &cobra.Command{
	Use:   "action [block] [list] [" + strings.Join(db.ActionNames, "|") + "] [on|off]",
	Short: "Choose what a blacklist does to the pages it matches",
	Args:  cobra.ExactArgs(4),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		on, err := parseOnOff(args[3])
		if err != nil {
			return err
		}
		return listAction(cmd, database, args[0], args[1], strings.ToLower(args[2]), on)
	}),
}

var listThresholdCmd = &cobra.Command{
	Use:   "threshold [block] [list] [action] [count]",
	Short: "How many different entries a page's text needs before an action applies",
	Long: `For a blacklist that looks at page text: how many of its different entries
a page must contain before the action applies (repeats count once). For
example greyscale at 1 and block at 3. Lowering it is always allowed; raising
it loosens the list.`,
	Args: cobra.ExactArgs(4),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		n, err := strconv.Atoi(args[3])
		if err != nil {
			return fmt.Errorf("a threshold is a number from 1 to %d", db.MaxThreshold)
		}
		return listThreshold(cmd, database, args[0], args[1], strings.ToLower(args[2]), n)
	}),
}

var listExportCmd = &cobra.Command{
	Use:   "export [block] [list]",
	Short: "Print a list's entries, one per line",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		_, l, err := findList(database, args[0], args[1])
		if err != nil {
			return err
		}
		for _, e := range l.Entries {
			fmt.Println(e)
		}
		return nil
	}),
}

var listImportCmd = &cobra.Command{
	Use:   "import [block] [list] [file]",
	Short: "Add entries from a text file with one entry per line",
	Args:  cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		entries, err := readEntryFile(args[2])
		if err != nil {
			return err
		}
		if err := listAdd(cmd, database, args[0], args[1], entries); err != nil {
			return err
		}
		fmt.Printf("Imported %d entries into '%s' of '%s'.\n", len(entries), args[1], args[0])
		return nil
	}),
}

func init() {
	listCreateCmd.Flags().Bool("exceptions", false, "Make it an exception list")
	listCreateCmd.Flags().Bool("url", false, "Look at the address (the default)")
	listCreateCmd.Flags().Bool("title", false, "Look at the page title")
	listCreateCmd.Flags().Bool("text", false, "Look at the page text")
	addNowFlag(listDeleteCmd, listAddCmd, listRemoveCmd, listWhereCmd, listActionCmd, listThresholdCmd, listImportCmd)
	listCmd.AddCommand(listShowCmd, listCreateCmd, listRenameCmd, listDeleteCmd, listAddCmd, listRemoveCmd,
		listWhereCmd, listActionCmd, listThresholdCmd, listExportCmd, listImportCmd)
	rootCmd.AddCommand(listCmd)
}
