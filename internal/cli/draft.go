// draft.go — block drafts (§4.3): an opened block is always a draft;
// nothing applies until Save edits (owner 2026-11-17).
//
//	kc9k block draft B          # a draft copy to edit ("B (draft)")
//	kc9k block draft B save     # apply what was changed in it to B, through the gates
//	kc9k block draft B cancel   # drop it
//
// A draft is an inactive copy of what the block's Websites and Apps tabs
// hold: its lists (entries, scopes, actions, counts, test mode), phrase
// sets, social media settings, apps, window titles and their actions. It
// is never enforced (it can't be started). It keeps what the block was when
// it was made (its base), so Save applies only what was changed in the
// draft, each as the matching command would, on the block as it is now: a
// change made to the block meanwhile (an entry from the browser's popup, a
// recording) stays. On a locked block the weakening changes are refused
// and named; the rest apply.
//
// What the Breaks & allowance and Bypass prevention tabs set (allowance,
// Pomodoro, reward, text and delay breaks, the block's bypass prevention)
// is kept on the draft as the commands given (draftQueued), the last of
// each kind, and run at save through their own gates (2026-11-21, SPEC
// §4.3). Taking a break or an allowance session, the automation, the
// affected users and locks are changed on the block itself, at once.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// draftName is the name of block's draft.
func draftName(block string) string { return block + " (draft)" }

const settingDraftOf = db.SettingDraftOf // + draft name → its block

// draftOf returns the block a block is the draft of, or "".
func draftOf(database *db.DB, name string) string {
	return database.DraftOf(name)
}

// draftRefused lists the commands that change what a draft doesn't hold
// (§4.3: a draft holds what the block's Websites and Apps tabs show;
// saving carries only that), with how many arguments they take before they
// change anything (fewer only show). On a draft such a change would be lost
// at save, or (an allowance, an autostart) switch the draft on: refused.
var draftRefused = map[string]int{
	"allowance start": 1, "allowance end": 1,
	"schedule add": 1, "schedule remove": 1, "schedule break": 2, "schedule lock": 2,
	"autostart": 2,
	"break start": 1, "break end": 1, "break request": 1,
	"place rule": 1, "block users": 2, "automation": 3,
	"schedule apply": 2, "schedule take-off": 2, // §3.6 Schedule (2026-11-17); a draft joins no mode (mode add)
}

// draftQueued are the commands of the Breaks & allowance and Bypass
// prevention tabs: on a draft they are kept, the last of each kind, and run
// on the block at save (§4.3). Each with its kind and how many arguments it
// takes before it changes anything ("bypass" names its block by --block).
var draftQueued = map[string]struct {
	kind string
	args int
}{
	"allowance set": {"Allowance", 2}, "allowance remove": {"Allowance", 1},
	"pomodoro set": {"Pomodoro", 3}, "pomodoro off": {"Pomodoro", 1},
	"break reward": {"Reward", 2}, "break text": {"Text break", 2}, "break delay": {"Delay break", 2},
	"bypass set": {"Bypass prevention", 0}, "bypass off": {"Bypass prevention", 0},
}

// queuedEdit is one command kept on a draft until save.
type queuedEdit struct {
	Kind string   `json:"kind"`
	Args []string `json:"args"` // the command, naming the block (not its draft)
	Text string   `json:"text"` // what it sets, for the interface
}

// settingDraftQueue + draft name holds its queued edits (JSON).
const settingDraftQueue = "draft.queue."

// draftQueue returns the edits kept on a draft.
func draftQueue(database *db.DB, draft string) []queuedEdit {
	raw, _ := database.GetSetting(settingDraftQueue + draft)
	var q []queuedEdit
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &q)
	}
	return q
}

// queuedTarget returns the block a command of draftQueued changes, or "".
func queuedTarget(cmd *cobra.Command, args []string) (string, string) {
	path := strings.TrimPrefix(cmd.CommandPath(), rootCmd.Name()+" ")
	q, ok := draftQueued[path]
	if !ok {
		return "", ""
	}
	if strings.HasPrefix(path, "bypass ") {
		block, _ := cmd.Flags().GetString("block")
		return path, block
	}
	if len(args) < q.args || len(args) == 0 {
		return "", ""
	}
	return path, args[0]
}

