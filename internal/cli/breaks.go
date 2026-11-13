// breaks.go — kc9k break: breaks a block allows.
//
//	kc9k break                           # what each block allows and has earned
//	kc9k break allowance work 10m --per 1h
//	kc9k break allowance work off
//	kc9k break relock work 5m            # unlocking gives a 5-minute break
//	kc9k break start work [5m]           # take earned break time
//	kc9k break end work
package cli

import (
	"fmt"
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

Allowing more breaks loosens a block, so it is refused while the block is
held. Allowing fewer, or none, is always possible.`,
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
				fmt.Printf("  allowance: %s per %s of blocking; %s earned\n", seconds(k.Allowance), seconds(k.Per), seconds(k.Balance()))
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

// parseBreak parses a break length.
func parseBreak(text string, max int64) (int64, error) {
	d, err := time.ParseDuration(text)
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
	Use:   "allowance [block] [time|off]",
	Short: "Break time earned per stretch of blocking, e.g. 10m --per 1h",
	Args:  cobra.ExactArgs(2),
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
			k.BalanceMs = min(k.BalanceMs, k.Allowance*1000)
		}
		if err := saveBreaks(database, args[0], old, k); err != nil {
			return err
		}
		if k.Allowance == 0 {
			fmt.Printf("Block '%s' has no break allowance.\n", args[0])
		} else {
			fmt.Printf("Block '%s' earns %s of break per %s of blocking.\n", args[0], seconds(k.Allowance), seconds(k.Per))
		}
		return nil
	}),
}

var breakRelockCmd = &cobra.Command{
	Use:   "relock [block] [time|off]",
	Short: "Make unlocking a break of this length instead of the end of the block",
	Args:  cobra.ExactArgs(2),
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
		k, err := database.GetBreaks(name)
		if err != nil {
			return err
		}
		if k.Allowance == 0 {
			return fmt.Errorf("block '%s' has no break allowance", name)
		}
		trusted, err := database.TrustedNow(time.Now())
		if err != nil {
			return err
		}
		if until, _ := database.PausedUntil(name); until > trusted {
			return fmt.Errorf("block '%s' is already on a break for %s more", name, seconds(until-trusted))
		}
		take := k.Balance()
		if len(args) == 2 {
			want, err := parseBreak(args[1], db.MaxBreakSeconds)
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
		k.BalanceMs -= take * 1000
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
			k.BalanceMs = min(k.BalanceMs+(until-trusted)*1000, k.Allowance*1000)
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
	breakCmd.AddCommand(breakAllowanceCmd, breakRelockCmd, breakStartCmd, breakEndCmd)
	rootCmd.AddCommand(breakCmd)
}
