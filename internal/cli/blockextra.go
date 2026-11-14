// blockextra.go — additional block subcommands: the action shortcut, and
// JSON export/import.
//
//	kc9k block action <block> block|greyscale|no-images on|off
//	kc9k block export <block>            # prints JSON to stdout
//	kc9k block import <file>             # creates a block from JSON
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

// listExport is one website list in an export.
type listExport struct {
	Name    string          `json:"name"`
	Kind    string          `json:"kind"`
	Where   []string        `json:"where"`
	Actions []db.ListAction `json:"actions,omitempty"`
	Entries []string        `json:"entries"`
}

// blockExport is the JSON shape for export/import. Mode, Actions, Sites,
// Patterns, Exceptions, Keywords and Thresholds are read from exports made
// before website lists, and turned into lists as the database did.
type blockExport struct {
	Name  string       `json:"name"`
	Lists []listExport `json:"lists,omitempty"`
	// PhraseSets: R-PHRASE-10.
	PhraseSets []phraseSetExport `json:"phrase_sets,omitempty"`
	Apps       []string          `json:"apps"`
	Titles     []string          `json:"titles,omitempty"`
	Nepsis     *db.Nepsis        `json:"nepsis,omitempty"`

	Mode       string         `json:"mode,omitempty"`
	Actions    []string       `json:"actions,omitempty"`
	Sites      []string       `json:"sites,omitempty"`
	Patterns   []string       `json:"patterns,omitempty"`
	Exceptions []string       `json:"exceptions,omitempty"`
	Keywords   []string       `json:"page_keywords,omitempty"`
	Thresholds *db.Thresholds `json:"keyword_thresholds,omitempty"`
}

// legacyLists turns an export from before website lists into lists.
func (in blockExport) legacyLists() ([]listExport, error) {
	actions, err := db.ParseActions(in.Actions)
	if err != nil {
		return nil, err
	}
	thresholds := db.DefaultThresholds
	if in.Thresholds != nil {
		thresholds = *in.Thresholds
	}
	listActions := func(withThresholds bool) []db.ListAction {
		var out []db.ListAction
		for _, name := range actions.List() {
			n := 1
			if withThresholds && thresholds.Get(name) >= 1 {
				n = thresholds.Get(name)
			}
			out = append(out, db.ListAction{Name: name, Distinct: n, Hits: 1})
		}
		return out
	}
	blocked := append(append([]string{}, in.Sites...), in.Patterns...)
	allowed := append([]string{}, in.Exceptions...)
	if strings.EqualFold(strings.TrimSpace(in.Mode), db.ModeAllowlist) {
		blocked = append([]string{"*.*"}, in.Patterns...)
		allowed = append(append([]string{}, in.Sites...), in.Exceptions...)
	}
	out := []listExport{
		{Name: db.DefaultBlockedList, Kind: db.ListBlacklist, Where: []string{db.PlaceURL}, Actions: listActions(false), Entries: blocked},
		{Name: db.DefaultExceptionsList, Kind: db.ListExceptions, Where: []string{db.PlaceURL}, Entries: allowed},
	}
	if len(in.Keywords) > 0 {
		out = append(out, listExport{Name: db.DefaultWordsList, Kind: db.ListBlacklist, Where: []string{db.PlaceText},
			Actions: listActions(true), Entries: in.Keywords})
	}
	return out, nil
}

