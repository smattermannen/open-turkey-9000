// projectwallpapers.go — modes' wallpapers and custom actions, the
// wallpaper settings, and custom actions in general in project files
// (§3.13 Project, R-MODE-5, R-CUSTOM-1; owner 2026-11-17). Import only
// adds: a mode that exists here keeps its wallpapers and custom actions,
// monk mode and Frozen Chicken take the file's only when they have none,
// and the wallpaper settings only when none are set here. A file whose
// custom actions would open or run something here is imported only once
// that is confirmed (--yes), since a shared file can come from anyone.
package cli

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/smattermannen/kc9k/internal/circadian"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

// wallpaperExport is the wallpaper settings, Frozen Chicken's wallpapers
// and the pictures of the modes' wallpapers and of the default.
type wallpaperExport struct {
	Settings      *enforce.WallpaperSettings `json:"settings,omitempty"`
	FrozenChicken []circadian.Wallpaper      `json:"frozen_chicken,omitempty"`
	Pictures      map[string]string          `json:"pictures,omitempty"` // stored name -> base64
}

// exportWallpapers adds the wallpaper settings and the pictures that the
// modes (already in out) and the default show.
func exportWallpapers(database *db.DB, out *projectExport) {
	pictures := map[string]string{}
	add := func(image string) {
		if _, have := pictures[image]; have || image == "" {
			return
		}
		if data, err := os.ReadFile(filepath.Join(enforce.WallpapersDir(), filepath.Base(image))); err == nil {
			pictures[image] = base64.StdEncoding.EncodeToString(data)
		}
	}
	for _, m := range out.Modes {
		for _, w := range m.Wallpapers {
			add(w.Image)
		}
	}
	e := &wallpaperExport{FrozenChicken: enforce.ModeWallpapers(database, enforce.FrozenChickenWallpapers)}
	for _, w := range e.FrozenChicken {
		add(w.Image)
	}
	if raw, _ := database.GetSetting("wallpaper"); raw != "" {
		s := enforce.LoadWallpaperSettings(database)
		e.Settings = &s
		add(s.Custom)
	}
	if len(pictures) > 0 {
		e.Pictures = pictures
	}
	if e.Settings != nil || len(e.FrozenChicken) > 0 || len(pictures) > 0 {
		out.Wallpaper = e
	}
}

// checkWallpapers checks the file's wallpaper pictures and the times of the
// modes' wallpapers before anything is written.
func checkWallpapers(in projectExport) (map[string][]byte, error) {
	pictures := map[string][]byte{}
	if in.Wallpaper != nil {
		for name, encoded := range in.Wallpaper.Pictures {
			raw, err := checkedWallpaper(name, encoded)
			if err != nil {
				return nil, err
			}
			pictures[name] = raw
		}
	}
	times := func(what string, ws []circadian.Wallpaper) error {
		for _, w := range ws {
			if w.From < 0 || w.From >= circadian.DayMinutes || w.To < 0 || w.To >= circadian.DayMinutes {
				return fmt.Errorf("%s: a wallpaper's times can't be read", what)
			}
		}
		return nil
	}
	for _, m := range in.Modes {
		if err := times("mode '"+m.Name+"'", m.Wallpapers); err != nil {
			return nil, err
		}
		if m.Custom != "" {
			if err := db.CheckCustomParam(m.Custom); err != nil {
				return nil, fmt.Errorf("mode '%s': %w", m.Name, err)
			}
		}
	}
	if in.Wallpaper != nil {
		if err := times(db.FrozenChickenName, in.Wallpaper.FrozenChicken); err != nil {
			return nil, err
		}
	}
	return pictures, nil
}

// keepPictured returns the wallpapers whose picture is in the file, and
// names the rest.
func keepPictured(ws []circadian.Wallpaper, pictures map[string][]byte, who string) []circadian.Wallpaper {
	var keep []circadian.Wallpaper
	for _, w := range ws {
		if _, ok := pictures[w.Image]; ok {
			keep = append(keep, w)
		} else {
			fmt.Printf("%s: the wallpaper %s isn't in the file: left out.\n", who, w.Name)
		}
	}
	return keep
}

// storePictures writes the pictures the wallpapers show into the
// wallpapers folder.
func storePictures(pictures map[string][]byte, ws []circadian.Wallpaper, extra ...string) error {
	names := extra
	for _, w := range ws {
		names = append(names, w.Image)
	}
	dir := enforce.WallpapersDir()
	for _, name := range names {
		raw, ok := pictures[name]
		if !ok {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		_ = os.Chmod(dir, 0o755)
		path := filepath.Join(dir, filepath.Base(name))
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			return fmt.Errorf("failed to store a wallpaper: %w", err)
		}
		_ = os.Chmod(path, 0o644)
	}
	return nil
}

