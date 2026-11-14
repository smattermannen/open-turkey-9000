// circadian.go — kc9k circadian: the circadian rhythm (§4.10). Settings that
// follow the time of day: three automation clips (backlight, software
// brightness, colour temperature), wallpapers by time, dark mode, and dark
// pages in the browser.
//
// Nothing here is locked or gated: the circadian rhythm is a comfort, not a
// block, and KC9K never puts a level back after the user changed it.
package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/circadian"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

// maxWallpaperSize bounds a wallpaper: photographs are larger than lists.
const maxWallpaperSize = 32 << 20

// circadianNow is the clock (replaceable in tests).
var circadianNow = time.Now

// The answer of `kc9k circadian show --json`.
type circadianView struct {
	On         bool                `json:"on"`
	Minute     float64             `json:"minute"`
	Lanes      []circadianLaneView `json:"lanes"`
	Presets    []circadianPreset   `json:"presets"`
	Wallpapers []circadianPicture  `json:"wallpapers"`
	Dark       circadianDarkView   `json:"dark"`
	DarkPages  bool                `json:"dark_pages"`
	DarkExempt []string            `json:"dark_exempt"`
	Hardware   circadianHardware   `json:"hardware"`
}

type circadianLaneView struct {
	Name        string            `json:"name"`
	Title       string            `json:"title"`
	Unit        string            `json:"unit"`
	Min         float64           `json:"min"`
	Max         float64           `json:"max"`
	Neutral     float64           `json:"neutral"`
	Detail      string            `json:"detail"`
	On          bool              `json:"on"`
	Points      []circadian.Point `json:"points"`
	Value       float64           `json:"value"`
	HasValue    bool              `json:"has_value"`
	PausedUntil int64             `json:"paused_until"`
	Available   bool              `json:"available"`
}

type circadianPreset struct {
	Name   string                       `json:"name"`
	Title  string                       `json:"title"`
	Detail string                       `json:"detail"`
	Lanes  map[string][]circadian.Point `json:"lanes"`
}

type circadianPicture struct {
	Index  int    `json:"index"`
	Image  string `json:"image"`
	Name   string `json:"name"`
	Path   string `json:"path"`
	From   string `json:"from"`
	To     string `json:"to"`
	Active bool   `json:"active"`
}

type circadianDarkView struct {
	Mode      string `json:"mode"`
	LightFrom string `json:"light_from"`
	LightTo   string `json:"light_to"`
	Now       string `json:"now"`
}

type circadianHardware struct {
	Daemon     bool     `json:"daemon"`
	Backlights []string `json:"backlights"`
	External   []string `json:"external"`
	DDCUtil    bool     `json:"ddcutil"`
	Gamma      string   `json:"gamma"`
	NightLight bool     `json:"night_light"`
}

