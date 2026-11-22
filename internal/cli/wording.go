package cli

// wording.go — help texts on Windows (ROADMAP W16): the commands' texts
// are written for Linux; on Windows each Linux-only name is replaced by
// Windows' own (wording_windows.go applies it to every command). Kept
// here, without a build tag, so its table is compiled on both systems.

import (
	"strings"

	"github.com/spf13/cobra"
)

// windowsWords replaces Linux's names in help texts with Windows' own,
// longest first where one contains another.
var windowsWords = strings.NewReplacer(
	"Emergency exit: in GRUB, press e on the boot entry and add\nkc9k.nofreeze to the line that starts with \"linux\".",
	"Emergency exit: start Windows in Safe Mode (Settings › System › Recovery ›\nAdvanced startup, or F8 while it starts): a freeze doesn't hold there\nunless no-escape is on.",
	"If it's lost: boot the recovery entry of the boot menu and run, as root,\n'kc9k-bin recover' (not while no-escape is on).",
	"If it's lost: start Windows in Safe Mode and run 'kc9k recover' in an\nadministrator's console (not while no-escape is on).",
	"If one is lost, root can remove them all:\n  sudo python3 -c \"import sqlite3; c=sqlite3.connect('/var/lib/kc9k/kc9k.db'); c.execute(\\\"DELETE FROM settings WHERE key LIKE 'access.password.%'\\\"); c.commit()\"",
	"If one is lost: start Windows in Safe Mode and run 'kc9k recover --access'\nin an administrator's console (not while no-escape is on).",
	"root-equivalent\ngroups, the root password,", "other administrator\naccounts, the built-in Administrator,",
	"Hotkeys are set up as Cinnamon and\nGNOME keyboard shortcuts.", "Hotkeys are registered by KC9K's agent in your\nsession.",
	"(System Monitor, htop, …)", "(Task Manager, Process Explorer, …)",
	"and no guest session at the login screen (LightDM)", "and Windows' Guest account off",
	"file:///home/me/games/*", "file:///C:/Users/me/games/*",
	"~/Pictures/", `C:\Users\me\Pictures\`,
	"short of root", "short of SYSTEM",
	" (use via systemd)", "",
	"Killer Chicken 9000 daemon", "Killer Chicken 9000 service's work (the service runs it)",
	"/etc/hosts", "the hosts file",
	"The daemon", "The service",
	"the daemon", "the service",
)

// rewordForWindows applies windowsWords to every command's texts.
func rewordForWindows(c *cobra.Command) {
	c.Short = windowsWords.Replace(c.Short)
	c.Long = windowsWords.Replace(c.Long)
	c.Example = windowsWords.Replace(c.Example)
	for _, sub := range c.Commands() {
		rewordForWindows(sub)
	}
}
