// strict.go — strict mode and reading a password without exposing it.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/brunodcdo/open-turkey/internal/db"
	"github.com/brunodcdo/open-turkey/internal/enforce"
	"github.com/spf13/cobra"
)

var strictCmd = &cobra.Command{
	Use:       "strict [on|off]",
	Short:     "Take away sudo and pkexec while a block is locked",
	ValidArgs: []string{"on", "off"},
	Long: `With strict mode on, the daemon denies sudo and pkexec for every user while
a block is locked or Frozen Turkey runs. Only open-turkey itself keeps working.
Rights come back by themselves when the lock ends, and strict mode can't be
turned off during a lock.

If something goes wrong, add openturkey.nofreeze to the kernel line in GRUB,
or remove /etc/sudoers.d/zz-open-turkey-lockdown from recovery mode.`,
	Args: cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		switch args[0] {
		case "on":
			if err := database.SetSetting(enforce.SettingStrictUser, os.Getenv("SUDO_USER")); err != nil {
				return err
			}
		case "off":
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
				return fmt.Errorf("strict mode can't be turned off during a lock")
			}
		default:
			return fmt.Errorf("use 'on' or 'off'")
		}
		if err := database.SetSetting(enforce.SettingStrict, args[0]); err != nil {
			return err
		}
		fmt.Printf("Strict mode is %s\n", args[0])
		return nil
	}),
}

// readPasswordStdin reads one line, so a password never shows in a process list.
func readPasswordStdin() (string, error) {
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	password := strings.TrimRight(line, "\r\n")
	if password == "" {
		if err != nil {
			return "", fmt.Errorf("failed to read password: %w", err)
		}
		return "", fmt.Errorf("the password is empty")
	}
	return password, nil
}

func init() {
	rootCmd.AddCommand(strictCmd)
}