// toLists validates exported lists and turns them into database lists.
func toLists(in []listExport) (db.Lists, error) {
	var out db.Lists
	for _, le := range in {
		if err := db.CheckListName(le.Name); err != nil {
			return nil, err
		}
		if le.Kind != db.ListBlacklist && le.Kind != db.ListExceptions {
			return nil, fmt.Errorf("list '%s' has an unknown kind %q", le.Name, le.Kind)
		}
		l := db.WebList{Name: le.Name, Kind: le.Kind}
		for _, place := range le.Where {
			if !db.IsPlace(place) {
				return nil, fmt.Errorf("list '%s' looks at an unknown place %q", le.Name, place)
			}
			l.Where = l.Where.With(place, true)
		}
		if !l.Where.Any() {
			l.Where.URL = true
		}
		if len(le.Entries) > 0 {
			entries, err := prepareEntries(l.Where, le.Entries)
			if err != nil {
				return nil, fmt.Errorf("list '%s': %w", le.Name, err)
			}
			l.Entries = entries
		}
		if le.Kind == db.ListBlacklist {
			for _, a := range le.Actions {
				if !db.IsAction(a.Name) {
					return nil, fmt.Errorf("list '%s' has an unknown action %q", le.Name, a.Name)
				}
				if a.Distinct < 1 || a.Distinct > db.MaxThreshold {
					a.Distinct = 1
				}
				if a.Hits < 1 || a.Hits > db.MaxThreshold {
					a.Hits = 1
				}
				if err := db.CheckActionParams(a.Name, a.Param, a.Minutes); err != nil {
					return nil, fmt.Errorf("list '%s': %w", le.Name, err)
				}
				l.Actions = append(l.Actions, a)
			}
			if len(l.Actions) == 0 {
				l.Actions = []db.ListAction{{Name: db.ActionBlock, Distinct: 1, Hits: 1}}
			}
		}
		out = append(out, l)
	}
	return out, nil
}

// reapplyIfActive re-applies all layers when the named block is currently
// active, so an edit takes effect immediately.
func reapplyIfActive(database *db.DB, name string) error {
	active, err := database.IsBlockActive(name)
	if err != nil {
		return err
	}
	if active {
		return enforce.ApplyFromDB(database)
	}
	return nil
}

