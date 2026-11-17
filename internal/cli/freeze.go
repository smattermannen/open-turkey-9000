// freeze.go — Frozen Chicken and the JSON state used by the graphical interface.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/circadian"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/smattermannen/kc9k/internal/freeze"
	"github.com/smattermannen/kc9k/internal/quota"
	"github.com/smattermannen/kc9k/internal/schedule"
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
			return fmt.Errorf("unknown mode %q (use lock, logout, shutdown or apps)", mode)
		}

		var d time.Duration
		var err error
		if untilSpec, _ := cmd.Flags().GetString("until"); untilSpec != "" {
			if forDur != "" {
				return fmt.Errorf("give --for or --until, not both")
			}
			at, err := untilToday(time.Now(), untilSpec)
			if err != nil {
				return fmt.Errorf("invalid --until %q: %w", untilSpec, err)
			}
			d = time.Until(at).Round(time.Minute)
		} else if d, err = parseUserDuration(forDur); err != nil {
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
		if err := requireLockPassword(database); err != nil {
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
		// The daemon locks the screen again every half second during a
		// lock freeze, so failing to lock it at once only warns. A test's
		// database (KC9K_DB) never locks the real screen.
		if mode == freeze.ModeLock && os.Getenv("KC9K_DB") == "" {
			if err := blocker.LockSessions(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: the screen couldn't be locked at once (%v); "+
					"the service locks it within a second.\n", err)
			}
		}
		return nil
	},
}

// guiWin is a schedule window for the interface's week calendar.
type guiWin struct {
	Days  int   `json:"days"` // bit 0 is Sunday
	Start int   `json:"start"`
	End   int   `json:"end"`
	Allow bool  `json:"allow"`
	Break int64 `json:"break,omitempty"` // §3.6: seconds of its one break
	// Source names the schedule (§3.6) the window comes from; "" for its
	// own.
	Source string `json:"source,omitempty"`
}

// guiSchedule is a schedule kept outside blocks (§3.6), for the Schedules
// page: its windows and what uses it.
type guiSchedule struct {
	Name    string   `json:"name"`
	Windows []guiWin `json:"windows"`
	Blocks  []string `json:"blocks"` // blocks using it, not through a mode
	Modes   []string `json:"modes"`
}

// guiPreset is a preset kept here (§3.2 Preset) for the Presets page.
type guiPreset struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	What string `json:"what"`
}

// guiMode is a mode (§3.2, §3.8) for the Blocks page.
type guiMode struct {
	Name       string                `json:"name"`
	Monk       bool                  `json:"monk"`
	Active     bool                  `json:"active"` // monk mode: its automation runs it
	Blocks     []string              `json:"blocks"`
	Windows    []guiWin              `json:"windows"` // its own and its schedules', numbered as 'schedule list'
	Schedules  []string              `json:"schedules"`
	Places     []string              `json:"places"`
	Automation db.AutomationSettings `json:"automation"`
	Continuous bool                  `json:"continuous"`
	Running    bool                  `json:"running"`   // monk mode: its automation runs it now
	On         int                   `json:"on"`        // its blocks switched on
	Locked     int                   `json:"locked"`    // its blocks locked
	MonkLeft   int64                 `json:"monk_left"` // monk mode: seconds it runs on
	// Frozen Chicken (§3.8, owner 2026-11-17): a mode whose automation is
	// its scheduled freezes.
	Freeze     bool   `json:"freeze"`
	FreezeLeft int64  `json:"freeze_left"` // seconds it is frozen on
	FreezeMode string `json:"freeze_mode"`
	// Custom: what it opens or runs when it becomes enabled (§3.9).
	Custom []string `json:"custom"`
	// RunLeft: seconds a Mode action's run has left (R-MODE-4), locked.
	RunLeft int64 `json:"run_left"`
	// TodayWindows: its windows still to come or open today (§4.1).
	TodayWindows []string `json:"today_windows"`
	// Wallpapers: its wallpapers (R-MODE-5), as circadian rhythm's.
	Wallpapers []circadianPicture `json:"wallpapers"`
}

// guiWallpaper is the wallpaper settings outside circadian rhythm (§4.10).
type guiWallpaper struct {
	OverrideModes bool   `json:"override_modes"` // circadian rhythm's wallpapers win over modes'
	Default       string `json:"default"`        // latest, circadian or custom
	CustomName    string `json:"custom_name"`
	CustomPath    string `json:"custom_path"`
}

func guiWallpaperSettings(database *db.DB) guiWallpaper {
	s := enforce.LoadWallpaperSettings(database)
	g := guiWallpaper{OverrideModes: s.CircadianFirst, Default: s.Default, CustomName: s.CustomName}
	if s.Custom != "" {
		g.CustomPath = filepath.Join(enforce.WallpapersDir(), s.Custom)
	}
	return g
}

