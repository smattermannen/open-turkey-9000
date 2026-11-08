// blockextra.go — additional block subcommands: browser patterns, mode
// switching, and JSON export/import.
//
//	kc9k block add-pattern <block> '*casino*' '*://*/*ads*'
//	kc9k block remove-pattern <block> '*casino*'
//	kc9k block set-mode <block> allowlist|blocklist
//	kc9k block action <block> block|greyscale|no-images on|off
//	kc9k block export <block>            # prints JSON to stdout
//	kc9k block import <file>             # creates a block from JSON
//
// Patterns are wildcard/keyword URL filters enforced ONLY at the browser layer
// (they can't be expressed in /etc/hosts or iptables).
package cli

import (
	"encoding/json"
	"fmt"
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

// blockExport is the JSON shape for export/import.
type blockExport struct {
	Name       string   `json:"name"`
	Mode       string   `json:"mode"`
	Actions    []string `json:"actions,omitempty"`
	Sites      []string `json:"sites"`
	Apps       []string `json:"apps"`
	Patterns   []string `json:"patterns"`
	Exceptions []string `json:"exceptions,omitempty"`
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

var blockAddPatternCmd = &cobra.Command{
	Use:   "add-pattern [block] [patterns...]",
	Short: "Add browser URL patterns (wildcards/keywords) to a block",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		patterns := args[1:]
		for _, p := range patterns {
			if !blocker.IsValidPattern(p) {
				fmt.Fprintf(os.Stderr, "Error: invalid pattern %q\n", p)
				os.Exit(1)
			}
		}

		database, err := openDB()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		// Patterns are blocked in both modes, so adding one only strengthens
		// the block: no ensureEditable/gateEdit check needed.
		if err := database.AddPatterns(name, patterns); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := reapplyIfActive(database, name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Patterns added to block '%s'.\n", name)
	},
}

var blockRemovePatternCmd = &cobra.Command{
	Use:   "remove-pattern [block] [patterns...]",
	Short: "Remove browser URL patterns from a block",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		patterns := args[1:]

		database, err := openDB()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		detail, err := database.GetBlock(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := requireAll("pattern", name, detail.Patterns, patterns); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		proceed, err := gateEdit(cmd, database, name, false, db.Edit{Action: db.EditRemovePatterns, Values: patterns})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if !proceed {
			return
		}
		if err := database.RemovePatterns(name, patterns); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := reapplyIfActive(database, name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Patterns removed from block '%s'.\n", name)
	},
}

var blockSetModeCmd = &cobra.Command{
	Use:   "set-mode [block] [blocklist|allowlist]",
	Short: "Change a block's mode",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name, mode := args[0], args[1]

		database, err := openDB()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		if mode != db.ModeBlocklist && mode != db.ModeAllowlist {
			fmt.Fprintf(os.Stderr, "Error: invalid block mode '%s' (use '%s' or '%s')\n", mode, db.ModeBlocklist, db.ModeAllowlist)
			os.Exit(1)
		}
		proceed, err := gateEdit(cmd, database, name, false, db.Edit{Action: db.EditSetMode, Values: []string{mode}})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if !proceed {
			return
		}
		if err := database.SetBlockMode(name, mode); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := reapplyIfActive(database, name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Block '%s' mode set to %s.\n", name, mode)
	},
}

var blockActionCmd = &cobra.Command{
	Use:   "action [block] [" + strings.Join(db.ActionNames, "|") + "] [on|off]",
	Short: "Choose what a block does to its websites: block, greyscale, no-images",
	Long: `Every block can block its websites, show them in greyscale, block their
images, or any combination. In allowlist mode the actions apply to every site
except the listed ones. Apps are always blocked.

Greyscale and no-images work in browsers with the Killer Chicken extension.
Turning an action on is always allowed. Turning one off loosens the block, so
it is refused while the block is held, and queued for a block with allow
windows (or applied with --now after the typing challenge). At least one
action stays on. With only the block name, prints its actions.`,
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
			for _, action := range db.ActionNames {
				state := "off"
				if detail.Actions.Has(action) {
					state = "on"
				}
				fmt.Printf("%s %s\n", action, state)
			}
			return nil
		}
		action, value := strings.ToLower(args[1]), strings.ToLower(args[2])
		if !db.IsAction(action) {
			return fmt.Errorf("unknown action %q (use %s)", args[1], strings.Join(db.ActionNames, ", "))
		}
		if value != "on" && value != "off" {
			return fmt.Errorf("use 'on' or 'off'")
		}
		on := value == "on"
		if detail.Actions.Has(action) == on {
			fmt.Printf("Block '%s' already has %s %s.\n", name, action, value)
			return nil
		}
		next := detail.Actions.With(action, on)
		if !next.Any() {
			return fmt.Errorf("block '%s' needs at least one action; turn another on first", name)
		}
		if !on {
			proceed, err := gateEdit(cmd, database, name, false, db.Edit{Action: db.EditUnsetAction, Values: []string{action}})
			if err != nil || !proceed {
				return err
			}
		}
		if err := database.SetBlockActions(name, next); err != nil {
			return err
		}
		if err := reapplyIfActive(database, name); err != nil {
			return err
		}
		fmt.Printf("Block '%s' now does: %s.\n", name, next)
		return nil
	}),
}

