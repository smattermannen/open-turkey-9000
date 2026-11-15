// draft.go — block drafts (§4.3): one draft per block; nothing applies
// until Save.
//
//	kc9k block draft B          # a draft copy to edit ("B (draft)")
//	kc9k block draft B save     # apply its differences to B, through the gates
//	kc9k block draft B cancel   # drop it
//
// A draft is an inactive copy of the block's lists, apps and window titles
// that is never enforced (it can't be started). Save applies each
// difference as the matching command would, so on a locked block the
// weakening ones are refused and named; the rest apply. Other settings
// (schedules, allowance, phrase sets…) still apply at once.
package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

// draftName is the name of block's draft.
func draftName(block string) string { return block + " (draft)" }

const settingDraftOf = "draft-of." // + draft name → its block

// draftOf returns the block a block is the draft of, or "".
func draftOf(database *db.DB, name string) string {
	v, _ := database.GetSetting(settingDraftOf + name)
	return v
}

var blockDraftCmd = &cobra.Command{
	Use:   "draft [block] [save|cancel]",
	Short: "Edit a block as a draft: nothing applies until save",
	Args:  cobra.RangeArgs(1, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		block := args[0]
		if of := draftOf(database, block); of != "" {
			block = of // the draft's own name works too
		}
		draft := draftName(block)
		exists := draftOf(database, draft) == block
		if len(args) == 1 {
			if exists {
				fmt.Printf("Block '%s' has a draft: '%s'.\n", block, draft)
				return nil
			}
			// §3.11: a draft shows the contents, so it needs the block
			// password, and keeps it.
			if err := revealBlock(database, block); err != nil {
				return err
			}
			if err := database.DuplicateBlock(block, draft); err != nil {
				return err
			}
			if hash, _ := database.BlockViewPassword(block); hash != "" {
				if err := database.SetBlockViewPassword(draft, hash); err != nil {
					return err
				}
			}
			if err := database.SetSetting(settingDraftOf+draft, block); err != nil {
				return err
			}
			fmt.Printf("Draft '%s' made: edit it, then 'kc9k block draft %s save' (or cancel).\n", draft, block)
			return nil
		}
		if !exists {
			return fmt.Errorf("block '%s' has no draft", block)
		}
		switch args[1] {
		case "cancel":
			return dropDraft(database, draft)
		case "save":
			refusedChanges, err := saveDraft(cmd, database, block, draft)
			if err != nil {
				return err
			}
			if err := dropDraft(database, draft); err != nil {
				return err
			}
			if len(refusedChanges) > 0 {
				return fmt.Errorf("saved, except what would weaken block '%s' while it is locked:\n  %s",
					block, strings.Join(refusedChanges, "\n  "))
			}
			fmt.Printf("Draft saved to block '%s'.\n", block)
			return nil
		}
		return fmt.Errorf("use 'save' or 'cancel'")
	}),
}

func dropDraft(database *db.DB, draft string) error {
	if err := database.RemoveBlock(draft); err != nil {
		return err
	}
	return database.SetSetting(settingDraftOf+draft, "")
}