// guiList is one website list for the interface.
type guiList struct {
	ID      int             `json:"id"`
	Name    string          `json:"name"`
	Kind    string          `json:"kind"`
	Where   []string        `json:"where"`
	Actions []db.ListAction `json:"actions"`
	Entries []string        `json:"entries"`
	Test    bool            `json:"test"`
}

// guiPhraseSet is one phrase set for the interface; each phrase comes with
// its typed form.
type guiPhraseSet struct {
	ID      int             `json:"id"`
	Name    string          `json:"name"`
	Where   []string        `json:"where"`
	Actions []db.ListAction `json:"actions"`
	Test    bool            `json:"test"`
	Phrases []guiPhrase     `json:"phrases"`
}

type guiPhrase struct {
	db.Phrase
	Text string `json:"text"`
}

type guiBlock struct {
	// Triggers: what the block's apps and window titles do (owner,
	// 2026-11-16).
	Triggers map[string][]db.TriggerAction `json:"triggers"`
	// Automation (§3.6): its default and both/either; continuous without
	// windows or places.
	Automation db.AutomationSettings `json:"automation"`
	Continuous bool                  `json:"continuous"`
	// Mode is the mode it is in (§3.2; its automation is the mode's),
	// AppliedSchedules the schedules it uses (§3.6).
	Mode              string         `json:"mode"`
	AppliedSchedules  []string       `json:"applied_schedules"`
	Name              string         `json:"name"`
	Lists             []guiList      `json:"lists"`
	PhraseSets        []guiPhraseSet `json:"phrase_sets"`
	Apps              []string       `json:"apps"`
	Titles            []string       `json:"titles"`
	Nepsis            db.Nepsis      `json:"nepsis"`
	NepsisCount       int            `json:"nepsis_count"`
	Schedules         []string       `json:"schedules"`
	Windows           []guiWin       `json:"schedule_windows"`
	Users             []string       `json:"users"` // §3.2; empty: every account
	Active            bool           `json:"active"`
	Held              bool           `json:"held"`
	CharLocked        bool           `json:"char_locked"`
	PomodoroOn        int64          `json:"pomodoro_on"`
	PomodoroOff       int64          `json:"pomodoro_off"`
	PomodoroLong      int64          `json:"pomodoro_long"`
	PomodoroEvery     int64          `json:"pomodoro_every"`
	OnBreak           bool           `json:"on_break"`
	Password          bool           `json:"password_locked"`
	Restart           bool           `json:"restart_locked"`
	Delay             int64          `json:"delay_seconds"`
	StopSeconds       int64          `json:"stop_seconds"`
	DelayLeft         int64          `json:"delay_left"`            // the delay lock unlocks in this many seconds, once asked
	LockOrder         []string       `json:"lock_order,omitempty"`  // locks on, in unlock order (R-LOCK-6)
	TimerAfter        int64          `json:"timer_after,omitempty"` // a timer lock's seconds, counted once the locks before it are unlocked
	RangeLock         string         `json:"range_lock,omitempty"`  // §3.4, e.g. "mon,tue 09:00-17:00"
	RangeOn           bool           `json:"range_on,omitempty"`
	Scheduled         bool           `json:"scheduled"`                // has windows or a place rule (§3.3)
	HasPassword       bool           `json:"block_password,omitempty"` // §3.11
	DraftOf           string         `json:"draft_of,omitempty"`       // §4.3: this is a draft of that block
	Draft             string         `json:"draft,omitempty"`          // §4.3: this block's draft
	TodayWindows      []string       `json:"today_windows,omitempty"`  // §4.1: windows still to come or open today
	Autostarts        []string       `json:"autostarts"`               // §3.6
	Hidden            bool           `json:"hidden,omitempty"`         // contents left out until revealed
	Lifted            bool           `json:"lifted"`                   // active, but its windows or places lift it now
	Enabled           bool           `json:"enabled"`                  // §3.3: active and enforcing now
	LockSeconds       int64          `json:"lock_seconds"`
	QuotaLimit        int64          `json:"quota_limit"`
	QuotaUsed         int64          `json:"quota_used"`
	QuotaOpen         bool           `json:"quota_open"`
	QuotaNext         int64          `json:"quota_next"`
	QuotaPeriod       string         `json:"quota_period,omitempty"`        // "" day, week, month (§3.6)
	QuotaRefill       string         `json:"quota_refill,omitempty"`        // HH:MM
	QuotaSessions     int            `json:"quota_sessions,omitempty"`      // §3.6 session limit (0: none)
	QuotaSessionsUsed int            `json:"quota_sessions_used,omitempty"` // this period
	KeywordTest       bool           `json:"keyword_test"`
	Places            []string       `json:"places"`
	LoosenDelay       int64          `json:"loosen_delay"`
	ScheduleLock      bool           `json:"schedule_lock"`
	ScheduleLead      int            `json:"schedule_lead"` // minutes
	Social            []string       `json:"social"`
	// Breaks: the allowance (seconds per BreakPer of blocking), what is
	// earned, the relock time, and how long the block is lifted for now.
	BreakAllowance int64  `json:"break_allowance"`
	BreakPer       int64  `json:"break_per"`
	BreakBalance   int64  `json:"break_balance"`
	Relock         int64  `json:"relock"`
	DelayBreak     int64  `json:"delay_break,omitempty"`    // §3.6: its length
	BreakKeep      string `json:"break_keep,omitempty"`     // reward kept until this period ends ("": one break)
	DelayWait      int64  `json:"delay_wait,omitempty"`     // its countdown
	DelayBreakIn   int64  `json:"delay_break_in,omitempty"` // asked: seconds until it begins
	PausedSeconds  int64  `json:"paused_seconds"`
}