// importModeExtras gives a mode here the file's wallpapers and custom
// actions, each only when it has none (id < 0: a mode a dry run would add).
func importModeExtras(database *db.DB, id int, name string, m modeExport, pictures map[string][]byte, dry bool, say func(string, ...any)) error {
	ws := keepPictured(m.Wallpapers, pictures, "Mode '"+name+"'")
	if len(ws) > 0 && (id < 0 || len(enforce.ModeWallpapers(database, id)) == 0) {
		say("give mode '%s' %d wallpapers", name, len(ws))
		if !dry {
			if err := storePictures(pictures, ws); err != nil {
				return err
			}
			if err := enforce.SetModeWallpapers(database, id, ws); err != nil {
				return err
			}
		}
	}
	if m.Custom != "" {
		have := ""
		if id >= 0 {
			have, _ = database.ModeCustom(name)
		}
		if have == "" {
			say("let mode '%s' %s when it becomes enabled", name, db.DescribeCustom(m.Custom))
			if !dry {
				if err := database.SetModeCustom(name, m.Custom); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// importWallpaperSettings takes the file's wallpaper settings where none
// are set here, and Frozen Chicken's wallpapers where it has none.
func importWallpaperSettings(database *db.DB, in *wallpaperExport, pictures map[string][]byte, dry bool, say func(string, ...any)) error {
	if in == nil {
		return nil
	}
	if raw, _ := database.GetSetting("wallpaper"); raw == "" && in.Settings != nil {
		s := *in.Settings
		if _, ok := pictures[s.Custom]; !ok {
			if s.Default == enforce.WallpaperCustom {
				fmt.Println("The wallpaper default's picture isn't in the file: the latest wallpaper stays the default.")
				s.Default = enforce.WallpaperLatest
			}
			s.Custom, s.CustomName = "", ""
		}
		say("set the wallpaper settings (default: %s)", wallpaperDefaultWords(s))
		if !dry {
			if err := storePictures(pictures, nil, s.Custom); err != nil {
				return err
			}
			if err := enforce.SaveWallpaperSettings(database, s); err != nil {
				return err
			}
		}
	}
	fc := keepPictured(in.FrozenChicken, pictures, db.FrozenChickenName)
	if len(fc) > 0 && len(enforce.ModeWallpapers(database, enforce.FrozenChickenWallpapers)) == 0 {
		say("give Frozen Chicken %d wallpapers", len(fc))
		if !dry {
			if err := storePictures(pictures, fc); err != nil {
				return err
			}
			return enforce.SetModeWallpapers(database, enforce.FrozenChickenWallpapers, fc)
		}
	}
	return nil
}

// projectCustoms lists what the file's custom actions open or run (lists,
// phrase sets, apps and window titles, bypass prevention steps, modes,
// hotkeys of a backup).
func projectCustoms(in projectExport) []string {
	var out []string
	add := func(param string) {
		items, err := db.CustomItems(param)
		if err != nil {
			return
		}
		for _, it := range items {
			if s := it.String(); !slices.Contains(out, s) {
				out = append(out, s)
			}
		}
	}
	ladder := func(n *db.Nepsis) {
		if n == nil {
			return
		}
		for _, step := range n.Steps {
			for _, a := range step.Actions {
				if a.Kind == db.ConsequenceCustom {
					add(a.Param)
				}
			}
		}
	}
	for _, b := range in.Blocks {
		for _, l := range b.Lists {
			for _, a := range l.Actions {
				if a.Name == db.ActionCustom {
					add(a.Param)
				}
			}
		}
		for _, s := range b.PhraseSets {
			for _, a := range s.Actions {
				if a.Name == db.ActionCustom {
					add(a.Param)
				}
			}
		}
		for _, actions := range b.Triggers {
			for _, a := range actions {
				if a.Name == db.ConsequenceCustom {
					add(a.Param)
				}
			}
		}
		ladder(b.Nepsis)
	}
	ladder(in.Nepsis)
	for _, m := range in.Modes {
		if m.Custom != "" {
			add(m.Custom)
		}
	}
	if in.Backup != nil {
		for _, h := range in.Backup.Hotkeys {
			for _, a := range h.Steps() {
				if a.Kind == db.HotkeyCustom {
					add(a.Param)
				}
			}
		}
	}
	return out
}

// confirmCustoms asks before a file's custom actions are imported: they
// open or run things on this computer when their trigger fires.
func confirmCustoms(cmd *cobra.Command, customs []string) error {
	if f := cmd.Flags().Lookup("yes"); f != nil && f.Value.String() == "true" {
		return nil
	}
	list := "  " + strings.Join(customs, "\n  ")
	if !isTerminalInput() {
		return fmt.Errorf("the file's custom actions open or run these on this computer; add --yes to confirm:\n%s", list)
	}
	fmt.Printf("The file's custom actions open or run these on this computer when they fire:\n%s\nType yes to import them: ", list)
	answer, _ := stdinLines.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(answer)) != "yes" {
		return errNotConfirmed
	}
	return nil
}
