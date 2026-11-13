// project.go — kc9k project: the whole setup in one file, to move it to
// another computer or share it.
//
//	kc9k project export > my-setup.json
//	kc9k project import my-setup.json [--dry-run]
//
// A project holds every block (lists, apps, window titles, schedules, quota,
// Pomodoro, breaks, bypass detection), the clusters, the browser options
// and behaviour options, the global bypass detection and the scheduled
// Frozen Chicken windows, with the pictures the lists show. Locks,
// passwords, statistics and what is running right now are never part of it.
//
// Importing only adds: a block or cluster that already exists is left as it
// is, browser options are only ever turned on, and bypass detection is only
// replaced by a stricter one. So an import can't weaken anything that is
// held.
package cli

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/smattermannen/kc9k/internal/freeze"
	"github.com/smattermannen/kc9k/internal/quota"
	"github.com/smattermannen/kc9k/internal/schedule"
	"github.com/spf13/cobra"
)

const projectVersion = 1

type windowExport struct {
	Days  string `json:"days"`
	From  string `json:"from"`
	To    string `json:"to"`
	Allow bool   `json:"allow,omitempty"`
	Mode  string `json:"mode,omitempty"` // Frozen Chicken windows only
}

type pomodoroExport struct {
	Work  int64 `json:"work_seconds"`
	Break int64 `json:"break_seconds"`
	Long  int64 `json:"long_seconds,omitempty"`
	Every int64 `json:"every,omitempty"`
}

type breaksExport struct {
	Allowance int64 `json:"allowance_seconds,omitempty"`
	Per       int64 `json:"per_seconds,omitempty"`
	Relock    int64 `json:"relock_seconds,omitempty"`
}

// blockExtras is what a project adds to a block's export.
type blockExtras struct {
	Schedules []windowExport  `json:"schedules,omitempty"`
	Quota     int64           `json:"quota_seconds,omitempty"`
	Pomodoro  *pomodoroExport `json:"pomodoro,omitempty"`
	Breaks    *breaksExport   `json:"breaks,omitempty"`
}

type projectBlock struct {
	blockExport
	blockExtras
}

type clusterExport struct {
	Name    string   `json:"name"`
	Where   []string `json:"where"`
	Entries []string `json:"entries"`
}

type projectExport struct {
	KC9KProject int               `json:"kc9k_project"`
	Blocks      []projectBlock    `json:"blocks"`
	Clusters    []clusterExport   `json:"clusters,omitempty"`
	Browser     map[string]bool   `json:"browser_options,omitempty"`
	Options     map[string]bool   `json:"options,omitempty"`
	Nepsis      *db.Nepsis        `json:"bypass_detection,omitempty"`
	Freezes     []windowExport    `json:"frozen_chicken_schedules,omitempty"`
	Images      map[string]string `json:"images,omitempty"` // name -> base64
}

func exportWindow(w schedule.Window) windowExport {
	return windowExport{Days: schedule.FormatDays(w.Days), From: schedule.FormatMinutes(w.Start),
		To: schedule.FormatMinutes(w.End), Allow: w.Allow}
}

func (w windowExport) window() (schedule.Window, error) {
	days, err := schedule.ParseDays(w.Days)
	if err != nil {
		return schedule.Window{}, err
	}
	from, err := schedule.ParseTimeOfDay(w.From)
	if err != nil {
		return schedule.Window{}, err
	}
	to, err := schedule.ParseEndTime(w.To)
	if err != nil {
		return schedule.Window{}, err
	}
	if to <= from {
		return schedule.Window{}, fmt.Errorf("a window from %s to %s is empty", w.From, w.To)
	}
	return schedule.Window{Days: days, Start: from, End: to, Allow: w.Allow}, nil
}

// exportBlock collects one block, with the images its lists use.
func exportBlock(database *db.DB, name string, images map[string]string) (projectBlock, error) {
	detail, err := database.GetBlock(name)
	if err != nil {
		return projectBlock{}, err
	}
	out := projectBlock{blockExport: blockExport{Name: detail.Name, Apps: detail.Apps, Titles: detail.Titles, Lists: []listExport{}}}
	if out.Apps == nil {
		out.Apps = []string{}
	}
	if n, err := database.BlockNepsis(name); err != nil {
		return out, err
	} else if n.On() {
		out.Nepsis = &n
	}
	for _, l := range detail.Lists {
		out.Lists = append(out.Lists, listExport{Name: l.Name, Kind: l.Kind, Where: l.Where.List(), Actions: l.Actions, Entries: l.Entries})
		for _, a := range l.Actions {
			if a.Name == db.ActionImage && db.IsImageName(a.Param) && images != nil {
				if data, err := os.ReadFile(filepath.Join(db.ImagesDir(), a.Param)); err == nil {
					images[a.Param] = base64.StdEncoding.EncodeToString(data)
				}
			}
		}
	}
	schedules, err := database.ListSchedules(name)
	if err != nil {
		return out, err
	}
	for _, s := range schedules {
		out.Schedules = append(out.Schedules, exportWindow(s.Window()))
	}
	if q, err := database.GetQuota(name); err != nil {
		return out, err
	} else if q != nil {
		out.Quota = q.Limit
	}
	if p, err := database.GetPomodoro(name); err != nil {
		return out, err
	} else if p != nil {
		out.Pomodoro = &pomodoroExport{Work: p.On, Break: p.Off, Long: p.Long, Every: p.Every}
	}
	if k, err := database.GetBreaks(name); err != nil {
		return out, err
	} else if k.Allowance > 0 || k.Relock > 0 {
		out.Breaks = &breaksExport{Allowance: k.Allowance, Per: k.Per, Relock: k.Relock}
	}
	return out, nil
}