func circadianState(database *db.DB) circadianView {
	now := circadianNow()
	cfg := enforce.CircadianConfig(database)
	minute := circadian.MinuteOfDay(now)
	pauses := enforce.CircadianPauses(database, now)
	status, reported := enforce.ReadCircadianStatus(database)
	view := circadianView{On: cfg.On, Minute: math.Round(minute*100) / 100,
		Wallpapers: []circadianPicture{}, DarkPages: cfg.DarkPages, DarkExempt: cfg.DarkExempt,
		Hardware: circadianHardware{Daemon: reported, Backlights: status.Backlights, External: status.External,
			DDCUtil: status.DDCUtil, Gamma: status.Gamma, NightLight: status.NightLight}}
	if view.Hardware.Backlights == nil {
		view.Hardware.Backlights = []string{}
	}
	if view.Hardware.External == nil {
		view.Hardware.External = []string{}
	}
	if !reported {
		view.Hardware.Gamma = "unknown"
	}
	for _, info := range circadian.Lanes {
		l := cfg.Lanes[info.Name]
		value, ok := l.ValueAt(minute)
		available := !reported || status.Gamma != "unknown"
		if info.Name == circadian.LaneBacklight {
			available = !reported || len(status.Backlights)+len(status.External) > 0
		}
		view.Lanes = append(view.Lanes, circadianLaneView{Name: info.Name, Title: info.Title, Unit: info.Unit,
			Min: info.Min, Max: info.Max, Neutral: info.Neutral, Detail: info.Detail, On: l.On,
			Points: l.Points, Value: math.Round(value*10) / 10, HasValue: ok,
			PausedUntil: enforce.LanePausedUntil(pauses, info.Name), Available: available})
	}
	for _, p := range circadian.Presets {
		view.Presets = append(view.Presets, circadianPreset{p.Name, p.Title, p.Detail, p.Lanes})
	}
	active := cfg.WallpaperAt(minute)
	for i, w := range cfg.Wallpapers {
		view.Wallpapers = append(view.Wallpapers, circadianPicture{Index: i + 1, Image: w.Image, Name: w.Name,
			Path: filepath.Join(enforce.WallpapersDir(), w.Image),
			From: circadian.Clock(w.From), To: circadian.Clock(w.To), Active: i == active})
	}
	view.Dark = circadianDarkView{Mode: cfg.Dark.Mode, LightFrom: circadian.Clock(cfg.Dark.LightFrom),
		LightTo: circadian.Clock(cfg.Dark.LightTo), Now: "system"}
	if dark, decided := cfg.DarkAt(minute); decided {
		view.Dark.Now = map[bool]string{true: "dark", false: "light"}[dark]
	}
	return view
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func printCircadian(view circadianView) {
	fmt.Printf("Circadian rhythm: %s\n", onOff(view.On))
	for _, l := range view.Lanes {
		line := fmt.Sprintf("  %-20s %-3s", l.Title, onOff(l.On))
		if l.HasValue {
			line += fmt.Sprintf("  now %g %s", l.Value, l.Unit)
		}
		if l.PausedUntil > 0 {
			line += "  (paused until " + time.Unix(l.PausedUntil, 0).Format("15:04") + " after a change by hand)"
		}
		if !l.Available {
			line += "  (nothing to drive)"
		}
		fmt.Println(line)
		fmt.Printf("    %s\n", circadian.FormatPoints(l.Points))
	}
	for _, w := range view.Wallpapers {
		mark := ""
		if w.Active {
			mark = "  (now)"
		}
		fmt.Printf("  wallpaper %d: %s  %s-%s%s\n", w.Index, w.Name, w.From, w.To, mark)
	}
	switch view.Dark.Mode {
	case circadian.DarkScheduled:
		fmt.Printf("  dark mode: light between %s and %s (now %s)\n", view.Dark.LightFrom, view.Dark.LightTo, view.Dark.Now)
	case circadian.DarkSystem:
		fmt.Println("  dark mode: left to the system")
	default:
		fmt.Printf("  dark mode: always %s\n", view.Dark.Mode)
	}
	fmt.Printf("  dark pages: %s", onOff(view.DarkPages))
	if len(view.DarkExempt) > 0 {
		fmt.Printf(" (except %s)", strings.Join(view.DarkExempt, ", "))
	}
	fmt.Println()
	if view.Hardware.NightLight {
		fmt.Println("  The desktop's own night light is on and changes the same colours: kc9k circadian nightlight off")
	}
}

// editCircadian changes the settings and stores them.
func editCircadian(database *db.DB, change func(*circadian.Config) error) error {
	cfg := enforce.CircadianConfig(database)
	if err := change(&cfg); err != nil {
		return err
	}
	return enforce.SetCircadianConfig(database, cfg)
}

func laneArg(name string) (circadian.LaneInfo, error) {
	info, ok := circadian.FindLane(strings.ToLower(name))
	if !ok {
		names := []string{}
		for _, l := range circadian.Lanes {
			names = append(names, l.Name)
		}
		return info, fmt.Errorf("unknown clip %q (use %s)", name, strings.Join(names, ", "))
	}
	return info, nil
}

var circadianCmd = &cobra.Command{
	Use:   "circadian",
	Short: "The circadian rhythm: screen, wallpaper and dark mode by time of day",
	Long: `The circadian rhythm follows the time of day:

  three automation clips, 24-hour curves of points:
    backlight   the screen's real brightness, in percent
    software    brightness drawn by the graphics card, in percent
    kelvin      colour temperature (6500 is neutral, lower is warmer)
  wallpapers, each shown between two times
  dark mode: left to the system, always light, always dark, or light
  between two times
  dark pages: light web pages are shown dark while dark mode is on

None of it can be locked, and KC9K never overrides you: change a level by
hand and its clip pauses until its next point (or 'kc9k circadian resume').

  kc9k circadian on
  kc9k circadian preset natural
  kc9k circadian lane kelvin on --points "07:00=6500 19:00=6500~-0.4 22:00=2700"
  kc9k circadian point add backlight 23:00 30
  kc9k circadian wallpaper add ~/Pictures/dawn.jpg 06:00 12:00
  kc9k circadian dark light-between 07:00 20:00
  kc9k circadian dark-pages on
  kc9k circadian exempt add docs.example.com

A point is HH:MM=VALUE, or HH:MM=VALUE~TENSION: the tension (-1 to 1)
bends the curve from that point to the next.

Without arguments, shows the settings.`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		printCircadian(circadianState(database))
		return nil
	}),
}

var circadianShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the circadian rhythm's settings",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		view := circadianState(database)
		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
			data, err := json.MarshalIndent(view, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}
		printCircadian(view)
		return nil
	}),
}

func circadianSwitch(on bool) *cobra.Command {
	return &cobra.Command{
		Use:   onOff(on),
		Short: "Switch the circadian rhythm " + onOff(on),
		Args:  cobra.NoArgs,
		RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
			if err := editCircadian(database, func(c *circadian.Config) error { c.On = on; return nil }); err != nil {
				return err
			}
			fmt.Printf("Circadian rhythm %s.\n", onOff(on))
			return nil
		}),
	}
}

var circadianLaneCmd = &cobra.Command{
	Use:   "lane <backlight|software|kelvin> [on|off] [--points \"HH:MM=VALUE[~TENSION] …\"]",
	Short: "Switch a clip on or off, or give it new points",
	Args:  cobra.RangeArgs(1, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		info, err := laneArg(args[0])
		if err != nil {
			return err
		}
		if len(args) == 2 && args[1] != "on" && args[1] != "off" {
			return fmt.Errorf("usage: kc9k circadian lane %s on|off", info.Name)
		}
		text, _ := cmd.Flags().GetString("points")
		given := cmd.Flags().Changed("points")
		var points []circadian.Point
		if given {
			if points, err = circadian.ParsePoints(info, text); err != nil {
				return err
			}
		}
		if err := editCircadian(database, func(c *circadian.Config) error {
			if len(args) == 2 {
				c.Lanes[info.Name].On = args[1] == "on"
			}
			if given {
				c.Lanes[info.Name].Points = points
			}
			return nil
		}); err != nil {
			return err
		}
		l := enforce.CircadianConfig(database).Lanes[info.Name]
		fmt.Printf("%s %s: %s\n", info.Title, onOff(l.On), circadian.FormatPoints(l.Points))
		return nil
	}),
}

var circadianPointCmd = &cobra.Command{
	Use:   "point",
	Short: "Add or remove one point of a clip",
}

var circadianPointAddCmd = &cobra.Command{
	Use:   "add <clip> <HH:MM> <value> [--tension T]",
	Short: "Add a point (or change the one at that time)",
	Args:  cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		info, err := laneArg(args[0])
		if err != nil {
			return err
		}
		text := args[1] + "=" + args[2]
		if cmd.Flags().Changed("tension") {
			tension, _ := cmd.Flags().GetFloat64("tension")
			text += "~" + strconv.FormatFloat(tension, 'f', -1, 64)
		}
		points, err := circadian.ParsePoints(info, text)
		if err != nil {
			return err
		}
		if len(points) != 1 {
			return fmt.Errorf("give one point: a time and a value")
		}
		if err := editCircadian(database, func(c *circadian.Config) error {
			l := c.Lanes[info.Name]
			l.Points = circadian.CleanPoints(info, append(l.Points, points[0]))
			return nil
		}); err != nil {
			return err
		}
		fmt.Printf("%s: %s\n", info.Title, circadian.FormatPoints(enforce.CircadianConfig(database).Lanes[info.Name].Points))
		return nil
	}),
}

