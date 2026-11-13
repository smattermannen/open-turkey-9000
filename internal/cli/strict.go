// strict.go — strict mode and reading a password without exposing it.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/smattermannen/kc9k/internal/lock"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

var strictCmd = &cobra.Command{
	Use:       "strict [on|off|unlock]",
	Short:     "Take away sudo and pkexec while a block is locked",
	ValidArgs: []string{"on", "off", "unlock"},
	Long: `With strict mode on, the daemon denies sudo and pkexec for every user while
a block or setting is locked or Frozen Chicken runs. Only kc9k itself keeps
working. Rights come back by themselves when the lock ends.

Strict mode is off until you turn it on, and you can turn it off at any time,
unless it has its own setting lock, set when turning it on:

  --for DURATION    Timer lock: can't be turned off for this long (e.g. 2h).
  --until HH:MM     Timer lock until a clock time.
  --lock            Text lock ('kc9k strict unlock'), --lock-chars N long.
  --password-stdin  Password lock ('kc9k strict unlock').
  --restart         Stays on until the computer restarts.

or 'kc9k no-weakening' is on while something is locked. Strict mode's own
lock only keeps it on; sudo is taken away only while something else is locked.

If something goes wrong, add kc9k.nofreeze to the kernel line in GRUB,
or remove /etc/sudoers.d/zz-kc9k-lockdown from recovery mode.`,
	Args: cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		switch args[0] {
		case "on":
			return strictOn(cmd, database)
		case "off":
			if lockFlagsGiven(cmd) {
				return fmt.Errorf("lock options only go with 'on'")
			}
			return strictOff(database)
		case "unlock":
			return strictUnlock(database)
		}
		return fmt.Errorf("use 'on', 'off' or 'unlock'")
	}),
}

func strictOn(cmd *cobra.Command, database *db.DB) error {
	if lockFlagsGiven(cmd) {
		if err := requireLockPassword(database); err != nil {
			return err
		}
	}
	var next db.SettingLock
	var changes []string
	if lockFlagsGiven(cmd) {
		want, err := requestedSettingLock(cmd, database, enforce.StrictLockName)
		if err != nil {
			return err
		}
		cur, err := loadSettingLock(database, enforce.StrictLockName)
		if err != nil {
			return err
		}
		if next, changes, err = strengthenSettingLock(cur, want, cmd.Flags().Changed("lock-chars")); err != nil {
			return err
		}
		if len(changes) == 0 {
			return fmt.Errorf("strict mode already has those locks")
		}
	}
	if err := database.SetSetting(enforce.SettingStrictUser, os.Getenv("SUDO_USER")); err != nil {
		return err
	}
	if err := database.SetSetting(enforce.SettingStrict, "on"); err != nil {
		return err
	}
	if len(changes) > 0 {
		if err := database.SetSettingLock(next); err != nil {
			return err
		}
		fmt.Printf("Strict mode is on, %s\n", strings.Join(changes, ", "))
	} else {
		fmt.Println("Strict mode is on")
	}
	warnRootGroups(os.Getenv("SUDO_USER"))
	return nil
}

func strictOff(database *db.DB) error {
	if err := requireSettingsPassword(database); err != nil {
		return err
	}
	s, err := loadSettingLock(database, enforce.StrictLockName)
	if err != nil {
		return err
	}
	if err := s.refuseWhileHeld("strict mode"); err != nil {
		return refused(database, "", err)
	}
	if s.lock.LockChars > 0 || s.lock.PasswordHash != "" {
		return refused(database, "", fmt.Errorf("strict mode is locked. Use 'kc9k strict unlock' first"))
	}
	if err := refuseSettingWeakening(database, "strict mode"); err != nil {
		return err
	}
	if err := database.ClearSettingLock(enforce.StrictLockName); err != nil { // locks that ran out
		return err
	}
	if err := database.SetSetting(enforce.SettingStrict, "off"); err != nil {
		return err
	}
	fmt.Println("Strict mode is off")
	return nil
}

// strictUnlock lifts strict mode's text or password lock. Strict mode stays
// on: the lock is only lifted (§3.4).
func strictUnlock(database *db.DB) error {
	if err := requireSettingsPassword(database); err != nil {
		return err
	}
	s, err := loadSettingLock(database, enforce.StrictLockName)
	if err != nil {
		return err
	}
	if err := s.refuseWhileHeld("strict mode"); err != nil {
		return refused(database, "", err)
	}
	if s.lock.LockChars == 0 && s.lock.PasswordHash == "" {
		return fmt.Errorf("strict mode is not locked by a text or password lock")
	}
	if s.lock.PasswordHash != "" {
		password, err := readPasswordStdin("Setting password")
		if err != nil {
			return err
		}
		if !db.CheckPassword(s.lock.PasswordHash, password) {
			return refused(database, "", fmt.Errorf("wrong password; strict mode stays locked"))
		}
	}
	if s.lock.LockChars > 0 {
		ok, err := lock.RunStyled(s.lock.Style, s.lock.LockChars)
		if err != nil {
			return fmt.Errorf("failed to run the typing challenge: %w", err)
		}
		if !ok {
			_ = refused(database, "", fmt.Errorf("failed the unlock challenge"))
			fmt.Println("Unlock failed. Strict mode stays locked.")
			os.Exit(1)
		}
	}
	if err := database.ClearSettingLock(enforce.StrictLockName); err != nil {
		return err
	}
	fmt.Println("Strict mode is unlocked; it stays on until you turn it off.")
	return nil
}