// guiSettingLock is a browser setting's locks for the interface.
// guiHotkey is a hotkey with its locks in words: Lock lists them in unlock
// order ("" without locks), Kinds names them briefly.
type guiHotkey struct {
	db.Hotkey
	Lock  string `json:"lock"`
	Kinds string `json:"kinds"`
	// Does: what a press does, in words (§3.9 Hotkey action).
	Does string `json:"does"`
}

type guiSettingLock struct {
	Seconds      int64    `json:"lock_seconds"`
	CharLocked   bool     `json:"char_locked"`
	Password     bool     `json:"password_locked"`
	Restart      bool     `json:"restart_locked"`
	GuardsBlocks bool     `json:"guards_blocks"`
	Relock       int64    `json:"relock,omitempty"`      // re-lock seconds after unlocking
	RelockIn     int64    `json:"relock_in,omitempty"`   // unlocked: seconds until the locks are on again
	TimerAfter   int64    `json:"timer_after,omitempty"` // a timer lock counted once the locks before it are unlocked
	LockOrder    []string `json:"lock_order,omitempty"`  // locks on, in unlock order (R-LOCK-6)
}

// gui is the setting lock as the interface shows it.
func (s settingLockState) gui() guiSettingLock {
	l, t, boot := s.lock, s.trusted, s.bootID
	g := guiSettingLock{CharLocked: l.LayerOn(db.LayerText, t, boot), Password: l.LayerOn(db.LayerPassword, t, boot),
		Restart: l.LayerOn(db.LayerRestart, t, boot), Relock: l.Relock, TimerAfter: l.TimerAfter,
		LockOrder: l.LayersOn(t, boot)}
	if l.PendingRelock(t) && l.LiftedUntil > t {
		g.RelockIn = l.LiftedUntil - t
	}
	if l.LockedUntil > t {
		g.Seconds = l.LockedUntil - t
	}
	return g
}

// guiCluster is a cluster for the interface.
type guiCluster struct {
	Name    string   `json:"name"`
	Where   []string `json:"where"`
	Entries []string `json:"entries"`
	Phrases bool     `json:"phrases"`
}

// guiRecording is the link recording in progress.
type guiRecording struct {
	Cluster string `json:"cluster"`
	Block   string `json:"block"`
	List    string `json:"list"`
	Sites   int    `json:"sites"`
	// A word recording (§3.12): its words so far, and the most specific.
	Words bool     `json:"words"`
	Count int      `json:"word_count"`
	Pages int64    `json:"pages"`
	Top   []string `json:"top"`
}

// guiPlace is a saved place for the interface.
type guiPlace struct {
	Name     string   `json:"name"`
	Networks int      `json:"networks"`
	Need     int      `json:"need"`   // networks needed in range (§3.6)
	Blocks   []string `json:"blocks"` // blocks on there
	Modes    []string `json:"modes"`  // modes using it (their blocks are in Blocks too)
	Here     bool     `json:"here"`   // the computer is there now
}

