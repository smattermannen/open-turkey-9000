// templates.go — kc9k block-preset (§3.2; old name 'template', kept as an
// alias, §4.12): save a block's definition (lists, phrase sets, apps, window
// titles, its bypass prevention) as a block preset, and create blocks from
// it ('block create X --preset P'). Block presets are the user's own;
// nothing is shipped. In the code and DB they are "templates".
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagAliases lets a command take old flag names (§4.12: command and flag
// names follow the glossary, old ones stay as aliases), e.g. --template for
// --preset. Help shows only the new name.
func flagAliases(cmd *cobra.Command, old map[string]string) {
	cmd.Flags().SetNormalizeFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		if n, ok := old[name]; ok {
			name = n
		}
		return pflag.NormalizedName(name)
	})
}

var templateCmd = &cobra.Command{
	Use:     "block-preset",
	Aliases: []string{"template"},
	Short:   "Block presets: saved block definitions to create blocks from",
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		names, err := database.Templates()
		if err != nil {
			return err
		}
		if len(names) == 0 {
			fmt.Println("No block presets. Save one with 'kc9k block-preset save BLOCK NAME'.")
		}
		for _, n := range names {
			fmt.Println(n)
		}
		return nil
	}),
}

var templateSaveCmd = &cobra.Command{
	Use:   "save [block] [preset]",
	Short: "Save a block's definition as a block preset",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if err := revealBlock(database, args[0]); err != nil {
			return err
		}
		out, err := buildBlockExport(database, args[0])
		if err != nil {
			return err
		}
		out.Name = ""
		data, err := json.Marshal(out)
		if err != nil {
			return err
		}
		if err := database.SaveTemplate(args[1], string(data)); err != nil {
			return err
		}
		fmt.Printf("Block preset '%s' saved from block '%s'.\n", args[1], args[0])
		return nil
	}),
}

var templateRemoveCmd = &cobra.Command{
	Use:   "remove [preset]",
	Short: "Remove a block preset (blocks made from it stay)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if err := database.RemoveTemplate(args[0]); err != nil {
			return err
		}
		fmt.Printf("Block preset '%s' removed.\n", args[0])
		return nil
	}),
}

// createFromTemplate makes block name from a template (inactive).
func createFromTemplate(cmd *cobra.Command, database *db.DB, name, template string) error {
	data, err := database.Template(template)
	if err != nil {
		return err
	}
	var in blockExport
	if err := json.Unmarshal([]byte(data), &in); err != nil {
		return fmt.Errorf("block preset '%s' is damaged: %w", template, err)
	}
	in.Name = name
	return importBlockExport(cmd, in)
}

func init() {
	templateCmd.AddCommand(templateSaveCmd, templateRemoveCmd)
	rootCmd.AddCommand(templateCmd)
}