var circadianPointRemoveCmd = &cobra.Command{
	Use:   "remove <clip> <HH:MM>",
	Short: "Remove the point at that time",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		info, err := laneArg(args[0])
		if err != nil {
			return err
		}
		minute, err := circadian.ParseClock(args[1])
		if err != nil {
			return err
		}
		minute %= circadian.DayMinutes
		if err := editCircadian(database, func(c *circadian.Config) error {
			l := c.Lanes[info.Name]
			kept := slices.DeleteFunc(slices.Clone(l.Points), func(p circadian.Point) bool { return p.Minute == minute })
			if len(kept) == len(l.Points) {
				return fmt.Errorf("%s has no point at %s", info.Title, circadian.Clock(minute))
			}
			l.Points = kept
			return nil
		}); err != nil {
			return err
		}
		fmt.Printf("%s: %s\n", info.Title, circadian.FormatPoints(enforce.CircadianConfig(database).Lanes[info.Name].Points))
		return nil
	}),
}

var circadianPresetCmd = &cobra.Command{
	Use:   "preset [name]",
	Short: "Replace the three clips' points with a ready-made set (without a name: list them)",
	Args:  cobra.MaximumNArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 0 {
			for _, p := range circadian.Presets {
				fmt.Printf("%-12s %s. %s\n", p.Name, p.Title, p.Detail)
			}
			return nil
		}
		preset, ok := circadian.FindPreset(strings.ToLower(args[0]))
		if !ok {
			return fmt.Errorf("unknown preset %q (kc9k circadian preset lists them)", args[0])
		}
		if err := editCircadian(database, func(c *circadian.Config) error {
			for _, info := range circadian.Lanes {
				c.Lanes[info.Name].Points = slices.Clone(preset.Lanes[info.Name])
			}
			return nil
		}); err != nil {
			return err
		}
		fmt.Printf("The clips now follow %q.\n", preset.Title)
		return nil
	}),
}

var circadianPreviewCmd = &cobra.Command{
	Use:   "preview <clip> <value> | preview <clip> --end",
	Short: "Show a value on the screen for a few seconds, to try it out",
	Args:  cobra.RangeArgs(1, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		info, err := laneArg(args[0])
		if err != nil {
			return err
		}
		if end, _ := cmd.Flags().GetBool("end"); end {
			return enforce.PreviewCircadian(database, info.Name, 0, true, circadianNow())
		}
		if len(args) != 2 {
			return fmt.Errorf("usage: kc9k circadian preview %s <value>", info.Name)
		}
		value, err := strconv.ParseFloat(args[1], 64)
		if err != nil || math.IsNaN(value) {
			return fmt.Errorf("%q isn't a number", args[1])
		}
		return enforce.PreviewCircadian(database, info.Name, value, false, circadianNow())
	}),
}

var circadianResumeCmd = &cobra.Command{
	Use:   "resume [clip]",
	Short: "End the pause after a change by hand (of one clip, or of all)",
	Args:  cobra.MaximumNArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		lane := ""
		if len(args) == 1 {
			info, err := laneArg(args[0])
			if err != nil {
				return err
			}
			lane = info.Name
		}
		if err := enforce.ResumeCircadian(database, lane, circadianNow()); err != nil {
			return err
		}
		fmt.Println("Resumed.")
		return nil
	}),
}

var circadianWallpaperCmd = &cobra.Command{
	Use:   "wallpaper",
	Short: "Wallpapers by time of day",
}

// storeWallpaper copies a picture into the wallpapers folder under its
// SHA-256 and type, readable by every desktop user, and returns that name.
func storeWallpaper(path string) (string, error) {
	data, err := readAsCallerUpTo(path, maxWallpaperSize)
	if err != nil {
		return "", err
	}
	ext := map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/webp": "webp"}[http.DetectContentType(data)]
	if ext == "" {
		return "", fmt.Errorf("%s isn't a PNG, JPEG or WebP picture", path)
	}
	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:]) + "." + ext
	dir := enforce.WallpapersDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return "", fmt.Errorf("failed to store the wallpaper: %w", err)
	}
	// The umask may have narrowed them; the desktop reads as the user.
	_ = os.Chmod(dir, 0o755)
	_ = os.Chmod(filepath.Join(dir, name), 0o644)
	return name, nil
}

// wallpaperTimes says when a wallpaper shows: two equal times are the whole day.
func wallpaperTimes(from, to int) string {
	if from == to {
		return "all day"
	}
	return "from " + circadian.Clock(from) + " to " + circadian.Clock(to)
}

