// notifications.go — kc9k notifications: which desktop notifications to
// get and how many minutes ahead (§4.10). A preference: never locked, and
// turning one off is no weakening (it protects nothing). Frozen Chicken is
// always announced (R-WARN-1).
package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var notificationsCmd = &cobra.Command{
	Use:   "notifications [<kind> on|off | advance MINUTES]",
	Short: "Choose desktop notifications and how many minutes ahead they warn",
	Long: `Desktop notifications, each on or off:

  window-start  a block's window opens          window-end   it closes
  timer-end     a timer lock ends               pomodoro     work or rest begins
  allowance     an allowance session runs out   app-blocked  a blocked app was closed

advance sets how many minutes ahead they warn, e.g. 'advance 2,5' warns 5
and again 2 minutes before (1 to 120). A scheduled Frozen Chicken is always
announced, 5 minutes and 1 minute ahead. Without arguments, shows them.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		o := enforce.LoadNoticeOptions(database)
		if len(args) == 0 {
			for _, k := range enforce.NoticeKinds {
				fmt.Printf("%-13s %s\n", k, onOff(o.On(k)))
			}
			fmt.Printf("advance       %s minutes\n", joinInts(o.Advance))
			return nil
		}
		if len(args) != 2 {
			return fmt.Errorf("give a kind and on|off, or 'advance' and minutes")
		}
		if args[0] == "advance" {
			var mins []int
			for _, part := range strings.Split(args[1], ",") {
				n, err := strconv.Atoi(strings.TrimSpace(part))
				if err != nil || n < 1 || n > 120 {
					return fmt.Errorf("advance takes minutes from 1 to 120, e.g. 2,5")
				}
				mins = append(mins, n)
			}
			sort.Ints(mins)
			o.Advance = mins
			if err := enforce.SaveNoticeOptions(database, o); err != nil {
				return err
			}
			fmt.Printf("Notifications warn %s minutes ahead\n", joinInts(mins))
			return nil
		}
		known := false
		for _, k := range enforce.NoticeKinds {
			known = known || k == args[0]
		}
		if !known {
			return fmt.Errorf("unknown notification %q (use %s)", args[0], strings.Join(enforce.NoticeKinds, ", "))
		}
		var off []string
		for _, k := range o.Off {
			if k != args[0] {
				off = append(off, k)
			}
		}
		switch args[1] {
		case "on":
		case "off":
			off = append(off, args[0])
		default:
			return fmt.Errorf("use on or off")
		}
		o.Off = off
		if err := enforce.SaveNoticeOptions(database, o); err != nil {
			return err
		}
		fmt.Printf("%s notifications %s\n", args[0], args[1])
		return nil
	}),
}

func joinInts(ns []int) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ",")
}

func init() {
	rootCmd.AddCommand(notificationsCmd)
}
