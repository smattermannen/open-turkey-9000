// freeze.go — Frozen Chicken and the JSON state used by the graphical interface.
package cli

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
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
		if err := requireSettingsPassword(database); err != nil {
			return err
		}

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

// guiWin is a schedule window for the interface's week calendar.
type guiWin struct {
	Days  int  `json:"days"` // bit 0 is Sunday
	Start int  `json:"start"`
	End   int  `json:"end"`
	Allow bool `json:"allow"`
}

// guiList is one website list for the interface.
type guiList struct {
	ID      int             `json:"id"`
	Name    string          `json:"name"`
	Kind    string          `json:"kind"`
	Where   []string        `json:"where"`
	Actions []db.ListAction `json:"actions"`
	Entries []string        `json:"entries"`
}

type guiBlock struct {
	Name        string    `json:"name"`
	Lists       []guiList `json:"lists"`
	Apps        []string  `json:"apps"`
	Titles      []string  `json:"titles"`
	Nepsis      db.Nepsis `json:"nepsis"`
	NepsisCount int       `json:"nepsis_count"`
	Schedules   []string  `json:"schedules"`
	Windows     []guiWin  `json:"schedule_windows"`
	Active      bool      `json:"active"`
	Held        bool      `json:"held"`
	CharLocked  bool      `json:"char_locked"`
	PomodoroOn  int64     `json:"pomodoro_on"`
	PomodoroOff int64     `json:"pomodoro_off"`
	OnBreak     bool      `json:"on_break"`
	Password    bool      `json:"password_locked"`
	Restart     bool      `json:"restart_locked"`
	Delay       int64     `json:"delay_seconds"`
	StopSeconds int64     `json:"stop_seconds"`
	Scheduled   bool      `json:"scheduled"`
	LockSeconds int64     `json:"lock_seconds"`
	QuotaLimit  int64     `json:"quota_limit"`
	QuotaUsed   int64     `json:"quota_used"`
	QuotaOpen   bool      `json:"quota_open"`
	QuotaNext   int64     `json:"quota_next"`
	KeywordTest bool      `json:"keyword_test"`
}

// guiSettingLock is a browser setting's locks for the interface.
type guiSettingLock struct {
	Seconds      int64 `json:"lock_seconds"`
	CharLocked   bool  `json:"char_locked"`
	Password     bool  `json:"password_locked"`
	Restart      bool  `json:"restart_locked"`
	GuardsBlocks bool  `json:"guards_blocks"`
}