func clockPair(from, to string) (int, int, error) {
	f, err := circadian.ParseClock(from)
	if err != nil {
		return 0, 0, err
	}
	t, err := circadian.ParseClock(to)
	if err != nil {
		return 0, 0, err
	}
	return f % circadian.DayMinutes, t % circadian.DayMinutes, nil
}

var circadianWallpaperAddCmd = &cobra.Command{
	Use:   "add <image> <from HH:MM> <to HH:MM>",
	Short: "Show a picture as the wallpaper between two times",
	Args:  cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		from, to, err := clockPair(args[1], args[2])
		if err != nil {
			return err
		}
		image, err := storeWallpaper(args[0])
		if err != nil {
			return err
		}
		if err := editCircadian(database, func(c *circadian.Config) error {
			c.Wallpapers = append(c.Wallpapers, circadian.Wallpaper{Image: image, Name: filepath.Base(args[0]), From: from, To: to})
			return nil
		}); err != nil {
			return err
		}
		fmt.Printf("%s is the wallpaper %s.\n", filepath.Base(args[0]), wallpaperTimes(from, to))
		return nil
	}),
}

// wallpaperIndex reads a wallpaper's number as `show` prints it.
func wallpaperIndex(c *circadian.Config, text string) (int, error) {
	n, err := strconv.Atoi(text)
	if err != nil || n < 1 || n > len(c.Wallpapers) {
		return 0, fmt.Errorf("there is no wallpaper %q (kc9k circadian shows them with their numbers)", text)
	}
	return n - 1, nil
}

var circadianWallpaperTimeCmd = &cobra.Command{
	Use:   "time <number> <from HH:MM> <to HH:MM>",
	Short: "Change the times of a wallpaper",
	Args:  cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		from, to, err := clockPair(args[1], args[2])
		if err != nil {
			return err
		}
		return editCircadian(database, func(c *circadian.Config) error {
			i, err := wallpaperIndex(c, args[0])
			if err != nil {
				return err
			}
			c.Wallpapers[i].From, c.Wallpapers[i].To = from, to
			fmt.Printf("%s is the wallpaper %s.\n", c.Wallpapers[i].Name, wallpaperTimes(from, to))
			return nil
		})
	}),
}

var circadianWallpaperRemoveCmd = &cobra.Command{
	Use:   "remove <number>",
	Short: "Remove a wallpaper",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		var gone circadian.Wallpaper
		var left []circadian.Wallpaper
		if err := editCircadian(database, func(c *circadian.Config) error {
			i, err := wallpaperIndex(c, args[0])
			if err != nil {
				return err
			}
			gone = c.Wallpapers[i]
			c.Wallpapers = slices.Delete(c.Wallpapers, i, i+1)
			left = c.Wallpapers
			return nil
		}); err != nil {
			return err
		}
		// The file goes when no other time uses it. It may still be on the
		// desktop: the desktop keeps showing what it has loaded.
		if !slices.ContainsFunc(left, func(w circadian.Wallpaper) bool { return w.Image == gone.Image }) &&
			gone.Image == filepath.Base(gone.Image) && gone.Image != "" {
			_ = os.Remove(filepath.Join(enforce.WallpapersDir(), gone.Image))
		}
		fmt.Printf("Removed %s.\n", gone.Name)
		return nil
	}),
}

var circadianDarkCmd = &cobra.Command{
	Use:   "dark system|light|dark|light-between <HH:MM> <HH:MM>",
	Short: "Dark mode: left to the system, always light, always dark, or light between two times",
	Args:  cobra.RangeArgs(1, 3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		return editCircadian(database, func(c *circadian.Config) error {
			switch {
			case len(args) == 1 && slices.Contains([]string{circadian.DarkSystem, circadian.DarkLight, circadian.DarkDark}, args[0]):
				c.Dark.Mode = args[0]
				fmt.Printf("Dark mode: %s.\n", map[string]string{circadian.DarkSystem: "left to the system",
					circadian.DarkLight: "always light", circadian.DarkDark: "always dark"}[args[0]])
			case len(args) == 3 && args[0] == "light-between":
				from, to, err := clockPair(args[1], args[2])
				if err != nil {
					return err
				}
				if from == to {
					return fmt.Errorf("the two times are the same: use 'kc9k circadian dark light' for always light")
				}
				c.Dark = circadian.Dark{Mode: circadian.DarkScheduled, LightFrom: from, LightTo: to}
				fmt.Printf("Dark mode: light between %s and %s, dark otherwise.\n", circadian.Clock(from), circadian.Clock(to))
			default:
				return fmt.Errorf("usage: kc9k circadian dark system|light|dark|light-between <HH:MM> <HH:MM>")
			}
			return nil
		})
	}),
}

