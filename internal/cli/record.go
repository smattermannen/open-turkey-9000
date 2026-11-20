// record.go — link recording and word recording (§3.12).
//
//	kc9k record start <block> <list>     # record into a block's list
//	kc9k record start --preset <name>    # ... or into a list preset (made if needed)
//	kc9k record start ... --words        # rendered words instead of sites
//	kc9k record                          # what was recorded so far
//	kc9k record stop [--discard] [--cut N]  # add them and stop
//	kc9k record baseline [on|off]        # word counts of normal browsing
//
// While a recording is on, the extension reports the site of every page and
// frame the browser opens, and the daemon keeps them. Nothing reaches the
// target until 'stop', which adds them like 'kc9k list add' (so into an
// exception list of a held block it is refused, and the recording stays) or
// 'kc9k list-preset add'.
package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

// recordingList returns the block's list a recording goes to, by id (it may
// have been renamed since).
func recordingList(database *db.DB, r *db.Recording) (db.WebList, error) {
	detail, err := database.GetBlock(r.Block)
	if err != nil {
		return db.WebList{}, err
	}
	for _, l := range detail.Lists {
		if l.ID == r.ListID {
			return l, nil
		}
	}
	return db.WebList{}, fmt.Errorf("the list of block '%s' that was being recorded into is gone", r.Block)
}

func describeRecording(database *db.DB, r *db.Recording) string {
	if r.Cluster != "" {
		return fmt.Sprintf("list preset '%s'", r.Cluster)
	}
	if l, err := recordingList(database, r); err == nil {
		return fmt.Sprintf("the list '%s' of block '%s'", l.Name, r.Block)
	}
	return fmt.Sprintf("a list of block '%s' that is gone", r.Block)
}

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Record the sites you open into a list or a list preset",
	Long: `Start a recording, browse, stop: every site opened in the browser in
between (pages, and frames inside them, such as a login or a captcha from
another site) is added to a block's list or to a list preset. It makes a list of
the sites a task really needs, e.g. a school's pages with their sign-in
steps, to spare them in a block.

  kc9k record start study Exceptions      # or: kc9k record start --preset school
  ... browse ...
  kc9k record                             # what was recorded so far
  kc9k record stop                        # add the sites and stop (--discard: drop them)

The sites come from the extension in LibreWolf, private windows included,
and are kept on this computer only. Pages that a block stops are recorded
too. Nothing changes until 'stop', which follows the target's rules: into a
blacklist or a list preset is always allowed; into an exception list weakens the
block, so it is refused while the block is locked (the recording then stays,
to stop later or discard). Without a subcommand, shows the recording.`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		r, err := database.GetRecording()
		if err != nil {
			return err
		}
		if r == nil {
			fmt.Println("Not recording. Start with 'kc9k record start <block> <list>' or 'kc9k record start --preset <name>'.")
			return nil
		}
		if r.Words {
			words, err := database.RecordedWords()
			if err != nil {
				return err
			}
			fmt.Printf("Recording words into %s since %s: %d words on %d pages (baseline: %d pages)\n",
				describeRecording(database, r), time.Unix(r.Started, 0).Format("15:04"), len(words),
				database.RecordedPages(), database.BaselinePages())
			printWords(words, 50)
			return nil
		}
		sites, err := database.RecordedSites()
		if err != nil {
			return err
		}
		fmt.Printf("Recording into %s since %s: %d sites so far\n", describeRecording(database, r),
			time.Unix(r.Started, 0).Format("15:04"), len(sites))
		for _, s := range sites {
			fmt.Println("  " + s)
		}
		return nil
	}),
}

var recordStartCmd = &cobra.Command{
	Use:   "start [block] [list]",
	Short: "Start recording into a block's list, or with --preset into a list preset",
	Args:  cobra.RangeArgs(0, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		cluster, _ := cmd.Flags().GetString("preset")
		if (cluster == "") == (len(args) != 2) {
			return fmt.Errorf("give a block and one of its lists, or --preset <name>")
		}
		if r, err := database.GetRecording(); err != nil {
			return err
		} else if r != nil {
			return fmt.Errorf("already recording into %s; stop it first with 'kc9k record stop'", describeRecording(database, r))
		}
		words, _ := cmd.Flags().GetBool("words")
		target := db.Recording{Words: words}
		if cluster != "" {
			exists, err := database.ClusterExists(cluster)
			if err != nil {
				return err
			}
			if !exists {
				where, looks := db.Where{URL: true}, "url"
				if words {
					where, looks = db.Where{Text: true}, "text"
				}
				if err := database.CreateCluster(cluster, where); err != nil {
					return err
				}
				fmt.Printf("List preset '%s' created (looks at: %s).\n", cluster, looks)
			}
			c, err := database.GetCluster(cluster)
			if err != nil {
				return err
			}
			if c.Phrases {
				return fmt.Errorf("list preset '%s' holds phrases, which take no part in recording (R-PHRASE-10)", cluster)
			}
			if words && !c.Where.Text {
				return fmt.Errorf("list preset '%s' doesn't look at page text; recorded words wouldn't match there", cluster)
			}
			if !words && !c.Where.URL {
				return fmt.Errorf("list preset '%s' doesn't look at the url; recorded sites wouldn't match there", cluster)
			}
			target.Cluster = cluster
		} else {
			_, l, err := findList(database, args[0], args[1])
			if err != nil {
				return err
			}
			if words && !l.Where.Text {
				return fmt.Errorf("list '%s' doesn't look at page text; recorded words wouldn't match there", l.Name)
			}
			if !words && !l.Where.URL {
				return fmt.Errorf("list '%s' doesn't look at the url; recorded sites wouldn't match there", l.Name)
			}
			target.Block, target.ListID = args[0], l.ID
		}
		if err := database.StartRecording(target); err != nil {
			return err
		}
		if words {
			if !database.WordBaselineOn() {
				if err := database.SetWordBaseline(true); err != nil {
					return err
				}
				fmt.Println("The word baseline is now gathered from your normal browsing (word counts only, no addresses,\n" +
					"no private windows; kept on this computer) to rank recorded words. Off: 'kc9k record baseline off'.")
			}
			fmt.Printf("Recording words into %s. Browse in LibreWolf, then run 'kc9k record stop [--cut N]'.\n", describeRecording(database, &target))
			return nil
		}
		fmt.Printf("Recording into %s. Browse in LibreWolf, then run 'kc9k record stop'.\n", describeRecording(database, &target))
		return nil
	}),
}

var recordStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop recording and add the recorded sites (--discard: drop them)",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		r, err := database.GetRecording()
		if err != nil {
			return err
		}
		if r == nil {
			return fmt.Errorf("not recording")
		}
		cut, _ := cmd.Flags().GetInt("cut")
		if cut < 0 || cut > 99 {
			return fmt.Errorf("--cut is a percentage from 0 to 99")
		}
		var sites []string
		if r.Words {
			words, err := database.RecordedWords()
			if err != nil {
				return err
			}
			for _, w := range db.CutWords(words, cut) {
				sites = append(sites, w.Word)
			}
		} else if sites, err = database.RecordedSites(); err != nil {
			return err
		}
		kind := "sites"
		if r.Words {
			kind = "words"
		}
		if discard, _ := cmd.Flags().GetBool("discard"); discard || len(sites) == 0 {
			if err := database.EndRecording(); err != nil {
				return err
			}
			if discard {
				fmt.Printf("Recording stopped; %d %s dropped.\n", len(sites), kind)
			} else {
				fmt.Printf("Recording stopped; no %s were recorded.\n", kind)
			}
			return nil
		}
		where := describeRecording(database, r)
		if r.Cluster != "" {
			c, err := database.GetCluster(r.Cluster)
			if err != nil {
				return fmt.Errorf("%w (drop the recording with 'kc9k record stop --discard')", err)
			}
			if _, err := clusterAdd(database, c, sites); err != nil {
				return err
			}
		} else {
			l, err := recordingList(database, r)
			if err != nil {
				return fmt.Errorf("%w (drop the recording with 'kc9k record stop --discard')", err)
			}
			if err := listAdd(cmd, database, r.Block, l.Name, sites); err != nil {
				return fmt.Errorf("%w\nStill recording: stop again once this is allowed, or drop it with 'kc9k record stop --discard'", err)
			}
		}
		if err := database.EndRecording(); err != nil {
			return err
		}
		fmt.Printf("Recording stopped; %d %s for %s:\n", len(sites), kind, where)
		for _, s := range sites {
			fmt.Println("  " + s)
		}
		return nil
	}),
}

// printWords shows up to n ranked words with their counts.
func printWords(words []db.RecordedWord, n int) {
	if len(words) > 0 {
		fmt.Printf("  %-24s %8s %6s %11s\n", "word", "times", "pages", "specificity")
	}
	for i, w := range words {
		if i == n {
			fmt.Printf("  … %d more\n", len(words)-n)
			break
		}
		fmt.Printf("  %-24s %8d %6d %11.1f\n", w.Word, w.Count, w.Pages, w.Specificity)
	}
}

var recordBaselineCmd = &cobra.Command{
	Use:   "baseline [on|off]",
	Short: "The word baseline: word counts of normal browsing that rank recorded words",
	Long: `Word recording ranks words by specificity: how much more common they are on
the recorded pages than in normal browsing. The baseline is that normal
browsing: per word, the number of pages it showed on; no addresses, no
private windows, kept on this computer. A word recording turns it on; off
forgets it.`,
	Args: cobra.MaximumNArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 1 {
			if args[0] != "on" && args[0] != "off" {
				return fmt.Errorf("use 'on' or 'off'")
			}
			if err := database.SetWordBaseline(args[0] == "on"); err != nil {
				return err
			}
		}
		state := "off"
		if database.WordBaselineOn() {
			state = "on"
		}
		fmt.Printf("Word baseline: %s, %s pages\n", state, strconv.FormatInt(database.BaselinePages(), 10))
		return nil
	}),
}

func init() {
	recordStartCmd.Flags().String("preset", "", "Record into this list preset (made if there is none by that name)")
	flagAliases(recordStartCmd, map[string]string{"cluster": "preset"}) // §4.12
	recordStartCmd.Flags().Bool("words", false, "Record the rendered words of the pages opened, with counts, instead of their sites (§3.12)")
	recordStopCmd.Flags().Bool("discard", false, "Drop the recorded sites or words instead of adding them")
	recordStopCmd.Flags().Int("cut", 0, "Word recording: drop the lowest N % of words by specificity before saving")
	addNowFlag(recordStopCmd)
	recordCmd.AddCommand(recordStartCmd, recordStopCmd, recordBaselineCmd)
	rootCmd.AddCommand(recordCmd)
}