type guiState struct {
	FreezeSeconds int64                     `json:"freeze_seconds"`
	FreezeMode    string                    `json:"freeze_mode"`
	Strict        bool                      `json:"strict"`
	NoEscape      bool                      `json:"no_escape"`
	Stats         bool                      `json:"stats"`
	LibreWolf     string                    `json:"librewolf"`
	Extension     string                    `json:"extension_status"`
	SettingsPass  bool                      `json:"settings_password"`
	FreezeSched   []string                  `json:"freeze_schedules"`
	Browser       map[string]bool           `json:"browser"`
	BrowserLocks  map[string]guiSettingLock `json:"browser_locks"`
	Blocks        []guiBlock                `json:"blocks"`
	Hotkeys       []db.Hotkey               `json:"hotkeys"`
	Nepsis        db.Nepsis                 `json:"nepsis"`
	NepsisCount   int                       `json:"nepsis_count"`
	NepsisLock    guiSettingLock            `json:"nepsis_lock"`
	MonkBlock     string                    `json:"monk_block"`
	// Access says per area "none", "open" or "locked"; AccessUnused lists
	// the given passwords (by position) that opened nothing: wrong, or
	// changed since.
	Options      map[string]bool   `json:"options"`
	Access       map[string]string `json:"access"`
	AccessUnused []int             `json:"access_unused"`
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
		if state.FreezeSeconds > 0 {
			if state.FreezeMode, err = database.FreezeMode(); err != nil {
				return err
			}
		}
		noEscape, err := database.GetSetting(enforce.SettingNoEscape)
		if err != nil {
			return err
		}
		state.NoEscape = noEscape == "on"
		state.Stats = enforce.StatsOn(database)
		state.LibreWolf = blocker.LibreWolfVersion()
		state.Extension = extensionStatus(database)
		state.SettingsPass, _ = settingsPasswordSet(database)
		state.FreezeSched = []string{}
		if schedules, err := database.ListFreezeSchedules(); err == nil {
			for _, s := range schedules {
				state.FreezeSched = append(state.FreezeSched, fmt.Sprintf("%s (%s)", s.Window, s.Mode))
			}
		}
		state.Browser = map[string]bool{}
		state.BrowserLocks = map[string]guiSettingLock{}
		for _, name := range enforce.BrowserSettings {
			state.Browser[name] = enforce.BrowserOn(database, name)
			s, err := loadSettingLock(database, name)
			if err != nil {
				return err
			}
			g := guiSettingLock{
				CharLocked: s.lock.LockChars > 0, Password: s.lock.PasswordHash != "",
				Restart: s.lock.RestartLocked(s.bootID), GuardsBlocks: enforce.GuardsBlocks(name),
			}
			if s.lock.TimeLocked(s.trusted) {
				g.Seconds = s.lock.LockedUntil - s.trusted
			}
			state.BrowserLocks[name] = g
		}
		state.MonkBlock = enforce.MonkBlock
		if state.Nepsis, err = database.GlobalNepsis(); err != nil {
			return err
		}
		if state.Nepsis.On() {
			_, state.NepsisCount, _ = enforce.NepsisState(database, now, "", state.Nepsis)
		}
		if s, err := loadSettingLock(database, db.NepsisLockName); err == nil {
			state.NepsisLock = guiSettingLock{CharLocked: s.lock.LockChars > 0, Password: s.lock.PasswordHash != "",
				Restart: s.lock.RestartLocked(s.bootID)}
			if s.lock.TimeLocked(s.trusted) {
				state.NepsisLock.Seconds = s.lock.LockedUntil - s.trusted
			}
		}

		blocks, err := database.ListBlocks()
		if err != nil {
			return err
		}
		testing := database.TestBlocks()
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
				Name: detail.Name, Lists: []guiList{}, Apps: append([]string{}, detail.Apps...),
				Titles:    append([]string{}, detail.Titles...),
				Schedules: []string{},
				Active:    detail.Active, Held: hold.held(), CharLocked: detail.Active && detail.Locked,
				Scheduled: detail.Scheduled,
			}
			for _, l := range detail.Lists {
				actions := l.Actions
				if actions == nil {
					actions = []db.ListAction{}
				}
				g.Lists = append(g.Lists, guiList{ID: l.ID, Name: l.Name, Kind: l.Kind, Where: append([]string{}, l.Where.List()...),
					Actions: actions, Entries: l.Entries})
			}
			if detail.Active && detail.LockedUntil > trusted {
				g.LockSeconds = detail.LockedUntil - trusted
			}
			g.Windows = []guiWin{}
			for _, s := range schedules {
				g.Schedules = append(g.Schedules, s.Window().String())
				w := s.Window()
				g.Windows = append(g.Windows, guiWin{Days: w.Days, Start: w.Start, End: w.End, Allow: w.Allow})
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
			if g.Nepsis, err = database.BlockNepsis(b.Name); err != nil {
				return err
			}
			if g.Nepsis.On() {
				_, g.NepsisCount, _ = enforce.NepsisState(database, now, b.Name, g.Nepsis)
			}
			g.QuotaNext = quota.NoPending
			if q, err := database.GetQuota(b.Name); err != nil {
				return err
			} else if q != nil {
				s := q.Step(now.Unix(), quota.DayOf(now))
				g.QuotaLimit, g.QuotaUsed, g.QuotaOpen, g.QuotaNext = s.Limit, s.Used, s.Open, s.PendingLimit
			}
			g.KeywordTest = slices.Contains(testing, b.Name)
			state.Blocks = append(state.Blocks, g)
		}

		if state.Hotkeys, err = database.Hotkeys(); err != nil {
			return err
		}

		state.Options = map[string]bool{}
		for _, o := range enforce.Options {
			state.Options[o.Name] = enforce.OptionOn(database, o.Name)
		}
		if state.Access, state.AccessUnused, err = accessStatus(cmd, database); err != nil {
			return err
		}
		state.hideLocked()

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
