// autostart.go — kc9k autostart (§3.6): rules that switch a block on.
//
//	kc9k autostart B                          # lists them
//	kc9k autostart B add once 2026-11-20 08:00
//	kc9k autostart B add at 08:00 [mon-fri]   # each day (or those days)
//	kc9k autostart B add sign-in              # at sign-in and waking up
//	kc9k autostart B add next-window          # when its next window opens
//	kc9k autostart B remove N
//
// Adding one is free; removing one weakens (refused while the block is
// locked). Autostart sets no lock (R-LOCK-11).
package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
	"github.com/spf13/cobra"
)

var autostartCmd = &cobra.Command{
	Use:   "autostart [block] [add SPEC | remove N]",
	Short: "Switch a block on by itself: once, at a time of day, at sign-in or wake, or at its next window",
	Args:  cobra.MinimumNArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		name := args[0]
		if _, err := database.GetBlock(name); err != nil {
			return err
		}
		if len(args) == 1 {
			rules, err := database.Autostarts(name)
			if err != nil {
				return err
			}
			if len(rules) == 0 {
				fmt.Printf("Block '%s' has no autostart.\n", name)
			}
			for _, a := range rules {
				fmt.Printf("  [%d] %s\n", a.Num, a)
			}
			return nil
		}
		switch args[1] {
		case "add":
			a, err := parseAutostart(name, args[2:], time.Now())
			if err != nil {
				return err
			}
			if err := database.AddAutostart(a); err != nil {
				return err
			}
			fmt.Printf("Block '%s' switches on by itself: %s\n", name, a)
			return nil
		case "remove":
			if len(args) != 3 {
				return fmt.Errorf("use 'remove N' (see 'kc9k autostart %s')", name)
			}
			n, err := strconv.Atoi(args[2])
			if err != nil {
				return fmt.Errorf("invalid number %q", args[2])
			}
			rules, err := database.Autostarts(name)
			if err != nil {
				return err
			}
			for _, a := range rules {
				if a.Num != n {
					continue
				}
				if _, err := gateEdit(cmd, database, name, false, db.Edit{Action: "remove-autostart", Values: []string{a.String()}}); err != nil {
					return err
				}
				if err := database.RemoveAutostart(a.ID); err != nil {
					return err
				}
				fmt.Printf("Autostart %d of '%s' removed.\n", n, name)
				return nil
			}
			return fmt.Errorf("block '%s' has no autostart %d", name, n)
		}
		return fmt.Errorf("use 'add SPEC' or 'remove N'")
	}),
}

// parseAutostart reads "once DATE HH:MM", "at HH:MM [DAYS]", "sign-in" or
// "next-window".
func parseAutostart(block string, words []string, now time.Time) (db.Autostart, error) {
	a := db.Autostart{Block: block}
	if len(words) == 0 {
		return a, fmt.Errorf("say when: once DATE HH:MM, at HH:MM [DAYS], sign-in or next-window")
	}
	a.Kind = words[0]
	switch a.Kind {
	case db.AutostartOnce:
		if len(words) != 3 {
			return a, fmt.Errorf("use: once 2026-11-20 08:00")
		}
		t, err := time.ParseInLocation("2006-01-02 15:04", words[1]+" "+words[2], time.Local)
		if err != nil {
			return a, fmt.Errorf("invalid date and time %q (want 2026-11-20 08:00)", words[1]+" "+words[2])
		}
		if !t.After(now) {
			return a, fmt.Errorf("%s has passed", t.Format("2006-01-02 15:04"))
		}
		a.At = t.Unix()
	case db.AutostartAt:
		if len(words) < 2 {
			return a, fmt.Errorf("use: at 08:00 [mon-fri]")
		}
		m, err := schedule.ParseTimeOfDay(words[1])
		if err != nil {
			return a, err
		}
		a.Minute, a.Days = m, schedule.AllDays
		if len(words) > 2 {
			if a.Days, err = schedule.ParseDays(strings.Join(words[2:], " ")); err != nil {
				return a, err
			}
		}
	case db.AutostartSignIn, db.AutostartWindow:
		if len(words) != 1 {
			return a, fmt.Errorf("%s takes nothing more", a.Kind)
		}
	default:
		return a, fmt.Errorf("unknown autostart %q (use once, at, sign-in or next-window)", a.Kind)
	}
	return a, nil
}

func init() {
	rootCmd.AddCommand(autostartCmd)
}
