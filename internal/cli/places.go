// places.go — kc9k place: turn a block on where the computer is.
//
//	kc9k place                       # places, and whether you are there now
//	kc9k place add home              # remember the Wi-Fi networks in range as "home"
//	kc9k place need home 3           # there when 3 of its networks are in range
//	kc9k place rule work library on  # block 'work' is on at 'library'
//	kc9k place remove home
package cli

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

var placeCmd = &cobra.Command{
	Use:   "place",
	Short: "Places, known by their Wi-Fi networks, that switch blocks",
	Long: `A place is the set of Wi-Fi networks in range when you save it: your own and
the neighbours'. The computer is at the place when enough of them are in
range again: two unless you choose another number ('kc9k place need'). A block with a place rule is enabled while the computer is there and
lifted elsewhere (it stays switched on). With a schedule lock ('kc9k schedule
lock'), it is locked while there, as in a block window: it can't be switched
off or weakened until you leave.

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
			fmt.Printf("%s (needs %d of its %d networks)%s\n", p.Name, p.Needed(), len(p.Networks), here)
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
		need, _ := cmd.Flags().GetInt("need")
		if cmd.Flags().Changed("need") && need < 1 {
			return fmt.Errorf("--need is at least 1")
		}
		seen, err := enforce.ScanWifi(true)
		if err != nil {
			return fmt.Errorf("Wi-Fi can't be read (%s): %w", wifiUnreadableHint, err)
		}
		if len(seen) == 0 {
			return fmt.Errorf("no Wi-Fi network is in range, so this place has nothing to be known by")
		}
		existing, _ := database.GetPlace(args[0])
		total, err := database.SavePlace(args[0], seen)
		if err != nil {
			return err
		}
		p, err := database.GetPlace(args[0])
		if err != nil {
			return err
		}
		if cmd.Flags().Changed("need") {
			if err := setPlaceNeed(database, p, need, existing.ID == 0); err != nil {
				return err
			}
			p.Need = need
		}
		fmt.Printf("Place '%s' saved: it is known by %d networks; the computer is there when %d of them are in range.\n",
			p.Name, total, p.Needed())
		if total < 2 {
			fmt.Println("With a single network it is easy to miss; save it again when more are in range.")
		}
		return nil
	}),
}

var placeRemoveCmd = &cobra.Command{
	Use:   "remove [name]",
	Short: "Forget a place (refused while a block using it is locked)",
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

var placeNeedCmd = &cobra.Command{
	Use:   "need [place] [count]",
	Short: "How many of a place's networks must be in range for the computer to be there",
	Long: `The computer is at a place when this many of the networks it was saved with
are in range (two unless you choose another number; at most all of them).
Fewer makes the place easier to recognise, so blocks on there are enabled
more often: always allowed. More makes it harder, so it weakens the blocks
on there: refused while one of them is locked.`,
	Args: cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		p, err := database.GetPlace(args[0])
		if err != nil {
			return err
		}
		need, err := strconv.Atoi(args[1])
		if err != nil || need < 1 {
			return fmt.Errorf("the count is a number from 1 to %d (the networks '%s' knows)", len(p.Networks), p.Name)
		}
		if err := setPlaceNeed(database, p, need, false); err != nil {
			return err
		}
		p.Need = need
		fmt.Printf("The computer is at '%s' when %d of its %d networks are in range.\n", p.Name, p.Needed(), len(p.Networks))
		return nil
	}),
}

// setPlaceNeed sets how many networks a place needs. More than it knows
// is refused; more than before weakens the blocks on there (fresh: a place
// just made has no blocks yet).
func setPlaceNeed(database *db.DB, p db.Place, need int, fresh bool) error {
	if need > len(p.Networks) {
		return fmt.Errorf("'%s' knows only %d networks; save it again where more are in range, or ask for fewer", p.Name, len(p.Networks))
	}
	if !fresh && need > p.Needed() {
		for _, block := range p.Blocks {
			if err := ensureEditable(database, block, false); err != nil {
				return err
			}
		}
	}
	return database.SetPlaceNeed(p.ID, need)
}

var placeRuleCmd = &cobra.Command{
	Use:   "rule [block|mode] [place] [on|off]",
	Short: "Let a place switch a block or mode: on, or off when it is enabled by default",
	Args:  cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if args[2] != "on" && args[2] != "off" {
			return fmt.Errorf("usage: kc9k place rule <block|mode> <place> on|off")
		}
		on := args[2] == "on"
		t, err := resolveTarget(database, args[0], targetKind(cmd))
		if err != nil {
			return err
		}
		if t.Saved != nil {
			return fmt.Errorf("'%s' is a schedule: places go on blocks and modes", args[0])
		}
		p, err := database.GetPlace(args[1])
		if err != nil {
			return err
		}
		if t.Block != "" {
			if err := refuseInMode(database, t.Block); err != nil {
				return err
			}
		}
		// A rule switches nothing on (R-SCHED-1), so a block may get one
		// before it has anything to enforce. A change that lifts the block
		// somewhere it is enabled now weakens it, refused while it is locked
		// (with a schedule lock, also while the computer is there): a place
		// that turns off a block enabled by default, the first place of a
		// continuous block (enabled only there then), or removing a place
		// that turns it on (§3.6). A mode's goes to all its blocks.
		spec, err := targetSpec(database, t)
		if err != nil {
			return err
		}
		if on && !slices.Contains(spec.Places, p.ID) {
			spec.Places = append(spec.Places, p.ID)
			slices.Sort(spec.Places)
		} else if !on {
			spec.Places = slices.DeleteFunc(spec.Places, func(id int) bool { return id == p.ID })
		}
		if err := changeAutomation(cmd, database, t, spec); err != nil {
			return err
		}
		block := t.Name
		if t.Block == "" {
			block = t.what()
		}
		switch {
		case on && spec.DefaultOn:
			fmt.Printf("Block '%s' is lifted while the computer is at '%s' (it is enabled by default).\n", block, p.Name)
		case on && t.Block == "":
			fmt.Printf("%s is on while the computer is at '%s'.\n", strings.ToUpper(block[:1])+block[1:], p.Name)
		case on:
			fmt.Printf("Block '%s' is enabled while the computer is at '%s' (it is disabled by default).\n", block, p.Name)
		default:
			fmt.Printf("'%s' no longer switches %s.\n", p.Name, map[bool]string{true: "block '" + block + "'", false: block}[t.Block != ""])
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
	placeAddCmd.Flags().Int("need", db.DefaultPlaceNeed, "How many of its networks must be in range for the computer to be there")
	placeCmd.AddCommand(placeAddCmd, placeRemoveCmd, placeRuleCmd, placeNeedCmd)
	rootCmd.AddCommand(placeCmd)
}
