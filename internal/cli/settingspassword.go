// settingspassword.go — a password that guards KC9K's settings.
//
// kc9k runs as root without a password through sudo, so anything running as
// you could use it: loosen a block, or lock you out for a year with no-escape.
// With a settings password set, these ask for it:
//
//   - every loosening edit (anything gateEdit or ensureEditable sees as not
//     strengthening), stop and unlock;
//   - anything that creates a lock: start with a lock, browser options with
//     a lock, schedules, quotas, Pomodoro, Frozen Chicken;
//   - browser options turned off or unlocked, strict mode, no-escape, and
//     uninstalling;
//   - changing or removing the password itself.
//
// Everything else stays free: reading the state, starting a block without a
// lock, adding sites and apps, opening a quota session.
//
//	kc9k password set       # asks for the current one first, if set
//	kc9k password remove
package cli

import (
	"fmt"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

// SettingSettingsPassword holds the password's salted hash.
const SettingSettingsPassword = "settings.password"

// settingsPasswordChecked remembers, within one command, that the password
// was given, so a command with several protected steps asks only once.
var settingsPasswordChecked bool

func settingsPasswordSet(database *db.DB) (bool, error) {
	hash, err := database.GetSetting(SettingSettingsPassword)
	return hash != "", err
}

// requireSettingsPassword asks for the settings password, if one is set,
// and fails unless it is given correctly. It reads one line from standard
// input (hidden on a terminal), before any other password the command reads.
func requireSettingsPassword(database *db.DB) error {
	if settingsPasswordChecked {
		return nil
	}
	hash, err := database.GetSetting(SettingSettingsPassword)
	if err != nil || hash == "" {
		return err
	}
	password, err := readPasswordStdin("Settings password")
	if err != nil {
		return fmt.Errorf("this change needs the settings password")
	}
	if !db.CheckPassword(hash, password) {
		time.Sleep(time.Second) // slow down guessing
		return fmt.Errorf("wrong settings password")
	}
	settingsPasswordChecked = true
	return nil
}

var passwordCmd = &cobra.Command{
	Use:   "password",
	Short: "Set or remove the settings password",
	Long: `A settings password guards KC9K against anything running as you: loosening a
block, stopping or unlocking it, creating any kind of lock, turning browser
options off, strict mode, no-escape and uninstalling all ask for it. Starting
a block without a lock, adding sites or apps, and reading the state don't.

Give it to someone else to keep, and they decide when your blocks may loosen.
Passwords are read from standard input (hidden on a terminal), the current
one first.

If it's lost, root can remove it while nothing is locked:
  sudo python3 -c "import sqlite3; c=sqlite3.connect('/var/lib/kc9k/kc9k.db'); c.execute(\"DELETE FROM settings WHERE key='settings.password'\"); c.commit()"`,
}

var passwordSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set or change the settings password",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
		password, err := readPasswordStdin("New settings password")
		if err != nil {
			return err
		}
		if len(password) < 4 {
			return fmt.Errorf("use at least 4 characters")
		}
		if isTerminalInput() {
			again, err := readPasswordStdin("Repeat it")
			if err != nil {
				return err
			}
			if again != password {
				return fmt.Errorf("the two passwords differ; nothing changed")
			}
		}
		hash, err := db.HashPassword(password)
		if err != nil {
			return err
		}
		if err := database.SetSetting(SettingSettingsPassword, hash); err != nil {
			return err
		}
		fmt.Println("Settings password set.")
		return nil
	}),
}

var passwordRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove the settings password",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		set, err := settingsPasswordSet(database)
		if err != nil {
			return err
		}
		if !set {
			return fmt.Errorf("no settings password is set")
		}
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
		if err := database.SetSetting(SettingSettingsPassword, ""); err != nil {
			return err
		}
		fmt.Println("Settings password removed.")
		return nil
	}),
}

func init() {
	passwordCmd.AddCommand(passwordSetCmd, passwordRemoveCmd)
	rootCmd.AddCommand(passwordCmd)
}
