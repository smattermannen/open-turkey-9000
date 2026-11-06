// stats.go — statistics: time spent on websites and apps.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

type statsOutput struct {
	Enabled bool          `json:"enabled"`
	Since   string        `json:"since"`
	Rows    []db.UsageRow `json:"rows"`
}

var statsCmd = &cobra.Command{
	Use:   "stats [on|off|clear]",
	Short: "Show time spent on websites and apps, or turn statistics on or off",
	Long: `Statistics count the time each app is in the foreground and, in browsers
with the bundled extension, the time on each website. Time behind a locked
screen or after two minutes without input isn't counted. Everything stays on
this computer and is kept for a year.

  kc9k stats            totals for the last 7 days
  kc9k stats --days 30  totals for the last 30 days
  kc9k stats off        stop recording (on starts again)
  kc9k stats clear      delete all statistics`,
	ValidArgs: []string{"on", "off", "clear"},
	Args:      cobra.MaximumNArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 1 {
			switch args[0] {
			case "on", "off":
				if err := database.SetSetting(enforce.SettingStats, args[0]); err != nil {
					return err
				}
				fmt.Printf("Statistics turned %s.\n", args[0])
				return nil
			case "clear":
				if err := database.ClearUsage(); err != nil {
					return err
				}
				fmt.Println("Statistics deleted.")
				return nil
			default:
				return fmt.Errorf("use on, off or clear")
			}
		}

		days, _ := cmd.Flags().GetInt("days")
		if days < 1 || days > enforce.StatsKeepDays {
			return fmt.Errorf("--days must be between 1 and %d", enforce.StatsKeepDays)
		}
		since := enforce.Day(time.Now().AddDate(0, 0, -(days - 1)))
		rows, err := database.UsageSince(since)
		if err != nil {
			return err
		}
		out := statsOutput{Enabled: enforce.StatsOn(database), Since: since, Rows: rows}
		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
			data, err := json.Marshal(out)
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		if !out.Enabled {
			fmt.Println("Statistics are off ('kc9k stats on' to record).")
		}
		fmt.Printf("Since %s:\n", since)
		for _, kind := range []struct{ key, title string }{{db.UsageSite, "Websites"}, {db.UsageApp, "Apps"}} {
			totals := map[string]int64{}
			for _, r := range rows {
				if r.Kind == kind.key {
					totals[r.Name] += r.Seconds
				}
			}
			names := make([]string, 0, len(totals))
			for name := range totals {
				names = append(names, name)
			}
			sort.Slice(names, func(i, j int) bool {
				if totals[names[i]] != totals[names[j]] {
					return totals[names[i]] > totals[names[j]]
				}
				return names[i] < names[j]
			})
			fmt.Printf("\n%s:\n", kind.title)
			if len(names) == 0 {
				fmt.Println("  (nothing recorded)")
			}
			for i, name := range names {
				if i == 15 {
					fmt.Printf("  … and %d more\n", len(names)-15)
					break
				}
				fmt.Printf("  %-32s %s\n", name, formatSpent(totals[name]))
			}
		}
		return nil
	}),
}

func formatSpent(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	if d < time.Minute {
		return fmt.Sprintf("%ds", seconds)
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh %02dm", int(d.Hours()), int(d.Minutes())%60)
}

func init() {
	statsCmd.Flags().Int("days", 7, "How many days back to include, today counted")
	statsCmd.Flags().Bool("json", false, "Print the rows as JSON (for the graphical interface)")
	rootCmd.AddCommand(statsCmd)
}
