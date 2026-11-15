// recover.go — kc9k recover: a forgotten settings password or password lock
// is removable from recovery mode unless no-escape is on (§3.11, R-ESCAPE-1).
// Only root in a recovery boot (recovery menu entry, single user, rescue or
// emergency target) may run it.
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

// inRecovery reports whether this boot is a recovery boot, from the kernel
// command line. A variable for tests.
var inRecovery = func() bool {
	data, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return false
	}
	for _, word := range strings.Fields(string(data)) {
		switch word {
		case "single", "recovery", "S", "1", "emergency", "systemd.unit=rescue.target", "systemd.unit=emergency.target":
			return true
		}
	}
	return false
}

// isRoot is os.Geteuid() == 0; a variable for tests.
var isRoot = func() bool { return os.Geteuid() == 0 }

var recoverCmd = &cobra.Command{
	Use:   "recover",
	Short: "From recovery mode: remove a forgotten settings password and password locks",
	Long: `Run as root after booting the recovery menu entry. It removes the settings
password and every password lock (on blocks and settings); other locks stay.
With --access it also removes the access passwords. It refuses while
no-escape is on: then there is no way out but the locks themselves.`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if !isRoot() {
			return fmt.Errorf("run it as root, from recovery mode")
		}
		if !inRecovery() {
			return fmt.Errorf("this is not a recovery boot: choose the recovery entry in the boot menu first")
		}
		if v, _ := database.GetSetting(enforce.SettingNoEscape); v == "on" {
			return fmt.Errorf("no-escape is on, so there is no recovery from here (R-ESCAPE-1)")
		}
		if err := database.SetSetting(SettingSettingsPassword, ""); err != nil {
			return err
		}
		blocks, err := database.ListBlocks()
		if err != nil {
			return err
		}
		removed := 0
		for _, b := range blocks {
			e, err := database.GetLockExtras(b.Name)
			if err != nil || e.PasswordHash == "" {
				continue
			}
			e.PasswordHash = ""
			if err := database.SetLockExtras(b.Name, e); err != nil {
				return err
			}
			removed++
		}
		locks, err := database.SettingLocks()
		if err != nil {
			return err
		}
		for _, l := range locks {
			if l.PasswordHash == "" {
				continue
			}
			l.PasswordHash = ""
			if err := database.SetSettingLock(l); err != nil {
				return err
			}
			removed++
		}
		if access, _ := cmd.Flags().GetBool("access"); access {
			for _, area := range accessAreas {
				if err := database.SetSetting(db.AccessPasswordSetting+area, ""); err != nil {
					return err
				}
			}
			fmt.Println("Access passwords removed.")
		}
		fmt.Printf("Settings password removed, and %d password locks.\n", removed)
		return nil
	}),
}

func init() {
	recoverCmd.Flags().Bool("access", false, "Also remove the access passwords")
	rootCmd.AddCommand(recoverCmd)
}
