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
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
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
	Short: "Add a hotkey that starts a block",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		accel, err := blocker.ParseAccel(args[0])
		if err != nil {
			return err
		}
		if _, err := database.GetBlock(args[1]); err != nil {
			return err
		}
		noConfirm, _ := cmd.Flags().GetBool("no-confirm")
		req, err := parseLockFlags(cmd, time.Now())
		if err != nil {
			return err
		}
		if req.extras.Relock > 0 && !req.text && !req.password {
			return fmt.Errorf("--relock goes with a text or password lock")
		}
		if noConfirm && len(req.kinds) == 0 {
			return fmt.Errorf("--no-confirm goes with a lock: it is about the hotkey's lock")
		}
		h := db.Hotkey{Keys: accel, Block: args[1], LockArgs: lockArgs(cmd), NoConfirm: noConfirm}
		if len(req.kinds) > 0 {
			// The locks are set later without these questions, so ask now.
			if err := requireLockPassword(database); err != nil {
				return err
			}
			if req.password {
				password, err := readPasswordStdin("Password for the hotkey's password lock")
				if err != nil {
					return err
				}
				if h.PasswordHash, err = db.HashPassword(password); err != nil {
					return err
				}
			}
			// R-LOCK-12: the locks are confirmed here. With --no-confirm
			// ("Run without confirmation") this is the only time; otherwise
			// pressing the hotkey asks again.
			locks := pressLocks(args[1], req)
			for i := range locks {
				locks[i].Kind += ", set each time " + accel + " is pressed"
				if noConfirm {
					locks[i].Kind += ", without asking again"
				}
			}
			if err := confirmLocks(cmd, locks...); err != nil {
				return err
			}
		}
		id, err := database.AddHotkey(h)
		if err != nil {
			return err
		}
		fmt.Printf("Hotkey %d: %s starts '%s'%s.\n", id, accel, args[1], hotkeyLockText(h))
		return syncHotkeys(database)
	}),
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

// hotkeyRunCmd is what a hotkey's shortcut runs. It starts the block in this
// process, through 'start' with the hotkey's locks, and says how it went in a
// notification, since no terminal shows the output.
var hotkeyRunCmd = &cobra.Command{
	Use:    "run [number]",
	Short:  "Start a hotkey's block (what pressing it runs)",
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
		if locks := h.Args(); len(locks) > 0 {
			fail := func(err error) error {
				notify(fmt.Sprintf("Couldn't start '%s'", h.Block), err.Error())
				return err
			}
			req, err := parseLockArgs(locks, time.Now())
			if err != nil {
				return fail(fmt.Errorf("the hotkey's locks: %w", err))
			}
			if req.password && h.PasswordHash == "" {
				return fail(fmt.Errorf("the hotkey's password lock has no password; add the hotkey again"))
			}
			// R-LOCK-12: without "Run without confirmation" the press asks.
			if !h.NoConfirm {
				if !confirmOnDesktop("Lock '"+h.Block+"'?", fmt.Sprintf("Start '%s' with: %s. While it is on, it can't be weakened.", h.Block, describeLockList(pressLocks(h.Block, req)))) {
					notify(fmt.Sprintf("'%s' not locked", h.Block), "You didn't confirm; nothing was started.")
					return errNotConfirmed
				}
			}
			if err := startCmd.ParseFlags(locks); err != nil {
				return fail(err)
			}
			if err := startCmd.Flags().Set("yes", "true"); err != nil {
				return err
			}
			presetPasswordHash = h.PasswordHash
			defer func() { presetPasswordHash = "" }()
		}
		if err := startCmd.RunE(startCmd, []string{h.Block}); err != nil {
			notify(fmt.Sprintf("Couldn't start '%s'", h.Block), err.Error())
			return err
		}
		notify(fmt.Sprintf("'%s' is on", h.Block), "Started by hotkey"+hotkeyLockText(h)+".")
		return nil
	},
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
		desktop[i] = blocker.DesktopHotkey{
			ID: h.ID, Accel: h.Keys, Command: "/usr/local/bin/kc9k hotkey run " + strconv.Itoa(h.ID),
			Name: "KC9K: start " + h.Block + hotkeyKinds(h),
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
	hotkeyCmd.AddCommand(hotkeyAddCmd, hotkeyRemoveCmd, hotkeyListCmd, hotkeyRunCmd)
	rootCmd.AddCommand(hotkeyCmd)
}