// commandLine writes a command back as arguments, with block in place of
// draft: its path, arguments and the flags given.
func commandLine(cmd *cobra.Command, args []string, draft, block string) []string {
	line := strings.Fields(strings.TrimPrefix(cmd.CommandPath(), rootCmd.Name()+" "))
	for _, a := range args {
		if a == draft {
			a = block
		}
		line = append(line, a)
	}
	cmd.Flags().Visit(func(f *pflag.Flag) {
		if rootCmd.PersistentFlags().Lookup(f.Name) != nil {
			return // how the command was given (passwords on standard input), not what it sets
		}
		if slice, ok := f.Value.(pflag.SliceValue); ok {
			for _, v := range slice.GetSlice() {
				line = append(line, "--"+f.Name+"="+v)
			}
			return
		}
		v := f.Value.String()
		if v == draft {
			v = block
		}
		line = append(line, "--"+f.Name+"="+v)
	})
	return line
}

// queueOnDraft wraps the commands of draftQueued: on a draft the command
// is kept for save instead of run.
var queueOnce sync.Once

func queueOnDraft() {
	queueOnce.Do(func() {
		var wrap func(c *cobra.Command)
		wrap = func(c *cobra.Command) {
			path := strings.TrimPrefix(c.CommandPath(), rootCmd.Name()+" ")
			if _, ok := draftQueued[path]; ok && c.RunE != nil {
				run := c.RunE
				c.RunE = func(cmd *cobra.Command, args []string) error {
					if kept, err := keepOnDraft(cmd, args); kept || err != nil {
						return err
					}
					return run(cmd, args)
				}
			}
			for _, sub := range c.Commands() {
				wrap(sub)
			}
		}
		wrap(rootCmd)
	})
}

// keepOnDraft keeps a command of draftQueued given for a draft, replacing
// the one of its kind kept before; it reports whether it did.
func keepOnDraft(cmd *cobra.Command, args []string) (bool, error) {
	path, draft := queuedTarget(cmd, args)
	if draft == "" {
		return false, nil
	}
	database, err := openDB()
	if err != nil {
		return false, nil // the command says why
	}
	defer database.Close()
	block := draftOf(database, draft)
	if block == "" {
		return false, nil
	}
	line := commandLine(cmd, args, draft, block)
	// What it sets, in words: "1h, per week, refill 04:00".
	rest := []string{}
	for _, a := range line[len(strings.Fields(path)):] {
		name, value, flag := strings.Cut(strings.TrimPrefix(a, "--"), "=")
		switch {
		case a == block || a == "--block="+block || (flag && value == "true" && (name == "yes" || name == "no-confirm-ok")):
		case strings.HasPrefix(a, "--") && flag:
			rest = append(rest, name+" "+value)
		default:
			rest = append(rest, a)
		}
	}
	text := draftQueued[path].kind + ": " + strings.Join(rest, ", ")
	if strings.HasSuffix(path, " off") || strings.HasSuffix(path, " remove") || (len(rest) > 0 && rest[0] == "off") {
		text = draftQueued[path].kind + ": off"
	}
	q := slices.DeleteFunc(draftQueue(database, draft), func(e queuedEdit) bool { return e.Kind == draftQueued[path].kind })
	q = append(q, queuedEdit{Kind: draftQueued[path].kind, Args: line, Text: text})
	raw, err := json.Marshal(q)
	if err != nil {
		return true, err
	}
	if err := database.SetSetting(settingDraftQueue+draft, string(raw)); err != nil {
		return true, err
	}
	fmt.Printf("Kept in the draft of '%s' (%s): it applies with 'kc9k block draft %s save'.\n", block, text, block)
	return true, nil
}

