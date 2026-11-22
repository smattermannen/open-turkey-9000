// guidedsetup.go — kc9k guided-setup: the guided setup of §4.10 Device
// protection, a checklist for strict mode and no-escape (not a first-run
// wizard: SPEC §6 2026-11-14). It only reads; each step names the command
// that does it, and the GUI runs those commands.
package cli

import (
	"encoding/json"
	"fmt"
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
		Detail: strictDetail})

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
	steps = append(steps, accountSteps(strictUser)...)

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
		firmware.Detail = firmwareUnreachable + firmwareByHand
	}
	steps = append(steps, firmware)

	if others := otherSystems(); len(others) > 0 {
		steps = append(steps, setupStep{ID: "other-systems", Title: "Other systems in the boot menu", State: stepInfo,
			Detail: "These start from the boot menu, and from them this system's disk can be changed: " + strings.Join(others, ", ") + "."})
	}
	steps = append(steps, recoverySteps()...)

	escape := setupStep{ID: "no-escape", Title: "No-escape", State: doneIf(setting(enforce.SettingNoEscape) == "on"),
		Command: "kc9k no-escape on --yes",
		Detail:  escapeDetail}
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
