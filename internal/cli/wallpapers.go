// wallpapers.go — kc9k mode wallpaper and kc9k wallpaper (§4.10, R-MODE-5;
// owner 2026-11-17): a mode's wallpapers, laid out like circadian rhythm's
// (a picture between two times), which of the two override the other (one
// setting, shown in both places), and the wallpaper default for when
// neither has one and while the place isn't known (no Wi-Fi). The engine
// is enforce/wallpapers.go.
//
// Nothing here is locked or gated: a wallpaper protects nothing.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/smattermannen/kc9k/internal/circadian"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

// wallpaperPictures describes wallpapers for the interface, the one due at
// minute marked.
func wallpaperPictures(ws []circadian.Wallpaper, minute float64) []circadianPicture {
	out := []circadianPicture{}
	active := circadian.Config{Wallpapers: ws}.WallpaperAt(minute)
	for i, w := range ws {
		out = append(out, circadianPicture{Index: i + 1, Image: w.Image, Name: w.Name,
			Path: filepath.Join(enforce.WallpapersDir(), w.Image),
			From: circadian.Clock(w.From), To: circadian.Clock(w.To), Active: i == active})
	}
	return out
}

// forgetWallpaper deletes a stored picture nothing uses any more. It may
// still be on the desktop: the desktop keeps what it has loaded.
func forgetWallpaper(database *db.DB, image string) {
	if image != "" && image == filepath.Base(image) && !enforce.WallpaperInUse(database, image) {
		_ = os.Remove(filepath.Join(enforce.WallpapersDir(), image))
	}
}

// modeWallpaperTarget returns the id a mode's wallpapers are kept under,
// and its name; Frozen Chicken has its own.
func modeWallpaperTarget(database *db.DB, name string) (int, string, error) {
	if isFrozenChicken(name) {
		return enforce.FrozenChickenWallpapers, db.FrozenChickenName, nil
	}
	t, err := resolveTarget(database, name, db.KindMode)
	if err != nil {
		return 0, "", err
	}
	return t.Mode.ID, t.Mode.Name, nil
}

// pictureNumber reads a wallpaper's number as the list prints it.
func pictureNumber(ws []circadian.Wallpaper, text, list string) (int, error) {
	n, err := strconv.Atoi(text)
	if err != nil || n < 1 || n > len(ws) {
		return 0, fmt.Errorf("there is no wallpaper %q (%s lists them with their numbers)", text, list)
	}
	return n - 1, nil
}

