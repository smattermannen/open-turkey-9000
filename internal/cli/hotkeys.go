// hotkeys.go — key combinations that start a block.
//
//	kc9k hotkey add ctrl+alt+s study --for 1h
//	kc9k hotkey add ctrl+alt+w work --lock --restart
//	kc9k hotkey list
//	kc9k hotkey remove 2
//
// Each one becomes a Cinnamon and GNOME keyboard shortcut running 'kc9k hotkey run N',
// which starts the block (with the hotkey's locks, if it has any) and shows a
// notification.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var hotkeyCmd = &cobra.Command{
	Use:   "hotkey",
	Short: "Key combinations that start a block, optionally locked",
	Long: `A hotkey starts a block when you press it, for example to study:

  kc9k hotkey add ctrl+alt+s study --for 1h

Pressing Ctrl+Alt+S then starts the block 'study' with a one-hour timer lock.
A hotkey takes the same locks as 'kc9k start', in the same unlock order:
--for, --until, --lock (and --lock-chars, --lock-words, --lock-text),
--password-stdin (the password is given now), --restart, --delay, --range,
--relock. On a block that is already on, a press only adds or extends locks,
as 'start' does. Keys are modifiers (ctrl, alt, shift, super) and a key
joined with +; F1 to F24 also work alone. Hotkeys are set up as Cinnamon and
GNOME keyboard shortcuts.

A hotkey with a lock needs the settings password when you add it (if locks
need it), not when you press it.`,
}

