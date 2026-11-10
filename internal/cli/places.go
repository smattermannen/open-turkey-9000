// places.go — kc9k place: turn a block on where the computer is.
//
//	kc9k place                       # places, and whether you are there now
//	kc9k place add home              # remember the Wi-Fi networks in range as "home"
//	kc9k place rule work library on  # block 'work' is on at 'library'
//	kc9k place remove home
package cli

import (
	"fmt"
	"strings"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var placeCmd = &cobra.Command{
	Use:   "place",
	Short: "Places, known by their Wi-Fi networks, where blocks turn on",
	Long: `A place is the set of Wi-Fi networks in range when you save it: your own and
the neighbours'. The computer is at the place when two of them are in range
again. A block with a place rule is on while the computer is there, and is
held like a block in a schedule window: it can't be stopped or loosened
until you leave.

When no network at all can be seen (Wi-Fi switched off, or no adapter), the
computer counts as being at every place, and a switched-off Wi-Fi is
switched back on: turning Wi-Fi off is no way out. It is not a lock against
someone determined: a place far from every saved network is simply "not
there".

Saving a place again under the same name adds the networks seen now, which
makes it easier to recognise.`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		places, err := database.Places()
		if err != nil {
			return err
		}
		if len(places) == 0 {
			fmt.Println("No places saved. Save where you are now with 'kc9k place add <name>'.")
			return nil
		}
		seen, err := enforce.ScanWifi(false)
		if err != nil {
			fmt.Printf("(Wi-Fi can't be read: %v)\n", err)
		}
		for _, p := range places {
			here := ""
			if err == nil && len(seen) > 0 && enforce.AtPlace(p, seen) {
				here = " — you are here"
			}
			fmt.Printf("%s (%d networks)%s\n", p.Name, len(p.Networks), here)
			if len(p.Blocks) > 0 {
				fmt.Printf("  on here: %s\n", strings.Join(p.Blocks, ", "))
			}
		}
		return nil
	}),
}

var placeAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Remember the Wi-Fi networks in range now as a place",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		seen, err := enforce.ScanWifi(true)
		if err != nil {
			return fmt.Errorf("Wi-Fi can't be read (is NetworkManager running?): %w", err)
		}
		if len(seen) == 0 {
			return fmt.Errorf("no Wi-Fi network is in range, so this place has nothing to be known by")
		}
		total, err := database.SavePlace(args[0], seen)
		if err != nil {
			return err
		}
		fmt.Printf("Place '%s' saved: it is known by %d networks.\n", args[0], total)
		if total < 2 {
			fmt.Println("With a single network it is easy to miss; save it again when more are in range.")
		}
		return nil
	}),
}

var placeRemoveCmd = &cobra.Command{
	Use:   "remove [name]",
	Short: "Forget a place (refused while it holds a block)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		p, err := database.GetPlace(args[0])
		if err != nil {
			return err
		}
		// Its blocks lose a rule that turns them on: that loosens them.
		for _, block := range p.Blocks {
			if err := ensureEditable(database, block, false); err != nil {
				return err
			}
		}
		if err := database.DeletePlace(p.ID); err != nil {
			return err
		}
		fmt.Printf("Place '%s' removed.\n", p.Name)
		return nil
	}),
}

var placeRuleCmd = &cobra.Command{
	Use:   "rule [block] [place] [on|off]",
	Short: "Turn a block on while the computer is at a place",
	Args:  cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if args[2] != "on" && args[2] != "off" {
			return fmt.Errorf("usage: kc9k place rule <block> <place> on|off")
		}
		block, on := args[0], args[2] == "on"
		detail, err := database.GetBlock(block)
		if err != nil {
			return err
		}
		p, err := database.GetPlace(args[1])
		if err != nil {
			return err
		}
		if on && detail.Empty() {
			return fmt.Errorf("block '%s' has nothing to enforce yet", block)
		}
		// On makes the block stricter; off loosens it (and is refused while
		// the computer is at the place, like a schedule in its window).
		if err := ensureEditable(database, block, on); err != nil {
			return err
		}
		if err := database.SetPlaceRule(block, p.ID, on); err != nil {
			return err
		}
		if on {
			fmt.Printf("Block '%s' is on while the computer is at '%s'.\n", block, p.Name)
		} else {
			fmt.Printf("Block '%s' no longer turns on at '%s'.\n", block, p.Name)
		}
		return nil
	}),
}

// placeNames returns the places whose rule turns a block on.
func placeNames(places []db.Place, block string) []string {
	out := []string{}
	for _, p := range places {
		for _, b := range p.Blocks {
			if b == block {
				out = append(out, p.Name)
			}
		}
	}
	return out
}

func init() {
	placeCmd.AddCommand(placeAddCmd, placeRemoveCmd, placeRuleCmd)
	rootCmd.AddCommand(placeCmd)
}
