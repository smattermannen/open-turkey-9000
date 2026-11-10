// hotkeys.go — key combinations that start a block.
//
//	kc9k hotkey add ctrl+alt+s study --for 1h
//	kc9k hotkey list
//	kc9k hotkey remove 2
//
// Each one becomes a Cinnamon and GNOME keyboard shortcut running 'kc9k hotkey run N',
// which starts the block (time-locked when the hotkey says so) and shows a
// notification.
package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

// maxHotkeyLock caps a hotkey's time lock, as browser options' are.
const maxHotkeyLock = 24 * time.Hour

var hotkeyCmd = &cobra.Command{
	Use:   "hotkey",
	Short: "Key combinations that start a block, optionally time-locked",
	Long: `A hotkey starts a block when you press it, for example to study:

  kc9k hotkey add ctrl+alt+s study --for 1h

Pressing Ctrl+Alt+S then starts the block 'study' with a one-hour time lock
(on a block that is already on, it only extends the lock). Keys are
modifiers (ctrl, alt, shift, super) and a key joined with +; F1 to F24 also
work alone. Hotkeys are set up as Cinnamon and GNOME keyboard shortcuts.

A hotkey with a lock needs the settings password when you add it, not when
you press it.`,
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
		lockFor, _ := cmd.Flags().GetString("for")
		if lockFor != "" {
			d, err := time.ParseDuration(lockFor)
			if err != nil || d < time.Minute || d > maxHotkeyLock {
				return fmt.Errorf("--for must be a duration from 1m to 24h, like 1h or 90m")
			}
			// The lock is created later without asking, so ask now.
			if err := requireSettingsPassword(database); err != nil {
				return err
			}
		}
		id, err := database.AddHotkey(db.Hotkey{Keys: accel, Block: args[1], LockFor: lockFor})
		if err != nil {
			return err
		}
		fmt.Printf("Hotkey %d: %s starts '%s'%s.\n", id, accel, args[1], lockText(lockFor))
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
			fmt.Printf("%d. %s starts '%s'%s\n", h.ID, h.Keys, h.Block, lockText(h.LockFor))
		}
		return nil
	}),
}

// hotkeyRunCmd is what a hotkey's shortcut runs. It starts the block in this
// process, through 'start' with the hotkey's lock, and says how it went in a
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
		if h.LockFor != "" {
			if err := startCmd.Flags().Set("for", h.LockFor); err != nil {
				return err
			}
		}
		if err := startCmd.RunE(startCmd, []string{h.Block}); err != nil {
			notify(fmt.Sprintf("Couldn't start '%s'", h.Block), err.Error())
			return err
		}
		notify(fmt.Sprintf("'%s' is on", h.Block), "Started by hotkey"+lockText(h.LockFor)+".")
		return nil
	},
}

func lockText(lockFor string) string {
	if lockFor == "" {
		return ""
	}
	return ", locked for " + lockFor
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
			Name: fmt.Sprintf("KC9K: start %s%s", h.Block, lockText(h.LockFor)),
		}
	}
	if err := blocker.SyncHotkeys(desktop); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}
	return nil
}

func init() {
	hotkeyAddCmd.Flags().String("for", "", "Time-lock the block for this long when the hotkey starts it (e.g. 1h, 90m)")
	hotkeyCmd.AddCommand(hotkeyAddCmd, hotkeyRemoveCmd, hotkeyListCmd, hotkeyRunCmd)
	rootCmd.AddCommand(hotkeyCmd)
}
