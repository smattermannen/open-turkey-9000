// noweakening.go — the global setting "no weakening during active locks"
// (§3.5, R-WEAK-4): while any block or setting is locked, no setting can be
// weakened, strict mode included. Unlocked blocks stay freely editable.
//
//	kc9k no-weakening            # shows it
//	kc9k no-weakening on|off     # off is refused while anything is locked
package cli

import (
	"fmt"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

// SettingNoWeakening is "on" when no setting may be weakened while anything
// is locked.
const SettingNoWeakening = "no_weakening"

// refuseSettingWeakening refuses weakening the setting what while
// SettingNoWeakening is on and any block or setting is locked (R-WEAK-4).
// The refusal is logged like any refused weakening, never counted (R-WEAK-3).
func refuseSettingWeakening(database *db.DB, what string) error {
	on, err := database.GetSetting(SettingNoWeakening)
	if err != nil || on != "on" {
		return err
	}
	now := time.Now()
	trusted, err := database.TrustedNow(now)
	if err != nil {
		return err
	}
	locked, err := enforce.AnyLock(database, now, trusted)
	if err != nil || !locked {
		return err
	}
	return refused(database, "", fmt.Errorf("%s can't be weakened while a block or setting is locked (no weakening during active locks is on)", what))
}

var noWeakeningCmd = &cobra.Command{
	Use:       "no-weakening [on|off]",
	Short:     "No setting can be weakened while any block or setting is locked",
	ValidArgs: []string{"on", "off"},
	Long: `With no weakening during active locks on, no setting can be weakened while
any block or setting is locked: strict mode, browser settings, bypass
prevention, the settings password. Blocks that aren't locked stay freely
editable. Turning it off is refused while anything is locked.`,
	Args: cobra.MaximumNArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 0 {
			v, err := database.GetSetting(SettingNoWeakening)
			if err != nil {
				return err
			}
			if v != "on" {
				v = "off"
			}
			fmt.Printf("No weakening during active locks: %s\n", v)
			return nil
		}
		switch args[0] {
		case "on":
		case "off":
			if err := requireSettingsPassword(database); err != nil {
				return err
			}
			if err := refuseSettingWeakening(database, "no weakening during active locks"); err != nil {
				return err
			}
		default:
			return fmt.Errorf("use 'on' or 'off'")
		}
		if err := database.SetSetting(SettingNoWeakening, args[0]); err != nil {
			return err
		}
		fmt.Printf("No weakening during active locks: %s\n", args[0])
		return nil
	}),
}

func init() {
	rootCmd.AddCommand(noWeakeningCmd)
}
