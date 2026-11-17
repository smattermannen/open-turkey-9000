// presets.go — kc9k preset (§3.2 Preset, §4.4; owner 2026-11-17): saved
// copies outside blocks, of every type. Block presets are the templates
// ('kc9k template'), list and phrase set presets the clusters ('kc9k
// cluster'); a block's lists together, an automation, a schedule's windows
// and a hotkey are kept as presets here. Saving copies, using copies: a
// later change on either side changes nothing on the other. Using one goes
// through the same gates as the commands it stands for.
//
//	kc9k preset                                  # every preset, by type
//	kc9k preset save lists B NAME [--exceptions|--blacklists] [--actions]
//	kc9k preset save automation B|MODE NAME
//	kc9k preset save schedule B|MODE|SCHEDULE NAME
//	kc9k preset use NAME B|MODE
//	kc9k preset rename NAME NEW | delete NAME
package cli

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
	"github.com/spf13/cobra"
)

var presetKindWords = map[string]string{
	db.PresetLists:      "Lists presets",
	db.PresetAutomation: "Automation presets",
	db.PresetSchedule:   "Schedule presets",
	db.PresetHotkey:     "Hotkey presets",
}

var presetCmd = &cobra.Command{
	Use:   "preset",
	Short: "Presets: saved copies of lists, phrase sets, blocks, automations, schedules, hotkeys",
	Long: `A preset is a copy kept outside blocks, never enforced. Saving one copies,
and using one copies: changing either afterwards changes nothing on the
other. Block presets are 'kc9k template', list and phrase set presets
'kc9k cluster'; a block's lists together, automations, schedules' windows
and hotkeys are kept here.

  kc9k preset save lists BLOCK NAME [--blacklists|--exceptions] [--actions]
  kc9k preset save automation BLOCK|MODE NAME
  kc9k preset save schedule BLOCK|MODE|SCHEDULE NAME
  kc9k preset use NAME BLOCK|MODE        (a hotkey preset: NAME alone)
  kc9k preset rename NAME NEW
  kc9k preset delete NAME

--kind lists|automation|schedule|hotkey says which when names are shared;
--mode and --schedule say a target or source is a mode's or schedule's
name (a block's is read first).`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		templates, err := database.Templates()
		if err != nil {
			return err
		}
		clusters, err := database.ListClusters()
		if err != nil {
			return err
		}
		printGroup := func(title string, names []string) {
			if len(names) == 0 {
				return
			}
			fmt.Println(title)
			for _, n := range names {
				fmt.Println("  " + n)
			}
		}
		printGroup("Block presets", templates)
		var lists, phrases []string
		for _, c := range clusters {
			if c.Phrases {
				phrases = append(phrases, c.Name)
			} else {
				lists = append(lists, c.Name)
			}
		}
		printGroup("List presets", lists)
		printGroup("Phrase set presets", phrases)
		presets, err := database.Presets("")
		if err != nil {
			return err
		}
		for _, kind := range db.PresetKinds {
			var names []string
			for _, p := range presets {
				if p.Kind == kind {
					names = append(names, p.Name+" — "+describePreset(database, p))
				}
			}
			printGroup(presetKindWords[kind], names)
		}
		if len(templates)+len(clusters)+len(presets) == 0 {
			fmt.Println("No presets yet. Save one with 'kc9k preset save …', 'kc9k template save' or 'kc9k cluster save'.")
		}
		return nil
	}),
}

// describePreset says in a few words what a preset holds.
func describePreset(database *db.DB, p db.Preset) string {
	switch p.Kind {
	case db.PresetLists:
		var lp db.ListsPreset
		_ = json.Unmarshal(p.Data, &lp)
		var parts []string
		for _, l := range lp.Lists {
			parts = append(parts, fmt.Sprintf("%s (%s, %d)", l.Name, l.Kind, len(l.Entries)))
		}
		with := ""
		if lp.Actions {
			with = ", with actions"
		}
		return strings.Join(parts, ", ") + with
	case db.PresetAutomation:
		var spec db.AutomationSpec
		_ = json.Unmarshal(p.Data, &spec)
		all, _ := database.SavedSchedules()
		a, _ := spec.Automation(all)
		if a.Continuous() {
			return "no windows or places"
		}
		return automationWords(database, a)
	case db.PresetSchedule:
		var windows []schedule.Window
		_ = json.Unmarshal(p.Data, &windows)
		return automationWords(database, schedule.Automation{Windows: windows})
	}
	return ""
}

