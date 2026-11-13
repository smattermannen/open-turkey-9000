// pomodoro.go — Pomodoro breaks, and renaming and copying blocks.
package cli

import (
	"fmt"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

const minPomodoroPhase = 10 * time.Second

var pomodoroCmd = &cobra.Command{
	Use:   "pomodoro",
	Short: "Alternate a block between work and breaks",
}

var pomodoroSetCmd = &cobra.Command{
	Use:   "set [block] [work] [break]",
	Short: "Set a cycle, e.g. 25m 5m (refused while the block is locked)",
	Long: `Sets the block's rhythm: it is on for the work time and lifted for the
break, over and over, while it runs. With --long and --every, every so many
breaks is a long one:

  kc9k pomodoro set work 25m 5m --long 15m --every 4

While a block's work period runs, its quota session can't be opened.`,
	Args: cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		on, err := time.ParseDuration(args[1])
		if err != nil {
			return fmt.Errorf("invalid work time %q: %w", args[1], err)
		}
		off, err := time.ParseDuration(args[2])
		if err != nil {
			return fmt.Errorf("invalid break time %q: %w", args[2], err)
		}
		if on < minPomodoroPhase || off < minPomodoroPhase {
			return fmt.Errorf("work and break must each be at least %s", minPomodoroPhase)
		}
		p := db.Pomodoro{On: int64(on.Seconds()), Off: int64(off.Seconds())}
		longText, _ := cmd.Flags().GetString("long")
		every, _ := cmd.Flags().GetInt("every")
		if longText != "" || every != 0 {
			long, err := time.ParseDuration(longText)
			if err != nil || long < minPomodoroPhase {
				return fmt.Errorf("--long needs a time of at least %s, e.g. 15m", minPomodoroPhase)
			}
			if every < 2 || every > 20 {
				return fmt.Errorf("--every needs a number from 2 to 20: the long break comes after that many work periods")
			}
			p.Long, p.Every = int64(long.Seconds()), int64(every)
		}
		if err := ensureEditable(database, args[0], false); err != nil {
			return err
		}
		trusted, err := database.TrustedNow(time.Now())
		if err != nil {
			return err
		}
		p.Anchor = trusted
		if err := database.SetPomodoro(args[0], p); err != nil {
			return err
		}
		return reapplyIfActive(database, args[0])
	}),
}

var pomodoroOffCmd = &cobra.Command{
	Use:   "off [block]",
	Short: "Remove a cycle (always allowed: it makes the block stricter)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if err := database.ClearPomodoro(args[0]); err != nil {
			return err
		}
		return reapplyIfActive(database, args[0])
	}),
}

var blockRenameCmd = &cobra.Command{
	Use:   "rename [block] [new name]",
	Short: "Rename a block",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if args[0] == enforce.MonkBlock || args[1] == enforce.MonkBlock {
			return fmt.Errorf("the %s block belongs to monk mode and keeps its name", enforce.MonkBlock)
		}
		if err := ensureEditable(database, args[0], false); err != nil {
			return err
		}
		return database.RenameBlock(args[0], args[1])
	}),
}

var blockDuplicateCmd = &cobra.Command{
	Use:   "duplicate [block] [new name]",
	Short: "Copy a block's lists into a new inactive block",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		return database.DuplicateBlock(args[0], args[1])
	}),
}

func init() {
	pomodoroSetCmd.Flags().String("long", "", "A long break, e.g. 15m (with --every)")
	pomodoroSetCmd.Flags().Int("every", 0, "The long break comes after this many work periods")
	pomodoroCmd.AddCommand(pomodoroSetCmd, pomodoroOffCmd)
	rootCmd.AddCommand(pomodoroCmd)
	blockCmd.AddCommand(blockRenameCmd, blockDuplicateCmd)
}
