// recover.go — kc9k recover: a forgotten settings password or password lock
// is removable from recovery mode unless no-escape is on (§3.11, R-ESCAPE-1).
// Only root in a recovery boot (recovery menu entry, single user, rescue or
// emergency target) may run it.
package cli

import (
	"fmt"
	"slices"

	"github.com/smattermannen/kc9k/internal/platform"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

// inRecovery reports whether this boot is a recovery boot: the kernel
// command line on Linux, Safe Mode on Windows (recover_*.go). A variable for
// tests.
var inRecovery = recoveryBoot

// isRoot is platform.Privileged; a variable for tests.
var isRoot = platform.Privileged

var recoverCmd = &cobra.Command{
	Use:   "recover",
	Short: "From recovery mode: remove a forgotten settings password and password locks",
	Long: recoverIntro + ` It removes the settings
password and every password lock (on blocks and settings); other locks stay.
With --access it also removes the access passwords. It refuses while
no-escape is on: then there is no way out but the locks themselves.`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if !isRoot() {
			return fmt.Errorf("%s", recoverNotRoot)
		}
		if !inRecovery() {
			return fmt.Errorf("%s", recoverNotRecovery)
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
		// Every password lock: in its place, waiting in the queue (several
		// of one kind, §3.4) and in what re-lock brings back.
		removed := 0
		for _, b := range blocks {
			e, err := database.GetLockExtras(b.Name)
			if err != nil {
				continue
			}
			n := countPasswordLocks(e.PasswordHash, e.Queue)
			if n == 0 && passwordLocks(e.Config) == 0 {
				continue
			}
			e.PasswordHash, e.Queue, e.Config = "", withoutPasswordLocks(e.Queue), withoutPasswordLocks(e.Config)
			if err := database.SetLockExtras(b.Name, e); err != nil {
				return err
			}
			removed += n
		}
		locks, err := database.SettingLocks()
		if err != nil {
			return err
		}
		for _, l := range locks {
			n := countPasswordLocks(l.PasswordHash, l.Queue)
			if n == 0 && passwordLocks(l.Config) == 0 {
				continue
			}
			l.PasswordHash, l.Queue, l.Config = "", withoutPasswordLocks(l.Queue), withoutPasswordLocks(l.Config)
			if err := database.SetSettingLock(l); err != nil {
				return err
			}
			removed += n
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

// countPasswordLocks counts the password locks on: the placed one and the
// queued ones.
func countPasswordLocks(placed string, queue []db.QueuedLock) int {
	n := passwordLocks(queue)
	if placed != "" {
		n++
	}
	return n
}

// withoutPasswordLocks is q without its password locks.
func withoutPasswordLocks(q []db.QueuedLock) []db.QueuedLock {
	return slices.DeleteFunc(slices.Clone(q), func(l db.QueuedLock) bool { return l.Kind == db.LayerPassword })
}

func init() {
	recoverCmd.Flags().Bool("access", false, "Also remove the access passwords")
	rootCmd.AddCommand(recoverCmd)
}
