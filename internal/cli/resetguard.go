// resetguard.go — kc9k reset-guard: factory reset protection's installer
// media guard (§3.8). On is free; off needs the settings password and is
// refused while anything is locked, like tamper protection.
package cli

import (
	"fmt"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var resetGuardCmd = &cobra.Command{
	Use:   "reset-guard [on|off]",
	Short: "Factory reset protection: installer media guard while anything is locked",
	Long: `While this is on and any block or setting is locked, the pages that hand out
operating system images and USB image writers (Ventoy, Etcher, Rufus, the
distributions' download pages) are blocked, and image writers and disk
restore tools (USB Image Writer, Etcher, Popsicle, Startup Disk Creator,
Disks, …) are closed. Package mirrors stay open. It makes reinstalling slow
and deliberate; media made on another computer still work, and without a
firmware password the boot menu stays open.

Off is refused while anything is locked.`,
	Args: cobra.MaximumNArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 1 {
			switch args[0] {
			case "on":
			case "off":
				if err := requireSettingsPassword(database); err != nil {
					return err
				}
				now := time.Now()
				trusted, err := database.TrustedNow(now)
				if err != nil {
					return err
				}
				if locked, err := enforce.AnyLock(database, now, trusted); err != nil {
					return err
				} else if locked {
					return refused(database, "", fmt.Errorf("factory reset protection can't be turned off while a block or setting is locked"))
				}
			default:
				return fmt.Errorf("use 'on' or 'off'")
			}
			if err := database.SetSetting(enforce.SettingResetGuard, args[0]); err != nil {
				return err
			}
			if err := enforce.ApplyFromDB(database); err != nil {
				return err
			}
		}
		fmt.Printf("Installer media guard: %s (acts while anything is locked)\n", onOff(enforce.ResetGuardOn(database)))
		return nil
	}),
}

func init() {
	rootCmd.AddCommand(resetGuardCmd)
}
