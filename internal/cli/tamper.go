// tamper.go — tamper protection settings (§4.10).
//
//	kc9k tamper                 # shows them
//	kc9k tamper clock on|off    # block time and date changes
//
// They are protection settings: turning one on is always allowed; turning
// one off needs the settings password and is refused while any block or
// setting is locked (R-LOCK-9).
package cli

import (
	"fmt"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var tamperCmd = &cobra.Command{
	Use:   "tamper [clock] [on|off]",
	Short: "Tamper protection: ignore clock changes while a timer lock runs",
	Long: `Timer locks are measured in trusted time, which is the real clock unless
you turn tamper protection on:

  clock   while a timer lock runs, changing the date or time doesn't move it
          (time the computer is switched off isn't counted then either)

Turning it off is refused while any block or setting is locked.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 0 {
			v, err := database.GetSetting(db.SettingTamperClock)
			if err != nil {
				return err
			}
			if v != "on" {
				v = "off"
			}
			fmt.Printf("clock %s\n", v)
			return nil
		}
		if args[0] != "clock" {
			return fmt.Errorf("unknown tamper protection %q (use clock)", args[0])
		}
		if len(args) == 1 {
			v, err := database.GetSetting(db.SettingTamperClock)
			if err != nil {
				return err
			}
			fmt.Printf("clock %s\n", map[bool]string{true: "on", false: "off"}[v == "on"])
			return nil
		}
		switch args[1] {
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
				return refused(database, "", fmt.Errorf("tamper protection can't be turned off while a block or setting is locked"))
			}
		default:
			return fmt.Errorf("use 'on' or 'off'")
		}
		if err := database.SetSetting(db.SettingTamperClock, args[1]); err != nil {
			return err
		}
		fmt.Printf("Tamper protection for the clock is %s\n", args[1])
		return nil
	}),
}

func init() {
	rootCmd.AddCommand(tamperCmd)
}