var presetSaveCmd = &cobra.Command{
	Use:   "save [lists|automation|schedule] [block|mode|schedule] [name]",
	Short: "Save a copy as a preset",
	Args:  cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		kind, source, name := args[0], args[1], args[2]
		switch kind {
		case db.PresetLists:
			if err := revealBlock(database, source); err != nil {
				return err
			}
			detail, err := database.GetBlock(source)
			if err != nil {
				return err
			}
			onlyExceptions, _ := cmd.Flags().GetBool("exceptions")
			onlyBlacklists, _ := cmd.Flags().GetBool("blacklists")
			withActions, _ := cmd.Flags().GetBool("actions")
			lp := db.ListsPreset{Lists: []db.WebList{}, Actions: withActions}
			for _, l := range detail.Lists {
				if (onlyExceptions && !l.IsExceptions()) || (onlyBlacklists && l.IsExceptions()) {
					continue
				}
				l.ID = 0
				if !withActions {
					l.Actions = nil
				}
				lp.Lists = append(lp.Lists, l)
			}
			if len(lp.Lists) == 0 {
				return fmt.Errorf("block '%s' has no such lists", source)
			}
			if err := database.SavePreset(kind, name, lp); err != nil {
				return err
			}
			fmt.Printf("Preset '%s' saved: %d %s of '%s'.\n", name, len(lp.Lists), map[bool]string{true: "list", false: "lists"}[len(lp.Lists) == 1], source)
			return nil
		case db.PresetAutomation, db.PresetSchedule:
			t, err := resolveTarget(database, source, targetKind(cmd))
			if err != nil {
				return err
			}
			var spec db.AutomationSpec
			switch {
			case t.Saved != nil:
				if kind == db.PresetAutomation {
					return fmt.Errorf("'%s' is a schedule: save it as a schedule preset", source)
				}
				spec.Own = slices.Clone(t.Saved.Windows)
			default:
				if spec, err = targetSpec(database, t); err != nil {
					return err
				}
			}
			if kind == db.PresetSchedule {
				all, err := database.SavedSchedules()
				if err != nil {
					return err
				}
				a, _ := spec.Automation(all)
				windows := []schedule.Window{}
				for _, w := range a.Windows {
					w.Allow = false
					windows = append(windows, w)
				}
				if len(windows) == 0 {
					return fmt.Errorf("%s has no windows", t.what())
				}
				if err := database.SavePreset(kind, name, windows); err != nil {
					return err
				}
				fmt.Printf("Preset '%s' saved: the windows of %s.\n", name, t.what())
				return nil
			}
			if err := database.SavePreset(kind, name, spec); err != nil {
				return err
			}
			fmt.Printf("Preset '%s' saved: the automation of %s.\n", name, t.what())
			return nil
		}
		return fmt.Errorf("presets saved here are lists, automation or schedule ('kc9k template save' for a block, 'kc9k cluster save' for one list)")
	}),
}

var presetUseCmd = &cobra.Command{
	Use:   "use [preset] [block|mode]",
	Short: "Copy a preset into a block or mode",
	Args:  cobra.RangeArgs(1, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		kind, _ := cmd.Flags().GetString("kind")
		p, err := database.FindPreset(kind, args[0])
		if err != nil {
			return err
		}
		if p.Kind == db.PresetHotkey {
			return useHotkeyPreset(cmd, database, p)
		}
		if len(args) != 2 {
			return fmt.Errorf("say which block or mode to copy '%s' into", p.Name)
		}
		target := args[1]
		switch p.Kind {
		case db.PresetLists:
			var lp db.ListsPreset
			if err := json.Unmarshal(p.Data, &lp); err != nil {
				return err
			}
			return useListsPreset(cmd, database, target, lp)
		case db.PresetAutomation, db.PresetSchedule:
			t, err := resolveTarget(database, target, targetKind(cmd))
			if err != nil {
				return err
			}
			if t.Saved != nil {
				return fmt.Errorf("'%s' is a schedule: use presets on blocks and modes", target)
			}
			before, err := targetSpec(database, t)
			if err != nil {
				return err
			}
			after := before.Clone()
			if p.Kind == db.PresetAutomation {
				if err := json.Unmarshal(p.Data, &after); err != nil {
					return err
				}
				// What no longer exists (a deleted schedule or place) is left out.
				saved, _ := database.SavedSchedules()
				after.Schedules = slices.DeleteFunc(after.Schedules, func(id int) bool {
					return !slices.ContainsFunc(saved, func(s db.SavedSchedule) bool { return s.ID == id })
				})
				places, _ := database.Places()
				after.Places = slices.DeleteFunc(after.Places, func(id int) bool {
					return !slices.ContainsFunc(places, func(pl db.Place) bool { return pl.ID == id })
				})
			} else {
				var windows []schedule.Window
				if err := json.Unmarshal(p.Data, &windows); err != nil {
					return err
				}
				for _, w := range windows {
					for _, o := range after.Own {
						if o.Overlaps(w) {
							return fmt.Errorf("its window %s overlaps %s of %s; a block's windows never overlap", w, o, t.what())
						}
					}
					after.Own = append(after.Own, w)
				}
			}
			if err := changeAutomation(cmd, database, t, after); err != nil {
				return err
			}
			fmt.Printf("Preset '%s' copied into %s.\n", p.Name, t.what())
			return nil
		}
		return fmt.Errorf("unknown preset kind %q", p.Kind)
	}),
}

