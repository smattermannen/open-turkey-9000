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
	"errors"
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
	Short: "Set and lock browser settings: " + strings.Join(enforce.BrowserSettings, ", "),
	Long: `Browser settings can always be turned on. Each one can be locked on its own,
independently of any block, when it is turned on:

  --for DURATION    Timer lock: can't be turned off for this long (e.g. 2h, 7d is 168h).
  --until HH:MM     Timer lock until a clock time today (or tomorrow if already past).
  --lock            Typing challenge ('kc9k browser unlock <setting>') to turn it off.
  --lock-chars N    Challenge length in characters (default 300, 10 to 5000).
  --password-stdin  Password ('kc9k browser unlock <setting>') to turn it off.
  --restart         Stays on until the computer restarts.
  --relock D        With a text or password lock: unlocking lifts them for D
                    only, the setting stays on; turning it off meanwhile ends it.

Locks can be added to a setting that is already on, and only made stronger: a
timer lock can only be extended and a challenge only lengthened. unlock runs the
challenge and asks for the password, then turns the setting off; time and
restart locks can't be unlocked, only waited out.

block-devtools also can't be turned off while a block is locked or Frozen
Chicken runs, since blocks rely on it. block-devtools is on
until you turn it off: LibreWolf's developer tools can switch the extension
off, so turn it off only to develop websites. Browsers read it when they start.
no-private turns off private windows in LibreWolf (and Firefox), from the
browser's next start; during website blocks, older LibreWolf windows are
closed so it restarts. Without arguments, lists every option,
its locks, and the installed LibreWolf version.

The Killer Chicken extension isn't an option: it is always installed, and
while a block has websites (or web-only or force-search is on), browsers
without it are closed. It runs in the supported browsers, LibreWolf, Google
Chrome and Chromium; 'kc9k browsers' lists them and installs them. To close
every other browser at any time, turn on 'kc9k tamper unsupported-browsers'.`,
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
		if name == forceLibreWolfAlias {
			// Old name of block unsupported browsers (decision 2026-11-14).
			name = db.UnsupportedBrowsers
		}
		if name == "extension" || name == "require-extension" {
			return fmt.Errorf("the extension is always on: while a block has websites, browsers without it are closed (see 'kc9k status')")
		}
		if name == "images" || name == "greyscale" {
			return fmt.Errorf("%s is now chosen per block: 'kc9k block action <block> %s on'", name, map[string]string{"images": db.ActionNoImages, "greyscale": db.ActionGreyscale}[name])
		}
		if !enforce.IsLockableSetting(name) {
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
			if name == "no-private" {
				fmt.Println("Private windows are off from LibreWolf's next start; during website blocks, older LibreWolf windows are closed so it restarts.")
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

func installLibreWolf() error { return installBrowser("librewolf") }

// extensionBrowserInstalled is blocker.ExtensionBrowserInstalled, replaced
// in tests.
var extensionBrowserInstalled = blocker.ExtensionBrowserInstalled

// forceLibreWolfAlias is the hidden old name of block unsupported browsers
// in 'kc9k browser', kept for scripts and habits.
const forceLibreWolfAlias = "force-librewolf"

// settingCommand is the command that turns a lockable setting off or
// unlocks it.
func settingCommand(name, verb string) string {
	group := "browser"
	if name == db.UnsupportedBrowsers {
		group = "tamper"
	}
	if verb == "unlock" {
		return "kc9k " + group + " unlock " + name
	}
	return "kc9k " + group + " " + name + " off"
}

// settingTarget names a lockable setting in messages and lock confirmations.
func settingTarget(name string) string {
	if name == db.UnsupportedBrowsers {
		return "tamper protection 'block unsupported browsers'"
	}
	return "browser setting '" + name + "'"
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
	if s.lock.TextOrPasswordLocked(s.trusted) {
		if s.lock.LockChars > 0 {
			parts = append(parts, fmt.Sprintf("typing challenge (%d chars)", s.lock.LockChars))
		}
		if s.lock.PasswordHash != "" {
			parts = append(parts, "password")
		}
		if s.lock.Relock > 0 {
			parts = append(parts, "re-lock "+(time.Duration(s.lock.Relock)*time.Second).String())
		}
	}
	if s.lock.PendingRelock(s.trusted) {
		parts = append(parts, "unlocked: on again at "+time.Unix(s.lock.LiftedUntil, 0).Format("2006-01-02 15:04"))
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
	for _, f := range []string{"for", "until", "lock", "lock-chars", "password-stdin", "restart", "text-lock", "lock-words", "lock-text", "relock"} {
		if cmd.Flags().Lookup(f) != nil && cmd.Flags().Changed(f) {
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
	useLock, style, length, err := textLockRequested(cmd)
	if err != nil {
		return want, err
	}
	if useLock {
		want.LockChars, want.Style = length, style
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
	if want.Relock, err = relockRequested(cmd); err != nil {
		return want, err
	}
	return want, nil
}

// relockRequested reads --relock: seconds, or 0 when not given.
func relockRequested(cmd *cobra.Command) (int64, error) {
	if cmd.Flags().Lookup("relock") == nil {
		return 0, nil
	}
	spec, _ := cmd.Flags().GetString("relock")
	if spec == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(spec)
	if err != nil || d < time.Minute || d > 30*24*time.Hour {
		return 0, fmt.Errorf("invalid --relock %q (1m to 720h)", spec)
	}
	return int64(d / time.Second), nil
}

// addRelockFlag adds --relock to commands that lock a setting.
func addRelockFlag(cmds ...*cobra.Command) {
	for _, c := range cmds {
		c.Flags().String("relock", "", "Re-lock: after unlocking, the text and password locks come on again after this long (e.g. 30m), unless it is turned off meanwhile")
	}
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
	if !next.PendingRelock(cur.trusted) {
		next.LiftedUntil = 0
	} else if want.LockChars > 0 || want.PasswordHash != "" {
		// Unlocked with re-lock: a new text or password lock replaces the
		// lifted ones and holds at once.
		next.LockChars, next.Style, next.PasswordHash, next.Relock, next.LiftedUntil = 0, "", "", 0, 0
	}
	if want.LockedUntil != 0 {
		if want.LockedUntil <= next.LockedUntil {
			return next, nil, fmt.Errorf("%s is already locked until %s; a timer lock can only be extended", name, time.Unix(next.LockedUntil, 0).Format("2006-01-02 15:04"))
		}
		next.LockedUntil = want.LockedUntil
		changes = append(changes, "locked until "+time.Unix(next.LockedUntil, 0).Format("2006-01-02 15:04"))
	}
	if want.LockChars > 0 {
		switch {
		case next.LockChars == 0:
			next.LockChars, next.Style = want.LockChars, want.Style
			changes = append(changes, "text lock: "+lock.DescribeStyle(next.Style, next.LockChars))
		case want.Style != next.Style:
			return next, nil, fmt.Errorf("%s already has a text lock (%s); its kind can't change while locked", name, lock.DescribeStyle(next.Style, next.LockChars))
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
	if want.Relock > 0 {
		switch {
		case next.LockChars == 0 && next.PasswordHash == "":
			return next, nil, fmt.Errorf("--relock goes with a text or password lock")
		case next.Relock == 0 || want.Relock < next.Relock:
			// Sooner is stronger: the locks are lifted for less time.
			next.Relock = want.Relock
			changes = append(changes, "re-lock "+(time.Duration(next.Relock)*time.Second).String()+" after unlocking")
		case want.Relock > next.Relock:
			return next, nil, fmt.Errorf("%s re-locks %s after unlocking; that can't be made longer while locked", name, time.Duration(next.Relock)*time.Second)
		}
	}
	return next, changes, nil
}

// settingLocks lists the locks strengthenSettingLock adds to a setting, for
// confirmLocks (R-LOCK-12).
func settingLocks(target string, cur settingLockState, next db.SettingLock) []lockToSet {
	old := cur.lock
	if !old.TimeLocked(cur.trusted) {
		old.LockedUntil = 0
	}
	if !old.RestartLocked(cur.bootID) {
		old.RestartBoot = ""
	}
	var locks []lockToSet
	if next.LockedUntil != old.LockedUntil {
		locks = append(locks, lockToSet{target, "timer lock", timerLength(cur.trusted, next.LockedUntil)})
	}
	if next.LockChars != old.LockChars {
		length := ""
		if old.LockChars > 0 {
			length = "lengthened"
		}
		locks = append(locks, lockToSet{target, textLockKind(next.Style, next.LockChars), length})
	}
	if next.PasswordHash != old.PasswordHash {
		locks = append(locks, lockToSet{target, "password lock", ""})
	}
	if next.RestartBoot != old.RestartBoot {
		locks = append(locks, lockToSet{target, "restart lock", "until the computer restarts"})
	}
	if next.Relock != old.Relock {
		locks = append(locks, lockToSet{target, "re-lock", (time.Duration(next.Relock) * time.Second).String() + " after unlocking"})
	}
	return locks
}

func browserSettingOn(cmd *cobra.Command, database *db.DB, name string) error {
	if name == db.UnsupportedBrowsers && !extensionBrowserInstalled() {
		return fmt.Errorf("no supported browser is installed, so every browser would be closed; install one first ('kc9k browsers install librewolf|chrome|chromium')")
	}
	if !lockFlagsGiven(cmd) {
		return database.SetSetting(enforce.SettingKey(name), "on")
	}
	if err := requireLockPassword(database); err != nil {
		return err
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
	if err := confirmLocks(cmd, settingLocks(settingTarget(name), cur, next)...); err != nil {
		return err
	}
	if err := database.SetSetting(enforce.SettingKey(name), "on"); err != nil {
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
// block is locked or Frozen Chicken runs, and a tamper protection setting
// while anything is locked (§4.10). Call it after the setting's own lock is
// gone, or that lock counts too.
func refuseForBlocks(database *db.DB, name string, trusted int64) error {
	if name == db.UnsupportedBrowsers {
		locked, err := enforce.AnyLock(database, time.Now(), trusted)
		if err != nil {
			return err
		}
		if locked {
			return refused(database, "", fmt.Errorf("block unsupported browsers can't be turned off while a block or setting is locked or Frozen Chicken runs"))
		}
		return nil
	}
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
	if err := requireSettingsPassword(database); err != nil {
		return err
	}
	s, err := loadSettingLock(database, name)
	if err != nil {
		return err
	}
	if err := s.refuseWhileHeld(name); err != nil {
		return err
	}
	if s.lock.TextOrPasswordLocked(s.trusted) {
		return fmt.Errorf("%s is locked. Use '%s' first", name, settingCommand(name, "unlock"))
	}
	if err := refuseForBlocks(database, name, s.trusted); err != nil {
		return err
	}
	if err := refuseSettingWeakening(database, name); err != nil {
		return err
	}
	if err := database.ClearSettingLock(name); err != nil { // locks that ran out, or lifted by re-lock
		return err
	}
	return database.SetSetting(enforce.SettingKey(name), "off")
}

func unlockBrowserSetting(database *db.DB, name string) error {
	if name == forceLibreWolfAlias {
		name = db.UnsupportedBrowsers
	}
	if !enforce.IsLockableSetting(name) {
		return fmt.Errorf("unknown setting %q (use %s)", name, strings.Join(enforce.BrowserSettings, ", "))
	}
	s, relocked, err := unlockSetting(database, name, name,
		fmt.Sprintf("%s is not locked. Use '%s'", name, settingCommand(name, "off")))
	if err != nil {
		return err
	}
	if relocked {
		fmt.Printf("%s is unlocked for %s and stays on; then its locks are on again (re-lock). '%s' turns it off meanwhile.\n",
			name, time.Duration(s.lock.Relock)*time.Second, settingCommand(name, "off"))
		return nil
	}
	if err := refuseForBlocks(database, name, s.trusted); err != nil {
		fmt.Printf("%s is unlocked but stays on: %v\n", name, err)
		return nil
	}
	if err := refuseSettingWeakening(database, name); err != nil {
		fmt.Printf("%s is unlocked but stays on: %v\n", name, err)
		return nil
	}
	if err := database.SetSetting(enforce.SettingKey(name), "off"); err != nil {
		return err
	}
	fmt.Printf("%s unlocked and turned off.\n", name)
	return nil
}

// unlockSetting lifts a setting's text and password locks (setting lock
// name, shown as label): it asks for the settings password, refuses while
// a timer or restart lock holds, asks for the lock password and runs the
// challenge. Without re-lock the locks are removed; with re-lock they are
// lifted for that long (relocked true), then on again (§3.4).
func unlockSetting(database *db.DB, name, label, notLocked string) (settingLockState, bool, error) {
	if err := requireSettingsPassword(database); err != nil {
		return settingLockState{}, false, err
	}
	s, err := loadSettingLock(database, name)
	if err != nil {
		return s, false, err
	}
	if err := s.refuseWhileHeld(label); err != nil {
		return s, false, refused(database, "", err)
	}
	if !s.lock.TextOrPasswordLocked(s.trusted) {
		return s, false, errors.New(notLocked)
	}
	if s.lock.PasswordHash != "" {
		password, err := readPasswordStdin("Setting password")
		if err != nil {
			return s, false, err
		}
		if !db.CheckPassword(s.lock.PasswordHash, password) {
			return s, false, refused(database, "", fmt.Errorf("wrong password; %s stays locked", label))
		}
	}
	if s.lock.LockChars > 0 {
		ok, err := lock.RunStyled(s.lock.Style, s.lock.LockChars)
		if err != nil {
			return s, false, fmt.Errorf("failed to run the typing challenge: %w", err)
		}
		if !ok {
			_ = refused(database, "", fmt.Errorf("failed the unlock challenge"))
			fmt.Printf("Unlock failed. %s stays locked.\n", label)
			os.Exit(1)
		}
	}
	if s.lock.Relock > 0 {
		s.lock.LiftedUntil = s.trusted + s.lock.Relock
		return s, true, database.SetSettingLock(s.lock)
	}
	return s, false, database.ClearSettingLock(name)
}

func init() {
	browserCmd.Flags().String("for", "", "Timer lock: can't be turned off for this long (e.g. 2h, 90m)")
	browserCmd.Flags().String("until", "", "Timer lock: can't be turned off until this clock time (HH:MM)")
	browserCmd.Flags().Bool("lock", false, "Typing challenge lock ('kc9k browser unlock <setting>')")
	browserCmd.Flags().Int("lock-chars", lock.DefaultLockChars, "Number of characters in the unlock challenge")
	browserCmd.Flags().Bool("password-stdin", false, "Password lock: read the password from standard input")
	browserCmd.Flags().Bool("restart", false, "Restart lock: stays on until the computer restarts")
	addTextLockFlags(browserCmd)
	addRelockFlag(browserCmd)
	addYesFlag(browserCmd)
	rootCmd.AddCommand(browserCmd)
}
