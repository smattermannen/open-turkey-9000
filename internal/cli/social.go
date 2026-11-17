// social.go — kc9k social: social media settings. A YouTube playlist let
// through is an exception entry (§3.2); "playlist" here adds or removes it.
//
//	kc9k social                              # the settings there are
//	kc9k social work                         # what a block has on
//	kc9k social work youtube:no-shorts on
//	kc9k social work playlist PLxxxxxxxx on   # = block add-exception work youtube.com/playlist?list=PLxxxxxxxx
package cli

import (
	"fmt"
	"strings"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

var socialCmd = &cobra.Command{
	Use:   "social [block] [setting] [on|off]",
	Short: "Social media settings: keep a site, take a part of it away",
	Long: `Each setting takes one part of a site away while its block is on: Shorts,
the home feed, everything but messages. The site itself stays reachable, so
don't put it on the block's blacklist too. Turning a setting on makes the
block stricter and is always allowed; turning one off weakens it.

  kc9k social                              the settings there are
  kc9k social <block>                      what the block has on
  kc9k social <block> <setting> on|off
  kc9k social <block> playlist <id or address> on|off

A YouTube playlist is let through by its address in an exception list
(kc9k block add-exception <block> youtube.com/playlist?list=<id>): its page
and its videos stay open, every other video is blocked. "playlist" here
adds or removes that exception. The site's layout changes now and then; a setting that hides a part of a
page may then miss it until Killer Chicken 9000 is updated.`,
	Args: cobra.RangeArgs(0, 4),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 0 {
			site := ""
			for _, p := range db.SocialPresets {
				if p.Site != site {
					site = p.Site
					fmt.Printf("%s\n", site)
				}
				fmt.Printf("  %-28s %s\n", p.Name, p.Title)
			}
			return nil
		}
		block := args[0]
		detail, err := database.GetBlock(block)
		if err != nil {
			return err
		}
		if len(args) == 1 {
			if len(detail.Social) == 0 {
				fmt.Printf("Block '%s' has no social media settings.\n", block)
			}
			for _, p := range detail.Social {
				fmt.Println(p)
			}
			return nil
		}
		if strings.ToLower(args[1]) == "playlist" {
			if len(args) != 4 || (args[3] != "on" && args[3] != "off") {
				return fmt.Errorf("usage: kc9k social <block> playlist <id or address> on|off")
			}
			return socialPlaylist(cmd, database, detail, args[2], args[3] == "on")
		}
		if len(args) != 3 || (args[2] != "on" && args[2] != "off") {
			return fmt.Errorf("usage: kc9k social <block> <setting> on|off")
		}
		preset := strings.ToLower(args[1])
		if !db.IsSocialPreset(preset) {
			return fmt.Errorf("unknown setting %q ('kc9k social' lists them)", args[1])
		}
		on := args[2] == "on"
		if err := ensureEditable(database, block, on); err != nil {
			return err
		}
		if err := database.SetSocial(block, preset, on); err != nil {
			return err
		}
		fmt.Printf("%s %s for block '%s'\n", preset, args[2], block)
		return settle(database, block)
	}),
}

// socialPlaylist lets a playlist through a block, or stops doing so: its
// address added to the block's Exceptions (weakening, gated by the list's
// own gate in listAdd), or taken out of whichever exception list has it
// (stricter, always free). The extension keeps the video pages to the
// playlist's videos (db.withPlaylists).
func socialPlaylist(cmd *cobra.Command, database *db.DB, detail *db.BlockDetail, text string, on bool) error {
	id, ok := db.PlaylistID(text)
	if !ok {
		return fmt.Errorf("%q isn't a playlist: give its address (with list=… in it) or its ID (10 to 64 letters, digits, - or _, such as PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf)", text)
	}
	block, entry := detail.Name, db.PlaylistException(id)
	if on {
		if _, ok := detail.Lists.Find(db.DefaultExceptionsList); !ok {
			if _, err := database.CreateList(block, db.ListExceptions, db.DefaultExceptionsList, db.Where{URL: true}); err != nil {
				return err
			}
		}
		return listAdd(cmd, database, block, db.DefaultExceptionsList, []string{entry})
	}
	removed := false
	for _, l := range detail.Lists {
		if !l.IsExceptions() {
			continue
		}
		var have []string
		for _, e := range l.Entries {
			if got, ok := db.PlaylistInEntry(e); ok && got == id {
				have = append(have, e)
			}
		}
		if len(have) > 0 {
			if err := database.RemoveListEntries(l.ID, have); err != nil {
				return err
			}
			removed = true
		}
	}
	if !removed {
		return fmt.Errorf("block '%s' doesn't let the playlist %s through", block, id)
	}
	fmt.Printf("Block '%s' no longer lets the playlist %s through.\n", block, id)
	return settle(database, block)
}

func init() {
	addNowFlag(socialCmd)
	rootCmd.AddCommand(socialCmd)
}
