// record.go — link recording.
//
//	kc9k record start <block> <list>     # record into a block's list
//	kc9k record start --cluster <name>   # ... or into a cluster (made if needed)
//	kc9k record                          # what was recorded so far
//	kc9k record stop [--discard]         # add the sites and stop
//
// While a recording is on, the extension reports the site of every page and
// frame the browser opens, and the daemon keeps them. Nothing reaches the
// target until 'stop', which adds them like 'kc9k list add' (so into an
// exception list of a held block it is refused, and the recording stays) or
// 'kc9k cluster add'.
package cli

import (
	"fmt"
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
		return fmt.Sprintf("cluster '%s'", r.Cluster)
	}
	if l, err := recordingList(database, r); err == nil {
		return fmt.Sprintf("the list '%s' of block '%s'", l.Name, r.Block)
	}
	return fmt.Sprintf("a list of block '%s' that is gone", r.Block)
}

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Record the sites you open into a list or a cluster",
	Long: `Start a recording, browse, stop: every site opened in the browser in
between (pages, and frames inside them, such as a login or a captcha from
another site) is added to a block's list or to a cluster. It makes a list of
the sites a task really needs, e.g. a school's pages with their sign-in
steps, to spare them in a block.

  kc9k record start study Exceptions      # or: kc9k record start --cluster school
  ... browse ...
  kc9k record                             # what was recorded so far
  kc9k record stop                        # add the sites and stop (--discard: drop them)

The sites come from the extension in LibreWolf, private windows included,
and are kept on this computer only. Pages that a block stops are recorded
too. Nothing changes until 'stop', which follows the target's rules: into a
blacklist or a cluster is always allowed; into an exception list weakens the
block, so it is refused while the block is locked (the recording then stays,
to stop later or discard). Without a subcommand, shows the recording.`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		r, err := database.GetRecording()
		if err != nil {
			return err
		}
		if r == nil {
			fmt.Println("Not recording. Start with 'kc9k record start <block> <list>' or 'kc9k record start --cluster <name>'.")
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
	Short: "Start recording into a block's list, or with --cluster into a cluster",
	Args:  cobra.RangeArgs(0, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		cluster, _ := cmd.Flags().GetString("cluster")
		if (cluster == "") == (len(args) != 2) {
			return fmt.Errorf("give a block and one of its lists, or --cluster <name>")
		}
		if r, err := database.GetRecording(); err != nil {
			return err
		} else if r != nil {
			return fmt.Errorf("already recording into %s; stop it first with 'kc9k record stop'", describeRecording(database, r))
		}
		var target db.Recording
		if cluster != "" {
			exists, err := database.ClusterExists(cluster)
			if err != nil {
				return err
			}
			if !exists {
				if err := database.CreateTypedCluster(cluster, db.ListAddress, db.Where{URL: true}); err != nil {
					return err
				}
				fmt.Printf("Cluster '%s' created (looks at: url).\n", cluster)
			}
			c, err := database.GetCluster(cluster)
			if err != nil {
				return err
			}
			if c.Phrases {
				return fmt.Errorf("cluster '%s' holds phrases, which take no part in recording (R-PHRASE-10)", cluster)
			}
			if c.Type != db.ListAddress {
				return fmt.Errorf("cluster '%s' doesn't hold addresses; recorded sites wouldn't fit it", cluster)
			}
			target.Cluster = cluster
		} else {
			_, l, err := findList(database, args[0], args[1])
			if err != nil {
				return err
			}
			if !l.IsAddress() {
				return fmt.Errorf("list '%s' isn't an address list; recorded sites wouldn't fit it", l.Name)
			}
			target.Block, target.ListID = args[0], l.ID
		}
		if err := database.StartRecording(target); err != nil {
			return err
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
		sites, err := database.RecordedSites()
		if err != nil {
			return err
		}
		if discard, _ := cmd.Flags().GetBool("discard"); discard || len(sites) == 0 {
			if err := database.EndRecording(); err != nil {
				return err
			}
			if discard {
				fmt.Printf("Recording stopped; %d sites dropped.\n", len(sites))
			} else {
				fmt.Println("Recording stopped; no sites were opened.")
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
		fmt.Printf("Recording stopped; %d sites for %s:\n", len(sites), where)
		for _, s := range sites {
			fmt.Println("  " + s)
		}
		return nil
	}),
}

func init() {
	recordStartCmd.Flags().String("cluster", "", "Record into this cluster (made if there is none by that name)")
	recordStopCmd.Flags().Bool("discard", false, "Drop the recorded sites instead of adding them")
	addNowFlag(recordStopCmd)
	recordCmd.AddCommand(recordStartCmd, recordStopCmd)
	rootCmd.AddCommand(recordCmd)
}