var blockActionCmd = &cobra.Command{
	Use:   "action [block] [" + strings.Join(db.ActionNames, "|") + "] [on|off]",
	Short: "Turn an action on or off for all of a block's blacklists",
	Long: `A shortcut for 'kc9k list action' on every blacklist of the block: block
the pages they match, show them in greyscale, block their images, or any
combination. Apps are always blocked.

Turning an action on is always allowed. Turning one off weakens the block, so
it is refused while the block is locked. Each list keeps
at least one action. With only the block name, prints each list's actions.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 && len(args) != 3 {
			return fmt.Errorf("give a block, or a block, an action and on|off")
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
			for _, l := range detail.Lists.Blacklists() {
				fmt.Printf("%s: %s\n", l.Name, strings.Join(l.ActionNames(), ", "))
			}
			return nil
		}
		action := strings.ToLower(args[1])
		on, err := parseOnOff(args[2])
		if err != nil {
			return err
		}
		if err := db.CheckActionParams(action, "", 0); err != nil && on {
			return fmt.Errorf("%v; set it per list with 'kc9k list action'", err)
		}
		for _, l := range detail.Lists.Blacklists() {
			if err := listAction(cmd, database, name, l.Name, action, on, nil); err != nil {
				return fmt.Errorf("list '%s': %w", l.Name, err)
			}
		}
		return nil
	}),
}

var blockExportCmd = &cobra.Command{
	Use:   "export [block]",
	Short: "Print a block as JSON (redirect to a file to save it)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		detail, err := database.GetBlock(args[0])
		if err != nil {
			return err
		}
		out := blockExport{Name: detail.Name, Apps: detail.Apps, Titles: detail.Titles, Lists: []listExport{}}
		if n, err := database.BlockNepsis(detail.Name); err != nil {
			return err
		} else if n.On() {
			out.Nepsis = &n
		}
		if out.Apps == nil {
			out.Apps = []string{}
		}
		for _, l := range detail.Lists {
			out.Lists = append(out.Lists, listExport{Name: l.Name, Kind: l.Kind, Where: l.Where.List(),
				Actions: l.Actions, Entries: l.Entries})
		}
		if out.PhraseSets, err = exportPhraseSets(database, detail.Name); err != nil {
			return err
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}),
}

var blockImportCmd = &cobra.Command{
	Use:   "import [file]",
	Short: "Create a block from a JSON file (from 'block export')",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := readAsCaller(args[0])
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", args[0], err)
		}
		var in blockExport
		if err := json.Unmarshal(data, &in); err != nil {
			return fmt.Errorf("failed to parse %s: %w", args[0], err)
		}
		if in.Name == "" {
			return fmt.Errorf("import file has no block name")
		}
		exported := in.Lists
		if exported == nil {
			if exported, err = in.legacyLists(); err != nil {
				return err
			}
		}

		// Validate before writing anything.
		lists, err := toLists(exported)
		if err != nil {
			return fmt.Errorf("in import: %w", err)
		}
		phraseSets, err := toPhraseSets(in.PhraseSets)
		if err != nil {
			return fmt.Errorf("in import: %w", err)
		}
		for _, app := range in.Apps {
			if err := blocker.ValidAppEntry(app); err != nil {
				return fmt.Errorf("in import: %w", err)
			}
		}
		if err := checkTitleKeywords(in.Titles); err != nil {
			return fmt.Errorf("in import: %w", err)
		}
		if in.Nepsis != nil && in.Nepsis.On() {
			if err := in.Nepsis.Check(); err != nil {
				return fmt.Errorf("in import: bypass prevention: %w", err)
			}
		}

		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		if err := database.CreateBlock(in.Name); err != nil {
			return err
		}
		if err := database.SetLists(in.Name, lists); err != nil {
			return err
		}
		if err := addPhraseSets(database, in.Name, phraseSets); err != nil {
			return err
		}
		if len(in.Apps) > 0 {
			if err := database.AddApps(in.Name, in.Apps); err != nil {
				return err
			}
		}
		if len(in.Titles) > 0 {
			if err := database.AddTitles(in.Name, in.Titles); err != nil {
				return err
			}
		}
		if in.Nepsis != nil {
			if err := database.SetBlockNepsis(in.Name, *in.Nepsis); err != nil {
				return err
			}
		}
		fmt.Printf("Imported block '%s' (%d lists, %d phrase sets, %d apps).\n", in.Name, len(lists), len(phraseSets), len(in.Apps))
		return nil
	},
}

func init() {
	addNowFlag(blockActionCmd)
	blockCmd.AddCommand(blockActionCmd)
	blockCmd.AddCommand(blockExportCmd)
	blockCmd.AddCommand(blockImportCmd)
}

// maxImportSize bounds imported files: a link to /dev/zero mustn't keep the
// root process reading forever.
const maxImportSize = 4 << 20

// readAsCaller reads a user-supplied path with the invoking user's permissions.
// The CLI runs as root through sudo, so a plain read could reach files the user
// may not open. Without SUDO_UID the process really is the user or root.
func readAsCaller(path string) ([]byte, error) {
	return readAsCallerUpTo(path, maxImportSize)
}

// readAsCallerUpTo is readAsCaller for files of up to limit bytes.
func readAsCallerUpTo(path string, limit int64) ([]byte, error) {
	uidStr, gidStr := os.Getenv("SUDO_UID"), os.Getenv("SUDO_GID")
	if uidStr == "" || gidStr == "" || os.Geteuid() != 0 {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return readLimited(f, limit)
	}
	uid, err := strconv.ParseUint(uidStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid SUDO_UID: %w", err)
	}
	gid, err := strconv.ParseUint(gidStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid SUDO_GID: %w", err)
	}
	c := exec.Command("/bin/cat", "--", path)
	c.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}}
	out, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("cannot read file as user %d: %w", uid, err)
	}
	data, readErr := readLimited(out, limit)
	_ = c.Process.Kill() // stops cat if the file was too large
	waitErr := c.Wait()
	if readErr != nil {
		return nil, readErr
	}
	if waitErr != nil && len(data) == 0 {
		return nil, fmt.Errorf("cannot read file as user %d: %w", uid, waitErr)
	}
	return data, nil
}

// readLimited reads at most maxImportSize bytes, failing if there are more.
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("the file is larger than %d MiB", limit>>20)
	}
	return data, nil
}
