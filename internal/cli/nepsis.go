// nepsis.go — monk mode and bypass prevention.
//
//	kc9k monk --for 1h                       # monk mode: the monk-mode block, time-locked
//	kc9k nepsis                              # show bypass prevention
//	kc9k nepsis set --count 10 --within 5m --then monk --then-for 1h [--block B]
//	kc9k nepsis set ... --for 7d --lock      # global only: lock it like a browser option
//	kc9k nepsis off [--block B]
//	kc9k nepsis unlock                       # global: challenge or password
//
// Bypass prevention counts what KC9K stops (blocked pages, closed apps and
// windows, closed browsers, refused attempts to loosen or stop a held block).
// When --count of them happen within --within, it takes the consequence.
// The global one counts everything; a block's own counts that block's while
// it is on. Making either stricter is always allowed. Loosening a block's is
// gated like other edits to the block; loosening the global one is refused
// while its lock holds.
package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/smattermannen/kc9k/internal/lock"
	"github.com/spf13/cobra"
)

var monkCmd = &cobra.Command{
	Use:   "monk --for DURATION",
	Short: "Monk mode: block every website for a while, locked",
	Long: `Starts the block 'monk-mode' with a time lock that ends it. It is made the
first time with every website blocked (*.* in its Blocked list); add
exceptions, apps or anything else to it like any block. On a monk mode that
is already on, a longer time extends it.`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		length, _ := cmd.Flags().GetString("for")
		d, err := time.ParseDuration(length)
		if err != nil || d < time.Minute {
			return fmt.Errorf("give --for, at least 1m (e.g. 45m, 2h)")
		}
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
		if err := enforce.StartMonk(database, d); err != nil {
			return err
		}
		if err := enforce.ApplyFromDB(database); err != nil {
			return err
		}
		b, err := database.GetBlock(enforce.MonkBlock)
		if err != nil {
			return err
		}
		fmt.Printf("Monk mode is on until %s.\n", time.Unix(b.LockedUntil, 0).Format("2006-01-02 15:04"))
		return nil
	}),
}

// describeNepsis renders a bypass prevention, e.g. "10 within 5 min: monk for 1h0m0s".
func describeNepsis(n db.Nepsis) string {
	if !n.On() {
		return "off"
	}
	then := n.Consequence
	if n.ActionMinutes > 0 {
		then += " for " + (time.Duration(n.ActionMinutes) * time.Minute).String()
	}
	return fmt.Sprintf("%d within %d min: %s", n.Count, n.Minutes, then)
}

// readNepsis reads --count, --within, --then and --then-for.
func readNepsis(cmd *cobra.Command) (db.Nepsis, error) {
	var n db.Nepsis
	n.Count, _ = cmd.Flags().GetInt("count")
	within, _ := cmd.Flags().GetString("within")
	n.Consequence, _ = cmd.Flags().GetString("then")
	thenFor, _ := cmd.Flags().GetString("then-for")
	w, err := time.ParseDuration(within)
	if err != nil || w%time.Minute != 0 {
		return n, fmt.Errorf("invalid --within %q: give whole minutes, like 5m or 1h", within)
	}
	n.Minutes = int(w / time.Minute)
	if thenFor != "" {
		d, err := time.ParseDuration(thenFor)
		if err != nil || d%time.Minute != 0 || d < 0 {
			return n, fmt.Errorf("invalid --then-for %q: give whole minutes, like 30m or 2h", thenFor)
		}
		n.ActionMinutes = int(d / time.Minute)
	}
	return n, n.Check()
}

func nepsisEdit(n db.Nepsis) db.Edit {
	return db.Edit{Action: db.EditSetNepsis, Values: []string{strconv.Itoa(n.Count), strconv.Itoa(n.Minutes),
		n.Consequence, strconv.Itoa(n.ActionMinutes)}}
}