// runQueued runs a kept command on the block, as it was given.
func runQueued(line []string) error {
	c, rest, err := rootCmd.Find(line)
	if err != nil {
		return err
	}
	c.Flags().VisitAll(func(f *pflag.Flag) {
		if slice, ok := f.Value.(pflag.SliceValue); ok {
			_ = slice.Replace(nil)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	})
	if err := c.ParseFlags(rest); err != nil {
		return err
	}
	args := c.Flags().Args()
	if err := c.ValidateArgs(args); err != nil {
		return err
	}
	if c.RunE == nil {
		return fmt.Errorf("'%s' can't be run", strings.Join(line, " "))
	}
	return c.RunE(c, args)
}

// refuseDraftEdit refuses a command of draftRefused on a draft, naming the
// block to change instead.
func refuseDraftEdit(cmd *cobra.Command, args []string) error {
	path := strings.TrimPrefix(cmd.CommandPath(), rootCmd.Name()+" ")
	block := ""
	if n, ok := draftRefused[path]; ok && len(args) >= n && len(args) > 0 {
		block = args[0]
	}
	if block == "" {
		return nil
	}
	database, err := openDB()
	if err != nil {
		return nil // the command says why
	}
	defer database.Close()
	if of := draftOf(database, block); of != "" {
		return fmt.Errorf("'%s' is a draft of '%s': a draft holds what the block's tabs set until it is saved; "+
			"its automation, affected users and taking a break apply at once, so change this on '%s' itself", block, of, of)
	}
	return nil
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
			if err := makeDraft(database, block, draft); err != nil {
				_ = dropDraft(database, draft)
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
			// A monk or Mode action in the draft was acknowledged when it was
			// set there (requireMonkAck): saving doesn't ask again.
			if f := cmd.Flags().Lookup("no-confirm-ok"); f != nil {
				_ = f.Value.Set("true")
			}
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
	if detail, err := database.GetBlock(draft); err == nil {
		for _, l := range detail.Lists { // test mode is kept by list ID
			_ = database.SetTestList(l.ID, false)
		}
		if err := database.RemoveBlock(draft); err != nil {
			return err
		}
	}
	if err := database.SetSetting(settingDraftBase+draft, ""); err != nil {
		return err
	}
	if err := database.SetSetting(settingDraftQueue+draft, ""); err != nil {
		return err
	}
	return database.SetSetting(settingDraftOf+draft, "")
}

// settingDraftBase + draft name keeps what the block was when the draft was
// made (draftContent as JSON).
const settingDraftBase = "draft.base."

// draftContent is what a draft holds of a block (§4.3).
type draftContent struct {
	Lists      db.Lists                      `json:"lists"`
	Test       []string                      `json:"test"` // lists in test mode, by name
	PhraseSets []db.PhraseSet                `json:"phrase_sets"`
	Apps       []string                      `json:"apps"`
	Titles     []string                      `json:"titles"`
	Social     []string                      `json:"social"`
	Triggers   map[string][]db.TriggerAction `json:"triggers"`
}

var triggerGroups = []string{db.TriggerApps, db.TriggerTitles}

// readDraftContent reads what a draft holds of a block, from the block.
func readDraftContent(database *db.DB, block string) (draftContent, error) {
	detail, err := database.GetBlock(block)
	if err != nil {
		return draftContent{}, err
	}
	c := draftContent{Lists: detail.Lists, Apps: detail.Apps, Titles: detail.Titles, Social: detail.Social,
		Triggers: map[string][]db.TriggerAction{}}
	testing := database.TestLists()
	for _, l := range detail.Lists {
		if slices.Contains(testing, l.ID) {
			c.Test = append(c.Test, l.Name)
		}
	}
	if c.PhraseSets, err = database.PhraseSets(block); err != nil {
		return draftContent{}, err
	}
	for _, g := range triggerGroups {
		if c.Triggers[g], err = database.TriggerActions(block, g); err != nil {
			return draftContent{}, err
		}
	}
	return c, nil
}

// makeDraft copies what a draft holds of block into draft, and keeps it as
// the draft's base.
func makeDraft(database *db.DB, block, draft string) error {
	base, err := readDraftContent(database, block)
	if err != nil {
		return err
	}
	if err := database.DuplicateBlock(block, draft); err != nil {
		return err
	}
	if err := database.SetSetting(settingDraftOf+draft, block); err != nil {
		return err
	}
	if hash, _ := database.BlockViewPassword(block); hash != "" {
		if err := database.SetBlockViewPassword(draft, hash); err != nil {
			return err
		}
	}
	for _, preset := range base.Social {
		if err := database.SetSocial(draft, preset, true); err != nil {
			return err
		}
	}
	for _, g := range triggerGroups {
		if err := database.SetTriggerActions(draft, g, base.Triggers[g]); err != nil {
			return err
		}
	}
	copied, err := database.GetBlock(draft)
	if err != nil {
		return err
	}
	for _, l := range copied.Lists {
		if slices.Contains(base.Test, l.Name) {
			if err := database.SetTestList(l.ID, true); err != nil {
				return err
			}
		}
	}
	raw, err := json.Marshal(base)
	if err != nil {
		return err
	}
	return database.SetSetting(settingDraftBase+draft, string(raw))
}

// draftBase is what the block was when draft was made; a draft made before
// bases were kept takes the block as it is now.
func draftBase(database *db.DB, block, draft string) (draftContent, error) {
	if raw, _ := database.GetSetting(settingDraftBase + draft); raw != "" {
		var c draftContent
		if err := json.Unmarshal([]byte(raw), &c); err == nil {
			if c.Triggers == nil {
				c.Triggers = map[string][]db.TriggerAction{}
			}
			return c, nil
		}
	}
	return readDraftContent(database, block)
}

// missing returns what want has and have doesn't, in want's order.
func missing(have, want []string) []string {
	var out []string
	for _, w := range want {
		if !slices.Contains(have, w) {
			out = append(out, w)
		}
	}
	return out
}

// only keeps the values that are (in) or aren't (!in) in have.
func only(values, have []string, in bool) []string {
	var out []string
	for _, v := range values {
		if slices.Contains(have, v) == in {
			out = append(out, v)
		}
	}
	return out
}

// saveDraft applies what was changed in the draft since it was made to
// block as it is now, each change through its command's gate; it returns
// the changes refused.
func saveDraft(cmd *cobra.Command, database *db.DB, block, draft string) ([]string, error) {
	base, err := draftBase(database, block, draft)
	if err != nil {
		return nil, err
	}
	next, err := readDraftContent(database, draft)
	if err != nil {
		return nil, err
	}
	orig, err := database.GetBlock(block)
	if err != nil {
		return nil, err
	}
	var refusedChanges []string
	try := func(what string, err error) {
		if err != nil {
			refusedChanges = append(refusedChanges, what+": "+err.Error())
		}
	}
	// Lists: new ones made, then every list brought in line with what
	// changed in the draft (stronger steps first), then lists gone from the
	// draft deleted.
	made := map[string]bool{}
	for _, l := range next.Lists {
		if _, inBase := base.Lists.Find(l.Name); inBase {
			continue
		}
		if _, ok := orig.Lists.Find(l.Name); !ok {
			_, err := database.CreateList(block, l.Kind, l.Name, l.Where)
			try("making list "+l.Name, err)
			made[l.Name] = err == nil
		}
	}
	if orig, err = database.GetBlock(block); err != nil {
		return nil, err
	}
	testing := database.TestLists()
	for _, l := range next.Lists {
		o, ok := orig.Lists.Find(l.Name)
		if !ok {
			continue
		}
		b, inBase := base.Lists.Find(l.Name)
		baseTest := slices.Contains(base.Test, l.Name)
		if !inBase || made[l.Name] {
			// New in the draft: everything it has is a change.
			b, baseTest = o, slices.Contains(testing, o.ID)
			b.Entries = nil
		}
		for _, place := range db.Places {
			if l.Where.Has(place) && !b.Where.Has(place) && !o.Where.Has(place) {
				try("looking at "+place+" in "+l.Name, listWhere(cmd, database, block, l.Name, place, true))
			}
		}
		if add := only(missing(b.Entries, l.Entries), o.Entries, false); len(add) > 0 {
			try("adding to "+l.Name, listAdd(cmd, database, block, l.Name, add))
		}
		if !l.IsExceptions() {
			for _, a := range l.Actions {
				was := slices.IndexFunc(b.Actions, func(x db.ListAction) bool { return x.Name == a.Name })
				if was >= 0 && b.Actions[was] == a {
					continue // not changed in the draft
				}
				had := slices.IndexFunc(o.Actions, func(x db.ListAction) bool { return x.Name == a.Name })
				if had < 0 || o.Actions[had].Param != a.Param || o.Actions[had].Minutes != a.Minutes {
					try(a.Name+" in "+l.Name, listAction(cmd, database, block, l.Name, a.Name, true,
						&actionSettings{param: a.Param, minutes: a.Minutes}))
				}
				cur := db.ListAction{Distinct: 1, Hits: 1}
				if had >= 0 {
					cur = o.Actions[had]
				}
				if cur.Distinct != a.Distinct {
					try("threshold of "+a.Name+" in "+l.Name, listThreshold(cmd, database, block, l.Name, a.Name, a.Distinct))
				}
				if cur.Hits != a.Hits {
					try("hits of "+a.Name+" in "+l.Name, listHits(cmd, database, block, l.Name, a.Name, a.Hits))
				}
				if cur.Rate != a.Rate {
					try("rate of "+a.Name+" in "+l.Name, listRate(cmd, database, block, l.Name, a.Name, a.Rate))
				}
			}
			for _, a := range b.Actions {
				gone := !slices.ContainsFunc(l.Actions, func(x db.ListAction) bool { return x.Name == a.Name })
				if gone && slices.ContainsFunc(o.Actions, func(x db.ListAction) bool { return x.Name == a.Name }) {
					try(a.Name+" off in "+l.Name, listAction(cmd, database, block, l.Name, a.Name, false, nil))
				}
			}
		}
		test := slices.Contains(next.Test, l.Name)
		if test != baseTest && test != slices.Contains(testing, o.ID) {
			err := ensureEditable(database, block, !test) // test mode on weakens
			if err == nil {
				err = database.SetTestList(o.ID, test)
			}
			try(map[bool]string{true: "test mode on in ", false: "test mode off in "}[test]+l.Name, err)
		}
		if remove := only(missing(l.Entries, b.Entries), o.Entries, true); len(remove) > 0 {
			try("removing "+strings.Join(remove, ", ")+" from "+l.Name, listRemove(cmd, database, block, l.Name, remove))
		}
		for _, place := range db.Places {
			if !l.Where.Has(place) && b.Where.Has(place) && o.Where.Has(place) {
				try("no longer looking at "+place+" in "+l.Name, listWhere(cmd, database, block, l.Name, place, false))
			}
		}
	}
	for _, b := range base.Lists {
		if _, ok := next.Lists.Find(b.Name); ok {
			continue
		}
		if _, ok := orig.Lists.Find(b.Name); ok {
			try("deleting list "+b.Name, listDelete(cmd, database, block, b.Name))
		}
	}
	// Apps and window titles: adding is free, removing weakens.
	if add := only(missing(base.Apps, next.Apps), orig.Apps, false); len(add) > 0 {
		try("adding apps", addApps(database, block, add))
	}
	if remove := only(missing(next.Apps, base.Apps), orig.Apps, true); len(remove) > 0 {
		err := ensureEditable(database, block, false)
		if err == nil {
			err = database.RemoveApps(block, remove)
		}
		try("removing apps "+strings.Join(remove, ", "), err)
	}
	if add := only(missing(base.Titles, next.Titles), orig.Titles, false); len(add) > 0 {
		try("adding window titles", addTitles(database, block, add))
	}
	if remove := only(missing(next.Titles, base.Titles), orig.Titles, true); len(remove) > 0 {
		err := ensureEditable(database, block, false)
		if err == nil {
			err = database.RemoveTitles(block, remove)
		}
		try("removing window titles "+strings.Join(remove, ", "), err)
	}
	// Social media settings: on is free, off weakens.
	for _, preset := range only(missing(base.Social, next.Social), orig.Social, false) {
		err := ensureEditable(database, block, true)
		if err == nil {
			err = database.SetSocial(block, preset, true)
		}
		try(preset+" on", err)
	}
	for _, preset := range only(missing(next.Social, base.Social), orig.Social, true) {
		err := ensureEditable(database, block, false)
		if err == nil {
			err = database.SetSocial(block, preset, false)
		}
		try(preset+" off", err)
	}
	// What the apps and window titles do: the actions changed in the draft,
	// on the block's; weakening if any it has goes or gets shorter.
	for _, g := range triggerGroups {
		try(g+" actions", saveDraftTriggers(database, block, g, base.Triggers[g], next.Triggers[g]))
	}
	if err := saveDraftPhraseSets(database, block, base.PhraseSets, next.PhraseSets, try); err != nil {
		return refusedChanges, err
	}
	// The Breaks & allowance and Bypass prevention tabs: each kept command
	// runs now, through its own gate.
	for _, e := range draftQueue(database, draft) {
		try(e.Text, runQueued(e.Args))
	}
	if err := reapplyIfActive(database, block); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	return refusedChanges, nil
}

// saveDraftTriggers applies what was changed in a draft's trigger actions
// of one group to the block's: as 'kc9k block trigger', a new action or a
// longer time is free; an action gone, shorter, or once instead of for a
// while weakens.
func saveDraftTriggers(database *db.DB, block, group string, base, next []db.TriggerAction) error {
	if slices.Equal(base, next) {
		return nil
	}
	have, err := database.TriggerActions(block, group)
	if err != nil {
		return err
	}
	want := slices.Clone(have)
	for _, a := range next {
		if b, ok := db.FindTriggerAction(base, a.Name); ok && b == a {
			continue
		}
		want = slices.DeleteFunc(want, func(x db.TriggerAction) bool { return x.Name == a.Name })
		want = append(want, a)
	}
	for _, b := range base {
		if _, ok := db.FindTriggerAction(next, b.Name); !ok {
			want = slices.DeleteFunc(want, func(x db.TriggerAction) bool { return x.Name == b.Name })
		}
	}
	if slices.Equal(have, want) {
		return nil
	}
	strengthens := true
	for _, old := range have {
		a, ok := db.FindTriggerAction(want, old.Name)
		if !ok || (a.Minutes != old.Minutes && (old.Minutes == 0 || a.Minutes < old.Minutes)) {
			strengthens = false
		}
	}
	if err := ensureEditable(database, block, strengthens); err != nil {
		return err
	}
	return database.SetTriggerActions(block, group, want)
}

// saveDraftPhraseSets applies what was changed in the draft's phrase sets
// since it was made (base → next) to the block's sets as they are now, as
// saveDraft does lists: a set new in the draft is copied in, each other
// change goes through the same gate as its phrase-set command (stronger
// steps first), and a set gone from the draft is deleted.
func saveDraftPhraseSets(database *db.DB, block string, base, nextSets []db.PhraseSet, try func(string, error)) error {
	origSets, err := database.PhraseSets(block)
	if err != nil {
		return err
	}
	find := func(sets []db.PhraseSet, name string) (db.PhraseSet, bool) {
		i := slices.IndexFunc(sets, func(s db.PhraseSet) bool { return strings.EqualFold(s.Name, name) })
		if i < 0 {
			return db.PhraseSet{}, false
		}
		return sets[i], true
	}
	gate := func(strengthens bool, change func() error) error {
		if err := ensureEditable(database, block, strengthens); err != nil {
			return err
		}
		return change()
	}
	for _, n := range nextSets {
		o, ok := find(origSets, n.Name)
		b, inBase := find(base, n.Name)
		if !ok {
			if !inBase { // new in the draft (one deleted from the block meanwhile stays deleted)
				try("making phrase set "+n.Name, gate(true, func() error { return database.CopyPhraseSet(n.ID, block) }))
			}
			continue
		}
		if !inBase {
			b = o // made meanwhile under the same name: the draft's set is the change
		}
		label := "phrase set " + o.Name
		// Stronger first: phrases added, places ticked, actions on or
		// stricter, lower thresholds, test mode off.
		for _, p := range n.Phrases {
			if _, inB := storedPhrase(b, p); inB {
				continue
			}
			if _, has := storedPhrase(o, p); !has {
				try("adding "+p.String()+" to "+label, gate(true, func() error {
					_, err := database.AddPhrase(o.ID, p)
					return err
				}))
			}
		}
		where := o.Where
		for _, place := range []string{db.PlaceURL, db.PlaceTitle, db.PlaceText} {
			if n.Where.Has(place) && !b.Where.Has(place) && !where.Has(place) {
				next := where.With(place, true)
				err := gate(true, func() error { return database.SetPhraseSetWhere(o.ID, next) })
				if try("looking at "+place+" in "+label, err); err == nil {
					where = next
				}
			}
		}
		for _, a := range n.Actions {
			if was, inB := b.Action(a.Name); inB && was == a {
				continue // not changed in the draft
			}
			cur, has := o.Action(a.Name)
			if !has || cur.Param != a.Param || cur.Minutes != a.Minutes {
				stricter := !has || db.ParamsStricter(a.Name, cur.Param, a.Param, cur.Minutes, a.Minutes)
				try(a.Name+" in "+label, gate(stricter, func() error {
					return database.SetPhraseSetAction(o.ID, a.Name, true, a.Param, a.Minutes)
				}))
			}
			// A new action is stricter at any threshold; an old one when
			// it needs no more phrases than before.
			if !has && a.Distinct > 1 || has && a.Distinct != cur.Distinct {
				try("threshold of "+a.Name+" in "+label, gate(!has || a.Distinct <= cur.Distinct, func() error {
					return database.SetPhraseSetThreshold(o.ID, a.Name, a.Distinct)
				}))
			}
		}
		if b.Test && !n.Test && o.Test {
			try("test mode off in "+label, gate(true, func() error { return database.SetPhraseSetTest(o.ID, false) }))
		}
		// Then what weakens: phrases removed, places unticked, actions
		// off, test mode on.
		for _, p := range b.Phrases {
			if _, inN := storedPhrase(n, p); inN {
				continue
			}
			if stored, has := storedPhrase(o, p); has {
				try("removing "+p.String()+" from "+label, gate(false, func() error {
					_, err := database.RemovePhrase(o.ID, stored)
					return err
				}))
			}
		}
		for _, place := range []string{db.PlaceURL, db.PlaceTitle, db.PlaceText} {
			if !n.Where.Has(place) && b.Where.Has(place) && where.Has(place) {
				next := where.With(place, false)
				err := gate(false, func() error { return database.SetPhraseSetWhere(o.ID, next) })
				if try("no longer looking at "+place+" in "+label, err); err == nil {
					where = next
				}
			}
		}
		for _, a := range b.Actions {
			if _, inN := n.Action(a.Name); inN {
				continue
			}
			if _, has := o.Action(a.Name); has {
				try(a.Name+" off in "+label, gate(false, func() error {
					return database.SetPhraseSetAction(o.ID, a.Name, false, "", 0)
				}))
			}
		}
		if !b.Test && n.Test && !o.Test {
			try("test mode on in "+label, gate(false, func() error { return database.SetPhraseSetTest(o.ID, true) }))
		}
	}
	for _, b := range base {
		if _, ok := find(nextSets, b.Name); ok {
			continue
		}
		if o, ok := find(origSets, b.Name); ok {
			try("deleting phrase set "+o.Name, gate(len(o.Phrases) == 0, func() error { return database.DeletePhraseSet(o.ID) }))
		}
	}
	return nil
}

func init() {
	addYesFlag(blockDraftCmd)
	addMonkAckFlag(blockDraftCmd)
	blockCmd.AddCommand(blockDraftCmd)
}
