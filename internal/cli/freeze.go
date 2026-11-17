// freeze.go — Frozen Chicken and the JSON state used by the graphical interface.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
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

		d, err := parseUserDuration(forDur)
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
	Automation        db.AutomationSettings `json:"automation"`
	Continuous        bool                  `json:"continuous"`
	Name              string                `json:"name"`
	Lists             []guiList             `json:"lists"`
	PhraseSets        []guiPhraseSet        `json:"phrase_sets"`
	Apps              []string              `json:"apps"`
	Titles            []string              `json:"titles"`
	Nepsis            db.Nepsis             `json:"nepsis"`
	NepsisCount       int                   `json:"nepsis_count"`
	Schedules         []string              `json:"schedules"`
	Windows           []guiWin              `json:"schedule_windows"`
	Users             []string              `json:"users"` // §3.2; empty: every account
	Active            bool                  `json:"active"`
	Held              bool                  `json:"held"`
	CharLocked        bool                  `json:"char_locked"`
	PomodoroOn        int64                 `json:"pomodoro_on"`
	PomodoroOff       int64                 `json:"pomodoro_off"`
	PomodoroLong      int64                 `json:"pomodoro_long"`
	PomodoroEvery     int64                 `json:"pomodoro_every"`
	OnBreak           bool                  `json:"on_break"`
	Password          bool                  `json:"password_locked"`
	Restart           bool                  `json:"restart_locked"`
	Delay             int64                 `json:"delay_seconds"`
	StopSeconds       int64                 `json:"stop_seconds"`
	DelayLeft         int64                 `json:"delay_left"`            // the delay lock unlocks in this many seconds, once asked
	LockOrder         []string              `json:"lock_order,omitempty"`  // locks on, in unlock order (R-LOCK-6)
	TimerAfter        int64                 `json:"timer_after,omitempty"` // a timer lock's seconds, counted once the locks before it are unlocked
	RangeLock         string                `json:"range_lock,omitempty"`  // §3.4, e.g. "mon,tue 09:00-17:00"
	RangeOn           bool                  `json:"range_on,omitempty"`
	Scheduled         bool                  `json:"scheduled"`                // has windows or a place rule (§3.3)
	HasPassword       bool                  `json:"block_password,omitempty"` // §3.11
	DraftOf           string                `json:"draft_of,omitempty"`       // §4.3: this is a draft of that block
	Draft             string                `json:"draft,omitempty"`          // §4.3: this block's draft
	TodayWindows      []string              `json:"today_windows,omitempty"`  // §4.1: windows still to come or open today
	Autostarts        []string              `json:"autostarts"`               // §3.6
	Hidden            bool                  `json:"hidden,omitempty"`         // contents left out until revealed
	Lifted            bool                  `json:"lifted"`                   // active, but its windows or places lift it now
	Enabled           bool                  `json:"enabled"`                  // §3.3: active and enforcing now
	LockSeconds       int64                 `json:"lock_seconds"`
	QuotaLimit        int64                 `json:"quota_limit"`
	QuotaUsed         int64                 `json:"quota_used"`
	QuotaOpen         bool                  `json:"quota_open"`
	QuotaNext         int64                 `json:"quota_next"`
	QuotaPeriod       string                `json:"quota_period,omitempty"`        // "" day, week, month (§3.6)
	QuotaRefill       string                `json:"quota_refill,omitempty"`        // HH:MM
	QuotaSessions     int                   `json:"quota_sessions,omitempty"`      // §3.6 session limit (0: none)
	QuotaSessionsUsed int                   `json:"quota_sessions_used,omitempty"` // this period
	KeywordTest       bool                  `json:"keyword_test"`
	Places            []string              `json:"places"`
	LoosenDelay       int64                 `json:"loosen_delay"`
	ScheduleLock      bool                  `json:"schedule_lock"`
	ScheduleLead      int                   `json:"schedule_lead"` // minutes
	Social            []string              `json:"social"`
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
	ExtensionMissing   bool                      `json:"extension_missing"`
	ExtensionMissingIn []string                  `json:"extension_missing_in"`
	SettingsPass       bool                      `json:"settings_password"`
	FreezeSched        []string                  `json:"freeze_schedules"`
	Browser            map[string]bool           `json:"browser"`
	BrowserLocks       map[string]guiSettingLock `json:"browser_locks"`
	Blocks             []guiBlock                `json:"blocks"`
	Hotkeys            []guiHotkey               `json:"hotkeys"`
	Nepsis             db.Nepsis                 `json:"nepsis"`
	NepsisCount        int                       `json:"nepsis_count"`
	NepsisLock         guiSettingLock            `json:"nepsis_lock"`
	StrictLock         guiSettingLock            `json:"strict_lock"`
	NoWeakening        bool                      `json:"no_weakening"`
	PasswordLocks      bool                      `json:"password_for_locks"`
	DNS                string                    `json:"dns"`
	TamperClock        bool                      `json:"tamper_clock"`
	FileURLs           bool                      `json:"tamper_file_urls"`
	TaskManagers       bool                      `json:"tamper_task_managers"`
	TamperPrivate      bool                      `json:"tamper_private"`
	Grace              int                       `json:"tamper_grace"`
	AnythingLocked     bool                      `json:"anything_locked"`           // §4.10: tamper protection can't go off now
	DesktopWarn        string                    `json:"desktop_warning,omitempty"` // R-DESKTOP-1
	Notifications      enforce.NoticeOptions     `json:"notifications"`
	StrictUse          bool                      `json:"strict_use"`
	StrictExempt       []string                  `json:"strict_exempt"` // §3.8
	Templates          []string                  `json:"templates"`     // §3.2
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
		state.FreezeSched = []string{}
		if schedules, err := database.ListFreezeSchedules(); err == nil {
			for _, s := range schedules {
				text := fmt.Sprintf("%s (%s)", s.Window, s.Mode)
				if s.LockLead >= 0 {
					text += fmt.Sprintf(", locked from %s before", time.Duration(s.LockLead)*time.Minute)
				}
				state.FreezeSched = append(state.FreezeSched, text)
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
		state.Places = []guiPlace{}
		for _, p := range places {
			blocks := p.Blocks
			if blocks == nil {
				blocks = []string{}
			}
			state.Places = append(state.Places, guiPlace{Name: p.Name, Networks: len(p.Networks), Need: p.Needed(), Blocks: blocks})
		}
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
				g.Windows = append(g.Windows, guiWin{Days: w.Days, Start: w.Start, End: w.End, Allow: w.Allow, Break: s.Break})
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
			state.Hotkeys = append(state.Hotkeys, guiHotkey{h, strings.TrimPrefix(hotkeyLockText(h), " with "), hotkeyKinds(h)})
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
	freezeCmd.Flags().String("for", "", "How long to stay frozen (e.g. 30m, 2h; at most 24h)")
	freezeCmd.Flags().Bool("yes", false, "Confirm the freeze")
	freezeCmd.Flags().String("mode", freeze.ModeLock, "How to freeze: lock, logout, shutdown, or apps (allowed apps only: kc9k freeze apps)")
	_ = freezeCmd.MarkFlagRequired("for")
	guiStateCmd.Flags().StringArray("reveal", nil, "Show a block with a block password (its password on stdin)")
	rootCmd.AddCommand(freezeCmd, guiStateCmd)
}
