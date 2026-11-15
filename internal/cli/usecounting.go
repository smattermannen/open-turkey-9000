// usecounting.go — kc9k use-counting: strict use counting (§3.7, §4.10).
// Strict, an allowance session spends while any tab of its sites is open
// and while its apps run at all, not only in front. Going back to normal
// weakens (R-WEAK-4).
package cli

import (
	"fmt"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var useCountingCmd = &cobra.Command{
	Use:   "use-counting [normal|strict]",
	Short: "How allowance use is counted: normal (in front) or strict (background tabs, minimised apps)",
	Args:  cobra.RangeArgs(0, 1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 0 {
			if enforce.StrictUseOn(database) {
				fmt.Println("strict")
			} else {
				fmt.Println("normal")
			}
			return nil
		}
		switch args[0] {
		case "strict":
			if err := database.SetSetting(enforce.SettingStrictUse, "on"); err != nil {
				return err
			}
		case "normal":
			if enforce.StrictUseOn(database) {
				if err := requireSettingsPassword(database); err != nil {
					return err
				}
				if err := refuseSettingWeakening(database, "strict use counting"); err != nil {
					return err
				}
			}
			if err := database.SetSetting(enforce.SettingStrictUse, "off"); err != nil {
				return err
			}
		default:
			return fmt.Errorf("use normal or strict")
		}
		fmt.Printf("Use counting is %s\n", args[0])
		return nil
	}),
}

func init() {
	rootCmd.AddCommand(useCountingCmd)
}