// checkedBlock is a block from a file, validated and ready to write.
type checkedBlock struct {
	in      projectBlock
	lists   db.Lists
	windows []schedule.Window
}

// checkBlock validates a block from a file without writing anything.
func checkBlock(in projectBlock) (checkedBlock, error) {
	out := checkedBlock{in: in}
	if in.Name == "" {
		return out, fmt.Errorf("a block has no name")
	}
	exported := in.Lists
	var err error
	if exported == nil {
		if exported, err = in.legacyLists(); err != nil {
			return out, err
		}
	}
	if out.lists, err = toLists(exported); err != nil {
		return out, err
	}
	for _, app := range in.Apps {
		if err := blocker.ValidAppEntry(app); err != nil {
			return out, err
		}
	}
	if err := checkTitleKeywords(in.Titles); err != nil {
		return out, err
	}
	if in.Nepsis != nil && in.Nepsis.On() {
		if err := in.Nepsis.Check(); err != nil {
			return out, fmt.Errorf("bypass detection: %w", err)
		}
	}
	for _, w := range in.Schedules {
		win, err := w.window()
		if err != nil {
			return out, fmt.Errorf("schedule: %w", err)
		}
		out.windows = append(out.windows, win)
	}
	if in.Quota < 0 || in.Quota > int64(quota.MaxLimit.Seconds()) {
		return out, fmt.Errorf("quota out of range")
	}
	if p := in.Pomodoro; p != nil && (p.Work < int64(minPomodoroPhase.Seconds()) || p.Break < int64(minPomodoroPhase.Seconds()) ||
		p.Long < 0 || p.Every < 0 || p.Every > 20) {
		return out, fmt.Errorf("Pomodoro times out of range")
	}
	if k := in.Breaks; k != nil && (k.Allowance < 0 || k.Allowance > db.MaxBreakSeconds || k.Per < 0 || k.Per > db.MaxBreakPer ||
		k.Relock < 0 || k.Relock > db.MaxBreakSeconds || (k.Allowance > 0 && k.Allowance >= k.Per)) {
		return out, fmt.Errorf("break times out of range")
	}
	return out, nil
}

// createBlock writes a validated block as a new block.
func createBlock(database *db.DB, c checkedBlock) error {
	in := c.in
	if err := database.CreateBlock(in.Name); err != nil {
		return err
	}
	if err := database.SetLists(in.Name, c.lists); err != nil {
		return err
	}
	if len(in.Apps) > 0 {
		if err := database.AddApps(in.Name, in.Apps); err != nil {
			return err
		}
	}
	if len(in.Titles) > 0 {
		if err := database.AddTitles(in.Name, in.Titles); err != nil {
			return err
		}
	}
	if in.Nepsis != nil {
		if err := database.SetBlockNepsis(in.Name, *in.Nepsis); err != nil {
			return err
		}
	}
	for _, w := range c.windows {
		if err := database.AddSchedule(in.Name, w); err != nil {
			return err
		}
	}
	now := time.Now()
	if in.Quota > 0 {
		if err := database.SaveQuota(in.Name, quota.New(in.Quota, now.Unix(), quota.DayOf(now))); err != nil {
			return err
		}
	}
	if p := in.Pomodoro; p != nil {
		trusted, err := database.TrustedNow(now)
		if err != nil {
			return err
		}
		if err := database.SetPomodoro(in.Name, db.Pomodoro{On: p.Work, Off: p.Break, Long: p.Long, Every: p.Every, Anchor: trusted}); err != nil {
			return err
		}
	}
	if k := in.Breaks; k != nil {
		if err := database.SetBreaks(in.Name, db.Breaks{Allowance: k.Allowance, Per: k.Per, Relock: k.Relock}); err != nil {
			return err
		}
	}
	return nil
}

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "The whole setup in one file: export it, or import one",
}

var projectExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Print the whole setup as JSON (redirect to a file to save it)",
	Long: `Prints every block with its lists, apps, window titles, schedules, quota,
Pomodoro, breaks and bypass detection, the clusters, browser options,
behaviour options, the global bypass detection and the scheduled Frozen
Chicken windows. Locks, passwords and statistics are left out.`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		out := projectExport{KC9KProject: projectVersion, Blocks: []projectBlock{}, Browser: map[string]bool{},
			Options: map[string]bool{}, Images: map[string]string{}}
		blocks, err := database.ListBlocks()
		if err != nil {
			return err
		}
		for _, b := range blocks {
			if b.Name == enforce.MonkBlock {
				continue // made by itself where it is needed
			}
			pb, err := exportBlock(database, b.Name, out.Images)
			if err != nil {
				return err
			}
			out.Blocks = append(out.Blocks, pb)
		}
		clusters, err := database.ListClusters()
		if err != nil {
			return err
		}
		for _, c := range clusters {
			full, err := database.GetCluster(c.Name)
			if err != nil {
				return err
			}
			out.Clusters = append(out.Clusters, clusterExport{Name: full.Name, Where: full.Where.List(), Entries: full.Entries})
		}
		for _, name := range enforce.BrowserSettings {
			out.Browser[name] = enforce.BrowserOn(database, name)
		}
		for _, o := range enforce.Options {
			out.Options[o.Name] = enforce.OptionOn(database, o.Name)
		}
		if n, err := database.GlobalNepsis(); err != nil {
			return err
		} else if n.On() {
			out.Nepsis = &n
		}
		freezes, err := database.ListFreezeSchedules()
		if err != nil {
			return err
		}
		for _, f := range freezes {
			w := exportWindow(f.Window)
			w.Mode = f.Mode
			out.Freezes = append(out.Freezes, w)
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}),
}

