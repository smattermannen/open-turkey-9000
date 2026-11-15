// freezeapps.go — kc9k freeze apps: the app list of "allowed apps only"
// freezes (§3.8, R-FREEZE-1). Adding an app weakens such a freeze: refused
// while a freeze runs or anything is locked; removing one is free.
package cli

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var freezeAppsCmd = &cobra.Command{
	Use:   "apps [add|remove APP...]",
	Short: "The programs an 'allowed apps only' freeze lets run (--mode apps)",
	Args:  cobra.ArbitraryArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		apps := enforce.FreezeApps(database)
		if len(args) == 0 {
			if len(apps) == 0 {
				fmt.Println("No allowed apps: an 'allowed apps only' freeze lets only the system run.")
			}
			for _, a := range apps {
				fmt.Println(a)
			}
			return nil
		}
		if len(args) < 2 {
			return fmt.Errorf("use 'add APP...' or 'remove APP...'")
		}
		switch args[0] {
		case "add":
			for _, a := range args[1:] {
				if err := blocker.ValidAppEntry(a); err != nil {
					return err
				}
			}
			if err := requireSettingsPassword(database); err != nil {
				return err
			}
			now := time.Now()
			trusted, err := database.TrustedNow(now)
			if err != nil {
				return err
			}
			until, _ := database.FreezeUntil()
			if locked, err := enforce.AnyLock(database, now, trusted); err != nil {
				return err
			} else if locked || now.Unix() < until {
				return refused(database, "", fmt.Errorf("an allowed app weakens 'allowed apps only': refused while a freeze runs or anything is locked"))
			}
			for _, a := range args[1:] {
				if !slices.Contains(apps, a) {
					apps = append(apps, a)
				}
			}
		case "remove":
			apps = slices.DeleteFunc(apps, func(a string) bool { return slices.Contains(args[1:], a) })
		default:
			return fmt.Errorf("use 'add APP...' or 'remove APP...'")
		}
		data, err := json.Marshal(apps)
		if err != nil {
			return err
		}
		if err := database.SetSetting(enforce.SettingFreezeApps, string(data)); err != nil {
			return err
		}
		fmt.Printf("Allowed apps: %d\n", len(apps))
		return nil
	}),
}

func init() {
	freezeCmd.AddCommand(freezeAppsCmd)
}