var blockExportCmd = &cobra.Command{
	Use:   "export [block]",
	Short: "Print a block as JSON (redirect to a file to save it)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		detail, err := database.GetBlock(args[0])
		if err != nil {
			return err
		}
		out := blockExport{
			Name:       detail.Name,
			Mode:       detail.Mode,
			Actions:    detail.Actions.List(),
			Sites:      detail.Sites,
			Apps:       detail.Apps,
			Patterns:   detail.Patterns,
			Exceptions: detail.Exceptions,
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	},
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
		in.Mode = strings.ToLower(strings.TrimSpace(in.Mode))
		if in.Mode == "" {
			in.Mode = db.ModeBlocklist
		}
		if in.Mode != db.ModeBlocklist && in.Mode != db.ModeAllowlist {
			return fmt.Errorf("invalid mode %q in import (use '%s' or '%s')", in.Mode, db.ModeBlocklist, db.ModeAllowlist)
		}

		actions, err := db.ParseActions(in.Actions)
		if err != nil {
			return err
		}

		// Validate before writing anything.
		sites, err := normalizeDomains(in.Sites)
		if err != nil {
			return err
		}
		for _, p := range in.Patterns {
			if !blocker.IsValidPattern(p) {
				return fmt.Errorf("invalid pattern %q in import", p)
			}
		}
		for _, app := range in.Apps {
			if err := blocker.ValidAppEntry(app); err != nil {
				return fmt.Errorf("in import: %w", err)
			}
		}

		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		if err := database.CreateBlock(in.Name, in.Mode); err != nil {
			return err
		}
		if err := database.SetBlockActions(in.Name, actions); err != nil {
			return err
		}
		if len(sites) > 0 {
			if err := database.AddSites(in.Name, sites); err != nil {
				return err
			}
		}
		if len(in.Apps) > 0 {
			if err := database.AddApps(in.Name, in.Apps); err != nil {
				return err
			}
		}
		if len(in.Exceptions) > 0 {
			if err := database.AddExceptions(in.Name, in.Exceptions); err != nil {
				return err
			}
		}
		if len(in.Patterns) > 0 {
			if err := database.AddPatterns(in.Name, in.Patterns); err != nil {
				return err
			}
		}
		fmt.Printf("Imported block '%s' (%d sites, %d apps, %d patterns).\n",
			in.Name, len(sites), len(in.Apps), len(in.Patterns))
		return nil
	},
}

func init() {
	addNowFlag(blockRemovePatternCmd, blockSetModeCmd, blockActionCmd)

	blockCmd.AddCommand(blockAddPatternCmd)
	blockCmd.AddCommand(blockRemovePatternCmd)
	blockCmd.AddCommand(blockSetModeCmd)
	blockCmd.AddCommand(blockActionCmd)
	blockCmd.AddCommand(blockExportCmd)
	blockCmd.AddCommand(blockImportCmd)
}

// readAsCaller reads a user-supplied path with the invoking user's permissions.
// The CLI runs as root through sudo, so a plain read could reach files the user
// may not open. Without SUDO_UID the process really is the user or root.
func readAsCaller(path string) ([]byte, error) {
	uidStr, gidStr := os.Getenv("SUDO_UID"), os.Getenv("SUDO_GID")
	if uidStr == "" || gidStr == "" || os.Geteuid() != 0 {
		return os.ReadFile(path)
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
	data, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("cannot read file as user %d: %w", uid, err)
	}
	return data, nil
}
