// automation.go — kc9k automation: what switches a block by itself (§3.6,
// owner 2026-11-16): enabled or disabled by default, and with windows and
// places, whether switching needs both. Windows: kc9k schedule; places:
// kc9k place rule.
//
//	kc9k automation work                    # shows it
//	kc9k automation work default enabled    # its windows and places turn it off
//	kc9k automation work needs both         # a window and a place at once
package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
	"github.com/spf13/cobra"
)

var automationCmd = &cobra.Command{
	Use:   "automation [block|mode] [default enabled|disabled | needs both|either]",
	Short: "What switches a block by itself: its default, and both or either",
	Long: `While a block is on, its schedule (kc9k schedule) and its places (kc9k place
rule; any number) switch it by themselves:

  default disabled   its windows and places turn it on (the default)
  default enabled    its windows and places turn it off

With both windows and places, switching needs:

  needs either       a window or a place is enough (the default)
  needs both         a window and a place at once

A block without windows or places is continuous: enabled whenever it is on.
A mode's automation is all its blocks' (kc9k mode); monk mode's is always
disabled by default.
A change that lifts the block anywhere it is enabled now weakens it, refused
while it is locked; one that only adds enabled time is always allowed.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 && len(args) != 3 {
			return fmt.Errorf("give a block, or a block and default enabled|disabled or needs both|either")
		}
		return nil
	},
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		name := args[0]
		t, err := resolveTarget(database, name, targetKind(cmd))
		if err != nil {
			return err
		}
		if t.Saved != nil {
			return fmt.Errorf("'%s' is a schedule: its windows do what the default of each block using it says", name)
		}
		if len(args) == 1 {
			fmt.Println(describeTarget(database, t))
			return nil
		}
		spec, err := targetSpec(database, t)
		if err != nil {
			return err
		}
		after := spec
		switch args[1] + " " + args[2] {
		case "default enabled":
			after.DefaultOn = true
		case "default disabled":
			after.DefaultOn = false
		case "needs both":
			after.NeedBoth = true
		case "needs either":
			after.NeedBoth = false
		default:
			return fmt.Errorf("use 'default enabled', 'default disabled', 'needs both' or 'needs either'")
		}
		if after.DefaultOn == spec.DefaultOn && after.NeedBoth == spec.NeedBoth {
			fmt.Println(describeTarget(database, t))
			return nil
		}
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
		if err := changeAutomation(cmd, database, t, after); err != nil {
			return err
		}
		if t.Mode != nil {
			if t.Mode, err = func() (*db.Mode, error) { m, err := database.GetMode(t.Mode.Name); return &m, err }(); err != nil {
				return err
			}
		}
		fmt.Println(describeTarget(database, t))
		return nil
	}),
}

// automationStrengthens reports whether changing a block's automation
// from before to after only adds enabled time (free; anything else is
// refused while the block is locked). A block's schedule lock holds only
// while its automation has it enabled, so a change that leaves a block
// with a schedule lock continuous ends what the lock protects: refused
// too.
func automationStrengthens(database *db.DB, name string, before, after schedule.Automation) bool {
	if schedule.AutomationWeakens(before, after) {
		return false
	}
	if after.Continuous() && !before.Continuous() {
		if on, _, err := database.ScheduleLock(name); err != nil || on {
			return false
		}
	}
	return true
}

// describeAutomation says what switches a block, in words.
func describeAutomation(database *db.DB, name string, a schedule.Automation) string {
	if a.Continuous() {
		def := "disabled"
		if a.DefaultOn {
			def = "enabled"
		}
		return fmt.Sprintf("'%s' is continuous: enabled whenever it is on (no windows, no places; %s by default "+
			"once it has some).", name, def)
	}
	// One window or one place turns it; more turn it.
	verb := "turn"
	if len(a.Windows)+len(a.Places) == 1 {
		verb = "turns"
	}
	what := verb + " it on (disabled by default)"
	if a.DefaultOn {
		what = verb + " it off (enabled by default)"
	}
	var by []string
	if n := len(a.Windows); n > 0 {
		by = append(by, fmt.Sprintf("its %s (%d)", plural(n, "window", "windows"), n))
	}
	if len(a.Places) > 0 {
		places, _ := database.Places()
		var names []string
		for _, p := range places {
			if slices.Contains(a.Places, p.ID) {
				names = append(names, p.Name)
			}
		}
		by = append(by, fmt.Sprintf("its %s (%s)", plural(len(names), "place", "places"), strings.Join(names, ", ")))
	}
	text := fmt.Sprintf("'%s' is automated: %s %s", name, strings.Join(by, " and "), what)
	if len(a.Windows) > 0 && len(a.Places) > 0 {
		if a.NeedBoth {
			text += "; switching needs both at once"
		} else {
			text += "; either is enough"
		}
	}
	return text + "."
}

func init() {
	addYesFlag(automationCmd, placeRuleCmd)
	addTargetFlags(automationCmd, placeRuleCmd)
	rootCmd.AddCommand(automationCmd)
}
