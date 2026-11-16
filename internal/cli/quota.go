// quota.go — Allowances (§3.6): time a block may be used per period, spent
// in sessions that lift it. An allowance protects nothing by itself
// (R-LOCK-1): only locks do.
package cli

import (
	"fmt"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/smattermannen/kc9k/internal/quota"
	"github.com/smattermannen/kc9k/internal/schedule"
	"github.com/spf13/cobra"
)

var quotaCmd = &cobra.Command{
	Use:     "allowance",
	Aliases: []string{"quota"},
	Short:   "Manage allowances: time a block may be used per period",
	Long: `Give a block an allowance: time it may be used per period (a day unless
--per week, month, rolling for the last 24 hours, or window for each opening
of a block window), refilled at midnight or at --refill HH:MM. 'allowance
start' opens a session that lifts the block until it ends or the time runs
out. Time in a session is spent only while the block is in use: one of its
apps is in front, or one of its websites is in the focused tab (not while
you are idle) or plays sound or video, even muted (address and title lists;
page text lists don't count). While a session has websites, browsers
without the extension are closed; without a supported browser the extension
can't see tabs, so a session with websites spends all its time. Unused time
is not saved. A lock, an open block window or a Pomodoro work period closes
a session.

Setting an allowance switches the block on. An allowance protects nothing by
itself (R-LOCK-1): while the block is unlocked, every change applies at
once. While it is locked, an allowance can only get less time or fewer
sessions; removing it is always allowed.`,
}

// currentQuota loads a block's quota, advanced to now.
func currentQuota(database *db.DB, name string, now time.Time) (quota.State, error) {
	state, err := database.GetQuota(name)
	if err != nil {
		return quota.State{}, err
	}
	if state == nil {
		return quota.State{}, fmt.Errorf("block '%s' has no allowance", name)
	}
	return state.Step(now.Unix(), enforce.QuotaPeriod(database, name, *state, now)), nil
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
	Short: "Set a block's allowance (e.g. 30m; per day unless --per; --refill HH:MM)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		d, err := parseUserDuration(args[1])
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", args[1], err)
		}
		per, _ := cmd.Flags().GetString("per")
		if per == "day" {
			per = ""
		}
		if per != "" && per != "week" && per != "month" && per != "rolling" && per != "window" {
			return fmt.Errorf("--per is day, week, month, rolling (the last 24 hours) or window (each opening of a block window)")
		}
		maxSessions, _ := cmd.Flags().GetInt("sessions")
		if maxSessions < 0 || maxSessions > 100 {
			return fmt.Errorf("--sessions is 0 (no limit) to 100")
		}
		refill := 0
		if spec, _ := cmd.Flags().GetString("refill"); spec != "" {
			if refill, err = schedule.ParseTimeOfDay(spec); err != nil {
				return fmt.Errorf("--refill: %w", err)
			}
		}
		limit := int64(d.Seconds())

		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		now := time.Now()
		existing, err := database.GetQuota(name)
		if err != nil {
			return err
		}
		if existing != nil {
			// Flags not given keep what the allowance has.
			if !cmd.Flags().Changed("per") {
				per = existing.Period
			}
			if !cmd.Flags().Changed("refill") {
				refill = existing.Refill
			}
			if !cmd.Flags().Changed("sessions") {
				maxSessions = existing.MaxSessions
			}
		}
		if err := quota.ValidateFor(d, per); err != nil {
			return err
		}
		// R-LOCK-1: an allowance lifts its block during sessions, so a new
		// one, more time, more sessions (or no limit), or another period or
		// refill time (which can give time back) weakens the block: it needs
		// the settings password and is refused while the block is locked.
		// Less time or fewer sessions is always free.
		weakens := existing == nil || limit > existing.Limit || existing.Period != per || existing.Refill != refill ||
			(maxSessions != existing.MaxSessions && (maxSessions == 0 || (existing.MaxSessions != 0 && maxSessions > existing.MaxSessions)))
		if weakens {
			if err := ensureEditable(database, name, false); err != nil {
				return err
			}
		}
		var s quota.State
		if existing == nil {
			s = quota.New(limit, now.Unix(), "")
			s.Period, s.Refill, s.MaxSessions = per, refill, maxSessions
			s.Day = enforce.QuotaPeriod(database, name, s, now)
		} else {
			existing.Period, existing.Refill, existing.MaxSessions = per, refill, maxSessions
			s = existing.Step(now.Unix(), enforce.QuotaPeriod(database, name, *existing, now)).SetLimit(limit)
		}
		if err := database.SaveQuota(name, s); err != nil {
			return err
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
		fmt.Printf("Allowance of '%s' is %s per %s\n", name, d, map[bool]string{true: "day", false: per}[per == ""])
		return nil
	},
}

var quotaRemoveCmd = &cobra.Command{
	Use:   "remove [block]",
	Short: "Remove a block's allowance (always allowed: without sessions the block is stricter)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()
		s, err := database.GetQuota(args[0])
		if err != nil {
			return err
		}
		if s == nil {
			return fmt.Errorf("block '%s' has no allowance", args[0])
		}
		// No more sessions, and an open one ends: that only strengthens the
		// block. Whether it is switched on stays as it is (§3.3).
		if err := database.DeleteQuota(args[0]); err != nil {
			return err
		}
		if err := enforce.ApplyFromDB(database); err != nil {
			return err
		}
		fmt.Printf("Allowance of '%s' removed\n", args[0])
		return nil
	},
}

var quotaStartCmd = &cobra.Command{
	Use:   "start [block]",
	Short: "Open a block and start spending its allowance",
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
			return fmt.Errorf("the allowance of '%s' is spent for this period", name)
		}
		hold, err := blockHold(database, name, now)
		if err != nil {
			return err
		}
		if hold.held() {
			return fmt.Errorf("block '%s' is locked or in a scheduled window, so its allowance can't be used now", name)
		}
		if enforce.PomodoroWorking(database, name) {
			return fmt.Errorf("block '%s' is in a Pomodoro work period; its allowance can be used in the break", name)
		}

		if why := s.CanOpen(); why != "" {
			return fmt.Errorf("block '%s': %s", name, why) // §3.6 session limit
		}
		s = s.Opened(now.Unix())
		s.SessionEnd = 0
		if forDur != "" {
			d, err := parseUserDuration(forDur)
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
		// §3.3: the session lifts the block; it stays on.
		if err := ensureBlockActive(database, name, true); err != nil {
			return err
		}
		if err := enforce.ApplyFromDB(database); err != nil {
			return err
		}
		fmt.Printf("Block '%s' is open; %d minutes left in this period\n", name, s.Remaining()/60)
		return nil
	},
}

var quotaEndCmd = &cobra.Command{
	Use:   "end [block]",
	Short: "End an open allowance session",
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
		fmt.Printf("Block '%s' is on again; %d minutes left in this period\n", name, s.Remaining()/60)
		return nil
	},
}

func init() {
	quotaStartCmd.Flags().String("for", "", "End the session after this long (e.g. 10m)")
	quotaSetCmd.Flags().String("per", "day", "The period the allowance covers: day, week, month, rolling (the last 24 hours), or window (each opening of a block window)")
	quotaSetCmd.Flags().String("refill", "", "When the allowance starts afresh (HH:MM; default midnight)")
	quotaSetCmd.Flags().Int("sessions", 0, "Session limit: at most this many sessions per period, each ending 3 minutes after switching away (0: no limit)")
	quotaCmd.AddCommand(quotaSetCmd, quotaRemoveCmd, quotaStartCmd, quotaEndCmd)
	rootCmd.AddCommand(quotaCmd)
}
