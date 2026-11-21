// guidedsetup.go — kc9k guided-setup: the guided setup of §4.10 Device
// protection, a checklist for strict mode and no-escape (not a first-run
// wizard: SPEC §6 2026-11-14). It only reads; each step names the command
// that does it, and the GUI runs those commands.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/smattermannen/kc9k/internal/platform"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

// Step states.
const (
	stepDone    = "done"
	stepTodo    = "todo"
	stepByHand  = "by-hand" // only the user can do it, outside KC9K
	stepInfo    = "info"    // nothing to do; worth knowing
	stepUnknown = "unknown" // KC9K can't tell (not run as root)
)

// setupStep is one line of the checklist.
type setupStep struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	State    string `json:"state"`
	Detail   string `json:"detail"`
	Command  string `json:"command,omitempty"` // what does it, for a terminal
	Optional bool   `json:"optional,omitempty"`
}

// Files the checklist reads; tests point them at generated ones.
var (
	shadowPath  = "/etc/shadow"
	grubCfgPath = "/boot/grub/grub.cfg"
)

// rootPasswordState tells whether root can log in with a password: "locked",
// "password", or "" when /etc/shadow can't be read.
func rootPasswordState() string {
	data, err := os.ReadFile(shadowPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 2 || fields[0] != "root" {
			continue
		}
		if fields[1] == "" || strings.HasPrefix(fields[1], "!") || strings.HasPrefix(fields[1], "*") {
			if fields[1] == "" {
				return "password" // no password at all: anyone may log in as root
			}
			return "locked"
		}
		return "password"
	}
	return ""
}

// otherSystems lists the boot entries of other operating systems (Windows,
// another Linux) in GRUB's menu.
func otherSystems() []string {
	data, err := os.ReadFile(grubCfgPath)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "menuentry ") || !strings.Contains(t, "osprober") && !strings.Contains(t, "--class windows") {
			continue
		}
		name := t
		if parts := strings.SplitN(t, "'", 3); len(parts) == 3 {
			name = parts[1]
		}
		out = append(out, name)
	}
	return out
}

// riskyGroups lists the groups of rootGroups name is in.
func riskyGroups(name string) []string {
	u, err := user.Lookup(name)
	if err != nil {
		return nil
	}
	ids, _ := u.GroupIds()
	var out []string
	for _, id := range ids {
		if g, err := user.LookupGroupId(id); err == nil {
			for _, risky := range rootGroups {
				if g.Name == risky {
					out = append(out, risky)
				}
			}
		}
	}
	return out
}

func doneIf(ok bool) string {
	if ok {
		return stepDone
	}
	return stepTodo
}

