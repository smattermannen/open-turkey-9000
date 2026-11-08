// quota.go — Daily quotas: a block stays on, and a limited number of minutes
// per day can be spent with it open.
package cli

import (
	"fmt"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/smattermannen/kc9k/internal/quota"
	"github.com/spf13/cobra"
)

var quotaCmd = &cobra.Command{
	Use:   "quota",
	Short: "Manage daily quotas",
	Long: `Give a block a daily quota. The block stays on all the time, and
'quota start' opens it until the session ends or the minutes run out. All time
in an open session is spent, whether or not the sites are used. Unused time is
not saved, and the quota refills at midnight.

Lowering a quota applies at once. Raising or removing it applies at midnight.`,
}

// currentQuota loads a block's quota, advanced to now.
func currentQuota(database *db.DB, name string, now time.Time) (quota.State, error) {
	state, err := database.GetQuota(name)
	if err != nil {
		return quota.State{}, err
	}
	if state == nil {
		return quota.State{}, fmt.Errorf("block '%s' has no quota", name)
	}
	return state.Step(now.Unix(), quota.DayOf(now)), nil
}

// ensureBlockActive turns a quota block on or off. The daemon may make the same
// change at the same moment, so losing that race is not an error.
func ensureBlockActive(database *db.DB, name string, want bool) error {
	active, err := database.IsBlockActive(name)
	if err != nil || active == want {
		return err
	}
	if want {
		err = database.ActivateBlock(name, false, 0, 0, true)
	} else {
		err = database.DeactivateBlock(name)
	}
	if err != nil {
		if now, checkErr := database.IsBlockActive(name); checkErr == nil && now == want {
			return nil
		}
	}
	return err
}

var quotaSetCmd = &cobra.Command{
	Use:   "set [block] [duration]",
	Short: "Set a block's daily quota (e.g. 30m)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		d, err := time.ParseDuration(args[1])
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", args[1], err)
		}
		if err := quota.Validate(d); err != nil {
			return err
		}
		limit := int64(d.Seconds())

		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()
		if err := requireSettingsPassword(database); err != nil {
			return err
		}

		now := time.Now()
		existing, err := database.GetQuota(name)
		if err != nil {
			return err
		}
		var s quota.State
		if existing == nil {
			s = quota.New(limit, now.Unix(), quota.DayOf(now))
		} else {
			s = existing.Step(now.Unix(), quota.DayOf(now)).SetLimit(limit)
		}
		if err := database.SaveQuota(name, s); err != nil {
			return err
		}
		if s.PendingLimit == limit {
			fmt.Printf("Quota of '%s' becomes %s at midnight\n", name, d)
			return nil
		}

		detail, err := database.GetBlock(name)
		if err != nil {
			return err
		}
		if !detail.Active && !s.Open {
			if err := ensureBlockActive(database, name, true); err != nil {
				return err
			}
			if err := enforce.ApplyFromDB(database); err != nil {
				return err
			}
		}
		fmt.Printf("Quota of '%s' is %s per day\n", name, d)
		return nil
	},
}

var quotaRemoveCmd = &cobra.Command{
	Use:   "remove [block]",
	Short: "Remove a block's quota at midnight",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()
		if err := requireSettingsPassword(database); err != nil {
			return err
		}

		s, err := currentQuota(database, args[0], time.Now())
		if err != nil {
			return err
		}
		s.PendingLimit = 0
		if err := database.SaveQuota(args[0], s); err != nil {
			return err
		}
		fmt.Printf("Quota of '%s' will be removed at midnight\n", args[0])
		return nil
	},
}

var quotaStartCmd = &cobra.Command{
	Use:   "start [block]",
	Short: "Open a block and start spending its quota",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		forDur, _ := cmd.Flags().GetString("for")

		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		now := time.Now()
		s, err := currentQuota(database, name, now)
		if err != nil {
			return err
		}
		if s.Open {
			return fmt.Errorf("block '%s' already has an open session", name)
		}
		if s.Remaining() == 0 {
			return fmt.Errorf("today's quota for '%s' is spent", name)
		}
		hold, err := blockHold(database, name, now)
		if err != nil {
			return err
		}
		if hold.active && (hold.charLocked || hold.timeLocked() || hold.scheduledNow ||
			hold.passwordHash != "" || hold.restartLocked || hold.extras.Delay > 0) {
			return fmt.Errorf("block '%s' is locked or in a scheduled window, so its quota can't be used now", name)
		}

		s.Open, s.SessionEnd = true, 0
		if forDur != "" {
			d, err := time.ParseDuration(forDur)
			if err != nil {
				return fmt.Errorf("invalid --for duration %q: %w", forDur, err)
			}
			if d <= 0 {
				return fmt.Errorf("--for duration must be positive")
			}
			s.SessionEnd = now.Add(d).Unix()
		}
		if err := database.SaveQuota(name, s); err != nil {
			return err
		}
		if err := ensureBlockActive(database, name, false); err != nil {
			return err
		}
		if err := enforce.ApplyFromDB(database); err != nil {
			return err
		}
		fmt.Printf("Block '%s' is open; %d minutes left today\n", name, s.Remaining()/60)
		return nil
	},
}

var quotaEndCmd = &cobra.Command{
	Use:   "end [block]",
	Short: "End an open quota session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		s, err := currentQuota(database, name, time.Now())
		if err != nil {
			return err
		}
		if !s.Open {
			return fmt.Errorf("block '%s' has no open session", name)
		}
		s.Open, s.SessionEnd = false, 0
		if err := database.SaveQuota(name, s); err != nil {
			return err
		}
		if err := ensureBlockActive(database, name, true); err != nil {
			return err
		}
		if err := enforce.ApplyFromDB(database); err != nil {
			return err
		}
		fmt.Printf("Block '%s' is on again; %d minutes left today\n", name, s.Remaining()/60)
		return nil
	},
}

func init() {
	quotaStartCmd.Flags().String("for", "", "End the session after this long (e.g. 10m)")
	quotaCmd.AddCommand(quotaSetCmd, quotaRemoveCmd, quotaStartCmd, quotaEndCmd)
	rootCmd.AddCommand(quotaCmd)
}