var hotkeyAddCmd = &cobra.Command{
	Use:   "add [keys] [block]",
	Short: "Add a hotkey: what pressing the keys does",
	Long: `A hotkey runs its actions in order when its keys are pressed (§3.9
Hotkey action). The short form starts a block, with start's locks:

  kc9k hotkey add ctrl+alt+s study --for 1h --yes

Any actions, with --do (any number, in order):

  --do "start BLOCK"        switch a block on (locks: the short form, or --do-json)
  --do "stop BLOCK"         switch a block off (refused while it is locked)
  --do "mode-on MODE"       switch a mode on; "mode-off MODE" off
  --do "mode-run MODE 2h"   run a mode now, locked, for 2 hours
  --do "monk 2h"            monk mode, locked, for 2 hours
  --do "freeze 2h lock"     Frozen Chicken (lock, logout, shutdown or apps)
  --do "close-browser"      close the browsers
  --do "lock-screen [30m]"  lock the screen (for a while: Frozen Chicken)
  --do "logout [1h]", --do "shutdown [1h]"
  --do 'custom "open https://…" "run COMMAND"'   open or run things as you

A hotkey that locks asks for confirmation at each press unless
--no-confirm (its locks are then confirmed only now, R-LOCK-12). One that
switches something off needs the settings password now (if one is set),
not when pressed.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		accel, err := blocker.ParseAccel(args[0])
		if err != nil {
			return err
		}
		var actions []db.HotkeyAction
		if len(args) == 2 {
			if _, err := database.GetBlock(args[1]); err != nil {
				return err
			}
			actions = append(actions, db.HotkeyAction{Kind: db.HotkeyStart, Target: args[1], LockArgs: lockArgs(cmd)})
		} else if len(lockArgs(cmd)) > 0 {
			return fmt.Errorf("locks go with the block a hotkey starts (kc9k hotkey add KEYS BLOCK --for 1h)")
		}
		texts, _ := cmd.Flags().GetStringArray("do")
		for _, t := range texts {
			a, err := parseHotkeyAction(t)
			if err != nil {
				return err
			}
			actions = append(actions, a)
		}
		jsons, _ := cmd.Flags().GetStringArray("do-json")
		for _, t := range jsons {
			var a db.HotkeyAction
			if err := json.Unmarshal([]byte(t), &a); err != nil {
				return fmt.Errorf("invalid --do-json: %w", err)
			}
			actions = append(actions, a)
		}
		noConfirm, _ := cmd.Flags().GetBool("no-confirm")
		id, err := addHotkey(cmd, database, accel, actions, noConfirm)
		if err != nil {
			return err
		}
		h, _ := database.GetHotkey(id)
		fmt.Printf("Hotkey %d: %s %s.\n", id, accel, describeHotkey(h))
		return syncHotkeys(database)
	}),
}

// addHotkey checks and stores a hotkey's actions: each valid, its blocks
// and modes there; the settings password if one switches something off;
// its locks confirmed (R-LOCK-12) and a password lock's password read.
func addHotkey(cmd *cobra.Command, database *db.DB, accel string, actions []db.HotkeyAction, noConfirm bool) (int, error) {
	if len(actions) == 0 {
		return 0, fmt.Errorf("give what the hotkey does: a block to start, or --do")
	}
	h := db.Hotkey{Keys: accel, Actions: actions, NoConfirm: noConfirm}
	var locks []lockToSet
	weakens, password := false, false
	for i, a := range actions {
		if err := db.CheckHotkeyAction(a); err != nil {
			return 0, fmt.Errorf("action %d: %w", i+1, err)
		}
		switch a.Kind {
		case db.HotkeyStart, db.HotkeyStop:
			if _, err := database.GetBlock(a.Target); err != nil {
				return 0, fmt.Errorf("action %d: %w", i+1, err)
			}
			if h.Block == "" && a.Kind == db.HotkeyStart {
				// The first block it starts, as older hotkeys kept it.
				h.Block, h.LockArgs = a.Target, a.LockArgs
			}
		case db.HotkeyModeOn, db.HotkeyModeOff, db.HotkeyModeRun:
			if _, err := resolveTarget(database, a.Target, db.KindMode); err != nil && !isFrozenChicken(a.Target) {
				return 0, fmt.Errorf("action %d: %w", i+1, err)
			}
		}
		weakens = weakens || a.Weakens()
		if a.Kind == db.HotkeyStart && len(a.LockArgs) > 0 {
			req, err := parseLockArgs(a.LockArgs, time.Now())
			if err != nil {
				return 0, fmt.Errorf("action %d: %w", i+1, err)
			}
			if req.extras.Relock > 0 && !req.text && !req.password {
				return 0, fmt.Errorf("--relock goes with a text or password lock")
			}
			password = password || req.password
			locks = append(locks, pressLocks(a.Target, req)...)
		} else if a.Locks() {
			locks = append(locks, lockToSet{Target: hotkeyActionWords(a), Kind: "locked", Length: ""})
		}
	}
	if noConfirm && len(locks) == 0 {
		return 0, fmt.Errorf("--no-confirm goes with a lock: it is about the hotkey's locks")
	}
	if weakens {
		// Asked now, not at the press (as a hotkey's lock password is).
		if err := requireSettingsPassword(database); err != nil {
			return 0, err
		}
	}
	if len(locks) > 0 {
		if err := requireLockPassword(database); err != nil {
			return 0, err
		}
		if password {
			p, err := readPasswordStdin("Password for the hotkey's password lock")
			if err != nil {
				return 0, err
			}
			if h.PasswordHash, err = db.HashPassword(p); err != nil {
				return 0, err
			}
		}
		for i := range locks {
			locks[i].Kind += ", set each time " + accel + " is pressed"
			if noConfirm {
				locks[i].Kind += ", without asking again"
			}
		}
		if err := confirmLocks(cmd, locks...); err != nil {
			return 0, err
		}
	}
	return database.AddHotkey(h)
}

// parseHotkeyAction reads one --do, e.g. "mode-run Work 2h".
func parseHotkeyAction(text string) (db.HotkeyAction, error) {
	fields := splitOutsideQuotes(text, ' ')
	if len(fields) == 0 {
		return db.HotkeyAction{}, fmt.Errorf("empty --do")
	}
	a := db.HotkeyAction{Kind: strings.ToLower(fields[0])}
	rest := fields[1:]
	minutes := func(s string) (int, error) { return parseMinutes(s, "time") }
	var err error
	switch a.Kind {
	case db.HotkeyStart, db.HotkeyStop, db.HotkeyModeOn, db.HotkeyModeOff:
		a.Target = strings.Join(rest, " ")
	case db.HotkeyModeRun:
		if len(rest) < 2 {
			return a, fmt.Errorf("mode-run is \"mode-run MODE 2h\"")
		}
		a.Target = strings.Join(rest[:len(rest)-1], " ")
		a.Minutes, err = minutes(rest[len(rest)-1])
	case db.HotkeyMonk:
		if len(rest) != 1 {
			return a, fmt.Errorf("monk is \"monk 2h\"")
		}
		a.Minutes, err = minutes(rest[0])
	case db.HotkeyFreeze:
		if len(rest) < 1 || len(rest) > 2 {
			return a, fmt.Errorf("freeze is \"freeze 2h [lock|logout|shutdown|apps]\"")
		}
		a.Param = "lock"
		if len(rest) == 2 {
			a.Param = rest[1]
		}
		a.Minutes, err = minutes(rest[0])
	case db.HotkeyLockScreen, db.HotkeyLogout, db.HotkeyShutdown:
		if len(rest) == 1 {
			a.Minutes, err = minutes(rest[0])
		} else if len(rest) > 1 {
			return a, fmt.Errorf("%s takes at most a time", a.Kind)
		}
	case db.HotkeyCustom:
		a.Param, err = customFlag(rest)
	default:
		if len(rest) > 0 {
			return a, fmt.Errorf("%s takes nothing more", a.Kind)
		}
	}
	if err != nil {
		return a, err
	}
	return a, db.CheckHotkeyAction(a)
}

// hotkeyActionWords says an action in a few words.
func hotkeyActionWords(a db.HotkeyAction) string {
	length := humanDuration(time.Duration(a.Minutes) * time.Minute)
	switch a.Kind {
	case db.HotkeyStart:
		return "starts '" + a.Target + "'"
	case db.HotkeyStop:
		return "switches '" + a.Target + "' off"
	case db.HotkeyModeOn:
		return "switches the mode '" + a.Target + "' on"
	case db.HotkeyModeOff:
		return "switches the mode '" + a.Target + "' off"
	case db.HotkeyModeRun:
		return "runs the mode '" + a.Target + "', locked, for " + length
	case db.HotkeyMonk:
		return "starts monk mode, locked, for " + length
	case db.HotkeyFreeze:
		return "starts Frozen Chicken (" + a.Param + ") for " + length
	case db.HotkeyCloseBrowser:
		return "closes the browsers"
	case db.HotkeyCustom:
		return db.DescribeCustom(a.Param)
	}
	if a.Minutes > 0 {
		return a.Kind + " for " + length
	}
	return a.Kind
}

// describeHotkey says what a press does.
func describeHotkey(h db.Hotkey) string {
	var parts []string
	for _, a := range h.Steps() {
		text := hotkeyActionWords(a)
		if a.Kind == db.HotkeyStart && len(a.LockArgs) > 0 {
			if req, err := parseLockArgs(a.LockArgs, time.Now()); err == nil {
				text += " with " + describeLockList(pressLocks(a.Target, req))
			}
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, ", then ")
}

var hotkeyRemoveCmd = &cobra.Command{
	Use:   "remove [number]",
	Short: "Remove a hotkey",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("give the hotkey's number from 'kc9k hotkey list'")
		}
		if err := database.RemoveHotkey(id); err != nil {
			return err
		}
		fmt.Printf("Hotkey %d removed.\n", id)
		return syncHotkeys(database)
	}),
}

var hotkeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List hotkeys",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		keys, err := database.Hotkeys()
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			fmt.Println("No hotkeys.")
			return nil
		}
		for _, h := range keys {
			fmt.Printf("%d. %s starts '%s'%s\n", h.ID, h.Keys, h.Block, hotkeyLockText(h)+noConfirmText(h))
		}
		return nil
	}),
}

// hotkeyRunCmd is what a hotkey's shortcut runs: its actions in order, in
// this process, and a notification of how it went, since no terminal shows
// the output.
var hotkeyRunCmd = &cobra.Command{
	Use:    "run [number]",
	Short:  "Do what a hotkey does (what pressing it runs)",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("give the hotkey's number")
		}
		database, err := openDB()
		if err != nil {
			return err
		}
		h, err := database.GetHotkey(id)
		database.Close()
		if err != nil {
			notify("Killer Chicken 9000", err.Error())
			return err
		}
		// The settings password was asked when the hotkey was added.
		settingsPasswordChecked = true
		steps := h.Steps()
		// R-LOCK-12: without "Run without confirmation" a press that locks
		// asks first, once for all its locks; no means nothing is done.
		var locks []string
		for _, a := range steps {
			if a.Kind == db.HotkeyStart && len(a.LockArgs) > 0 {
				req, err := parseLockArgs(a.LockArgs, time.Now())
				if err != nil {
					notify("Hotkey "+h.Keys, "Its locks can't be read: "+err.Error())
					return err
				}
				if req.password && h.PasswordHash == "" {
					err := fmt.Errorf("the hotkey's password lock has no password; add the hotkey again")
					notify("Hotkey "+h.Keys, err.Error())
					return err
				}
				locks = append(locks, "start '"+a.Target+"' with "+describeLockList(pressLocks(a.Target, req)))
			} else if a.Locks() {
				locks = append(locks, hotkeyActionWords(a))
			}
		}
		if len(locks) > 0 && !h.NoConfirm {
			if !confirmOnDesktop("Lock?", "The hotkey "+h.Keys+" "+strings.Join(locks, ", then ")+
				". While it is on, it can't be weakened.") {
				notify("Hotkey "+h.Keys, "You didn't confirm; nothing was done.")
				return errNotConfirmed
			}
		}
		var done, failed []string
		for _, a := range steps {
			if err := runHotkeyAction(h, a); err != nil {
				failed = append(failed, hotkeyActionWords(a)+": "+err.Error())
				continue
			}
			done = append(done, hotkeyActionWords(a))
		}
		if len(failed) > 0 {
			notify("Hotkey "+h.Keys+": not all done", strings.Join(failed, "\n"))
			return fmt.Errorf("%s", strings.Join(failed, "; "))
		}
		notify("Hotkey "+h.Keys, strings.ToUpper(strings.Join(done, ", then ")[:1])+strings.Join(done, ", then ")[1:]+".")
		return nil
	},
}

// runHotkeyAction does one thing a hotkey does, as its command would; its
// locks were confirmed already (at the press, or when it was added).
func runHotkeyAction(h db.Hotkey, a db.HotkeyAction) error {
	switch a.Kind {
	case db.HotkeyStart:
		if len(a.LockArgs) > 0 {
			if err := startCmd.ParseFlags(a.LockArgs); err != nil {
				return err
			}
			if err := startCmd.Flags().Set("yes", "true"); err != nil {
				return err
			}
			presetPasswordHash = h.PasswordHash
			defer func() { presetPasswordHash = "" }()
		}
		return startCmd.RunE(startCmd, []string{a.Target})
	case db.HotkeyStop:
		return stopCmd.RunE(stopCmd, []string{a.Target})
	case db.HotkeyModeOn, db.HotkeyModeOff:
		c := modeOffCmd
		if a.Kind == db.HotkeyModeOn {
			c = modeOnCmd
			// Monk mode switched on asks first: the press was that.
			_ = c.Flags().Set("yes", "true")
		}
		return switchMode(c, a.Target, a.Kind == db.HotkeyModeOn)
	}
	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()
	why := "Hotkey " + h.Keys
	switch a.Kind {
	case db.HotkeyModeRun:
		err = enforce.StartModeRun(database, a.Target, time.Duration(a.Minutes)*time.Minute)
	case db.HotkeyMonk:
		err = enforce.StartMonk(database, time.Duration(a.Minutes)*time.Minute)
	case db.HotkeyFreeze:
		until := time.Now().Add(time.Duration(a.Minutes) * time.Minute).Unix()
		current, ferr := database.FreezeUntil()
		if ferr != nil {
			return ferr
		}
		if until > current {
			err = database.SetFreeze(until, a.Param)
		}
	case db.HotkeyCustom:
		err = enforce.Run(database, enforce.Consequence{Kind: db.ConsequenceCustom, Param: a.Param}, "")
	default: // close-browser, lock-screen, logout, shutdown
		err = enforce.Run(database, enforce.Consequence{Kind: a.Kind, Minutes: a.Minutes}, why)
	}
	if err != nil {
		return err
	}
	return enforce.ApplyFromDB(database)
}

func noConfirmText(h db.Hotkey) string {
	if h.NoConfirm {
		return " (runs without confirmation)"
	}
	return ""
}

// hotkeyKinds names a hotkey's lock kinds briefly, e.g. " (timer lock, text
// lock)", for its name in the desktop's keyboard settings.
func hotkeyKinds(h db.Hotkey) string {
	args := h.Args()
	if len(args) == 0 {
		return ""
	}
	req, err := parseLockArgs(args, time.Now())
	if err != nil || len(req.kinds) == 0 {
		return " (locked)"
	}
	names := make([]string, len(req.kinds))
	for i, k := range req.kinds {
		names[i] = db.LayerName(k)
	}
	return " (" + strings.Join(names, ", ") + ")"
}

// hotkeyLockText describes a hotkey's locks after "starts 'B'".
func hotkeyLockText(h db.Hotkey) string {
	args := h.Args()
	if len(args) == 0 {
		return ""
	}
	req, err := parseLockArgs(args, time.Now())
	if err != nil {
		return ", with locks that can't be read (" + err.Error() + ")"
	}
	return " with " + describeLockList(pressLocks(h.Block, req))
}

// confirmOnDesktop asks in a desktop dialog; tests replace it.
var confirmOnDesktop = func(title, text string) bool {
	return blocker.ConfirmDesktop(title, text, "Lock")
}

// syncDesktopHotkeys writes the desktop's shortcuts; tests replace it so
// they never touch the real desktop. A database used through KC9K_DB is a
// test's (a GUI smoke test's, say): its hotkeys stay off the desktop too.
var syncDesktopHotkeys = func(keys []blocker.DesktopHotkey) error {
	if os.Getenv("KC9K_DB") != "" {
		return nil
	}
	return blocker.SyncHotkeys(keys)
}

// syncHotkeys puts the hotkeys on the desktop. A failure there doesn't undo
// the change: they are synced again with the next one.
func syncHotkeys(database *db.DB) error {
	keys, err := database.Hotkeys()
	if err != nil {
		return err
	}
	desktop := make([]blocker.DesktopHotkey, len(keys))
	for i, h := range keys {
		name := describeHotkey(h)
		if len(name) > 60 {
			name = name[:57] + "…"
		}
		desktop[i] = blocker.DesktopHotkey{
			ID: h.ID, Accel: h.Keys, Command: "/usr/local/bin/kc9k hotkey run " + strconv.Itoa(h.ID),
			Name: "KC9K: " + name,
		}
	}
	if err := syncDesktopHotkeys(desktop); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}
	return nil
}

func init() {
	addStartLockFlags(hotkeyAddCmd) // the locks each press sets
	addYesFlag(hotkeyAddCmd)
	hotkeyAddCmd.Flags().Bool("no-confirm", false, "Run without confirmation: pressing the hotkey sets its locks without asking (they are confirmed only now)")
	hotkeyAddCmd.Flags().StringArray("do", nil, "An action, in order: start B, stop B, mode-on M, mode-off M, mode-run M 2h, monk 2h, freeze 2h lock, close-browser, lock-screen [D], logout [D], shutdown [D], custom \"open …\" \"run …\"")
	hotkeyAddCmd.Flags().StringArray("do-json", nil, "An action as JSON (kind, target, minutes, param, lock_args)")
	hotkeyCmd.AddCommand(hotkeyAddCmd, hotkeyRemoveCmd, hotkeyListCmd, hotkeyRunCmd)
	rootCmd.AddCommand(hotkeyCmd)
}