// guidedSetup builds the checklist, in the order to do it: what protects
// the setup first, no-escape (the point of no return) last.
func guidedSetup(database *db.DB) ([]setupStep, error) {
	setting := func(name string) string { v, _ := database.GetSetting(name); return v }
	var steps []setupStep

	pw, err := settingsPasswordSet(database)
	if err != nil {
		return nil, err
	}
	steps = append(steps, setupStep{ID: "settings-password", Title: "Settings password",
		State: doneIf(pw), Command: "kc9k password set",
		Detail: "Needed to weaken anything. Best typed by someone else, or written down away from the computer."})

	steps = append(steps, setupStep{ID: "no-weakening", Title: "No weakening during active locks",
		State: doneIf(setting(SettingNoWeakening) == "on"), Command: "kc9k no-weakening on",
		Detail: "While anything is locked, no setting (strict mode included) can be weakened."})

	var tamperOff, tamperCmds []string
	for _, t := range []struct {
		name string
		on   bool
	}{
		{"clock", tamperClockOn(database)},
		{"task-managers", setting(enforce.SettingTaskManagers) == "on"},
		{"private", enforce.PrivateTamperOn(database)},
		{"unsupported-browsers", enforce.UnsupportedBrowsersOn(database)},
	} {
		if !t.on {
			tamperOff = append(tamperOff, t.name)
			tamperCmds = append(tamperCmds, "kc9k tamper "+t.name+" on")
		}
	}
	detail := "Clock changes, task managers, private windows and Tor, unsupported browsers."
	if len(tamperOff) > 0 {
		detail += " Off: " + strings.Join(tamperOff, ", ") + "."
	}
	steps = append(steps, setupStep{ID: "tamper", Title: "Tamper protection", State: doneIf(len(tamperOff) == 0),
		Command: strings.Join(tamperCmds, "; "), Detail: detail})

	strictOn := setting(enforce.SettingStrict) == "on"
	steps = append(steps, setupStep{ID: "strict", Title: "Strict mode", State: doneIf(strictOn), Command: "kc9k strict on",
		Detail: "While anything is locked: no sudo or pkexec, no root login or su to root, nothing runs from /home or /tmp, no containers or virtual machines."})

	s, err := loadSettingLock(database, enforce.StrictLockName)
	if err != nil {
		return nil, err
	}
	steps = append(steps, setupStep{ID: "strict-lock", Title: "Strict mode's own lock", Optional: true,
		State: doneIf(s.lock.InForce(s.trusted, s.bootID)), Command: "kc9k strict on --for 30d",
		Detail: "Keeps strict mode switched on even while nothing else is locked."})

	strictUser := setting(enforce.SettingStrictUser)
	if strictUser == "" {
		strictUser = platform.CallerName()
	}
	if strictUser != "" {
		groups := riskyGroups(strictUser)
		step := setupStep{ID: "groups", Title: "No root-equivalent groups", State: doneIf(len(groups) == 0),
			Detail: strictUser + " is in no group that can get around strict mode."}
		if len(groups) > 0 {
			step.Detail = strictUser + " is in " + strings.Join(groups, ", ") + ", which gives raw access to the drives."
			step.Command = "sudo gpasswd -d " + strictUser + " " + strings.Join(groups, ",")
		}
		steps = append(steps, step)
	}

	root := setupStep{ID: "root-password", Title: "No root password"}
	switch rootPasswordState() {
	case "locked":
		root.State, root.Detail = stepDone, "root has no password: nobody can log in as root."
	case "password":
		root.State, root.Command = stepTodo, "sudo passwd -l root"
		root.Detail = "root has a password. Strict mode keeps it out of login and su during a lock, but recovery mode asks for it until no-escape is on."
	default:
		root.State, root.Detail = stepUnknown, "Can't read /etc/shadow (run as root to check)."
	}
	steps = append(steps, root)

	steps = append(steps, setupStep{ID: "reset-guard", Title: "Installer media guard",
		State: doneIf(enforce.ResetGuardOn(database)), Command: "kc9k reset-guard on",
		Detail: "While anything is locked: operating system and USB writer download pages blocked, image writers and Disks closed."})

	fw, err := firmwareStatus()
	if err != nil {
		return nil, err
	}
	firmware := setupStep{ID: "firmware", Title: "Firmware lock"}
	switch {
	case fw.Locked():
		firmware.State, firmware.Detail = stepDone, "Firmware password set; USB and network boot off; boot order locked."
		if m := fw.Missing(); len(m) > 0 {
			firmware.Detail += " Set by hand in the firmware's setup: " + strings.Join(m, ", ") + "."
		}
	case fw.Available():
		firmware.State, firmware.Command = stepTodo, "kc9k firmware-lock on"
		firmware.Detail = "Without a firmware password the boot menu stays open: a USB stick boots and reinstalls. A trusted person types the password and keeps it."
	default:
		firmware.State = stepByHand
		firmware.Detail = "This computer's firmware can't be changed from Linux. " + firmwareByHand
	}
	steps = append(steps, firmware)

	if others := otherSystems(); len(others) > 0 {
		steps = append(steps, setupStep{ID: "other-systems", Title: "Other systems in the boot menu", State: stepInfo,
			Detail: "These start from the boot menu, and from them this system's disk can be changed: " + strings.Join(others, ", ") + "."})
	}

	escape := setupStep{ID: "no-escape", Title: "No-escape", State: doneIf(setting(enforce.SettingNoEscape) == "on"),
		Command: "kc9k no-escape on --yes",
		Detail:  "Last: kc9k.nofreeze and recovery mode stop working, and the boot menu gets a password nobody knows. Until then they are the way out of a bug (R-ESCAPE-1); try it in a virtual machine first."}
	steps = append(steps, escape)
	return steps, nil
}

var guidedSetupCmd = &cobra.Command{
	Use:   "guided-setup",
	Short: "Checklist for strict mode, no-escape and factory reset protection",
	Long: `Lists the steps of the strictest setup in the order to take them, each with
whether it is done and the command that does it: settings password, no
weakening, tamper protection, strict mode and its lock, root-equivalent
groups, the root password, the installer media guard, the firmware lock, and
no-escape last. It changes nothing.`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		steps, err := guidedSetup(database)
		if err != nil {
			return err
		}
		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
			data, err := json.MarshalIndent(steps, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}
		marks := map[string]string{stepDone: "[x]", stepTodo: "[ ]", stepByHand: "[ ]", stepInfo: " i ", stepUnknown: "[?]"}
		for i, s := range steps {
			title := s.Title
			if s.Optional {
				title += " (optional)"
			}
			fmt.Printf("%s %2d. %s\n       %s\n", marks[s.State], i+1, title, s.Detail)
			if s.State != stepDone && s.Command != "" {
				fmt.Printf("       → %s\n", s.Command)
			}
		}
		return nil
	}),
}

func init() {
	guidedSetupCmd.Flags().Bool("json", false, "Print the checklist as JSON")
	rootCmd.AddCommand(guidedSetupCmd)
}