var circadianDarkPagesCmd = &cobra.Command{
	Use:   "dark-pages on|off",
	Short: "Show light web pages dark while dark mode is on",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if args[0] != "on" && args[0] != "off" {
			return fmt.Errorf("usage: kc9k circadian dark-pages on|off")
		}
		if err := editCircadian(database, func(c *circadian.Config) error { c.DarkPages = args[0] == "on"; return nil }); err != nil {
			return err
		}
		fmt.Printf("Dark pages %s.\n", args[0])
		return nil
	}),
}

var circadianExemptCmd = &cobra.Command{
	Use:   "exempt add|remove <site>…",
	Short: "Sites that dark pages leave as they are",
	Args:  cobra.MinimumNArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if args[0] != "add" && args[0] != "remove" {
			return fmt.Errorf("usage: kc9k circadian exempt add|remove <site>…")
		}
		var sites []string
		for _, arg := range args[1:] {
			site := blocker.NormalizeDomain(arg)
			if !blocker.IsValidDomain(site) {
				return fmt.Errorf("%q isn't a site", arg)
			}
			sites = append(sites, site)
		}
		return editCircadian(database, func(c *circadian.Config) error {
			for _, site := range sites {
				has := slices.Contains(c.DarkExempt, site)
				switch {
				case args[0] == "add" && !has:
					c.DarkExempt = append(c.DarkExempt, site)
				case args[0] == "remove" && !has:
					return fmt.Errorf("%s isn't exempt", site)
				case args[0] == "remove":
					c.DarkExempt = slices.DeleteFunc(c.DarkExempt, func(s string) bool { return s == site })
				}
			}
			slices.Sort(c.DarkExempt)
			fmt.Printf("Dark pages leave alone: %s\n", strings.Join(c.DarkExempt, ", "))
			return nil
		})
	}),
}

// turnOffNightLight is replaceable in tests.
var turnOffNightLight = blocker.TurnOffNightLight

var circadianNightLightCmd = &cobra.Command{
	Use:   "nightlight off",
	Short: "Switch the desktop's own night light off (it changes the same colours as the colour temperature clip)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if args[0] != "off" {
			return fmt.Errorf("usage: kc9k circadian nightlight off")
		}
		if os.Getenv("KC9K_DB") != "" {
			return nil // a test's database never reaches the desktop
		}
		if err := turnOffNightLight(); err != nil {
			return err
		}
		fmt.Println("The desktop's night light is off.")
		return nil
	}),
}

func init() {
	circadianShowCmd.Flags().Bool("json", false, "print everything as JSON, for the interface")
	circadianLaneCmd.Flags().String("points", "", "the clip's points: \"HH:MM=VALUE[~TENSION] …\"")
	circadianPointAddCmd.Flags().Float64("tension", 0, "bend of the curve to the next point, -1 to 1")
	circadianPreviewCmd.Flags().Bool("end", false, "the preview is over")
	circadianPointCmd.AddCommand(circadianPointAddCmd, circadianPointRemoveCmd)
	circadianWallpaperCmd.AddCommand(circadianWallpaperAddCmd, circadianWallpaperTimeCmd, circadianWallpaperRemoveCmd)
	circadianCmd.AddCommand(circadianShowCmd, circadianSwitch(true), circadianSwitch(false), circadianLaneCmd,
		circadianPointCmd, circadianPresetCmd, circadianPreviewCmd, circadianResumeCmd, circadianWallpaperCmd,
		circadianDarkCmd, circadianDarkPagesCmd, circadianExemptCmd, circadianNightLightCmd)
	rootCmd.AddCommand(circadianCmd)
}
