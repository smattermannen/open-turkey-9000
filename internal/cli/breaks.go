// breaks.go — kc9k break: breaks a block allows.
//
//	kc9k break                           # what each block allows and has earned
//	kc9k break allowance work 10m --per 1h
//	kc9k break allowance work off
//	kc9k break text work 5m            # unlocking gives a 5-minute break
//	kc9k break start work [5m]           # take earned break time
//	kc9k break end work
package cli

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var breakCmd = &cobra.Command{
	Use:   "break",
	Short: "Breaks a block allows: an allowance you earn, and unlocking as a break",
	Long: `A block can allow breaks, decided before it is locked.

An allowance earns break time while the block is on, for example 10 minutes
for every hour of blocking, saved up to one allowance. Take it when you
choose with 'break start'; the block is lifted for that long and comes back
by itself, locks and all. 'break end' ends a break early and keeps the rest.

With relock set, unlocking the block (its typing challenge or password)
doesn't end it: it lifts it for that long, then it is back with its locks.
Unlocking once more during that break lifts its locks; the block stays on.

Allowing more breaks weakens a block, so it is refused while the block is
locked. Allowing fewer, or none, is always possible.`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		blocks, err := database.ListBlocks()
		if err != nil {
			return err
		}
		trusted, err := database.TrustedNow(time.Now())
		if err != nil {
			return err
		}
		any := false
		for _, b := range blocks {
			k, err := database.GetBreaks(b.Name)
			if err != nil {
				return err
			}
			until, _ := database.PausedUntil(b.Name)
			if k == (db.Breaks{}) && until <= trusted {
				continue
			}
			any = true
			fmt.Printf("%s:\n", b.Name)
			if k.Allowance > 0 {
				fmt.Printf("  reward: %s per %s of blocking, %s; %s earned\n", seconds(k.Allowance), seconds(k.Per), keepWords(k.Keep), seconds(k.Balance()))
			}
			if k.Relock > 0 {
				fmt.Printf("  unlocking gives a break of %s\n", seconds(k.Relock))
			}
			if until > trusted {
				fmt.Printf("  on a break for %s more\n", seconds(until-trusted))
			}
		}
		if !any {
			fmt.Println("No block allows breaks.")
		}
		return nil
	}),
}

func seconds(n int64) time.Duration { return time.Duration(n) * time.Second }

// parseKeep reads --keep: one, a period (window, day, week, month) or a
// length of at least an hour (e.g. 10d), with no upper limit (§3.6).
func parseKeep(text string) (string, error) {
	text = strings.ToLower(strings.TrimSpace(text))
	switch {
	case text == "one" || text == "":
		return "", nil
	case slices.Contains(db.RewardKeeps, text):
		return text, nil
	}
	d, err := parseUserDuration(text)
	if err != nil || d < time.Hour {
		return "", fmt.Errorf("--keep is one, window, day, week, month, or a time of at least 1h (such as 10d)")
	}
	return strconv.FormatInt(int64(d/time.Second), 10), nil
}

// keepWords says how a reward is kept.
func keepWords(keep string) string {
	switch {
	case keep == "":
		return "saved up to one break"
	case db.KeepSeconds(keep) > 0:
		return "each part kept for " + humanDuration(seconds(db.KeepSeconds(keep))) + " after it is earned"
	}
	return "kept until the " + keep + " ends"
}

// parseBreak parses a break length.
func parseBreak(text string, max int64) (int64, error) {
	d, err := parseUserDuration(text)
	if err != nil || d < time.Minute || int64(d.Seconds()) > max {
		return 0, fmt.Errorf("%q isn't a time from 1m to %s", text, seconds(max))
	}
	return int64(d.Seconds()), nil
}

// saveBreaks saves new break settings, refusing looser ones on a held block.
func saveBreaks(database *db.DB, name string, old, k db.Breaks) error {
	if err := ensureEditable(database, name, !k.LooserThan(old)); err != nil {
		return err
	}
	return database.SetBreaks(name, k)
}

