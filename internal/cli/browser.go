// browser.go — settings for the extension and forced browser behaviour, and
// their locks.
//
//	kc9k browser                                   # list settings and locks
//	kc9k browser safesearch on [--for 2h] [--lock] # turn on, optionally locked
//	kc9k browser safesearch off                    # refused while locked
//	kc9k browser unlock safesearch                 # challenge or password
//	kc9k browser install-librewolf
package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/smattermannen/kc9k/internal/lock"
	"github.com/spf13/cobra"
)

var browserCmd = &cobra.Command{
	Use:   "browser [setting on|off | unlock setting | install-librewolf]",
	Short: "Set and lock browser options: " + strings.Join(enforce.BrowserSettings, ", "),
	Long: `Browser options can always be turned on. Each one can be locked on its own,
independently of any block, when it is turned on:

  --for DURATION    Time lock: can't be turned off for this long (e.g. 2h, 7d is 168h).
  --until HH:MM     Time lock until a clock time today (or tomorrow if already past).
  --lock            Typing challenge ('kc9k browser unlock <setting>') to turn it off.
  --lock-chars N    Challenge length in characters (default 300, 10 to 5000).
  --password-stdin  Password ('kc9k browser unlock <setting>') to turn it off.
  --restart         Stays on until the computer restarts.

Locks can be added to a setting that is already on, and only made stronger: a
time lock can only be extended and a challenge only lengthened. unlock runs the
challenge and asks for the password, then turns the setting off; time and
restart locks can't be unlocked, only waited out.

force-librewolf and block-devtools also can't be turned off while a block is
locked or Frozen Chicken runs, since blocks rely on them. block-devtools is on
until you turn it off: LibreWolf's developer tools can switch the extension
off, so turn it off only to develop websites. Browsers read it when they start. Without arguments, lists every option,
its locks, and the installed LibreWolf version.

The Killer Chicken extension isn't an option: it is always installed, and
while a block has websites (or web-only or force-search is on), browsers
without it are closed. That needs LibreWolf, the one browser that runs it;
'kc9k browser install-librewolf' installs it from its official repository
(via extrepo), as installing KC9K does.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 && args[0] == "install-librewolf" {
			return nil
		}
		if len(args) != 0 && len(args) != 2 {
			return fmt.Errorf("give no arguments to list options, a setting and on|off, unlock and a setting, or install-librewolf")
		}
		return nil
	},
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		switch {
		case len(args) == 0:
			return listBrowserSettings(database)
		case len(args) == 1:
			return installLibreWolf()
		case args[0] == "unlock":
			return unlockBrowserSetting(database, args[1])
		}
		name, value := args[0], args[1]
		if name == "extension" || name == "require-extension" {
			return fmt.Errorf("the extension is always on: while a block has websites, browsers without it are closed (see 'kc9k status')")
		}
		if name == "images" || name == "greyscale" {
			return fmt.Errorf("%s is now chosen per block: 'kc9k block action <block> %s on'", name, map[string]string{"images": db.ActionNoImages, "greyscale": db.ActionGreyscale}[name])
		}
		if !enforce.IsBrowserSetting(name) {
			return fmt.Errorf("unknown setting %q (use %s)", name, strings.Join(enforce.BrowserSettings, ", "))
		}
		switch value {
		case "on":
			if err := browserSettingOn(cmd, database, name); err != nil {
				return err
			}
			if name == "block-devtools" {
				fmt.Println("LibreWolf's developer tools are blocked from its next start; during website blocks, older LibreWolf windows are closed so it restarts.")
			}
			return nil
		case "off":
			if lockFlagsGiven(cmd) {
				return fmt.Errorf("lock options only go with 'on'")
			}
			if err := browserSettingOff(database, name); err != nil {
				return err
			}
			if name == "block-devtools" {
				fmt.Println("LibreWolf's developer tools work from its next start. They can switch the extension off, so wildcards, keywords, greyscale, images and web-only can be got around while they're available; domain blocks still hold.")
			}
			return nil
		}
		return fmt.Errorf("use 'on' or 'off'")
	}),
}

func installLibreWolf() error {
	if version := blocker.LibreWolfVersion(); version != "" {
		fmt.Printf("LibreWolf %s is already installed.\n", version)
		return nil
	}
	fmt.Println("Installing LibreWolf, this can take a few minutes…")
	if err := blocker.InstallLibreWolf(); err != nil {
		return err
	}
	fmt.Printf("LibreWolf %s installed. The extension is added the next time it starts.\n", blocker.LibreWolfVersion())
	return nil
}

// settingLockState is a setting's lock with the context to judge it.
type settingLockState struct {
	lock    db.SettingLock
	trusted int64
	bootID  string
}

func loadSettingLock(database *db.DB, name string) (settingLockState, error) {
	trusted, err := database.TrustedNow(time.Now())
	if err != nil {
		return settingLockState{}, err
	}
	bootID, err := db.BootID()
	if err != nil {
		return settingLockState{}, err
	}
	l, err := database.GetSettingLock(name)
	return settingLockState{lock: l, trusted: trusted, bootID: bootID}, err
}

// describe lists the locks in force, e.g. "locked until 2026-11-07 15:00,
// typing challenge"; "" when unlocked.
func (s settingLockState) describe() string {
	var parts []string
	if s.lock.TimeLocked(s.trusted) {
		parts = append(parts, "until "+time.Unix(s.lock.LockedUntil, 0).Format("2006-01-02 15:04"))
	}
	if s.lock.RestartLocked(s.bootID) {
		parts = append(parts, "until restart")
	}
	if s.lock.LockChars > 0 {
		parts = append(parts, fmt.Sprintf("typing challenge (%d chars)", s.lock.LockChars))
	}
	if s.lock.PasswordHash != "" {
		parts = append(parts, "password")
	}
	if len(parts) == 0 {
		return ""
	}
	return "locked: " + strings.Join(parts, ", ")
}

func listBrowserSettings(database *db.DB) error {
	for _, name := range enforce.BrowserSettings {
		state := "off"
		if enforce.BrowserOn(database, name) {
			state = "on"
		}
		s, err := loadSettingLock(database, name)
		if err != nil {
			return err
		}
		if text := s.describe(); text != "" {
			state += "  (" + text + ")"
		}
		fmt.Printf("%s %s\n", name, state)
	}
	version := blocker.LibreWolfVersion()
	if version == "" {
		version = "not-installed"
	}
	fmt.Printf("librewolf %s\n", version)
	return nil
}

func lockFlagsGiven(cmd *cobra.Command) bool {
	for _, f := range []string{"for", "until", "lock", "lock-chars", "password-stdin", "restart"} {
		if cmd.Flags().Changed(f) {
			return true
		}
	}
	return false
}

// requestedSettingLock reads the lock flags into the lock they ask for.
func requestedSettingLock(cmd *cobra.Command, database *db.DB, name string) (db.SettingLock, error) {
	want := db.SettingLock{Name: name}
	forDur, _ := cmd.Flags().GetString("for")
	untilClock, _ := cmd.Flags().GetString("until")
	if forDur != "" && untilClock != "" {
		return want, fmt.Errorf("use only one of --for or --until")
	}
	now := time.Now()
	switch {
	case forDur != "":
		d, err := time.ParseDuration(forDur)
		if err != nil {
			return want, fmt.Errorf("invalid --for duration %q: %w", forDur, err)
		}
		if d <= 0 {
			return want, fmt.Errorf("--for duration must be positive")
		}
		want.LockedUntil = now.Add(d).Unix()
	case untilClock != "":
		t, err := untilToday(now, untilClock)
		if err != nil {
			return want, err
		}
		want.LockedUntil = t.Unix()
	}
	if want.LockedUntil != 0 {
		trusted, err := database.TrustedNow(now)
		if err != nil {
			return want, err
		}
		want.LockedUntil += trusted - now.Unix()
	}
	if useLock, _ := cmd.Flags().GetBool("lock"); useLock {
		want.LockChars, _ = cmd.Flags().GetInt("lock-chars")
		if err := lock.CheckLockChars(want.LockChars); err != nil {
			return want, err
		}
	} else if cmd.Flags().Changed("lock-chars") {
		return want, fmt.Errorf("--lock-chars goes with --lock")
	}
	if usePassword, _ := cmd.Flags().GetBool("password-stdin"); usePassword {
		password, err := readPasswordStdin("Setting password")
		if err != nil {
			return want, err
		}
		if want.PasswordHash, err = db.HashPassword(password); err != nil {
			return want, err
		}
	}
	if useRestart, _ := cmd.Flags().GetBool("restart"); useRestart {
		boot, err := db.BootID()
		if err != nil {
			return want, err
		}
		want.RestartBoot = boot
	}
	return want, nil
}

// strengthenSettingLock merges the requested locks into the current ones. It
// only ever makes the lock stronger: an earlier end, a shorter challenge or a
// new password over an existing one is refused. Locks that have run out are
// dropped. changes describes what was added.
func strengthenSettingLock(cur settingLockState, want db.SettingLock, charsGiven bool) (db.SettingLock, []string, error) {
	next := cur.lock
	name := next.Name
	if !next.TimeLocked(cur.trusted) {
		next.LockedUntil = 0
	}
	if !next.RestartLocked(cur.bootID) {
		next.RestartBoot = ""
	}
	var changes []string
	if want.LockedUntil != 0 {
		if want.LockedUntil <= next.LockedUntil {
			return next, nil, fmt.Errorf("%s is already locked until %s; a time lock can only be extended", name, time.Unix(next.LockedUntil, 0).Format("2006-01-02 15:04"))
		}
		next.LockedUntil = want.LockedUntil
		changes = append(changes, "locked until "+time.Unix(next.LockedUntil, 0).Format("2006-01-02 15:04"))
	}
	if want.LockChars > 0 {
		switch {
		case next.LockChars == 0:
			next.LockChars = want.LockChars
			changes = append(changes, fmt.Sprintf("typing challenge (%d chars)", next.LockChars))
		case charsGiven && want.LockChars > next.LockChars:
			next.LockChars = want.LockChars
			changes = append(changes, fmt.Sprintf("challenge lengthened to %d chars", next.LockChars))
		case charsGiven && want.LockChars < next.LockChars:
			return next, nil, fmt.Errorf("%s already has a %d-character challenge; it can't be shortened", name, next.LockChars)
		}
	}
	if want.PasswordHash != "" {
		if next.PasswordHash != "" {
			return next, nil, fmt.Errorf("%s already has a password; it can't be replaced while locked", name)
		}
		next.PasswordHash = want.PasswordHash
		changes = append(changes, "password")
	}
	if want.RestartBoot != "" && next.RestartBoot == "" {
		next.RestartBoot = want.RestartBoot
		changes = append(changes, "until restart")
	}
	return next, changes, nil
}

func browserSettingOn(cmd *cobra.Command, database *db.DB, name string) error {
	if name == "force-librewolf" && blocker.LibreWolfVersion() == "" {
		return fmt.Errorf("LibreWolf isn't installed, so every browser would be closed; run 'kc9k browser install-librewolf' first")
	}
	if !lockFlagsGiven(cmd) {
		return database.SetSetting("browser."+name, "on")
	}
	want, err := requestedSettingLock(cmd, database, name)
	if err != nil {
		return err
	}
	cur, err := loadSettingLock(database, name)
	if err != nil {
		return err
	}
	next, changes, err := strengthenSettingLock(cur, want, cmd.Flags().Changed("lock-chars"))
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		return fmt.Errorf("%s already has those locks", name)
	}
	if err := database.SetSetting("browser."+name, "on"); err != nil {
		return err
	}
	if err := database.SetSettingLock(next); err != nil {
		return err
	}
	fmt.Printf("%s is on, %s\n", name, strings.Join(changes, ", "))
	return nil
}

// refuseWhileHeld returns why a setting can't be turned off by unlocking it,
// or nil. Time and restart locks can only be waited out.
func (s settingLockState) refuseWhileHeld(name string) error {
	if s.lock.TimeLocked(s.trusted) {
		return fmt.Errorf("%s is locked until %s and can't be turned off before then", name, time.Unix(s.lock.LockedUntil, 0).Format("2006-01-02 15:04"))
	}
	if s.lock.RestartLocked(s.bootID) {
		return fmt.Errorf("%s stays on until the computer restarts", name)
	}
	return nil
}

// refuseForBlocks refuses turning off a setting that blocks rely on while a
// block is locked or Frozen Chicken runs.
func refuseForBlocks(database *db.DB, name string, trusted int64) error {
	if !enforce.GuardsBlocks(name) {
		return nil
	}
	locked, err := enforce.BlocksLocked(database, time.Now(), trusted)
	if err != nil {
		return err
	}
	if locked {
		return fmt.Errorf("%s can't be turned off while a block is locked or Frozen Chicken runs", name)
	}
	return nil
}

func browserSettingOff(database *db.DB, name string) error {
	s, err := loadSettingLock(database, name)
	if err != nil {
		return err
	}
	if err := s.refuseWhileHeld(name); err != nil {
		return err
	}
	if s.lock.LockChars > 0 || s.lock.PasswordHash != "" {
		return fmt.Errorf("%s is locked. Use 'kc9k browser unlock %s' first", name, name)
	}
	if err := refuseForBlocks(database, name, s.trusted); err != nil {
		return err
	}
	if err := database.ClearSettingLock(name); err != nil { // locks that ran out
		return err
	}
	return database.SetSetting("browser."+name, "off")
}

func unlockBrowserSetting(database *db.DB, name string) error {
	if !enforce.IsBrowserSetting(name) {
		return fmt.Errorf("unknown setting %q (use %s)", name, strings.Join(enforce.BrowserSettings, ", "))
	}
	s, err := loadSettingLock(database, name)
	if err != nil {
		return err
	}
	if err := s.refuseWhileHeld(name); err != nil {
		return err
	}
	if s.lock.LockChars == 0 && s.lock.PasswordHash == "" {
		return fmt.Errorf("%s is not locked. Use 'kc9k browser %s off'", name, name)
	}
	if s.lock.PasswordHash != "" {
		password, err := readPasswordStdin("Setting password")
		if err != nil {
			return err
		}
		if !db.CheckPassword(s.lock.PasswordHash, password) {
			return fmt.Errorf("wrong password; %s stays locked", name)
		}
	}
	if s.lock.LockChars > 0 {
		success, err := lock.RunChallenge(s.lock.LockChars)
		if err != nil {
			return fmt.Errorf("failed to run the typing challenge: %w", err)
		}
		if !success {
			fmt.Printf("Unlock failed. %s stays on and locked.\n", name)
			os.Exit(1)
		}
	}
	if err := database.ClearSettingLock(name); err != nil {
		return err
	}
	if err := refuseForBlocks(database, name, s.trusted); err != nil {
		fmt.Printf("%s is unlocked but stays on: %v\n", name, err)
		return nil
	}
	if err := database.SetSetting("browser."+name, "off"); err != nil {
		return err
	}
	fmt.Printf("%s unlocked and turned off.\n", name)
	return nil
}

func init() {
	browserCmd.Flags().String("for", "", "Time lock: can't be turned off for this long (e.g. 2h, 90m)")
	browserCmd.Flags().String("until", "", "Time lock: can't be turned off until this clock time (HH:MM)")
	browserCmd.Flags().Bool("lock", false, "Typing challenge lock ('kc9k browser unlock <setting>')")
	browserCmd.Flags().Int("lock-chars", lock.DefaultLockChars, "Number of characters in the unlock challenge")
	browserCmd.Flags().Bool("password-stdin", false, "Password lock: read the password from standard input")
	browserCmd.Flags().Bool("restart", false, "Restart lock: stays on until the computer restarts")
	rootCmd.AddCommand(browserCmd)
}
