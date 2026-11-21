// installguard.go — the "prepare-install" guard: tamper protection "block
// the installer" (§4.10). The installed binary judges a new install before
// it is copied: 'make install' (and so install.sh) runs it whenever KC9K is
// already installed, and the package's prerm runs it for a reinstall or an
// older version. With the setting on and anything locked it refuses
// (non-zero exit), logged as a refusal. Removal is prepare-uninstall's.
package cli

import (
	"fmt"
	"time"

	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var prepareInstallCmd = &cobra.Command{
	Use:    "prepare-install",
	Short:  "Internal: refuse a reinstall or an older version while locked (tamper installer)",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()
		if !enforce.TamperOn(database, enforce.SettingTamperInstaller) {
			return nil
		}
		now := time.Now()
		trusted, err := database.TrustedNow(now)
		if err != nil {
			return err
		}
		locked, err := enforce.AnyLock(database, now, trusted)
		if err != nil {
			return err
		}
		if locked {
			return refused(database, "", fmt.Errorf("reinstalling Killer Chicken 9000 or installing an older version is refused while a block or setting is locked (tamper protection: block the installer)"))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(prepareInstallCmd)
}