// setBlockNepsis changes a block's bypass prevention; loosening is gated.
func setBlockNepsis(cmd *cobra.Command, database *db.DB, block string, n db.Nepsis) error {
	old, err := database.BlockNepsis(block)
	if err != nil {
		return err
	}
	if old == n {
		return nil
	}
	proceed, err := gateEdit(cmd, database, block, n.StricterThan(old), nepsisEdit(n))
	if err != nil || !proceed {
		return err
	}
	if err := database.SetBlockNepsis(block, n); err != nil {
		return err
	}
	fmt.Printf("Bypass prevention for block '%s': %s\n", block, describeNepsis(n))
	return nil
}

// setGlobalNepsis changes the global bypass prevention, with any locks the
// flags ask for. Loosening is refused while a lock holds.
func setGlobalNepsis(cmd *cobra.Command, database *db.DB, n db.Nepsis) error {
	old, err := database.GlobalNepsis()
	if err != nil {
		return err
	}
	if !n.StricterThan(old) {
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
		s, err := loadSettingLock(database, db.NepsisLockName)
		if err != nil {
			return err
		}
		if err := s.refuseWhileHeld("bypass prevention"); err != nil {
			return refused(database, "", err)
		}
		if s.lock.LockChars > 0 || s.lock.PasswordHash != "" {
			return refused(database, "", fmt.Errorf("bypass prevention is locked. Use 'kc9k nepsis unlock' first"))
		}
		if err := database.ClearSettingLock(db.NepsisLockName); err != nil { // locks that ran out
			return err
		}
	}
	if lockFlagsGiven(cmd) {
		if !n.On() {
			return fmt.Errorf("lock options only go with 'set'")
		}
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
		want, err := requestedSettingLock(cmd, database, db.NepsisLockName)
		if err != nil {
			return err
		}
		cur, err := loadSettingLock(database, db.NepsisLockName)
		if err != nil {
			return err
		}
		next, changes, err := strengthenSettingLock(cur, want, cmd.Flags().Changed("lock-chars"))
		if err != nil {
			return err
		}
		if err := database.SetSettingLock(next); err != nil {
			return err
		}
		if len(changes) > 0 {
			fmt.Printf("Bypass prevention %s\n", strings.Join(changes, ", "))
		}
	}
	if err := database.SetGlobalNepsis(n); err != nil {
		return err
	}
	fmt.Printf("Bypass prevention for the whole computer: %s\n", describeNepsis(n))
	return nil
}

var nepsisCmd = &cobra.Command{
	Use:   "nepsis",
	Short: "Bypass prevention: act when KC9K stops many things in a short time",
	Long: `Bypass prevention counts what KC9K stops: blocked pages, closed apps and
windows, browsers closed for running without the extension, and refused
attempts to loosen or stop a held block. When --count of them happen within
--within, it takes a consequence:

  close-browser   close every browser
  lock-screen     lock the screen (--then-for, required)
  logout          log out (--then-for: stay logged out)
  shutdown        shut down (--then-for: stay off)
  monk            monk mode (--then-for, required; see 'kc9k monk')

Staying locked, logged out or off goes through Frozen Chicken (at most a
day). The whole computer has one (it counts every event), and each block
can have its own (--block: it counts that block's events while it is on);
when both are set, both apply. Without a subcommand, shows them.

Making one stricter (fewer events, a longer window, the same consequence for
longer) is always allowed. Anything else loosens it: a block's is gated like
other edits to the block, and the global one can be locked like a browser
option (--for, --until, --lock, --password-stdin, --restart on 'set').`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		now := time.Now()
		n, err := database.GlobalNepsis()
		if err != nil {
			return err
		}
		line := "whole computer: " + describeNepsis(n)
		if n.On() {
			count, _ := enforce.NepsisCount(database, now, "", n)
			line += fmt.Sprintf(" (%d so far)", count)
		}
		if s, err := loadSettingLock(database, db.NepsisLockName); err == nil && s.describe() != "" {
			line += "  (" + s.describe() + ")"
		}
		fmt.Println(line)
		blocks, err := database.ListBlocks()
		if err != nil {
			return err
		}
		for _, b := range blocks {
			bn, err := database.BlockNepsis(b.Name)
			if err != nil {
				return err
			}
			if bn.On() {
				count, _ := enforce.NepsisCount(database, now, b.Name, bn)
				fmt.Printf("block %s: %s (%d so far)\n", b.Name, describeNepsis(bn), count)
			}
		}
		return nil
	}),
}