// saveDraft applies the draft's differences to block, each through its
// command's gate; it returns the changes refused.
func saveDraft(cmd *cobra.Command, database *db.DB, block, draft string) ([]string, error) {
	orig, err := database.GetBlock(block)
	if err != nil {
		return nil, err
	}
	next, err := database.GetBlock(draft)
	if err != nil {
		return nil, err
	}
	var refusedChanges []string
	try := func(what string, err error) {
		if err != nil {
			refusedChanges = append(refusedChanges, what+": "+err.Error())
		}
	}
	missing := func(have, want []string) []string {
		var out []string
		for _, w := range want {
			if !slices.Contains(have, w) {
				out = append(out, w)
			}
		}
		return out
	}
	// Lists: new ones made, then every list brought in line (stronger steps
	// first), then lists gone from the draft deleted.
	for _, l := range next.Lists {
		if _, ok := orig.Lists.Find(l.Name); !ok {
			_, err := database.CreateList(block, l.Kind, l.Name, l.Where)
			try("making list "+l.Name, err)
		}
	}
	if orig, err = database.GetBlock(block); err != nil {
		return nil, err
	}
	for _, l := range next.Lists {
		o, ok := orig.Lists.Find(l.Name)
		if !ok {
			continue
		}
		for _, place := range db.Places {
			if l.Where.Has(place) && !o.Where.Has(place) {
				try("looking at "+place+" in "+l.Name, listWhere(cmd, database, block, l.Name, place, true))
			}
		}
		if add := missing(o.Entries, l.Entries); len(add) > 0 {
			try("adding to "+l.Name, listAdd(cmd, database, block, l.Name, add))
		}
		if !l.IsExceptions() {
			for _, a := range l.Actions {
				had := slices.IndexFunc(o.Actions, func(x db.ListAction) bool { return x.Name == a.Name })
				if had < 0 || o.Actions[had].Param != a.Param || o.Actions[had].Minutes != a.Minutes {
					try(a.Name+" in "+l.Name, listAction(cmd, database, block, l.Name, a.Name, true,
						&actionSettings{param: a.Param, minutes: a.Minutes}))
				}
				if had >= 0 && o.Actions[had].Distinct != a.Distinct {
					try("threshold of "+a.Name+" in "+l.Name, listThreshold(cmd, database, block, l.Name, a.Name, a.Distinct))
				}
				if had >= 0 && o.Actions[had].Hits != a.Hits {
					try("hits of "+a.Name+" in "+l.Name, listHits(cmd, database, block, l.Name, a.Name, a.Hits))
				}
				if had >= 0 && o.Actions[had].Rate != a.Rate {
					try("rate of "+a.Name+" in "+l.Name, listRate(cmd, database, block, l.Name, a.Name, a.Rate))
				}
			}
			for _, a := range o.Actions {
				if !slices.ContainsFunc(l.Actions, func(x db.ListAction) bool { return x.Name == a.Name }) {
					try(a.Name+" off in "+l.Name, listAction(cmd, database, block, l.Name, a.Name, false, nil))
				}
			}
		}
		if remove := missing(l.Entries, o.Entries); len(remove) > 0 {
			try("removing "+strings.Join(remove, ", ")+" from "+l.Name, listRemove(cmd, database, block, l.Name, remove))
		}
		for _, place := range db.Places {
			if !l.Where.Has(place) && o.Where.Has(place) {
				try("no longer looking at "+place+" in "+l.Name, listWhere(cmd, database, block, l.Name, place, false))
			}
		}
	}
	for _, o := range orig.Lists {
		if _, ok := next.Lists.Find(o.Name); !ok {
			try("deleting list "+o.Name, listDelete(cmd, database, block, o.Name))
		}
	}
	// Apps and window titles: adding is free, removing weakens.
	if add := missing(orig.Apps, next.Apps); len(add) > 0 {
		try("adding apps", addApps(database, block, add))
	}
	if remove := missing(next.Apps, orig.Apps); len(remove) > 0 {
		err := ensureEditable(database, block, false)
		if err == nil {
			err = database.RemoveApps(block, remove)
		}
		try("removing apps "+strings.Join(remove, ", "), err)
	}
	if add := missing(orig.Titles, next.Titles); len(add) > 0 {
		try("adding window titles", addTitles(database, block, add))
	}
	if remove := missing(next.Titles, orig.Titles); len(remove) > 0 {
		err := ensureEditable(database, block, false)
		if err == nil {
			err = database.RemoveTitles(block, remove)
		}
		try("removing window titles "+strings.Join(remove, ", "), err)
	}
	if err := reapplyIfActive(database, block); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	return refusedChanges, nil
}

func init() {
	addYesFlag(blockDraftCmd)
	blockCmd.AddCommand(blockDraftCmd)
}