var projectImportCmd = &cobra.Command{
	Use:   "import [file]",
	Short: "Add what a project file holds to this computer's setup",
	Long: `Adds the blocks and clusters of a project file that don't exist here yet,
turns on the browser options it has on, adds its scheduled Frozen Chicken
windows, and takes its global bypass detection if that is at least as strict
as the one here. Nothing that exists is changed or removed, so importing
never weakens a block. Imported blocks are not started.

With --dry-run, says what would happen and changes nothing.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dry, _ := cmd.Flags().GetBool("dry-run")
		data, err := readAsCaller(args[0])
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", args[0], err)
		}
		var in projectExport
		if err := json.Unmarshal(data, &in); err != nil {
			return fmt.Errorf("failed to parse %s: %w", args[0], err)
		}
		if in.KC9KProject == 0 {
			return fmt.Errorf("%s is not a project file (one block is imported with 'kc9k block import')", args[0])
		}
		if in.KC9KProject > projectVersion {
			return fmt.Errorf("%s was made by a newer Killer Chicken 9000", args[0])
		}

		// Validate everything before writing anything.
		var blocks []checkedBlock
		for _, b := range in.Blocks {
			c, err := checkBlock(b)
			if err != nil {
				return fmt.Errorf("block '%s': %w", b.Name, err)
			}
			blocks = append(blocks, c)
		}
		type checkedCluster struct {
			c     clusterExport
			where db.Where
		}
		var clusters []checkedCluster
		for _, c := range in.Clusters {
			if err := db.CheckListName(c.Name); err != nil {
				return fmt.Errorf("cluster '%s': %w", c.Name, err)
			}
			var where db.Where
			for _, place := range c.Where {
				if !db.IsPlace(place) {
					return fmt.Errorf("cluster '%s': unknown place %q", c.Name, place)
				}
				where = where.With(place, true)
			}
			if !where.Any() {
				return fmt.Errorf("cluster '%s' looks nowhere", c.Name)
			}
			entries, err := prepareEntries(where, c.Entries)
			if err != nil {
				return fmt.Errorf("cluster '%s': %w", c.Name, err)
			}
			c.Entries = entries
			clusters = append(clusters, checkedCluster{c, where})
		}
		type checkedFreeze struct {
			w    schedule.Window
			mode string
		}
		var freezes []checkedFreeze
		for _, f := range in.Freezes {
			w, err := f.window()
			if err != nil || !freeze.ValidMode(f.Mode) {
				return fmt.Errorf("a Frozen Chicken window is invalid")
			}
			freezes = append(freezes, checkedFreeze{w, f.Mode})
		}
		if in.Nepsis != nil && in.Nepsis.On() {
			if err := in.Nepsis.Check(); err != nil {
				return fmt.Errorf("bypass detection: %w", err)
			}
		}
		images := map[string][]byte{}
		for name, encoded := range in.Images {
			raw, err := base64.StdEncoding.DecodeString(encoded)
			sum := sha256.Sum256(raw)
			if err != nil || !db.IsImageName(name) || !strings.HasPrefix(name, hex.EncodeToString(sum[:])+".") || len(raw) > maxImportSize {
				return fmt.Errorf("the picture %s in the file is damaged", name)
			}
			images[name] = raw
		}

		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()
		say := func(format string, a ...any) {
			if dry {
				format = "Would: " + format
			}
			fmt.Printf(format+"\n", a...)
		}

		// Scheduled freezes are locks: they need the settings password.
		existing, err := database.ListFreezeSchedules()
		if err != nil {
			return err
		}
		var newFreezes []checkedFreeze
		for _, f := range freezes {
			have := false
			for _, e := range existing {
				have = have || (e.Window == f.w && e.Mode == f.mode)
			}
			if !have {
				newFreezes = append(newFreezes, f)
			}
		}
		if !dry && (len(newFreezes) > 0 || len(blocks) > 0) {
			if err := requireLockPassword(database); err != nil {
				return err
			}
		}

		if !dry && len(images) > 0 {
			if err := os.MkdirAll(db.ImagesDir(), 0o755); err != nil {
				return err
			}
			for name, raw := range images {
				path := filepath.Join(db.ImagesDir(), name)
				if _, err := os.Stat(path); err != nil {
					if err := os.WriteFile(path, raw, 0o644); err != nil {
						return err
					}
				}
			}
		}
		for _, c := range blocks {
			if _, err := database.GetBlock(c.in.Name); err == nil {
				fmt.Printf("Block '%s' exists already: left as it is.\n", c.in.Name)
				continue
			}
			say("add block '%s' (%d lists, %d apps, %d schedules)", c.in.Name, len(c.lists), len(visibleApps(c.in.Apps)), len(c.windows))
			if !dry {
				if err := createBlock(database, c); err != nil {
					return fmt.Errorf("block '%s': %w", c.in.Name, err)
				}
			}
		}
		for _, c := range clusters {
			if exists, err := database.ClusterExists(c.c.Name); err != nil {
				return err
			} else if exists {
				fmt.Printf("Cluster '%s' exists already: left as it is.\n", c.c.Name)
				continue
			}
			say("add cluster '%s' (%d entries)", c.c.Name, len(c.c.Entries))
			if !dry {
				if err := database.CreateCluster(c.c.Name, c.where); err != nil {
					return err
				}
				created, err := database.GetCluster(c.c.Name)
				if err != nil {
					return err
				}
				if _, err := database.AddClusterEntries(created.ID, c.c.Entries); err != nil {
					return err
				}
			}
		}
		for _, name := range enforce.BrowserSettings {
			if !in.Browser[name] || enforce.BrowserOn(database, name) {
				continue
			}
			if name == "force-librewolf" && blocker.LibreWolfVersion() == "" {
				fmt.Println("Browser option force-librewolf is left off: LibreWolf isn't installed here.")
				continue
			}
			say("turn on the browser option %s", name)
			if !dry {
				if err := database.SetSetting("browser."+name, "on"); err != nil {
					return err
				}
			}
		}
		for _, o := range enforce.Options {
			if want, given := in.Options[o.Name]; given && want != enforce.OptionOn(database, o.Name) {
				say("turn the option %s %s", o.Name, map[bool]string{true: "on", false: "off"}[want])
				if !dry {
					if err := enforce.SetOption(database, o.Name, want); err != nil {
						return err
					}
				}
			}
		}
		if in.Nepsis != nil && in.Nepsis.On() {
			old, err := database.GlobalNepsis()
			if err != nil {
				return err
			}
			switch {
			case !in.Nepsis.StricterThan(old):
				fmt.Println("The bypass detection here is stricter in some way: left as it is.")
			default:
				say("set the global bypass detection (%d steps)", len(in.Nepsis.Steps))
				if !dry {
					if err := database.SetGlobalNepsis(*in.Nepsis); err != nil {
						return err
					}
				}
			}
		}
		for _, f := range newFreezes {
			say("add a scheduled Frozen Chicken: %s (%s)", f.w, f.mode)
			if !dry {
				if err := database.AddFreezeSchedule(f.w, f.mode); err != nil {
					return err
				}
			}
		}
		if !dry {
			if err := enforce.ApplyFromDB(database); err != nil {
				return err
			}
			fmt.Println("Import finished. Imported blocks are not started.")
		}
		return nil
	},
}

func init() {
	projectImportCmd.Flags().Bool("dry-run", false, "Say what would be imported and change nothing")
	projectCmd.AddCommand(projectExportCmd, projectImportCmd)
	rootCmd.AddCommand(projectCmd)
}
