// dns.go — which DNS server KC9K asks for blocked sites' real addresses
// (§4.10). Only the firewall layer needs them.
//
//	kc9k dns                     # shows the choice and the servers asked
//	kc9k dns network|quad9|IP,IP
package cli

import (
	"fmt"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var dnsCmd = &cobra.Command{
	Use:   "dns [network|quad9|IP[,IP...]]",
	Short: "Choose the DNS server asked for blocked sites' real addresses",
	Long: `The firewall layer needs the real addresses of blocked sites, which /etc/hosts
hides from the computer's own lookups, so KC9K asks a DNS server directly:

  network   the network's own DNS server, the one the computer already uses,
            so nobody new learns which sites are blocked; Quad9 (9.9.9.9) when
            it can't answer (default)
  quad9     Quad9 only
  IP,IP     your own servers, in order

Changing it needs the settings password, and is refused while anything is
locked if 'kc9k no-weakening' is on.`,
	Args: cobra.MaximumNArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 0 {
			choice, err := database.GetSetting(enforce.SettingDNS)
			if err != nil {
				return err
			}
			if choice == "" {
				choice = blocker.DNSNetwork
			}
			blocker.SetDNSChoice(choice)
			fmt.Printf("DNS: %s (asks %s)\n", choice, joinOr(blocker.DNSServers(), "nobody"))
			return nil
		}
		choice := args[0]
		if err := blocker.ValidDNSChoice(choice); err != nil {
			return err
		}
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
		if err := refuseSettingWeakening(database, "the DNS setting"); err != nil {
			return err
		}
		if err := database.SetSetting(enforce.SettingDNS, choice); err != nil {
			return err
		}
		blocker.SetDNSChoice(choice)
		fmt.Printf("DNS: %s (asks %s)\n", choice, joinOr(blocker.DNSServers(), "nobody"))
		return nil
	}),
}

func joinOr(items []string, none string) string {
	if len(items) == 0 {
		return none
	}
	out := items[0]
	for _, s := range items[1:] {
		out += ", " + s
	}
	return out
}

func init() {
	rootCmd.AddCommand(dnsCmd)
}
