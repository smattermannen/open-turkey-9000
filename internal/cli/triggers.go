// triggers.go — kc9k block trigger: what a block's apps, its window titles
// and each of its place rules do when they fire (owner, 2026-11-16; §1 "any
// trigger to any action"), one action set per group.
//
//	kc9k block trigger work                              # every group's actions
//	kc9k block trigger work apps logout on --for 30m     # running an app logs out
//	kc9k block trigger work apps close off               # and the app stays open
//	kc9k block trigger work place:library monk on --for 2h --no-confirm-ok
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

var blockTriggerCmd = &cobra.Command{
	Use:   "trigger [block] [apps|titles|place:NAME] [action] [on|off]",
	Short: "What a block's apps, window titles and place rules do when they fire",
	Long: `A block's apps, its window titles and each of its place rules each have one
set of actions, like a list's:

  apps      close (the default: the app is closed)
  titles    close (the default: the window is closed)
  place:N   enable (the default: the block is enabled while at place N)

and for all of them close-browser, lock-screen (--for, required), logout and
shutdown (--for: that long; without, once), monk (--for, required; add
--no-confirm-ok: monk mode started this way asks nothing). Logout, shutdown
and lock-screen are announced a minute ahead. An app or a window fires its
actions when it is found (at most once a minute), a place when the
computer arrives there.

Turning an action on, or a longer --for, is always allowed; turning one off,
or a shorter --for, weakens the block. With only the block, prints them.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 && len(args) != 4 {
			return fmt.Errorf("give a block, or a block, a group (apps, titles or place:NAME), an action and on or off")
		}
		return nil
	},
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		block := args[0]
		if _, err := database.GetBlock(block); err != nil {
			return err
		}
		if len(args) == 1 {
			return printTriggers(database, block)
		}
		group, label, err := triggerGroup(database, block, args[1])
		if err != nil {
			return err
		}
		on, err := parseOnOff(args[3])
		if err != nil {
			return err
		}
		a := db.TriggerAction{Name: strings.ToLower(args[2])}
		if spec, _ := cmd.Flags().GetString("for"); spec != "" {
			if !on {
				return fmt.Errorf("--for only goes with 'on'")
			}
			d, err := parseUserDuration(spec)
			if err != nil || d < time.Minute {
				return fmt.Errorf("invalid --for %q (1m or more)", spec)
			}
			a.Minutes = int(d / time.Minute)
		}
		if err := db.CheckTriggerAction(group, a); err != nil {
			return err
		}
		actions, err := database.TriggerActions(block, group)
		if err != nil {
			return err
		}
		old, had := db.FindTriggerAction(actions, a.Name)
		next := []db.TriggerAction{}
		for _, x := range actions {
			if x.Name != a.Name {
				next = append(next, x)
			}
		}
		strengthens := true
		switch {
		case on && had && a.Minutes == old.Minutes:
			fmt.Printf("%s of '%s' already %s.\n", label, block, describeTrigger(a))
			return nil
		case on:
			next = append(next, a)
			// A longer time is stricter; a shorter one, or once instead of
			// for a while, weakens.
			strengthens = !had || (old.Minutes > 0 && a.Minutes > old.Minutes)
			if a.Name == db.ConsequenceMonk {
				if err := requireMonkAck(cmd, label+" of '"+block+"'"); err != nil {
					return err
				}
			}
		case !had:
			fmt.Printf("%s of '%s' don't %s.\n", label, block, a.Name)
			return nil
		default:
			strengthens = false
		}
		if err := ensureEditable(database, block, strengthens); err != nil {
			return err
		}
		if err := database.SetTriggerActions(block, group, next); err != nil {
			return err
		}
		fmt.Printf("%s of '%s': %s\n", label, block, describeTriggers(next))
		return nil
	}),
}

// triggerGroup reads a group argument: apps, titles or place:NAME (a place
// the block has a rule at).
func triggerGroup(database *db.DB, block, arg string) (group, label string, err error) {
	switch lower := strings.ToLower(arg); {
	case lower == db.TriggerApps:
		return db.TriggerApps, "Apps", nil
	case lower == db.TriggerTitles:
		return db.TriggerTitles, "Window titles", nil
	case strings.HasPrefix(lower, "place:"):
		p, err := database.GetPlace(arg[len("place:"):])
		if err != nil {
			return "", "", err
		}
		for _, b := range p.Blocks {
			if b == block {
				return db.PlaceTrigger(p.ID), "The rule at '" + p.Name + "'", nil
			}
		}
		return "", "", fmt.Errorf("block '%s' has no rule at '%s' ('kc9k place rule %s %s on')", block, p.Name, block, p.Name)
	}
	return "", "", fmt.Errorf("%q isn't a group: use apps, titles or place:NAME", arg)
}

func describeTrigger(a db.TriggerAction) string {
	if a.Minutes > 0 {
		return fmt.Sprintf("%s %s", a.Name, humanDuration(time.Duration(a.Minutes)*time.Minute))
	}
	return a.Name
}

func describeTriggers(actions []db.TriggerAction) string {
	if len(actions) == 0 {
		return "no actions (they do nothing)"
	}
	parts := make([]string, len(actions))
	for i, a := range actions {
		parts[i] = describeTrigger(a)
	}
	return strings.Join(parts, ", ")
}

func printTriggers(database *db.DB, block string) error {
	for _, g := range []struct{ group, label string }{{db.TriggerApps, "apps"}, {db.TriggerTitles, "titles"}} {
		actions, err := database.TriggerActions(block, g.group)
		if err != nil {
			return err
		}
		fmt.Printf("%s: %s\n", g.label, describeTriggers(actions))
	}
	places, err := database.Places()
	if err != nil {
		return err
	}
	for _, p := range places {
		for _, b := range p.Blocks {
			if b == block {
				actions, err := database.TriggerActions(block, db.PlaceTrigger(p.ID))
				if err != nil {
					return err
				}
				fmt.Printf("place:%s: %s\n", p.Name, describeTriggers(actions))
			}
		}
	}
	return nil
}

func init() {
	blockTriggerCmd.Flags().String("for", "", "How long lock-screen, logout, shutdown or monk lasts (e.g. 30m, 2h)")
	addMonkAckFlag(blockTriggerCmd)
	blockCmd.AddCommand(blockTriggerCmd)
}