var breakAllowanceCmd = &cobra.Command{
	Use:     "reward [block] [time|off]",
	Aliases: []string{"allowance"},
	Short:   "Break time earned per stretch of blocking, e.g. 10m --per 1h",
	Args:    cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		old, err := database.GetBreaks(args[0])
		if err != nil {
			return err
		}
		if _, err := database.GetBlock(args[0]); err != nil {
			return err
		}
		k := old
		if args[1] == "off" {
			k.Allowance, k.Per, k.BalanceMs = 0, 0, 0
		} else {
			if k.Allowance, err = parseBreak(args[1], db.MaxBreakSeconds); err != nil {
				return err
			}
			per, _ := cmd.Flags().GetString("per")
			if k.Per, err = parseBreak(per, db.MaxBreakPer); err != nil {
				return fmt.Errorf("--per: %w", err)
			}
			if k.Allowance >= k.Per {
				return fmt.Errorf("the break must be shorter than the time it is earned over")
			}
			if cmd.Flags().Changed("keep") {
				keep, err := parseKeep(cmd.Flag("keep").Value.String())
				if err != nil {
					return err
				}
				trusted, err := database.TrustedNow(time.Now())
				if err != nil {
					return err
				}
				k = k.WithKeep(keep, enforce.RewardPeriod(database, args[0], keep, time.Unix(trusted, 0)), trusted)
			}
			if k.Keep == "" {
				k.BalanceMs = min(k.BalanceMs, k.Allowance*1000)
			}
		}
		if err := saveBreaks(database, args[0], old, k); err != nil {
			return err
		}
		if k.Allowance == 0 {
			fmt.Printf("Block '%s' has no reward.\n", args[0])
		} else {
			fmt.Printf("Block '%s' earns %s of break per %s of blocking, %s.\n", args[0], seconds(k.Allowance), seconds(k.Per), keepWords(k.Keep))
		}
		return nil
	}),
}

var breakRelockCmd = &cobra.Command{
	Use:     "text [block] [time|off]",
	Aliases: []string{"relock"},
	Short:   "Text break: unlocking gives a break of this length, and the block stays locked",
	Args:    cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		old, err := database.GetBreaks(args[0])
		if err != nil {
			return err
		}
		if _, err := database.GetBlock(args[0]); err != nil {
			return err
		}
		k := old
		k.Relock = 0
		if args[1] != "off" {
			if k.Relock, err = parseBreak(args[1], db.MaxBreakSeconds); err != nil {
				return err
			}
		}
		if err := saveBreaks(database, args[0], old, k); err != nil {
			return err
		}
		if k.Relock == 0 {
			fmt.Printf("Unlocking block '%s' lifts its locks; the block stays on.\n", args[0])
		} else {
			fmt.Printf("Unlocking block '%s' gives a break of %s; unlocking again during the break lifts its locks.\n", args[0], seconds(k.Relock))
		}
		return nil
	}),
}

var breakDelayCmd = &cobra.Command{
	Use:   "delay [block] [length wait|off]",
	Short: "Delay break: asking for it ('break request') gives a break of length after a countdown of wait",
	Long: `A delay break (§3.6) is a preset break that starts after a countdown: with
'kc9k break delay work 10m 30m', 'kc9k break request work' starts a 30-minute
countdown, then the block is lifted for 10 minutes. The block stays locked.
A longer break or a shorter wait weakens; 'off' is always allowed.`,
	Args: cobra.RangeArgs(2, 3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		old, err := database.GetBreaks(args[0])
		if err != nil {
			return err
		}
		if _, err := database.GetBlock(args[0]); err != nil {
			return err
		}
		k := old
		if args[1] == "off" {
			k.DelayLength, k.DelayWait, k.DelayAt = 0, 0, 0
		} else {
			if len(args) != 3 {
				return fmt.Errorf("give the break's length and the wait, e.g. 10m 30m")
			}
			if k.DelayLength, err = parseBreak(args[1], db.MaxBreakSeconds); err != nil {
				return err
			}
			if k.DelayWait, err = parseBreak(args[2], db.MaxBreakPer); err != nil {
				return err
			}
		}
		if err := saveBreaks(database, args[0], old, k); err != nil {
			return err
		}
		if k.DelayLength == 0 {
			fmt.Printf("Block '%s' has no delay break.\n", args[0])
		} else {
			fmt.Printf("Block '%s': 'kc9k break request %s' gives a break of %s after %s.\n", args[0], args[0], seconds(k.DelayLength), seconds(k.DelayWait))
		}
		return nil
	}),
}

var breakRequestCmd = &cobra.Command{
	Use:   "request [block]",
	Short: "Ask for the delay break: it starts after its countdown",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		k, err := database.GetBreaks(args[0])
		if err != nil {
			return err
		}
		if k.DelayLength == 0 {
			return fmt.Errorf("block '%s' has no delay break ('kc9k break delay %s 10m 30m')", args[0], args[0])
		}
		trusted, err := database.TrustedNow(time.Now())
		if err != nil {
			return err
		}
		if k.DelayAt > trusted {
			return fmt.Errorf("the delay break of '%s' begins in %s", args[0], seconds(k.DelayAt-trusted))
		}
		k.DelayAt = trusted + k.DelayWait
		if err := database.SetBreaks(args[0], k); err != nil {
			return err
		}
		fmt.Printf("The break of '%s' begins in %s and lasts %s.\n", args[0], seconds(k.DelayWait), seconds(k.DelayLength))
		return nil
	}),
}

