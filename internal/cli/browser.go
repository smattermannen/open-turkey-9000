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
	if err != nil {
		return settingLockState{}, err
	}
	// Layered locks (R-LOCK-6): start the lock after any that are gone.
	if l.Advance(trusted, bootID) {
		if err := database.SetSettingLock(l); err != nil {
			return settingLockState{}, err
		}
	}
	return settingLockState{lock: l, trusted: trusted, bootID: bootID}, nil
}

// describe lists the locks in force, e.g. "locked until 2026-11-07 15:00,
// typing challenge"; "" when unlocked.
func (s settingLockState) describe() string {
	var parts []string
	for _, k := range s.lock.LayersOn(s.trusted, s.bootID) {
		switch k {
		case db.LayerTimer:
			if s.lock.TimerAfter > 0 {
				parts = append(parts, fmt.Sprintf("timer %s after the locks before it", time.Duration(s.lock.TimerAfter)*time.Second))
			} else {
				parts = append(parts, "until "+time.Unix(s.lock.LockedUntil, 0).Format("2006-01-02 15:04"))
			}
		case db.LayerRestart:
			parts = append(parts, "until restart")
		case db.LayerText:
			parts = append(parts, fmt.Sprintf("typing challenge (%d chars)", s.lock.LockChars))
		case db.LayerPassword:
			parts = append(parts, "password")
		}
	}
	if s.lock.Relock > 0 && s.lock.TextOrPasswordLocked(s.trusted) {
		parts = append(parts, "re-lock "+(time.Duration(s.lock.Relock)*time.Second).String())
	}
	if s.lock.PendingRelock(s.trusted) && s.lock.LiftedUntil > s.trusted {
		parts = append(parts, "unlocked: on again at "+time.Unix(s.lock.LiftedUntil, 0).Format("2006-01-02 15:04"))
	}
	if len(parts) == 0 {
		return ""
	}
	sep := ", "
	if len(s.lock.LayersOn(s.trusted, s.bootID)) > 1 {
		sep = " → " // unlocked in this order (R-LOCK-6)
	}
	return "locked: " + strings.Join(parts, sep)
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
		want.TimerAfter = int64(d / time.Second)
	case untilClock != "":
		t, err := untilToday(now, untilClock)
		if err != nil {
			return want, err
		}
		want.LockedUntil = t.Unix()
	}
	want.Order = strings.Join(lockKindsInOrder(cmd), ",")
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
// dropped. New locks go last, in the order their flags were given
// (want.Order, R-LOCK-6/7); a timer lock for a duration (want.TimerAfter)
// behind a lock still on waits for it, and so does a restart lock. changes
// describes what was added.
func strengthenSettingLock(cur settingLockState, want db.SettingLock, charsGiven bool) (db.SettingLock, []string, error) {
	next := cur.lock
	name := next.Name
	trusted, boot := cur.trusted, cur.bootID
	next.Advance(trusted, boot)
	if next.LockedUntil <= trusted && next.TimerAfter == 0 {
		next.LockedUntil = 0
	}
	if next.RestartBoot != "" && !next.RestartLocked(boot) {
		next.RestartBoot = ""
	}
	if next.PendingRelock(trusted) && (want.LockChars > 0 || want.PasswordHash != "") {
		// Unlocked with re-lock: a new text or password lock replaces the
		// lifted ones and holds at once.
		next.LockChars, next.Style, next.PasswordHash, next.Relock, next.LiftedUntil, next.LiftedKinds = 0, "", "", 0, 0, ""
	}
	order := next.Layers()
	kinds := db.ParseLayerOrder(want.Order)
	kinds = db.AppendLayers(kinds, db.LayerTimer, db.LayerText, db.LayerPassword, db.LayerRestart)
	anyOn := func() bool { return len(next.LayersOn(trusted, boot)) > 0 }
	var changes []string
	for _, kind := range kinds {
		switch kind {
		case db.LayerTimer:
			if want.LockedUntil == 0 {
				continue
			}
			switch {
			case next.TimerAfter > 0:
				if want.TimerAfter == 0 {
					return next, nil, fmt.Errorf("%s has a timer lock that counts from unlocking the locks before it; extend it with --for", name)
				}
				if want.TimerAfter <= next.TimerAfter {
					return next, nil, fmt.Errorf("%s already has a timer lock of %s after the locks before it; a timer lock can only be extended", name, time.Duration(next.TimerAfter)*time.Second)
				}
				next.TimerAfter = want.TimerAfter
				changes = append(changes, "timer lock of "+(time.Duration(next.TimerAfter)*time.Second).String()+" after the locks before it")
			case next.LockedUntil != 0:
				if want.LockedUntil <= next.LockedUntil {
					return next, nil, fmt.Errorf("%s is already locked until %s; a timer lock can only be extended", name, time.Unix(next.LockedUntil, 0).Format("2006-01-02 15:04"))
				}
				next.LockedUntil = want.LockedUntil
				changes = append(changes, "locked until "+time.Unix(next.LockedUntil, 0).Format("2006-01-02 15:04"))
			case want.TimerAfter > 0 && anyOn():
				next.TimerAfter = want.TimerAfter
				order = db.AppendLayers(order, db.LayerTimer)
				changes = append(changes, "timer lock of "+(time.Duration(next.TimerAfter)*time.Second).String()+" after the locks before it")
			default:
				next.LockedUntil = want.LockedUntil
				order = db.AppendLayers(order, db.LayerTimer)
				changes = append(changes, "locked until "+time.Unix(next.LockedUntil, 0).Format("2006-01-02 15:04"))
			}
		case db.LayerText:
			if want.LockChars == 0 {
				continue
			}
			switch {
			case next.LockChars == 0:
				next.LockChars, next.Style = want.LockChars, want.Style
				order = db.AppendLayers(order, db.LayerText)
				changes = append(changes, "text lock: "+lock.DescribeStyle(next.Style, next.LockChars))
			case want.Style != next.Style:
				return next, nil, fmt.Errorf("%s already has a text lock (%s); its kind can't change while locked", name, lock.DescribeStyle(next.Style, next.LockChars))
			case charsGiven && want.LockChars > next.LockChars:
				next.LockChars = want.LockChars
				changes = append(changes, fmt.Sprintf("challenge lengthened to %d chars", next.LockChars))
			case charsGiven && want.LockChars < next.LockChars:
				return next, nil, fmt.Errorf("%s already has a %d-character challenge; it can't be shortened", name, next.LockChars)
			}
		case db.LayerPassword:
			if want.PasswordHash == "" {
				continue
			}
			if next.PasswordHash != "" {
				return next, nil, fmt.Errorf("%s already has a password; it can't be replaced while locked", name)
			}
			next.PasswordHash = want.PasswordHash
			order = db.AppendLayers(order, db.LayerPassword)
			changes = append(changes, "password")
		case db.LayerRestart:
			if want.RestartBoot == "" || next.RestartBoot != "" {
				continue
			}
			next.RestartBoot = want.RestartBoot
			if anyOn() {
				next.RestartBoot = db.RestartPending
			}
			order = db.AppendLayers(order, db.LayerRestart)
			changes = append(changes, "until restart")
		}
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
	next.Order = strings.Join(order, ",")
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
	if next.TimerAfter != old.TimerAfter {
		locks = append(locks, lockToSet{target, "timer lock", (time.Duration(next.TimerAfter)*time.Second).String() + ", counting from when the locks before it are unlocked"})
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
		length := "until the computer restarts"
		if next.RestartBoot == db.RestartPending {
			length = "until a restart after the locks before it are unlocked"
		}
		locks = append(locks, lockToSet{target, "restart lock", length})
	}
	if order := next.Layers(); len(order) > 1 {
		locks = append(locks, lockToSet{target, "unlock order", strings.Join(order, " → ")})
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
	if s.lock.TimerAfter > 0 {
		return fmt.Errorf("%s has a timer lock of %s that starts once the locks before it are unlocked", name, time.Duration(s.lock.TimerAfter)*time.Second)
	}
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
	s, relocked, rest, err := unlockSetting(database, name, name,
		fmt.Sprintf("%s is not locked. Use '%s'", name, settingCommand(name, "off")))
	if err != nil {
		return err
	}
	if relocked {
		fmt.Printf("%s is unlocked for %s and stays on; then its locks are on again (re-lock). '%s' turns it off meanwhile.\n",
			name, time.Duration(s.lock.Relock)*time.Second, settingCommand(name, "off"))
	}
	if len(rest) > 0 {
		nextSettingLock(name, s, rest, settingCommand(name, "unlock"))
		return nil
	}
	if relocked {
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

// unlockSetting unlocks the first of a setting's locks still on (setting
// lock name, shown as label; R-LOCK-6): it asks for the settings password,
// refuses while that first lock is a timer or restart lock, asks for the
// lock password or runs the challenge. Without re-lock the lock is gone;
// with re-lock it is lifted (relocked true once text and password are both
// lifted), then on again (§3.4). rest are the locks still on after it.
func unlockSetting(database *db.DB, name, label, notLocked string) (s settingLockState, relocked bool, rest []string, err error) {
	if err := requireSettingsPassword(database); err != nil {
		return s, false, nil, err
	}
	if s, err = loadSettingLock(database, name); err != nil {
		return s, false, nil, err
	}
	on := s.lock.LayersOn(s.trusted, s.bootID)
	if len(on) == 0 {
		return s, false, nil, errors.New(notLocked)
	}
	switch on[0] {
	case db.LayerTimer, db.LayerRestart:
		return s, false, nil, refused(database, "", s.refuseWhileHeld(label))
	case db.LayerPassword:
		password, err := readPasswordStdin("Setting password")
		if err != nil {
			return s, false, nil, err
		}
		if !db.CheckPassword(s.lock.PasswordHash, password) {
			return s, false, nil, refused(database, "", fmt.Errorf("wrong password; %s stays locked", label))
		}
	case db.LayerText:
		ok, err := lock.RunStyled(s.lock.Style, s.lock.LockChars)
		if err != nil {
			return s, false, nil, fmt.Errorf("failed to run the typing challenge: %w", err)
		}
		if !ok {
			_ = refused(database, "", fmt.Errorf("failed the unlock challenge"))
			fmt.Printf("Unlock failed. %s stays locked.\n", label)
			os.Exit(1)
		}
	}
	s.lock.LiftTextLayer(on[0], s.trusted, s.bootID)
	rest = s.lock.LayersOn(s.trusted, s.bootID)
	relocked = s.lock.Relock > 0 && s.lock.LiftedUntil > s.trusted
	if len(rest) == 0 && !relocked {
		return s, false, nil, database.ClearSettingLock(name)
	}
	return s, relocked, rest, database.SetSettingLock(s.lock)
}

// nextSettingLock says which lock holds a setting after one was unlocked.
func nextSettingLock(label string, s settingLockState, rest []string, unlockCmd string) {
	switch rest[0] {
	case db.LayerTimer:
		fmt.Printf("%s: next, its timer lock until %s.\n", label, time.Unix(s.lock.LockedUntil, 0).Format("2006-01-02 15:04"))
	case db.LayerRestart:
		fmt.Printf("%s: next, its restart lock, until the computer restarts.\n", label)
	default:
		fmt.Printf("%s: next, its %s ('%s').\n", label, db.LayerName(rest[0]), unlockCmd)
	}
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
	trackLockFlagOrder(browserCmd)
	rootCmd.AddCommand(browserCmd)
}
