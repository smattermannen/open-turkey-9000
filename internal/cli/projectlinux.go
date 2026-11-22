// projectlinux.go — a project made on Linux, imported on Windows (W56,
// §4.10 Project "Linux-only"). The file is read as it is (a UTF-8 BOM and
// CRLF line ends are accepted); its Linux-only parts are kept in what is
// imported, listed as "not used on Windows", never an error and never
// turned into Windows names.
package cli

import (
	"bytes"
	"fmt"
	"runtime"
	"slices"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
)

// projectOS is the system a project is imported on; tests set it to
// "windows" to check the Windows import on Linux.
var projectOS = runtime.GOOS

// projectText takes a UTF-8 BOM off a project file. CRLF needs nothing:
// JSON allows it between values, and strings in the file can't hold one.
func projectText(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
}

// linuxOnlyApp says why a block's app entry is Linux-only where the
// import runs ("" when it is used here).
func linuxOnlyApp(entry string) string {
	if projectOS != "windows" {
		return ""
	}
	return blocker.LinuxOnlyApp(entry)
}

// linuxOnlyParts lists what a project holds that is not used on Windows.
func linuxOnlyParts(in projectExport) []string {
	var out []string
	add := func(s string) {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	customs := func(where, param string) {
		items, err := db.CustomItems(param)
		if err != nil {
			return
		}
		for _, it := range items {
			switch {
			case it.Run != "" && blocker.LinuxOnlyCommand(it.Run) != "":
				add(fmt.Sprintf("%s: custom action %q (%s)", where, it.String(), blocker.LinuxOnlyCommand(it.Run)))
			case it.Open != "" && blocker.LinuxPath(it.Open):
				add(fmt.Sprintf("%s: custom action %q (a Linux path)", where, it.String()))
			}
		}
	}
	ladder := func(where string, n *db.Nepsis) {
		if n == nil {
			return
		}
		for _, step := range n.Steps {
			for _, a := range step.Actions {
				if a.Kind == db.ConsequenceCustom {
					customs(where, a.Param)
				}
			}
		}
	}
	for _, b := range in.Blocks {
		where := "block '" + b.Name + "'"
		for _, app := range b.Apps {
			if why := blocker.LinuxOnlyApp(app); why != "" {
				add(fmt.Sprintf("%s: app %s (%s)", where, app, why))
			}
		}
		for _, l := range b.Lists {
			for _, a := range l.Actions {
				if a.Name == db.ActionCustom {
					customs(where, a.Param)
				}
			}
		}
		for _, p := range b.PhraseSets {
			for _, a := range p.Actions {
				if a.Name == db.ActionCustom {
					customs(where, a.Param)
				}
			}
		}
		for _, group := range []string{db.TriggerApps, db.TriggerTitles} {
			for _, a := range b.Triggers[group] {
				if a.Name == db.ActionCustom {
					customs(where, a.Param)
				}
			}
		}
		ladder(where, b.Nepsis)
	}
	ladder("global bypass prevention", in.Nepsis)
	for _, m := range in.Modes {
		if m.Custom != "" {
			customs("mode '"+m.Name+"'", m.Custom)
		}
	}
	if in.Backup != nil {
		for _, l := range in.Backup.SettingLocks {
			if l.Name == "strict" || l.Name == "no-escape" { // PAM guard, GRUB/systemd: device settings, never in a file from KC9K
				add(fmt.Sprintf("setting lock %s (a Linux PAM or systemd setting)", l.Name))
			}
		}
		for _, h := range in.Backup.Hotkeys {
			for _, a := range h.Steps() {
				if a.Kind == db.HotkeyCustom {
					customs("hotkey "+h.Keys, a.Param)
				}
			}
		}
	}
	return out
}

// sayLinuxOnly lists a project's Linux-only parts on Windows.
func sayLinuxOnly(in projectExport) {
	if projectOS != "windows" {
		return
	}
	parts := linuxOnlyParts(in)
	if len(parts) == 0 {
		return
	}
	fmt.Println("Not used on Windows (made for Linux; kept as it is, so the project still works there):")
	for _, p := range parts {
		fmt.Printf("  %s\n", p)
	}
}