var breakStartCmd = &cobra.Command{
	Use:   "start [block] [time]",
	Short: "Take earned break time (all of it, or the time given)",
	Args:  cobra.RangeArgs(1, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		name := args[0]
		detail, err := database.GetBlock(name)
		if err != nil {
			return err
		}
		if !detail.Active {
			return fmt.Errorf("block '%s' is not on", name)
		}
		// §3.6: the open window's own break, once each time it is open.
		if taken, err := takeWindowBreak(database, name); err != nil || taken {
			return err
		}
		k, err := database.GetBreaks(name)
		if err != nil {
			return err
		}
		if k.Allowance == 0 {
			return fmt.Errorf("block '%s' has no reward", name)
		}
		trusted, err := database.TrustedNow(time.Now())
		if err != nil {
			return err
		}
		if until, _ := database.PausedUntil(name); until > trusted {
			return fmt.Errorf("block '%s' is already on a break for %s more", name, seconds(until-trusted))
		}
		k = k.Expire(enforce.RewardPeriod(database, name, k.Keep, time.Unix(trusted, 0)), trusted)
		take := k.Balance()
		if len(args) == 2 {
			// Any part of what is earned: a saved-up reward has no limit.
			want, err := parseBreak(args[1], 100*365*86400)
			if err != nil {
				return err
			}
			if want > take {
				return fmt.Errorf("only %s of break is earned so far", seconds(take))
			}
			take = want
		}
		if take < 60 {
			return fmt.Errorf("less than a minute of break is earned so far (%s per %s of blocking)", seconds(k.Allowance), seconds(k.Per))
		}
		k = k.Take(take * 1000)
		if err := database.SetBreaks(name, k); err != nil {
			return err
		}
		if err := database.SetPause(name, trusted+take); err != nil {
			return err
		}
		if err := enforce.ApplyFromDB(database); err != nil {
			return err
		}
		fmt.Printf("Block '%s' is on a break for %s. It comes back by itself.\n", name, seconds(take))
		return nil
	}),
}

var breakEndCmd = &cobra.Command{
	Use:   "end [block]",
	Short: "End a break early; the rest of it is kept",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		name := args[0]
		trusted, err := database.TrustedNow(time.Now())
		if err != nil {
			return err
		}
		until, err := database.PausedUntil(name)
		if err != nil {
			return err
		}
		if until <= trusted {
			return fmt.Errorf("block '%s' is not on a break", name)
		}
		k, err := database.GetBreaks(name)
		if err != nil {
			return err
		}
		if k.Allowance > 0 {
			k = k.GiveBack((until - trusted) * 1000)
			if err := database.SetBreaks(name, k); err != nil {
				return err
			}
		}
		if err := database.SetPause(name, 0); err != nil {
			return err
		}
		if err := enforce.ApplyFromDB(database); err != nil {
			return err
		}
		fmt.Printf("The break of block '%s' is over.\n", name)
		return nil
	}),
}

func init() {
	breakAllowanceCmd.Flags().String("per", "1h", "The stretch of blocking the break is earned over")
	breakAllowanceCmd.Flags().String("keep", "one", "How reward is kept: one (up to one break); accumulating until the window, day, week or month ends; or each part for a time after it is earned (at least 1h, no upper limit, e.g. 10d)")
	breakCmd.AddCommand(breakAllowanceCmd, breakRelockCmd, breakDelayCmd, breakRequestCmd, breakStartCmd, breakEndCmd)
	rootCmd.AddCommand(breakCmd)
}

// takeWindowBreak gives the break of the block window open now, if it has
// one not yet taken this time; taken reports whether it did.
func takeWindowBreak(database *db.DB, name string) (bool, error) {
	schedules, err := database.ListSchedules(name)
	if err != nil {
		return false, err
	}
	now := time.Now()
	for _, s := range schedules {
		w := s.Window()
		if s.Break == 0 || s.Allow || !w.ActiveAt(now) {
			continue
		}
		occurrence := now.Format("2006-01-02") + " " + strconv.Itoa(w.Start)
		if s.BreakTaken == occurrence {
			if k, err := database.GetBreaks(name); err == nil && k.Allowance > 0 {
				return false, nil // earned reward can still be taken
			}
			return false, fmt.Errorf("the break of this window is taken already; the next one comes with its next opening")
		}
		trusted, err := database.TrustedNow(now)
		if err != nil {
			return false, err
		}
		if until, _ := database.PausedUntil(name); until > trusted {
			return false, fmt.Errorf("block '%s' is already on a break for %s more", name, seconds(until-trusted))
		}
		if err := database.TakeScheduleBreak(s.ID, occurrence); err != nil {
			return false, err
		}
		if err := database.SetPause(name, trusted+s.Break); err != nil {
			return false, err
		}
		if err := enforce.ApplyFromDB(database); err != nil {
			return false, err
		}
		fmt.Printf("Block '%s' is on a break for %s (this window's break).\n", name, seconds(s.Break))
		return true, nil
	}
	return false, nil
}
