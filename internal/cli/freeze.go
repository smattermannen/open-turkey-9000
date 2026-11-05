// freeze.go — Frozen Chicken and the JSON state used by the graphical interface.
package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/smattermannen/kc9k/internal/freeze"
	"github.com/smattermannen/kc9k/internal/quota"
	"github.com/spf13/cobra"
)

var freezeCmd = &cobra.Command{
	Use:   "freeze",
	Short: "Frozen Chicken: keep the screen locked for a set time",
	Long: `Lock every session and keep it locked until the time has passed.

There is no way to cancel from the desktop. A running freeze can only be
extended. The time keeps counting while the computer is off.

Emergency exit: in GRUB, press e on the boot entry and add
` + freeze.BypassParam + ` to the line that starts with "linux".`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		forDur, _ := cmd.Flags().GetString("for")
		yes, _ := cmd.Flags().GetBool("yes")
		mode, _ := cmd.Flags().GetString("mode")
		if !freeze.ValidMode(mode) {
			return fmt.Errorf("unknown mode %q (use lock, logout or shutdown)", mode)
		}

		d, err := time.ParseDuration(forDur)
		if err != nil {
			return fmt.Errorf("invalid --for duration %q: %w", forDur, err)
		}
		if err := freeze.Validate(d); err != nil {
			return err
		}
		if !yes {
			return fmt.Errorf("this locks the computer for %s and cannot be cancelled; add --yes to confirm", d)
		}

		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		until := time.Now().Add(d).Unix()
		current, err := database.FreezeUntil()
		if err != nil {
			return err
		}
		if until <= current {
			return fmt.Errorf("already frozen until %s; a freeze can only be extended", time.Unix(current, 0).Format("2006-01-02 15:04"))
		}
		if err := database.SetFreeze(until, mode); err != nil {
			return err
		}
		fmt.Printf("Frozen until %s\n", time.Unix(until, 0).Format("2006-01-02 15:04"))
		if mode == freeze.ModeLock {
			return blocker.LockSessions()
		}
		return nil
	},
}

type guiBlock struct {
	Name        string   `json:"name"`
	Mode        string   `json:"mode"`
	Sites       []string `json:"sites"`
	Exceptions  []string `json:"exceptions"`
	Limited     []string `json:"firefox_only"`
	Apps        []string `json:"apps"`
	Schedules   []string `json:"schedules"`
	Active      bool     `json:"active"`
	Held        bool     `json:"held"`
	CharLocked  bool     `json:"char_locked"`
	PomodoroOn  int64    `json:"pomodoro_on"`
	PomodoroOff int64    `json:"pomodoro_off"`
	OnBreak     bool     `json:"on_break"`
	Password    bool     `json:"password_locked"`
	Restart     bool     `json:"restart_locked"`
	Delay       int64    `json:"delay_seconds"`
	StopSeconds int64    `json:"stop_seconds"`
	Scheduled   bool     `json:"scheduled"`
	LockSeconds int64    `json:"lock_seconds"`
	QuotaLimit  int64    `json:"quota_limit"`
	QuotaUsed   int64    `json:"quota_used"`
	QuotaOpen   bool     `json:"quota_open"`
	QuotaNext   int64    `json:"quota_next"`
}

type guiState struct {
	FreezeSeconds int64           `json:"freeze_seconds"`
	Strict        bool            `json:"strict"`
	Browser       map[string]bool `json:"browser"`
	Blocks        []guiBlock      `json:"blocks"`
}

var guiStateCmd = &cobra.Command{
	Use:    "gui-state",
	Short:  "Internal: print the state as JSON for the graphical interface",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		now := time.Now()
		trusted, err := database.TrustedNow(now)
		if err != nil {
			return err
		}
		state := guiState{Blocks: []guiBlock{}}
		until, err := database.FreezeUntil()
		if err != nil {
			return err
		}
		if until > now.Unix() {
			state.FreezeSeconds = until - now.Unix()
		}

		strict, err := database.GetSetting(enforce.SettingStrict)
		if err != nil {
			return err
		}
		state.Strict = strict == "on"
		state.Browser = map[string]bool{}
		for _, name := range enforce.BrowserSettings {
			state.Browser[name] = enforce.BrowserOn(database, name)
		}

		blocks, err := database.ListBlocks()
		if err != nil {
			return err
		}
		for _, b := range blocks {
			detail, err := database.GetBlock(b.Name)
			if err != nil {
				return err
			}
			hold, err := blockHold(database, b.Name, now)
			if err != nil {
				return err
			}
			schedules, err := database.ListSchedules(b.Name)
			if err != nil {
				return err
			}
			g := guiBlock{
				Name: detail.Name, Mode: detail.Mode,
				Sites: append(append([]string{}, detail.Sites...), detail.Patterns...), Apps: append([]string{}, detail.Apps...),
				Exceptions: append([]string{}, detail.Exceptions...), Limited: []string{},
				Schedules: []string{},
				Active:    detail.Active, Held: hold.held(), CharLocked: detail.Active && detail.Locked,
				Scheduled: detail.Scheduled,
			}
			if detail.Active && detail.LockedUntil > trusted {
				g.LockSeconds = detail.LockedUntil - trusted
			}
			for _, s := range schedules {
				g.Schedules = append(g.Schedules, s.Window().String())
			}
			for _, p := range append(append([]string{}, detail.Patterns...), detail.Exceptions...) {
				if blocker.BrowserLimited(p) {
					g.Limited = append(g.Limited, p)
				}
			}
			g.Password, g.Restart, g.Delay = hold.passwordHash != "", hold.restartLocked, hold.extras.Delay
			if hold.extras.StopAt > trusted {
				g.StopSeconds = hold.extras.StopAt - trusted
			}
			if p, err := database.GetPomodoro(b.Name); err != nil {
				return err
			} else if p != nil {
				g.PomodoroOn, g.PomodoroOff = p.On, p.Off
				g.OnBreak = detail.Active && quota.OnBreak(p.On, p.Off, p.Anchor, trusted)
			}
			g.QuotaNext = quota.NoPending
			if q, err := database.GetQuota(b.Name); err != nil {
				return err
			} else if q != nil {
				s := q.Step(now.Unix(), quota.DayOf(now))
				g.QuotaLimit, g.QuotaUsed, g.QuotaOpen, g.QuotaNext = s.Limit, s.Used, s.Open, s.PendingLimit
			}
			state.Blocks = append(state.Blocks, g)
		}

		out, err := json.Marshal(state)
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	},
}

func init() {
	freezeCmd.Flags().String("for", "", "How long to stay frozen (e.g. 30m, 2h; at most 24h)")
	freezeCmd.Flags().Bool("yes", false, "Confirm the freeze")
	freezeCmd.Flags().String("mode", freeze.ModeLock, "How to freeze: lock, logout or shutdown")
	_ = freezeCmd.MarkFlagRequired("for")
	rootCmd.AddCommand(freezeCmd, guiStateCmd)
}
