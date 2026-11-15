// tamper.go — tamper protection settings (§4.10).
//
//	kc9k tamper                                       # shows them
//	kc9k tamper clock on|off                          # block time and date changes
//	kc9k tamper file-urls on|off                      # allow file:// blocking
//	kc9k tamper unsupported-browsers on [lock flags]  # block unsupported browsers
//	kc9k tamper unlock unsupported-browsers
//
// They are protection settings: turning one on is always allowed; turning
// one off needs the settings password and is refused while any block or
// setting is locked (R-LOCK-9). Block unsupported browsers also has its own
// setting lock, carried over from the browser setting force-librewolf it
// replaced (decision 2026-11-14).
package cli

import (
	"fmt"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/smattermannen/kc9k/internal/lock"
	"github.com/spf13/cobra"
)

var tamperCmd = &cobra.Command{
	Use:   "tamper [clock|file-urls|unsupported-browsers [on|off] | unlock unsupported-browsers]",
	Short: "Tamper protection: clock changes, file:// blocking, unsupported browsers",
	Long: `Tamper protection settings:

  clock                 while a timer lock runs, changing the date or time
                        doesn't move it (time the computer is switched off
                        isn't counted then either)
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
  --relock D        With a text or password lock: unlocking lifts them for D only.

Locks can only be made stronger. unlock runs the challenge and asks for the
password, then turns it off if nothing else is locked.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 0 {
			fmt.Printf("clock %s\n", onOff(tamperClockOn(database)))
			fmt.Printf("file-urls %s\n", onOff(enforce.FileURLsOn(database)))
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
		key, ok := simpleTamper[args[0]]
		if !ok {
			return fmt.Errorf("unknown tamper protection %q (use clock, file-urls or %s)", args[0], db.UnsupportedBrowsers)
		}
		if lockFlagsGiven(cmd) {
			return fmt.Errorf("the %s setting has no lock of its own", args[0])
		}
		if len(args) == 1 {
			v, _ := database.GetSetting(key)
			fmt.Printf("%s %s\n", args[0], onOff(v == "on"))
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
		if args[0] == "clock" {
			fmt.Printf("Tamper protection for the clock is %s\n", args[1])
			return nil
		}
		fmt.Printf("file:// blocking is %s\n", args[1])
		return enforce.ApplyFromDB(database)
	}),
}

// simpleTamper are the tamper protection settings without a lock of their
// own, by name: on is free, off refused while anything is locked.
var simpleTamper = map[string]string{"clock": db.SettingTamperClock, "file-urls": enforce.SettingFileURLs}

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
	tamperCmd.Flags().String("for", "", "Timer lock: can't be turned off for this long (e.g. 2h, 90m)")
	tamperCmd.Flags().String("until", "", "Timer lock: can't be turned off until this clock time (HH:MM)")
	tamperCmd.Flags().Bool("lock", false, "Text lock ('kc9k tamper unlock unsupported-browsers')")
	tamperCmd.Flags().Int("lock-chars", lock.DefaultLockChars, "Number of characters in the unlock challenge")
	tamperCmd.Flags().Bool("password-stdin", false, "Password lock: read the password from standard input")
	tamperCmd.Flags().Bool("restart", false, "Restart lock: stays on until the computer restarts")
	addTextLockFlags(tamperCmd)
	addRelockFlag(tamperCmd)
	addYesFlag(tamperCmd)
	trackLockFlagOrder(tamperCmd)
	rootCmd.AddCommand(tamperCmd)
}
