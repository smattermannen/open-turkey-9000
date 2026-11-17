// projectmodes.go — schedules kept outside blocks and modes in project
// files (§3.13 Project, §3.2 Mode, §3.6 Schedule; owner 2026-11-17).
//
// A block's windows are all still in its "schedules" (so an older KC9K
// reads them as its own); those that are copies of a schedule say
// "copy_of"; a block names its mode ("in_mode").
// Importing, such a window is left out when the schedule comes along, and
// the block applies the schedule instead. Modes hold their own windows,
// schedules, places (backups only) and settings; blocks name their mode.
// Import only adds: a schedule or mode of that name here is left as it is
// (the file's blocks then use the one here), monk mode's automation is
// taken only when it has none here, and it is never switched on (that
// asks for confirmation: 'kc9k mode on monk').
package cli

import (
	"fmt"
	"slices"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// savedScheduleExport is a schedule kept outside blocks.
type savedScheduleExport struct {
	Name    string         `json:"name"`
	Windows []windowExport `json:"windows"`
}

// modeExport is a mode; its blocks name it ("in_mode").
type modeExport struct {
	Name       string                 `json:"name"`
	Monk       bool                   `json:"monk,omitempty"`
	Windows    []windowExport         `json:"windows,omitempty"` // its own
	Schedules  []string               `json:"schedules,omitempty"`
	Places     []string               `json:"places,omitempty"` // backups only
	Automation *db.AutomationSettings `json:"automation,omitempty"`
}

// exportModes adds the schedules and modes to out, and each block's mode
// and schedules; places only in a backup.
func exportModes(database *db.DB, out *projectExport, share bool) error {
	saved, err := database.SavedSchedules()
	if err != nil {
		return err
	}
	for _, s := range saved {
		e := savedScheduleExport{Name: s.Name, Windows: []windowExport{}}
		for _, w := range s.Windows {
			e.Windows = append(e.Windows, exportWindow(w))
		}
		out.Schedules = append(out.Schedules, e)
	}
	places, err := database.Places()
	if err != nil {
		return err
	}
	modes, err := database.Modes()
	if err != nil {
		return err
	}
	for _, m := range modes {
		if m.Monk && m.Spec.Equal(db.AutomationSpec{}) {
			continue
		}
		e := modeExport{Name: m.Name, Monk: m.Monk}
		for _, w := range m.Spec.Own {
			e.Windows = append(e.Windows, exportWindow(w))
		}
		for _, id := range m.Spec.Schedules {
			e.Schedules = append(e.Schedules, savedName(saved, id))
		}
		if !share {
			for _, p := range places {
				if slices.Contains(m.Spec.Places, p.ID) {
					e.Places = append(e.Places, p.Name)
				}
			}
		}
		if m.Spec.DefaultOn || m.Spec.NeedBoth {
			s := m.Spec.AutomationSettings
			e.Automation = &s
		}
		out.Modes = append(out.Modes, e)
	}
	for i := range out.Blocks {
		b := &out.Blocks[i]
		if m, in, err := database.ModeOf(b.Name); err != nil {
			return err
		} else if in {
			b.InMode = m.Name
		}
		spec, err := database.BlockAutomationSpec(b.Name)
		if err != nil {
			return err
		}
		for _, id := range spec.Schedules {
			b.AppliedSchedules = append(b.AppliedSchedules, savedName(saved, id))
		}
	}
	return nil
}

// checkedModes is the schedules and modes of a file, checked.
type checkedModes struct {
	schedules map[string][]schedule.Window // by the file's name
	modes     []modeExport
	windows   map[string][]schedule.Window // a mode's own, by its name
}

// checkModes checks a file's schedules and modes before anything is
// written: windows readable, a schedule's or mode's own never overlapping,
// every schedule named there.
func checkModes(in projectExport) (checkedModes, error) {
	out := checkedModes{schedules: map[string][]schedule.Window{}, windows: map[string][]schedule.Window{}, modes: in.Modes}
	read := func(what string, exported []windowExport) ([]schedule.Window, error) {
		var ws []schedule.Window
		for _, e := range exported {
			w, err := e.window()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", what, err)
			}
			w.Allow = false
			for _, o := range ws {
				if o.Overlaps(w) {
					return nil, fmt.Errorf("%s: windows overlap", what)
				}
			}
			ws = append(ws, w)
		}
		return ws, nil
	}
	for _, s := range in.Schedules {
		if err := db.CheckListName(s.Name); err != nil {
			return out, fmt.Errorf("schedule '%s': %w", s.Name, err)
		}
		ws, err := read("schedule '"+s.Name+"'", s.Windows)
		if err != nil {
			return out, err
		}
		out.schedules[s.Name] = ws
	}
	for _, m := range in.Modes {
		if err := db.CheckListName(m.Name); err != nil {
			return out, fmt.Errorf("mode '%s': %w", m.Name, err)
		}
		ws, err := read("mode '"+m.Name+"'", m.Windows)
		if err != nil {
			return out, err
		}
		if m.Monk && m.Automation != nil && m.Automation.DefaultOn {
			return out, fmt.Errorf("monk mode is never enabled by default")
		}
		out.windows[m.Name] = ws
		for _, s := range m.Schedules {
			if _, ok := out.schedules[s]; !ok {
				return out, fmt.Errorf("mode '%s' uses the schedule '%s', which isn't in the file", m.Name, s)
			}
		}
	}
	for _, b := range in.Blocks {
		for _, s := range b.AppliedSchedules {
			if _, ok := out.schedules[s]; !ok {
				return out, fmt.Errorf("block '%s' uses the schedule '%s', which isn't in the file", b.Name, s)
			}
		}
	}
	return out, nil
}