// useListsPreset copies a lists preset's lists into a block, each as a new
// list (a name it has gets a number), through the gates of the list
// commands: blacklists and their entries are free, exceptions weaken.
func useListsPreset(cmd *cobra.Command, database *db.DB, block string, lp db.ListsPreset) error {
	detail, err := database.GetBlock(block)
	if err != nil {
		return err
	}
	for _, l := range lp.Lists {
		name := l.Name
		for n := 2; ; n++ {
			if _, taken := detail.Lists.Find(name); !taken {
				break
			}
			name = fmt.Sprintf("%s %d", l.Name, n)
		}
		if l.IsExceptions() && len(l.Entries) > 0 {
			if err := ensureEditable(database, block, false); err != nil {
				return fmt.Errorf("the exception list %s: %w", l.Name, err)
			}
		}
		if _, err := database.CreateList(block, l.Kind, name, l.Where); err != nil {
			return err
		}
		if len(l.Entries) > 0 {
			if err := listAdd(cmd, database, block, name, l.Entries); err != nil {
				return err
			}
		}
		if lp.Actions && !l.IsExceptions() {
			made, _ := database.GetBlock(block)
			ml, _ := made.Lists.Find(name)
			for _, a := range l.Actions {
				if err := listAction(cmd, database, block, name, a.Name, true, &actionSettings{param: a.Param, minutes: a.Minutes}); err != nil {
					return err
				}
				if a.Distinct > 1 {
					if err := listThreshold(cmd, database, block, name, a.Name, a.Distinct); err != nil {
						return err
					}
				}
			}
			for _, a := range ml.Actions {
				if !slices.ContainsFunc(l.Actions, func(x db.ListAction) bool { return x.Name == a.Name }) {
					if err := listAction(cmd, database, block, name, a.Name, false, nil); err != nil {
						return err
					}
				}
			}
		}
		if detail, err = database.GetBlock(block); err != nil {
			return err
		}
		fmt.Printf("List '%s' copied into block '%s'.\n", name, block)
	}
	return settle(database, block)
}

// useHotkeyPreset is filled in with hotkey actions (§3.9, §4.13).
var useHotkeyPreset = func(cmd *cobra.Command, database *db.DB, p db.Preset) error {
	return fmt.Errorf("hotkey presets are added on the Hotkeys page")
}

var presetRenameCmd = &cobra.Command{
	Use:   "rename [preset] [new name]",
	Short: "Rename a preset",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		kind, _ := cmd.Flags().GetString("kind")
		p, err := database.FindPreset(kind, args[0])
		if err != nil {
			return err
		}
		if err := database.RenamePreset(p.ID, args[1]); err != nil {
			return err
		}
		fmt.Printf("Preset '%s' renamed '%s'.\n", p.Name, args[1])
		return nil
	}),
}

var presetDeleteCmd = &cobra.Command{
	Use:   "delete [preset]",
	Short: "Delete a preset (what was copied from it stays)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		kind, _ := cmd.Flags().GetString("kind")
		p, err := database.FindPreset(kind, args[0])
		if err != nil {
			return err
		}
		if err := database.DeletePreset(p.ID); err != nil {
			return err
		}
		fmt.Printf("Preset '%s' deleted.\n", p.Name)
		return nil
	}),
}

func init() {
	presetSaveCmd.Flags().Bool("blacklists", false, "lists: only the blacklists")
	presetSaveCmd.Flags().Bool("exceptions", false, "lists: only the exception lists")
	presetSaveCmd.Flags().Bool("actions", false, "lists: with their actions")
	presetSaveCmd.MarkFlagsMutuallyExclusive("blacklists", "exceptions")
	for _, c := range []*cobra.Command{presetUseCmd, presetRenameCmd, presetDeleteCmd} {
		c.Flags().String("kind", "", "lists, automation, schedule or hotkey: which preset, when names are shared")
	}
	addTargetFlags(presetSaveCmd, presetUseCmd)
	addYesFlag(presetUseCmd)
	addNowFlag(presetUseCmd)
	presetCmd.AddCommand(presetSaveCmd, presetUseCmd, presetRenameCmd, presetDeleteCmd)
	rootCmd.AddCommand(presetCmd)
}
