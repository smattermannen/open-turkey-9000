// blockusers.go — affected users (§3.2): kc9k block users B [all | USER...].
// A block for some accounts only is enforced for them alone: its sites in
// their browsers' extension (not in /etc/hosts, the firewall or browser
// policies, which every account shares), its apps closed in their
// sessions. Fewer users weakens the block (refused while locked); more, or
// all, is always allowed.
package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var blockUsersCmd = &cobra.Command{
	Use:   "users [block] [all | USER...]",
	Short: "The accounts a block applies to (default all)",
	Long: `Without users, shows them. 'all' makes the block apply to every account.
A block for some accounts only blocks its websites through the extension in
their browsers (the hosts file, firewall and browser policies are shared by
every account) and closes its apps in their sessions only. Fewer users
weakens the block: refused while it is locked.`,
	Args: cobra.MinimumNArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		name := args[0]
		current, err := database.BlockUsers(name)
		if err != nil {
			return err
		}
		if len(args) == 1 {
			if len(current) == 0 {
				fmt.Printf("Block '%s' applies to every account.\n", name)
			} else {
				fmt.Printf("Block '%s' applies to: %s\n", name, strings.Join(current, ", "))
			}
			return nil
		}
		var want []string
		if !(len(args) == 2 && args[1] == "all") {
			known := accounts()
			for _, u := range args[1:] {
				if !slices.Contains(known, u) {
					return fmt.Errorf("'%s' is not an account of this computer (accounts: %s)", u, strings.Join(known, ", "))
				}
				want = append(want, u)
			}
			if !slices.ContainsFunc(known, func(u string) bool { return !slices.Contains(want, u) }) {
				want = nil // every account: the same as all
			}
		}
		// Fewer users than before weakens: from all to some, or dropping one.
		weakens := len(want) > 0 && (len(current) == 0 || slices.ContainsFunc(current, func(u string) bool { return !slices.Contains(want, u) }))
		if weakens {
			if err := requireSettingsPassword(database); err != nil {
				return err
			}
			if err := ensureEditable(database, name, false); err != nil {
				return err
			}
		}
		if err := database.SetBlockUsers(name, want); err != nil {
			return err
		}
		if err := enforce.ApplyFromDB(database); err != nil {
			return err
		}
		if len(want) == 0 {
			fmt.Printf("Block '%s' applies to every account.\n", name)
		} else {
			fmt.Printf("Block '%s' applies to: %s\n", name, strings.Join(want, ", "))
		}
		return nil
	}),
}

// accounts lists the computer's people accounts; replaceable in tests.
var accounts = blocker.RegularUsers

func init() {
	blockCmd.AddCommand(blockUsersCmd)
}
