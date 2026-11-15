// templates.go — kc9k template (§3.2): save a block's definition (lists,
// phrase sets, apps, window titles, its bypass prevention) as a template,
// and create blocks from it ('block create X --template T'). Templates are
// the user's own; nothing is shipped.
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Saved block definitions to create blocks from",
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		names, err := database.Templates()
		if err != nil {
			return err
		}
		if len(names) == 0 {
			fmt.Println("No templates. Save one with 'kc9k template save BLOCK NAME'.")
		}
		for _, n := range names {
			fmt.Println(n)
		}
		return nil
	}),
}

var templateSaveCmd = &cobra.Command{
	Use:   "save [block] [template]",
	Short: "Save a block's definition as a template",
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
		fmt.Printf("Template '%s' saved from block '%s'.\n", args[1], args[0])
		return nil
	}),
}

var templateRemoveCmd = &cobra.Command{
	Use:   "remove [template]",
	Short: "Remove a template (blocks made from it stay)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if err := database.RemoveTemplate(args[0]); err != nil {
			return err
		}
		fmt.Printf("Template '%s' removed.\n", args[0])
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
		return fmt.Errorf("template '%s' is damaged: %w", template, err)
	}
	in.Name = name
	return importBlockExport(cmd, in)
}

func init() {
	templateCmd.AddCommand(templateSaveCmd, templateRemoveCmd)
	rootCmd.AddCommand(templateCmd)
}