// rootGroups are groups strict mode can't take away whose members can still
// act as root: disk gives raw access to the drives, where any file can be
// changed. (docker, lxd, incus and libvirt are handled by closing their
// sockets during a lock.)
var rootGroups = []string{"disk"}

// warnRootGroups tells the user which of those groups they are in.
func warnRootGroups(name string) {
	if name == "" {
		return
	}
	u, err := user.Lookup(name)
	if err != nil {
		return
	}
	ids, err := u.GroupIds()
	if err != nil {
		return
	}
	for _, id := range ids {
		g, err := user.LookupGroupId(id)
		if err != nil {
			continue
		}
		for _, risky := range rootGroups {
			if g.Name == risky {
				fmt.Printf("Warning: %s is in the %s group, which can get around strict mode. Remove it with: sudo gpasswd -d %s %s\n", name, risky, name, risky)
			}
		}
	}
}

// stdinLines reads standard input a line at a time for every password a
// command asks for, so a second password piped after the first (the settings
// password, then a lock's) isn't lost in a buffer.
var stdinLines = bufio.NewReader(os.Stdin)

// isTerminalInput reports whether standard input is a terminal.
var isTerminalInput = func() bool {
	_, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
	return err == nil
}

// readPasswordStdin reads one line, so a password never shows in a process list.
// On a terminal it shows the prompt and hides what is typed; piped input (the
// graphical interface) is read as is, without a prompt.
func readPasswordStdin(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	if old, err := unix.IoctlGetTermios(fd, unix.TCGETS); err == nil {
		fmt.Fprint(os.Stderr, prompt+": ")
		quiet := *old
		quiet.Lflag &^= unix.ECHO
		if err := unix.IoctlSetTermios(fd, unix.TCSETS, &quiet); err == nil {
			defer func() {
				_ = unix.IoctlSetTermios(fd, unix.TCSETS, old)
				fmt.Fprintln(os.Stderr)
			}()
		}
	}
	line, err := stdinLines.ReadString('\n')
	password := strings.TrimRight(line, "\r\n")
	if password == "" {
		if err != nil {
			return "", fmt.Errorf("failed to read password: %w", err)
		}
		return "", fmt.Errorf("the password is empty")
	}
	return password, nil
}

var noEscapeCmd = &cobra.Command{
	Use:   "no-escape [on|off]",
	Short: "Remove every way out of a lock short of reinstalling",
	Long: `With no-escape on, the kernel parameter stops working, recovery entries are
removed from GRUB, and GRUB gets a random password that is never stored, so boot
entries can't be edited. It can't be turned off during a lock.

If a bug leaves the computer locked, the only fix is booting other media or
reinstalling, which erases the system. Test it in a virtual machine first.
Booting a USB stick still works unless the firmware is locked with a password.`,
	Args: cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
		switch args[0] {
		case "on":
			yes, _ := cmd.Flags().GetBool("yes")
			if !yes {
				return fmt.Errorf("the only way out will be reinstalling; add --yes to confirm")
			}
			if err := blocker.ApplyBootLock(); err != nil {
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
				return fmt.Errorf("no-escape can't be turned off during a lock")
			}
			if err := blocker.RemoveBootLock(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("use 'on' or 'off'")
		}
		return database.SetSetting(enforce.SettingNoEscape, args[0])
	}),
}

func init() {
	noEscapeCmd.Flags().Bool("yes", false, "Confirm")
	strictCmd.Flags().String("for", "", "Timer lock: can't be turned off for this long (e.g. 2h, 90m)")
	strictCmd.Flags().String("until", "", "Timer lock: can't be turned off until this clock time (HH:MM)")
	strictCmd.Flags().Bool("lock", false, "Text lock ('kc9k strict unlock')")
	strictCmd.Flags().Int("lock-chars", lock.DefaultLockChars, "Number of characters in the unlock challenge")
	strictCmd.Flags().Bool("password-stdin", false, "Password lock: read the password from standard input")
	strictCmd.Flags().Bool("restart", false, "Restart lock: stays on until the computer restarts")
	addTextLockFlags(strictCmd)
	rootCmd.AddCommand(strictCmd, noEscapeCmd)
}
