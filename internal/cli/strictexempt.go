// strictexempt.go — strict mode's exempt folders (§3.8): during lockdown
// nothing runs from /home, /tmp, /var/tmp or /dev/shm except from these.
// Adding one weakens strict mode: settings password, refused while strict
// mode is locked or no weakening holds. Removing one is free.
//
//	kc9k strict exempt                 # lists them
//	kc9k strict exempt add DIR
//	kc9k strict exempt remove DIR
package cli

import (
	"fmt"
	"slices"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var strictExemptCmd = &cobra.Command{
	Use:   "exempt [add|remove DIR]",
	Short: "Folders programs may still start from during strict mode's lockdown",
	Args:  cobra.RangeArgs(0, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		dirs := enforce.ExemptFolders(database)
		if len(args) == 0 {
			if len(dirs) == 0 {
				fmt.Println("No exempt folders.")
			}
			for _, d := range dirs {
				fmt.Println(d)
			}
			return nil
		}
		if len(args) != 2 {
			return fmt.Errorf("use 'add DIR' or 'remove DIR'")
		}
		dir := args[1]
		switch args[0] {
		case "add":
			if err := blocker.ExemptFolderError(dir); err != nil {
				return err
			}
			if slices.Contains(dirs, dir) {
				return nil
			}
			if err := requireSettingsPassword(database); err != nil {
				return err
			}
			s, err := loadSettingLock(database, enforce.StrictLockName)
			if err != nil {
				return err
			}
			if s.lock.InForce(s.trusted, s.bootID) {
				return refused(database, "", fmt.Errorf("strict mode is locked; an exempt folder weakens it"))
			}
			if err := refuseSettingWeakening(database, "strict mode's exempt folders"); err != nil {
				return err
			}
			dirs = append(dirs, dir)
		case "remove":
			if !slices.Contains(dirs, dir) {
				return fmt.Errorf("%s isn't exempt", dir)
			}
			dirs = slices.DeleteFunc(dirs, func(d string) bool { return d == dir })
			if err := blocker.UnexemptFolder(dir); err != nil {
				fmt.Printf("Note: %v\n", err)
			}
		default:
			return fmt.Errorf("use 'add DIR' or 'remove DIR'")
		}
		if err := enforce.SetExemptFolders(database, dirs); err != nil {
			return err
		}
		fmt.Printf("Exempt folders: %d\n", len(dirs))
		return nil
	}),
}

func init() {
	strictCmd.AddCommand(strictExemptCmd)
}