var modeWallpaperCmd = &cobra.Command{
	Use:   "wallpaper <mode> [add <image> <from HH:MM> <to HH:MM> | time <number> <from> <to> | remove <number>]",
	Short: "A mode's wallpapers: each shown between two times while the mode is enabled",
	Long: `A mode's wallpapers (R-MODE-5): while the mode is enabled, each picture
is the wallpaper between its two times, laid out like circadian rhythm's
(two equal times: all day; past midnight it wraps round; where times
overlap, the one that started last shows). They override circadian
rhythm's wallpapers unless 'kc9k wallpaper priority circadian' says the
other way round. Of several enabled modes, the one enabled last wins.

A mode switched by places shows its wallpapers only once KC9K knows where
the computer is; until then (no Wi-Fi, the first seconds after boot or
waking) the wallpaper default shows: see 'kc9k wallpaper default'.

  kc9k mode wallpaper School                             its wallpapers
  kc9k mode wallpaper School add ~/Pictures/school.jpg 07:00 07:00
  kc9k mode wallpaper School time 1 08:00 16:00
  kc9k mode wallpaper School remove 1

Monk mode and Frozen Chicken take wallpapers too.`,
	Args: cobra.RangeArgs(1, 5),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		id, name, err := modeWallpaperTarget(database, args[0])
		if err != nil {
			return err
		}
		ws := enforce.ModeWallpapers(database, id)
		list := "'kc9k mode wallpaper " + name + "'"
		if len(args) == 1 {
			if len(ws) == 0 {
				fmt.Printf("Mode '%s' has no wallpapers.\n", name)
				return nil
			}
			fmt.Printf("Wallpapers of '%s' (while it is enabled):\n", name)
			for _, p := range wallpaperPictures(ws, circadian.MinuteOfDay(circadianNow())) {
				mark := ""
				if p.Active {
					mark = "  (due now)"
				}
				fmt.Printf("  %d: %s  %s%s\n", p.Index, p.Name, wallpaperTimes(mustClock(p.From), mustClock(p.To)), mark)
			}
			return nil
		}
		switch {
		case args[1] == "add" && len(args) == 5:
			from, to, err := clockPair(args[3], args[4])
			if err != nil {
				return err
			}
			image, err := storeWallpaper(args[2])
			if err != nil {
				return err
			}
			ws = append(ws, circadian.Wallpaper{Image: image, Name: filepath.Base(args[2]), From: from, To: to})
			if err := enforce.SetModeWallpapers(database, id, ws); err != nil {
				return err
			}
			fmt.Printf("%s is the wallpaper of '%s' %s.\n", filepath.Base(args[2]), name, wallpaperTimes(from, to))
		case args[1] == "time" && len(args) == 5:
			i, err := pictureNumber(ws, args[2], list)
			if err != nil {
				return err
			}
			from, to, err := clockPair(args[3], args[4])
			if err != nil {
				return err
			}
			ws[i].From, ws[i].To = from, to
			if err := enforce.SetModeWallpapers(database, id, ws); err != nil {
				return err
			}
			fmt.Printf("%s is the wallpaper of '%s' %s.\n", ws[i].Name, name, wallpaperTimes(from, to))
		case args[1] == "remove" && len(args) == 3:
			i, err := pictureNumber(ws, args[2], list)
			if err != nil {
				return err
			}
			gone := ws[i]
			if err := enforce.SetModeWallpapers(database, id, slices.Delete(ws, i, i+1)); err != nil {
				return err
			}
			forgetWallpaper(database, gone.Image)
			fmt.Printf("Removed %s.\n", gone.Name)
		default:
			return fmt.Errorf("usage: kc9k mode wallpaper <mode> [add <image> <from> <to> | time <number> <from> <to> | remove <number>]")
		}
		return nil
	}),
}

// mustClock reads a time circadian.Clock wrote.
func mustClock(text string) int {
	m, _ := circadian.ParseClock(text)
	return m % circadian.DayMinutes
}

// wallpaperDefaultWords says what the default shows.
func wallpaperDefaultWords(s enforce.WallpaperSettings) string {
	switch s.Default {
	case enforce.WallpaperCircadian:
		return "circadian rhythm's wallpaper for the time of day (even with circadian rhythm off)"
	case enforce.WallpaperCustom:
		return s.CustomName
	}
	return "the latest wallpaper (what shows stays: at a new place, maybe the last place's)"
}

var wallpaperCmd = &cobra.Command{
	Use:   "wallpaper",
	Short: "Wallpapers: whether modes or circadian rhythm win, and the wallpaper default",
	Long: `Modes (kc9k mode wallpaper) and circadian rhythm (kc9k circadian
wallpaper) both show wallpapers by time. When both have one, modes win
unless circadian rhythm is set to override them; in the interface this is
"Override circadian rhythm wallpapers" on a mode and "Override modes" on
the circadian rhythm page, the same setting seen from each side.

The wallpaper default shows when neither has a wallpaper, and while the
place isn't known (no Wi-Fi, the first seconds after boot or waking) for
modes switched by places: the latest wallpaper (it stays as it is, which at
a new place can be the last place's), circadian rhythm's for the time of
day, or a picture of your own. With the default a picture or circadian
rhythm, it is also put on before the computer sleeps, so a laptop closed at
home and opened at school starts with it, not with home's.

  kc9k wallpaper                                 the settings and what shows now
  kc9k wallpaper priority modes|circadian        which win
  kc9k wallpaper default latest|circadian        the default
  kc9k wallpaper default custom ~/Pictures/plain.png`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		s := enforce.LoadWallpaperSettings(database)
		if s.CircadianFirst {
			fmt.Println("Circadian rhythm's wallpapers override modes'.")
		} else {
			fmt.Println("Modes' wallpapers override circadian rhythm's.")
		}
		fmt.Println("Default: " + wallpaperDefaultWords(s))
		image, why := enforce.WallpaperNow(database, time.Now())
		if image == "" {
			fmt.Println("Now: " + why + ".")
		} else {
			fmt.Printf("Now: %s, from %s.\n", wallpaperName(database, image), why)
		}
		return nil
	}),
}