type guiState struct {
	FreezeSeconds int64    `json:"freeze_seconds"`
	FreezeMode    string   `json:"freeze_mode"`
	FreezeApps    []string `json:"freeze_apps"` // §3.8: allowed apps only
	Strict        bool     `json:"strict"`
	NoEscape      bool     `json:"no_escape"`
	ResetGuard    bool     `json:"reset_guard"` // §3.8 installer media guard
	Stats         bool     `json:"stats"`
	StatsPrivate  bool     `json:"stats_skip_private"` // §4.9
	Accounts      []string `json:"accounts"`           // §3.2: people accounts of this computer
	LibreWolf     string   `json:"librewolf"`
	Chrome        bool     `json:"chrome"` // Google Chrome or Chromium installed
	Extension     string   `json:"extension_status"`
	// ExtensionMissing: no supported browser, or the extension is needed
	// and a supported browser runs without it (§4.10 install dialog):
	// those in ExtensionMissingIn.
	ExtensionMissing   bool     `json:"extension_missing"`
	ExtensionMissingIn []string `json:"extension_missing_in"`
	SettingsPass       bool     `json:"settings_password"`
	FreezeSched        []string `json:"freeze_schedules"`
	// FreezeSchedLocks are the scheduled freezes' setting locks (§3.8),
	// in the same order.
	FreezeSchedLocks []guiSettingLock          `json:"freeze_schedule_locks"`
	Browser          map[string]bool           `json:"browser"`
	BrowserLocks     map[string]guiSettingLock `json:"browser_locks"`
	Blocks           []guiBlock                `json:"blocks"`
	Hotkeys          []guiHotkey               `json:"hotkeys"`
	Nepsis           db.Nepsis                 `json:"nepsis"`
	NepsisCount      int                       `json:"nepsis_count"`
	NepsisLock       guiSettingLock            `json:"nepsis_lock"`
	StrictLock       guiSettingLock            `json:"strict_lock"`
	NoWeakening      bool                      `json:"no_weakening"`
	PasswordLocks    bool                      `json:"password_for_locks"`
	DNS              string                    `json:"dns"`
	TamperClock      bool                      `json:"tamper_clock"`
	FileURLs         bool                      `json:"tamper_file_urls"`
	TaskManagers     bool                      `json:"tamper_task_managers"`
	TamperPrivate    bool                      `json:"tamper_private"`
	Grace            int                       `json:"tamper_grace"`
	AnythingLocked   bool                      `json:"anything_locked"`           // §4.10: tamper protection can't go off now
	DesktopWarn      string                    `json:"desktop_warning,omitempty"` // R-DESKTOP-1
	Notifications    enforce.NoticeOptions     `json:"notifications"`
	StrictUse        bool                      `json:"strict_use"`
	StrictExempt     []string                  `json:"strict_exempt"` // §3.8
	Templates        []string                  `json:"templates"`     // §3.2: block presets
	Presets          []guiPreset               `json:"presets"`       // §3.2 Preset: the rest kept as presets
	// UnsupportedBrowsers is block unsupported browsers, with its lock;
	// Browsers the supported browsers (§4.10 Browsers).
	UnsupportedBrowsers bool           `json:"unsupported_browsers"`
	UnsupportedLock     guiSettingLock `json:"unsupported_browsers_lock"`
	Browsers            []guiBrowser   `json:"browsers"`
	MonkSeconds         int64          `json:"monk_seconds"` // left of monk mode; 0 when it is off
	// Access says per area "none", "open" or "locked"; AccessUnused lists
	// the given passwords (by position) that opened nothing: wrong, or
	// changed since.
	Options       map[string]bool   `json:"options"`
	Clusters      []guiCluster      `json:"clusters"`
	Recording     *guiRecording     `json:"recording"`
	Places        []guiPlace        `json:"places"`
	Modes         []guiMode         `json:"modes"`     // §3.2, §3.8 (owner 2026-11-17)
	Wallpaper     guiWallpaper      `json:"wallpaper"` // §4.10, R-MODE-5
	Schedules     []guiSchedule     `json:"schedules"` // §3.6 schedules kept outside blocks
	SocialPresets []db.SocialPreset `json:"social_presets"`
	Access        map[string]string `json:"access"`
	AccessUnused  []int             `json:"access_unused"`
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
		// §3.11: blocks with a block password show their contents only
		// when revealed: --reveal B, its password on stdin, in order.
		revealed := map[string]bool{}
		reveal, _ := cmd.Flags().GetStringArray("reveal")
		for _, name := range reveal {
			if err := revealBlock(database, name); err == nil {
				revealed[name] = true
			}
		}
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
		state.ResetGuard = enforce.ResetGuardOn(database)
		state.Stats = enforce.StatsOn(database)
		if state.FreezeApps = enforce.FreezeApps(database); state.FreezeApps == nil {
			state.FreezeApps = []string{}
		}
		if state.Accounts = accounts(); state.Accounts == nil {
			state.Accounts = []string{}
		}
		if v, _ := database.GetSetting(enforce.SettingStatsPrivate); v == "skip" {
			state.StatsPrivate = true
		}
		state.LibreWolf = blocker.LibreWolfVersion()
		state.Chrome = blocker.ChromeInstalled()
		state.Extension = extensionStatus(database)
		state.ExtensionMissing = extensionMissing(database)
		state.ExtensionMissingIn = []string{}
		if state.ExtensionMissing && extensionBrowserInstalled() {
			state.ExtensionMissingIn = browsersWithoutExtension(database)
		}
		state.SettingsPass, _ = settingsPasswordSet(database)
		state.FreezeSched, state.FreezeSchedLocks = []string{}, []guiSettingLock{}
		if schedules, err := database.ListFreezeSchedules(); err == nil {
			for _, s := range schedules {
				text := fmt.Sprintf("%s (%s)", s.Window, s.Mode)
				if s.LockLead >= 0 {
					text += fmt.Sprintf(", locked from %s before", time.Duration(s.LockLead)*time.Minute)
				}
				g := guiSettingLock{}
				if l, err := loadSettingLock(database, freezeLockName(s.ID)); err == nil {
					g = l.gui()
				}
				if len(g.LockOrder) > 0 {
					text += ", locked (" + strings.Join(g.LockOrder, ", ") + ")"
				}
				state.FreezeSched = append(state.FreezeSched, text)
				state.FreezeSchedLocks = append(state.FreezeSchedLocks, g)
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
			g := s.gui()
			g.GuardsBlocks = enforce.GuardsBlocks(name)
			state.BrowserLocks[name] = g
		}
		if trusted, err := database.TrustedNow(now); err == nil {
			if until := enforce.MonkEnd(database, trusted); until > 0 {
				state.MonkSeconds = until - trusted
			}
		}
		if state.Nepsis, err = database.GlobalNepsis(); err != nil {
			return err
		}
		if state.Nepsis.On() {
			_, state.NepsisCount, _ = enforce.NepsisState(database, now, "", state.Nepsis)
		}
		if s, err := loadSettingLock(database, db.NepsisLockName); err == nil {
			state.NepsisLock = s.gui()
		}
		if s, err := loadSettingLock(database, enforce.StrictLockName); err == nil {
			state.StrictLock = s.gui()
		}
		if v, err := database.GetSetting(SettingNoWeakening); err == nil {
			state.NoWeakening = v == "on"
		}
		if state.DNS, err = database.GetSetting(enforce.SettingDNS); err != nil {
			return err
		}
		state.UnsupportedBrowsers = enforce.UnsupportedBrowsersOn(database)
		if s, err := loadSettingLock(database, db.UnsupportedBrowsers); err == nil {
			state.UnsupportedLock = s.gui()
			state.UnsupportedLock.GuardsBlocks = true
		}
		state.Browsers = supportedBrowsers(database)
		if v, err := database.GetSetting(db.SettingTamperClock); err == nil {
			state.TamperClock = v == "on"
		}
		state.FileURLs = enforce.FileURLsOn(database)
		tm, _ := database.GetSetting(enforce.SettingTaskManagers)
		state.TaskManagers = tm == "on"
		state.TamperPrivate = enforce.PrivateTamperOn(database)
		state.Grace = enforce.ExtensionGraceSeconds(database)
		state.AnythingLocked, _ = enforce.AnyLock(database, now, trusted)
		state.DesktopWarn = blocker.DesktopWarning()
		state.Notifications = enforce.LoadNoticeOptions(database)
		state.StrictUse = enforce.StrictUseOn(database)
		state.StrictExempt = append([]string{}, enforce.ExemptFolders(database)...)
		state.Presets = []guiPreset{}
		if presets, err := database.Presets(""); err == nil {
			for _, p := range presets {
				state.Presets = append(state.Presets, guiPreset{Kind: p.Kind, Name: p.Name, What: describePreset(database, p)})
			}
		}
		if state.Templates, err = database.Templates(); err != nil {
			return err
		}
		if v, err := database.GetSetting(SettingPasswordForLocks); err == nil {
			state.PasswordLocks = v == "on" && state.SettingsPass
		}

		blocks, err := database.ListBlocks()
		if err != nil {
			return err
		}
		testing := database.TestLists()
		state.Clusters = []guiCluster{}
		clusters, err := database.ListClusters()
		if err != nil {
			return err
		}
		for _, c := range clusters {
			full, err := database.GetCluster(c.Name)
			if err != nil {
				return err
			}
			entries := full.Entries
			if entries == nil {
				entries = []string{}
			}
			state.Clusters = append(state.Clusters, guiCluster{Name: full.Name, Where: full.Where.List(), Entries: entries, Phrases: full.Phrases})
		}
		if rec, err := database.GetRecording(); err != nil {
			return err
		} else if rec != nil {
			g := &guiRecording{Cluster: rec.Cluster, Block: rec.Block}
			if rec.ListID != 0 {
				if l, err := database.GetList(rec.ListID); err == nil {
					g.List = l.Name
				}
			}
			if sites, err := database.RecordedSites(); err == nil {
				g.Sites = len(sites)
			}
			if g.Words = rec.Words; rec.Words {
				g.Top = []string{}
				if words, err := database.RecordedWords(); err == nil {
					g.Count, g.Pages = len(words), database.RecordedPages()
					for _, w := range words[:min(len(words), 15)] {
						g.Top = append(g.Top, w.Word)
					}
				}
			}
			state.Recording = g
		}
		places, err := database.Places()
		if err != nil {
			return err
		}
		modes, err := database.Modes()
		if err != nil {
			return err
		}
		saved, err := database.SavedSchedules()
		if err != nil {
			return err
		}
		state.Places = []guiPlace{}
		var seen []db.Network
		if len(places) > 0 {
			seen, _ = enforce.ScanWifi(false)
		}
		for _, p := range places {
			blocks := p.Blocks
			if blocks == nil {
				blocks = []string{}
			}
			g := guiPlace{Name: p.Name, Networks: len(p.Networks), Need: p.Needed(), Blocks: blocks, Modes: []string{},
				Here: len(seen) > 0 && enforce.AtPlace(p, seen)}
			for _, m := range modes {
				if slices.Contains(m.Spec.Places, p.ID) {
					g.Modes = append(g.Modes, m.Name)
				}
			}
			state.Places = append(state.Places, g)
		}
		state.Schedules = guiSchedules(database, saved, modes)
		state.Modes = guiModes(database, saved, modes, places, now, trusted)
		state.Wallpaper = guiWallpaperSettings(database)
		lifted := enforce.ScheduleLifted(database, now)
		enabled := enforce.EnabledNames(database, trusted)
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
				Name: detail.Name, Lists: []guiList{}, Apps: visibleApps(detail.Apps),
				Titles:    append([]string{}, detail.Titles...),
				Schedules: []string{},
				Active:    detail.Active, Held: hold.held(), CharLocked: detail.Active && hold.charLocked,
				Scheduled: len(schedules) > 0 || enforce.HasScheduleOrPlace(database, b.Name),
				Lifted:    detail.Active && lifted[b.Name],
				Enabled:   enabled[b.Name],
			}
			g.TodayWindows = windowsLeftToday(db.Windows(schedules), now)
			g.Autostarts = []string{}
			if rules, err := database.Autostarts(b.Name); err == nil {
				for _, a := range rules {
					g.Autostarts = append(g.Autostarts, a.String())
				}
			}
			if g.Users, _ = database.BlockUsers(b.Name); g.Users == nil {
				g.Users = []string{}
			}
			g.DraftOf = draftOf(database, b.Name)
			if draftOf(database, draftName(b.Name)) == b.Name {
				g.Draft = draftName(b.Name)
			}
			if hash, _ := database.BlockViewPassword(b.Name); hash != "" {
				g.HasPassword = true
				g.Hidden = !revealed[b.Name]
			}
			for _, l := range detail.Lists {
				actions := l.Actions
				if actions == nil {
					actions = []db.ListAction{}
				}
				g.Lists = append(g.Lists, guiList{ID: l.ID, Name: l.Name, Kind: l.Kind, Where: append([]string{}, l.Where.List()...),
					Actions: actions, Entries: l.Entries, Test: slices.Contains(testing, l.ID)})
				if slices.Contains(testing, l.ID) {
					g.KeywordTest = true
				}
			}
			sets, err := database.PhraseSets(detail.Name)
			if err != nil {
				return err
			}
			g.PhraseSets = []guiPhraseSet{}
			for _, s := range sets {
				gs := guiPhraseSet{ID: s.ID, Name: s.Name, Where: append([]string{}, s.Where.List()...),
					Actions: append([]db.ListAction{}, s.Actions...), Test: s.Test, Phrases: []guiPhrase{}}
				for _, p := range s.Phrases {
					gs.Phrases = append(gs.Phrases, guiPhrase{Phrase: p, Text: p.Key()})
				}
				g.PhraseSets = append(g.PhraseSets, gs)
			}
			if detail.Active && detail.LockedUntil > trusted {
				g.LockSeconds = detail.LockedUntil - trusted
			}
			g.Windows = []guiWin{}
			for _, s := range schedules {
				g.Schedules = append(g.Schedules, s.Window().String())
				w := s.Window()
				g.Windows = append(g.Windows, guiWin{Days: w.Days, Start: w.Start, End: w.End, Allow: w.Allow, Break: s.Break,
					Source: savedName(saved, s.Source)})
			}
			g.Mode, g.AppliedSchedules = "", []string{}
			if m, in, _ := database.ModeOf(b.Name); in {
				g.Mode = m.Name
			}
			if spec, err := database.BlockAutomationSpec(b.Name); err == nil {
				for _, id := range spec.Schedules {
					g.AppliedSchedules = append(g.AppliedSchedules, savedName(saved, id))
				}
			}
			g.Password, g.Restart, g.Delay = hold.passwordHash != "", hold.restartLocked, hold.extras.Delay
			g.LockOrder = hold.layers
			g.TimerAfter = hold.extras.TimerAfter
			if hold.extras.RangeDays != 0 {
				g.RangeLock = describeRange(hold.extras)
				g.RangeOn = hold.extras.InRange(now)
			}
			if hold.extras.StopAt > trusted {
				g.StopSeconds = hold.extras.StopAt - trusted
			}
			if hold.extras.DelayUntil > trusted {
				g.DelayLeft = hold.extras.DelayUntil - trusted
			}
			if p, err := database.GetPomodoro(b.Name); err != nil {
				return err
			} else if p != nil {
				g.PomodoroOn, g.PomodoroOff = p.On, p.Off
				g.OnBreak = detail.Active && p.Cycle().OnBreak(trusted)
				g.PomodoroLong, g.PomodoroEvery = p.Long, p.Every
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
				s := q.Step(now.Unix(), enforce.QuotaPeriod(database, b.Name, *q, now))
				g.QuotaLimit, g.QuotaUsed, g.QuotaOpen, g.QuotaNext = s.Limit, s.Used, s.Open, s.PendingLimit
				g.QuotaPeriod, g.QuotaRefill = s.Period, schedule.FormatMinutes(s.Refill)
				g.QuotaSessions, g.QuotaSessionsUsed = s.MaxSessions, s.Sessions
			}
			g.Places = placeNames(places, b.Name)
			if a, err := enforce.AutomationOf(database, b.Name); err == nil {
				g.Automation = db.AutomationSettings{DefaultOn: a.DefaultOn, NeedBoth: a.NeedBoth}
				g.Continuous = a.Continuous()
			}
			g.Triggers = map[string][]db.TriggerAction{}
			for _, group := range []string{db.TriggerApps, db.TriggerTitles} {
				if g.Triggers[group], err = database.TriggerActions(b.Name, group); err != nil {
					return err
				}
			}
			g.LoosenDelay, _ = database.LoosenDelay(b.Name)
			g.ScheduleLock, g.ScheduleLead = hold.scheduleLock, hold.scheduleLead
			g.Social = append([]string{}, detail.Social...)
			k, err := database.GetBreaks(b.Name)
			if err != nil {
				return err
			}
			g.BreakAllowance, g.BreakPer, g.BreakBalance, g.Relock = k.Allowance, k.Per, k.Balance(), k.Relock
			g.DelayBreak, g.DelayWait = k.DelayLength, k.DelayWait
			g.BreakKeep = k.Keep
			if k.DelayAt > trusted {
				g.DelayBreakIn = k.DelayAt - trusted
			}
			if until, _ := database.PausedUntil(b.Name); detail.Active && until > trusted {
				g.PausedSeconds = until - trusted
			}
			if g.Hidden {
				// §3.11: the names of lists and sets stay; their entries,
				// apps and titles don't.
				for i := range g.Lists {
					g.Lists[i].Entries = []string{}
				}
				for i := range g.PhraseSets {
					g.PhraseSets[i].Phrases = []guiPhrase{}
				}
				g.Apps, g.Titles = []string{}, []string{}
			}
			state.Blocks = append(state.Blocks, g)
		}

		hotkeys, err := database.Hotkeys()
		if err != nil {
			return err
		}
		state.Hotkeys = []guiHotkey{}
		for _, h := range hotkeys {
			state.Hotkeys = append(state.Hotkeys, guiHotkey{h, strings.TrimPrefix(hotkeyLockText(h), " with "), hotkeyKinds(h), describeHotkey(h)})
		}

		state.SocialPresets = db.SocialPresets
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
	freezeCmd.Flags().String("for", "", "How long to stay frozen (e.g. 30m, 2h, 3d; at most a week)")
	freezeCmd.Flags().String("until", "", "Frozen until a time (HH:MM, or YYYY-MM-DD HH:MM; at most a week ahead)")
	freezeCmd.Flags().Bool("yes", false, "Confirm the freeze")
	freezeCmd.Flags().String("mode", freeze.ModeLock, "How to freeze: lock, logout, shutdown, or apps (allowed apps only: kc9k freeze apps)")
	_ = freezeCmd.MarkFlagRequired("for")
	guiStateCmd.Flags().StringArray("reveal", nil, "Show a block with a block password (its password on stdin)")
	rootCmd.AddCommand(freezeCmd, guiStateCmd)
}

