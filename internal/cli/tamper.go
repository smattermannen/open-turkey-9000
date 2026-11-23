// tamper.go — tamper protection settings (§4.10).
//
//	kc9k tamper                                       # shows them
//	kc9k tamper clock on|off                          # block time and date changes
//	kc9k tamper file-urls on|off                      # allow file:// blocking
//	kc9k tamper guest|extension-stop|installer|inactive-tabs|embedded on|off
//	kc9k tamper unsupported-browsers on [lock flags]  # block unsupported browsers
//	kc9k tamper unlock unsupported-browsers
//
// They are protection settings: turning one on is always allowed; turning
// one off needs the settings password and is refused while any block or
// setting is locked (R-LOCK-9). extension-stop, inactive-tabs and embedded
// are on until turned off (enforce.TamperOn). Block unsupported browsers also has its own
// setting lock, carried over from the browser setting force-librewolf it
// replaced (decision 2026-11-14).
package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/smattermannen/kc9k/internal/lock"
	"github.com/spf13/cobra"
)

var tamperCmd = &cobra.Command{
	Use:   "tamper [SETTING [on|off] | grace SECONDS | unlock unsupported-browsers]",
	Short: "Tamper protection: clock changes, file:// blocking, unsupported browsers",
	Long: `Tamper protection settings:

  clock                 while a timer lock runs, changing the date or time
                        doesn't move it (time the computer is switched off
                        isn't counted then either)
  task-managers         close task managers (System Monitor, htop, …) while
                        anything is locked
  private               close private windows and Tor: private windows are
                        turned off in every browser (they close when it
                        restarts) and Tor Browser is closed
  guest                 close guest profiles: guest browsing off in every
                        Chromium-family browser (Chrome and Chromium always)
                        and no guest session at the login screen (LightDM)
  extension-stop        close browsers if the extension stops (on unless
                        turned off): browsers without the Killer Chicken
                        extension are closed while a block needs it; off,
                        only while anything is locked
  installer             block the installer: while anything is locked,
                        reinstalling or installing an older version is
                        refused too (by the package manager and install.sh);
                        removal always is
  inactive-tabs         block inactive tabs (on unless turned off): tabs in
                        the background are blocked at once; off, when you
                        switch to them
  embedded              block embedded content (on unless turned off): a
                        blocked site inside another page gets the block
                        page; off, the extension leaves frames alone (hosts
                        and the firewall still block whole domains)
  grace SECONDS         how long a browser may run without the extension
                        checking in (10 to 600; default 60); longer weakens
  file-urls             allow file:// blocking: blacklist entries such as
                        file:///home/me/games/* block local files in the
                        browsers (a block's own file:// exceptions, file://*
                        by default, still spare them)
  unsupported-browsers  block unsupported browsers: every browser but the
                        supported ones (LibreWolf, Google Chrome, Chromium)
                        running the Killer Chicken extension is closed, at
                        any time; see 'kc9k browsers'. Can't be turned on
                        while no supported browser is installed, and nothing
                        is closed while none is.

Turning one off is refused while any block or setting is locked.

unsupported-browsers can also be locked when turned on, like a browser setting:

  --for DURATION    Timer lock: can't be turned off for this long.
  --until HH:MM     Timer lock until a clock time today (or tomorrow).
  --lock            Text lock ('kc9k tamper unlock unsupported-browsers').
  --lock-chars N    Challenge length in characters (default 300).
  --password-stdin  Password lock.
  --restart         Stays on until the computer restarts.
  --range SPEC      Range lock: on at these times.
  --relock D        Re-lock: D after its last lock is unlocked, every lock comes
                    on again, in order.

Locks stack as on a block: an option given twice is two locks, a lock added
later goes last, and a lock that is on is never changed. unlock lifts the
first lock still on, then turns it off if nothing else is locked.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 0 {
			for _, name := range tamperOrder {
				fmt.Printf("%s %s\n", name, onOff(enforce.TamperOn(database, simpleTamper[name])))
			}
			fmt.Printf("grace %ds\n", enforce.ExtensionGraceSeconds(database))
			return printUnsupportedBrowsers(database)
		}
		if args[0] == "unlock" {
			if len(args) != 2 || (args[1] != db.UnsupportedBrowsers && args[1] != forceLibreWolfAlias) {
				return fmt.Errorf("use 'kc9k tamper unlock %s'", db.UnsupportedBrowsers)
			}
			return unlockBrowserSetting(database, db.UnsupportedBrowsers)
		}
		if args[0] == db.UnsupportedBrowsers {
			if len(args) == 1 {
				return printUnsupportedBrowsers(database)
			}
			switch args[1] {
			case "on":
				if err := browserSettingOn(cmd, database, db.UnsupportedBrowsers); err != nil {
					return err
				}
				if !lockFlagsGiven(cmd) {
					fmt.Println("Block unsupported browsers is on.")
				}
				return nil
			case "off":
				if lockFlagsGiven(cmd) {
					return fmt.Errorf("lock flags only go with 'on'")
				}
				if err := browserSettingOff(database, db.UnsupportedBrowsers); err != nil {
					return err
				}
				fmt.Println("Block unsupported browsers is off.")
				return nil
			}
			return fmt.Errorf("use 'on' or 'off'")
		}
		if args[0] == "grace" {
			return tamperGrace(database, args[1:])
		}
		key, ok := simpleTamper[args[0]]
		if !ok {
			return fmt.Errorf("unknown tamper protection %q (use %s, grace or %s)", args[0], strings.Join(tamperOrder, ", "), db.UnsupportedBrowsers)
		}
		if lockFlagsGiven(cmd) {
			return fmt.Errorf("the %s setting has no lock of its own", args[0])
		}
		if len(args) == 1 {
			fmt.Printf("%s %s\n", args[0], onOff(enforce.TamperOn(database, key)))
			return nil
		}
		switch args[1] {
		case "on":
		case "off":
			if err := requireSettingsPassword(database); err != nil {
				return err
			}
			now := time.Now()
			trusted, err := database.TrustedNow(now)
			if err != nil {
				return err
			}
			if locked, err := enforce.AnyLock(database, now, trusted); err != nil {
				return err
			} else if locked {
				return refused(database, "", fmt.Errorf("tamper protection can't be turned off while a block or setting is locked"))
			}
		default:
			return fmt.Errorf("use 'on' or 'off'")
		}
		if err := database.SetSetting(key, args[1]); err != nil {
			return err
		}
		fmt.Printf(tamperMessages[args[0]]+"\n", args[1])
		switch args[0] {
		case "clock", "task-managers", "installer":
			return nil // read where they act
		}
		return enforce.ApplyFromDB(database)
	}),
}

// simpleTamper are the tamper protection settings without a lock of their
// own, by name: on is free, off refused while anything is locked.
var simpleTamper = map[string]string{"clock": db.SettingTamperClock, "file-urls": enforce.SettingFileURLs,
	"task-managers": enforce.SettingTaskManagers, "private": enforce.SettingTamperPrivate,
	"guest": enforce.SettingTamperGuest, "extension-stop": enforce.SettingExtensionStop,
	"installer": enforce.SettingTamperInstaller, "inactive-tabs": enforce.SettingInactiveTabs,
	"embedded": enforce.SettingEmbedded}

// tamperOrder is the order 'kc9k tamper' shows the simple ones in.
var tamperOrder = []string{"clock", "file-urls", "task-managers", "private", "guest", "extension-stop",
	"installer", "inactive-tabs", "embedded"}

// tamperMessages say what a simple setting does when switched.
var tamperMessages = map[string]string{
	"clock":          "Tamper protection for the clock is %s",
	"file-urls":      "file:// blocking is %s",
	"task-managers":  "Closing task managers while locked is %s",
	"private":        "Closing private windows and Tor is %s",
	"guest":          "Closing guest profiles is %s",
	"extension-stop": "Closing browsers if the extension stops is %s",
	"installer":      "Blocking the installer while locked is %s",
	"inactive-tabs":  "Blocking inactive tabs is %s",
	"embedded":       "Blocking embedded content is %s",
}

func tamperClockOn(database *db.DB) bool {
	v, err := database.GetSetting(db.SettingTamperClock)
	return err == nil && v == "on"
}

func printUnsupportedBrowsers(database *db.DB) error {
	state := onOff(enforce.UnsupportedBrowsersOn(database))
	s, err := loadSettingLock(database, db.UnsupportedBrowsers)
	if err != nil {
		return err
	}
	if text := s.describe(); text != "" {
		state += "  (" + text + ")"
	}
	fmt.Printf("%s %s\n", db.UnsupportedBrowsers, state)
	return nil
}

func init() {
	tamperCmd.Flags().String("for", "", "Timer lock: can't be turned off for this long (e.g. 90m, 2h, 3d, 1w)")
	tamperCmd.Flags().String("until", "", "Timer lock: can't be turned off until this time (HH:MM, or a date: YYYY-MM-DD HH:MM)")
	tamperCmd.Flags().Bool("lock", false, "Text lock ('kc9k tamper unlock unsupported-browsers')")
	tamperCmd.Flags().Int("lock-chars", lock.DefaultLockChars, "Number of characters in the unlock challenge")
	tamperCmd.Flags().Bool("password-stdin", false, "Password lock: read the password from standard input")
	tamperCmd.Flags().Bool("partner-holds", false, "Password locks: KC9K makes the password up and sends it only to your accountability partners (§4.15)")
	tamperCmd.Flags().Bool("restart", false, "Restart lock: stays on until the computer restarts")
	addTextLockFlags(tamperCmd)
	addRelockFlag(tamperCmd)
	addRangeFlags(tamperCmd)
	addYesFlag(tamperCmd)
	trackLockFlagOrder(tamperCmd)
	rootCmd.AddCommand(tamperCmd)
}

// tamperGrace shows or sets the grace for a missing extension. A longer
// grace weakens: refused while anything is locked, like turning tamper
// protection off.
func tamperGrace(database *db.DB, args []string) error {
	cur := enforce.ExtensionGraceSeconds(database)
	if len(args) == 0 {
		fmt.Printf("grace %ds\n", cur)
		return nil
	}
	n, err := strconv.Atoi(strings.TrimSuffix(args[0], "s"))
	if err != nil || n < 10 || n > 600 {
		return fmt.Errorf("the grace is 10 to 600 seconds")
	}
	if n > cur {
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
		now := time.Now()
		trusted, err := database.TrustedNow(now)
		if err != nil {
			return err
		}
		if locked, err := enforce.AnyLock(database, now, trusted); err != nil {
			return err
		} else if locked {
			return refused(database, "", fmt.Errorf("a longer grace weakens tamper protection: refused while a block or setting is locked"))
		}
	}
	if err := database.SetSetting(enforce.SettingExtensionGrace, strconv.Itoa(n)); err != nil {
		return err
	}
	fmt.Printf("Browsers without the extension are closed after %d seconds\n", n)
	return nil
}