// wallpaperName is a stored picture's file name as it was added.
func wallpaperName(database *db.DB, image string) string {
	all := enforce.CircadianConfig(database).Wallpapers
	all = append(all, enforce.ModeWallpapers(database, enforce.FrozenChickenWallpapers)...)
	if modes, err := database.Modes(); err == nil {
		for _, m := range modes {
			all = append(all, enforce.ModeWallpapers(database, m.ID)...)
		}
	}
	for _, w := range all {
		if w.Image == image {
			return w.Name
		}
	}
	if s := enforce.LoadWallpaperSettings(database); s.Custom == image {
		return s.CustomName
	}
	return image
}

var wallpaperPriorityCmd = &cobra.Command{
	Use:   "priority modes|circadian",
	Short: "Which wallpapers win when a mode and circadian rhythm both have one",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if args[0] != "modes" && args[0] != "circadian" {
			return fmt.Errorf("usage: kc9k wallpaper priority modes|circadian")
		}
		s := enforce.LoadWallpaperSettings(database)
		s.CircadianFirst = args[0] == "circadian"
		if err := enforce.SaveWallpaperSettings(database, s); err != nil {
			return err
		}
		if s.CircadianFirst {
			fmt.Println("Circadian rhythm's wallpapers override modes' now.")
		} else {
			fmt.Println("Modes' wallpapers override circadian rhythm's now.")
		}
		return nil
	}),
}

var wallpaperDefaultCmd = &cobra.Command{
	Use:   "default latest|circadian|custom [<image>]",
	Short: "What shows when no mode or circadian rhythm wallpaper does, and while the place isn't known",
	Args:  cobra.RangeArgs(1, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		s := enforce.LoadWallpaperSettings(database)
		old := s.Custom
		switch {
		case (args[0] == enforce.WallpaperLatest || args[0] == enforce.WallpaperCircadian) && len(args) == 1:
			s.Default = args[0] // the custom picture is kept for when Custom is chosen again
		case args[0] == enforce.WallpaperCustom && len(args) == 2:
			image, err := storeWallpaper(args[1])
			if err != nil {
				return err
			}
			s.Default, s.Custom, s.CustomName = args[0], image, filepath.Base(args[1])
		case args[0] == enforce.WallpaperCustom && len(args) == 1 && s.Custom != "":
			s.Default = args[0] // the picture chosen before
		default:
			return fmt.Errorf("usage: kc9k wallpaper default latest|circadian|custom <image>")
		}
		if err := enforce.SaveWallpaperSettings(database, s); err != nil {
			return err
		}
		if old != "" && old != s.Custom {
			forgetWallpaper(database, old)
		}
		fmt.Println("Wallpaper default: " + wallpaperDefaultWords(s) + ".")
		return nil
	}),
}

// wallpaperBeforeSleepCmd is run by the sleep hook (blocker.SyncSleepHook).
var wallpaperBeforeSleepCmd = &cobra.Command{
	Use:    "before-sleep",
	Short:  "Internal: put the wallpaper default on before the computer sleeps",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		return enforce.WallpaperBeforeSleep(database, time.Now())
	}),
}

func init() {
	modeCmd.AddCommand(modeWallpaperCmd)
	wallpaperCmd.AddCommand(wallpaperPriorityCmd, wallpaperDefaultCmd, wallpaperBeforeSleepCmd)
	rootCmd.AddCommand(wallpaperCmd)
}