var nepsisSetCmd = &cobra.Command{
	Use:   "set --count N --within D --then CONSEQUENCE [--then-for D] [--block B]",
	Short: "Turn bypass prevention on, or change it",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		n, err := readNepsis(cmd)
		if err != nil {
			return err
		}
		if block, _ := cmd.Flags().GetString("block"); block != "" {
			if lockFlagsGiven(cmd) {
				return fmt.Errorf("a block's bypass prevention is held by the block's own locks")
			}
			return setBlockNepsis(cmd, database, block, n)
		}
		return setGlobalNepsis(cmd, database, n)
	}),
}

var nepsisOffCmd = &cobra.Command{
	Use:   "off [--block B]",
	Short: "Turn bypass prevention off (loosens it)",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if block, _ := cmd.Flags().GetString("block"); block != "" {
			return setBlockNepsis(cmd, database, block, db.Nepsis{})
		}
		return setGlobalNepsis(cmd, database, db.Nepsis{})
	}),
}

var nepsisUnlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Remove the challenge or password lock on the global bypass prevention",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
		s, err := loadSettingLock(database, db.NepsisLockName)
		if err != nil {
			return err
		}
		if err := s.refuseWhileHeld("bypass prevention"); err != nil {
			return refused(database, "", err)
		}
		if s.lock.LockChars == 0 && s.lock.PasswordHash == "" {
			return fmt.Errorf("bypass prevention is not locked")
		}
		if s.lock.PasswordHash != "" {
			password, err := readPasswordStdin("Setting password")
			if err != nil {
				return err
			}
			if !db.CheckPassword(s.lock.PasswordHash, password) {
				return refused(database, "", fmt.Errorf("wrong password; bypass prevention stays locked"))
			}
		}
		if s.lock.LockChars > 0 {
			ok, err := lock.RunChallenge(s.lock.LockChars)
			if err != nil {
				return fmt.Errorf("failed to run the typing challenge: %w", err)
			}
			if !ok {
				_ = refused(database, "", fmt.Errorf("failed the unlock challenge"))
				fmt.Println("Unlock failed. Bypass prevention stays locked.")
				os.Exit(1)
			}
		}
		if err := database.ClearSettingLock(db.NepsisLockName); err != nil {
			return err
		}
		fmt.Println("Bypass prevention is unlocked; it stays on until you change it.")
		return nil
	}),
}

func init() {
	monkCmd.Flags().String("for", "", "How long (e.g. 45m, 2h)")
	nepsisSetCmd.Flags().Int("count", 0, "How many events")
	nepsisSetCmd.Flags().String("within", "", "Within how long (e.g. 5m)")
	nepsisSetCmd.Flags().String("then", "", "The consequence: "+strings.Join(db.Consequences, ", "))
	nepsisSetCmd.Flags().String("then-for", "", "How long the consequence lasts (e.g. 1h)")
	for _, c := range []*cobra.Command{nepsisSetCmd, nepsisOffCmd} {
		c.Flags().String("block", "", "A block's own bypass prevention, instead of the global one")
	}
	nepsisSetCmd.Flags().String("for", "", "Global only. Time lock: can't be loosened for this long")
	nepsisSetCmd.Flags().String("until", "", "Global only. Time lock until this clock time (HH:MM)")
	nepsisSetCmd.Flags().Bool("lock", false, "Global only. Typing challenge lock ('kc9k nepsis unlock')")
	nepsisSetCmd.Flags().Int("lock-chars", lock.DefaultLockChars, "Number of characters in the unlock challenge")
	nepsisSetCmd.Flags().Bool("password-stdin", false, "Global only. Password lock, read from standard input")
	nepsisSetCmd.Flags().Bool("restart", false, "Global only. Can't be loosened until the computer restarts")
	addNowFlag(nepsisSetCmd, nepsisOffCmd)
	nepsisCmd.AddCommand(nepsisSetCmd, nepsisOffCmd, nepsisUnlockCmd)
	rootCmd.AddCommand(monkCmd, nepsisCmd)
}