// savedName returns the name of the schedule with id, "" for 0.
func savedName(saved []db.SavedSchedule, id int) string {
	for _, s := range saved {
		if s.ID == id {
			return s.Name
		}
	}
	return ""
}

// guiWins turns windows into the interface's, with where each comes from.
func guiWins(saved []db.SavedSchedule, rows []db.SourcedWindow, allow bool) []guiWin {
	out := []guiWin{}
	for _, r := range rows {
		out = append(out, guiWin{Days: r.Days, Start: r.Start, End: r.End, Allow: allow, Source: savedName(saved, r.Source)})
	}
	return out
}

// guiSchedules describes the schedules kept outside blocks.
func guiSchedules(database *db.DB, saved []db.SavedSchedule, modes []db.Mode) []guiSchedule {
	out := []guiSchedule{}
	for _, s := range saved {
		g := guiSchedule{Name: s.Name, Windows: []guiWin{}, Blocks: []string{}, Modes: []string{}}
		for _, w := range s.Windows {
			g.Windows = append(g.Windows, guiWin{Days: w.Days, Start: w.Start, End: w.End})
		}
		blocks, ids, _ := database.ScheduleUsers(s.ID)
		for _, m := range modes {
			if slices.Contains(ids, m.ID) {
				g.Modes = append(g.Modes, m.Name)
			}
		}
		for _, b := range blocks {
			if _, in, _ := database.ModeOf(b); !in {
				g.Blocks = append(g.Blocks, b)
			}
		}
		out = append(out, g)
	}
	return out
}

