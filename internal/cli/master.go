//go:build devmaster

// master.go — a master password for testing. It only exists in builds made with
// TAGS=devmaster, so a normal build has no such command at all.
package cli

import (
	"fmt"
	"os"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

const masterHashPath = "/var/lib/kc9k/master.hash"

var masterCmd = &cobra.Command{
	Use:   "master",
	Short: "Testing only: lift every lock, and the settings password, with the master password from standard input",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		stored, err := os.ReadFile(masterHashPath)
		if err != nil {
			return fmt.Errorf("no master password set; run 'kc9k master-set' first")
		}
		password, err := readPasswordStdin("Master password")
		if err != nil {
			return err
		}
		if !db.CheckPassword(string(stored), password) {
			return fmt.Errorf("wrong master password")
		}

		quotas, err := database.QuotaBlocks()
		if err != nil {
			return err
		}
		for _, name := range quotas {
			if err := database.DeleteQuota(name); err != nil {
				return err
			}
		}
		active, err := database.GetActiveBlocks()
		if err != nil {
			return err
		}
		for _, b := range active {
			if err := database.DeactivateBlock(b.BlockName); err != nil {
				return err
			}
		}
		locks, err := database.SettingLocks()
		if err != nil {
			return err
		}
		for _, l := range locks {
			if err := database.ClearSettingLock(l.Name); err != nil {
				return err
			}
		}
		freezeSchedules, err := database.ListFreezeSchedules()
		if err != nil {
			return err
		}
		for _, s := range freezeSchedules {
			if err := database.RemoveFreezeSchedule(s.ID); err != nil {
				return err
			}
		}
		if err := database.SetFreeze(0, "lock"); err != nil {
			return err
		}
		if err := database.SetSetting(SettingSettingsPassword, ""); err != nil {
			return err
		}
		if err := database.SetSetting(enforce.SettingStrict, "off"); err != nil {
			return err
		}
		if err := database.SetSetting(enforce.SettingNoEscape, "off"); err != nil {
			return err
		}
		if err := blocker.RemoveLockdown(); err != nil {
			return err
		}
		if err := blocker.RemoveNoexec(); err != nil {
			return err
		}
		if err := blocker.RemoveBootLock(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: boot lock: %v\n", err)
		}
		if err := enforce.ApplyFromDB(database); err != nil {
			return err
		}
		fmt.Println("All locks lifted.")
		if _, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS); err == nil {
			fmt.Println("On a text console: type 'exit', then press Ctrl+Alt+F7 (or Ctrl+Alt+F1) to get back to the desktop.")
		}
		return nil
	}),
}

var masterSetCmd = &cobra.Command{
	Use:   "master-set",
	Short: "Testing only: set the master password once, from standard input",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat(masterHashPath); err == nil {
			return fmt.Errorf("the master password is already set")
		}
		password, err := readPasswordStdin("New master password")
		if err != nil {
			return err
		}
		hash, err := db.HashPassword(password)
		if err != nil {
			return err
		}
		return os.WriteFile(masterHashPath, []byte(hash), 0600)
	},
}

func init() {
	rootCmd.AddCommand(masterCmd, masterSetCmd)
}