// importSchedules adds the file's schedules (one here of that name is used
// as it is), and returns their ids here by the file's names (-1 in a dry
// run). blockNames are the file's blocks, which keep their names.
func importSchedules(database *db.DB, c checkedModes, blockNames []string, dry bool, say func(string, ...any)) (map[string]int, error) {
	ids := map[string]int{}
	names := make([]string, 0, len(c.schedules))
	for name := range c.schedules {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if s, err := database.GetSavedSchedule(name); err == nil {
			fmt.Printf("Schedule '%s' exists already: left as it is, and used by the file's blocks.\n", name)
			ids[name] = s.ID
			continue
		}
		as := name
		for n := 2; ; n++ {
			what, err := database.NameTaken(as)
			if err != nil {
				return nil, err
			}
			if what == "" && !slices.Contains(blockNames, as) {
				break
			}
			as = fmt.Sprintf("%s-%d", name, n)
		}
		say("add schedule '%s' (%d windows)", as, len(c.schedules[name]))
		if dry {
			ids[name] = -1
			continue
		}
		id, err := database.CreateSavedSchedule(as)
		if err != nil {
			return nil, err
		}
		if err := database.SetSavedScheduleWindows(id, c.schedules[name]); err != nil {
			return nil, err
		}
		ids[name] = id
	}
	return ids, nil
}

// applyImportedSchedules gives an imported block the schedules it used.
func applyImportedSchedules(database *db.DB, block string, used []string, ids map[string]int, dry bool, say func(string, ...any)) error {
	if len(used) == 0 {
		return nil
	}
	say("block '%s' uses the schedules %s", block, quoted(used))
	if dry {
		return nil
	}
	spec, err := database.BlockAutomationSpec(block)
	if err != nil {
		return err
	}
	for _, name := range used {
		if id := ids[name]; id > 0 && !slices.Contains(spec.Schedules, id) {
			spec.Schedules = append(spec.Schedules, id)
		}
	}
	return database.SetBlockAutomation(block, spec)
}

// importModes adds the file's modes with their blocks (renamed: the file's
// block names as created here), and monk mode's automation when it has
// none here.
func importModes(database *db.DB, c checkedModes, in projectExport, renamed map[string]string, scheduleIDs, placeIDs map[string]int,
	dry bool, say func(string, ...any)) error {
	for _, m := range c.modes {
		spec := db.AutomationSpec{Own: c.windows[m.Name]}
		if m.Automation != nil {
			spec.AutomationSettings = *m.Automation
		}
		for _, s := range m.Schedules {
			spec.Schedules = append(spec.Schedules, scheduleIDs[s])
		}
		for _, p := range m.Places {
			if id, ok := placeIDs[p]; ok {
				spec.Places = append(spec.Places, id)
			} else {
				fmt.Printf("Mode '%s': the place '%s' isn't in the file or here, so it is left out.\n", m.Name, p)
			}
		}
		if m.Monk {
			here, err := database.MonkMode()
			if err != nil {
				return err
			}
			if !here.Spec.Equal(db.AutomationSpec{}) {
				fmt.Println("Monk mode has windows or places here already: left as it is.")
				continue
			}
			say("give monk mode its windows and places from the file (switch it on with 'kc9k mode on monk')")
			if !dry {
				if err := database.SetModeAutomation(here.ID, spec); err != nil {
					return err
				}
			}
			continue
		}
		var members []string
		for _, b := range in.Blocks {
			if b.InMode == m.Name {
				members = append(members, renamed[b.Name])
			}
		}
		target, err := database.GetMode(m.Name)
		if err == nil {
			fmt.Printf("Mode '%s' exists already: left as it is; the file's blocks in it join it.\n", m.Name)
			spec = target.Spec
		} else {
			as := m.Name
			for n := 2; ; n++ {
				what, err := database.NameTaken(as)
				if err != nil {
					return err
				}
				if what == "" {
					break
				}
				as = fmt.Sprintf("%s-%d", m.Name, n)
			}
			say("add mode '%s' with %d blocks", as, len(members))
			if dry {
				continue
			}
			if err := database.CreateMode(as); err != nil {
				return err
			}
			if target, err = database.GetMode(as); err != nil {
				return err
			}
			if err := database.SetModeAutomation(target.ID, spec); err != nil {
				return err
			}
		}
		if dry {
			continue
		}
		for _, b := range members {
			if err := database.SetBlockMode(b, target.ID); err != nil {
				return err
			}
			if err := database.SetBlockAutomation(b, spec); err != nil {
				return err
			}
		}
	}
	return nil
}