// guiModes describes the modes: monk mode first.
func guiModes(database *db.DB, saved []db.SavedSchedule, modes []db.Mode, places []db.Place, now time.Time, trusted int64) []guiMode {
	out := []guiMode{}
	for _, m := range modes {
		a, rows := m.Spec.Automation(saved)
		g := guiMode{Name: m.Name, Monk: m.Monk, Active: m.Active, Blocks: append([]string{}, m.Blocks...),
			Windows: guiWins(saved, rows, m.Spec.DefaultOn), Schedules: []string{}, Places: []string{},
			Automation: m.Spec.AutomationSettings, Continuous: a.Continuous(),
			TodayWindows: windowsLeftToday(a.Windows, now), Custom: []string{},
			RunLeft:    enforce.ModeRunLeft(database, m.Name, trusted),
			Wallpapers: wallpaperPictures(enforce.ModeWallpapers(database, m.ID), circadian.MinuteOfDay(now))}
		if param, err := database.ModeCustom(m.Name); err == nil && param != "" {
			if items, err := db.CustomItems(param); err == nil {
				for _, it := range items {
					g.Custom = append(g.Custom, it.String())
				}
			}
		}
		if g.TodayWindows == nil {
			g.TodayWindows = []string{}
		}
		for _, id := range m.Spec.Schedules {
			g.Schedules = append(g.Schedules, savedName(saved, id))
		}
		for _, p := range places {
			if slices.Contains(m.Spec.Places, p.ID) {
				g.Places = append(g.Places, p.Name)
			}
		}
		for _, b := range m.Blocks {
			if on, _ := database.IsBlockActive(b); on {
				g.On++
			}
			if hold, err := blockHold(database, b, now); err == nil && hold.held() {
				g.Locked++
			}
		}
		if m.Monk {
			g.Running = enforce.MonkAutomationOn(database, now)
			if end := enforce.MonkEnd(database, trusted); end > 0 {
				g.MonkLeft = end - trusted // run for a set time: locked
				g.Running = true
			}
		}
		out = append(out, g)
		if m.Monk {
			out = append(out, guiFrozenChicken(database, now))
		}
	}
	return out
}

// guiFrozenChicken is Frozen Chicken as a mode: on while a freeze runs (it
// can't end early), its windows the scheduled freezes.
func guiFrozenChicken(database *db.DB, now time.Time) guiMode {
	g := guiMode{Name: db.FrozenChickenName, Freeze: true, Blocks: []string{}, Windows: []guiWin{},
		Schedules: []string{}, Places: []string{}, TodayWindows: []string{}, Custom: []string{},
		Wallpapers: wallpaperPictures(enforce.ModeWallpapers(database, enforce.FrozenChickenWallpapers), circadian.MinuteOfDay(now))}
	if until, err := database.FreezeUntil(); err == nil && until > now.Unix() {
		g.FreezeLeft, g.Running, g.Active = until-now.Unix(), true, true
		g.FreezeMode, _ = database.FreezeMode()
	}
	schedules, _ := database.ListFreezeSchedules()
	var windows []schedule.Window
	for _, s := range schedules {
		g.Windows = append(g.Windows, guiWin{Days: s.Window.Days, Start: s.Window.Start, End: s.Window.End})
		windows = append(windows, s.Window)
	}
	g.Continuous = len(windows) == 0
	if today := windowsLeftToday(windows, now); today != nil {
		g.TodayWindows = today
	}
	return g
}
