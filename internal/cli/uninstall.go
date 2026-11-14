// uninstall.go — the "prepare-uninstall" guard command.
//
// WHY THIS EXISTS
// ---------------
// Two problems with a naive uninstall:
//
//  1. It was an easy escape hatch. "sudo make uninstall" would remove the
//     daemon while a locked block was active, sidestepping the typing
//     challenge entirely.
//
//  2. It left the system broken. Uninstalling removed the binary but never
//     cleaned the blocking layers, so /etc/hosts entries, firewall rules and
//     browser policies stayed in place with no tool left to remove them.
//
// The Makefile's uninstall target runs this command FIRST. It:
//   - refuses (non-zero exit) if any block is held (locked, or in or about to
//     enter a schedule window) or has allow windows, so uninstalling can't
//     bypass a lock or an allow-window block's delay/challenge; and
//   - otherwise deactivates all blocks and removes every blocking layer, so the
//     system is left clean.
//
// It's hidden because users don't call it directly — "make uninstall" does.
package cli

import (
	"fmt"
	"github.com/smattermannen/kc9k/internal/webext"
	"os"
	"path/filepath"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var prepareUninstallCmd = &cobra.Command{
	Use:    "prepare-uninstall",
	Short:  "Internal: refuse if locked, otherwise clean all layers before uninstall",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()
		if err := requireSettingsPassword(database); err != nil {
			return err
		}

		active, err := database.GetActiveBlocks()
		if err != nil {
			return err
		}
		blocks, err := database.ListBlocks()
		if err != nil {
			return err
		}

		if until, err := database.FreezeUntil(); err != nil {
			return err
		} else if until > time.Now().Unix() {
			return fmt.Errorf("cannot uninstall during Frozen Chicken")
		}
		if trusted, err := database.TrustedNow(time.Now()); err != nil {
			return err
		} else if enforce.MonkOn(database, trusted) {
			return fmt.Errorf("cannot uninstall during monk mode")
		}
		freezeSchedules, err := database.ListFreezeSchedules()
		if err != nil {
			return err
		}
		for _, s := range freezeSchedules {
			if enforce.FreezeScheduleHeld(s, time.Now()) {
				return fmt.Errorf("cannot uninstall: freeze schedule %d (%s) is locked", s.Num, s.Window)
			}
		}

		// A locked browser setting can't be undone by uninstalling either.
		trusted, err := database.TrustedNow(time.Now())
		if err != nil {
			return err
		}
		if locked, err := enforce.AnySettingLocked(database, trusted); err != nil {
			return err
		} else if locked {
			return fmt.Errorf("cannot uninstall while a browser setting is locked (see 'kc9k browser')")
		}

		// Refuse while any block is held (char-locked, time-locked, or in or
		// about to enter a scheduled window) — uninstalling must not be a way
		// around a lock. Inactive blocks are checked too, for upcoming windows.
		now := time.Now()
		var held []string
		for _, b := range blocks {
			hold, err := blockHold(database, b.Name, now)
			if err != nil {
				return err
			}
			if hold.held() {
				held = append(held, b.Name)
			}
		}
		if len(held) > 0 {
			return fmt.Errorf("cannot uninstall: locked block(s): %v. Unlock them first, or wait for their timer locks and schedule locks", held)
		}

		// Deactivate everything so the daemon (and this command) leave nothing
		// enforced.
		for _, b := range active {
			if err := database.DeactivateBlock(b.BlockName); err != nil {
				return fmt.Errorf("failed to deactivate block '%s': %w", b.BlockName, err)
			}
		}

		// Remove every blocking layer so the system is left clean.
		if err := blocker.RemoveHosts(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to clean /etc/hosts: %v\n", err)
		}
		if err := blocker.RemoveFirewall(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to clean firewall: %v\n", err)
		}
		if err := blocker.RemoveBrowserPolicies(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to clean browser policies: %v\n", err)
		}
		for _, warning := range undoSystemChanges() {
			fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
		}

		fmt.Println("Killer Chicken 9000 blocking layers removed; ready to uninstall.")
		return nil
	},
}

// undoSystemChanges reverts what KC9K changes outside its blocks, so an
// uninstall leaves nothing behind:
//   - the extension: browsers are told to remove it, since a leftover copy
//     would keep enforcing its last rules forever with no daemon to update it;
//   - the forced search engine, blocked developer tools, LibreWolf's
//     permission for the unsigned extension, and KC9K's locks in LibreWolf's
//     settings (your own overrides file applies directly again);
//   - the SafeSearch lines in /etc/hosts, which pin Google to one address;
//   - the no-escape boot lock, whose GRUB password nobody knows;
//   - the extension files.
//
// It returns a warning for each step that failed.
func undoSystemChanges() []string {
	var warnings []string
	note := func(what string, err error) {
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to %s: %v", what, err))
		}
	}
	note("remove the extension from browsers", blocker.EnsureBrowserSetup(blocker.BrowserSetup{RemoveExtensionID: webext.ID}))
	note("remove the extension from Chrome", blocker.RemoveChromeSetup())
	note("restore LibreWolf's extension signing", blocker.SyncLibreWolfSigning(false))
	note("remove KC9K's locks from LibreWolf's settings", blocker.SyncLibreWolfLocks(false))
	note("remove forced SafeSearch", blocker.SyncSafeSearch(false))
	if blocker.BootLockInstalled() {
		note("remove the no-escape boot lock", blocker.RemoveBootLock())
	}
	files, _ := filepath.Glob(filepath.Join(webext.Dir, "kc9k*.xpi"))
	files = append(files, webext.ChromeKeyPath)
	for _, f := range files {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			note("remove "+f, err)
		}
	}
	return warnings
}

func init() {
	rootCmd.AddCommand(prepareUninstallCmd)
}
