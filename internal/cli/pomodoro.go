// pomodoro.go — Pomodoro breaks, and renaming and copying blocks.
package cli

import (
	"fmt"
	"time"

	"github.com/brunodcdo/open-turkey/internal/db"
	"github.com/spf13/cobra"
)

const minPomodoroPhase = 10 * time.Second

var pomodoroCmd = &cobra.Command{
	Use:   "pomodoro",
	Short: "Alternate a block between work and breaks",
}

var pomodoroSetCmd = &cobra.Command{
	Use:   "set [block] [work] [break]",
	Short: "Set a cycle, e.g. 25m 5m (refused while the block is held)",
	Args:  cobra.ExactArgs(3),
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
		if err := ensureEditable(database, args[0], false); err != nil {
			return err
		}
		trusted, err := database.TrustedNow(time.Now())
		if err != nil {
			return err
		}
		if err := database.SetPomodoro(args[0], db.Pomodoro{On: int64(on.Seconds()), Off: int64(off.Seconds()), Anchor: trusted}); err != nil {
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
	pomodoroCmd.AddCommand(pomodoroSetCmd, pomodoroOffCmd)
	rootCmd.AddCommand(pomodoroCmd)
	blockCmd.AddCommand(blockRenameCmd, blockDuplicateCmd)
}
