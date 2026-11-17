// weblists.go — a block's website lists: blacklists and exceptions.
//
//	kc9k list show <block> [list]
//	kc9k list create <block> <list> [--exceptions] [--url] [--title] [--text]
//	kc9k list rename <block> <list> <new name>
//	kc9k list delete <block> <list>
//	kc9k list add <block> <list> <entries...>
//	kc9k list remove <block> <list> <entries...>
//	kc9k list where <block> <list> url|title|text on|off
//	kc9k list action <block> <list> block|greyscale|no-images on|off
//	kc9k list threshold <block> <list> <action> <count>
//	kc9k list hits <block> <list> <action> <count>
//	kc9k list export <block> <list>
//	kc9k list import <block> <list> <file>
//
// Each list ticks where it looks: the address (url), the page title (the
// tab title) and the page text. A blacklist's actions apply to the pages it
// matches; an exception list spares the pages it matches from every
// blacklist of its block. What strengthens a block is always allowed: adding
// to a blacklist, ticking its places and actions, lowering its thresholds,
// and trimming exceptions. Everything else loosens it and is gated like any
// loosening edit (see gateEdit).
package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

// findList returns a block and its named list.
func findList(database *db.DB, block, list string) (*db.BlockDetail, db.WebList, error) {
	detail, err := database.GetBlock(block)
	if err != nil {
		return nil, db.WebList{}, err
	}
	l, ok := detail.Lists.Find(list)
	if !ok {
		var names []string
		for _, have := range detail.Lists {
			names = append(names, "'"+have.Name+"'")
		}
		return nil, db.WebList{}, fmt.Errorf("block '%s' has no list named '%s' (it has %s)", block, list, strings.Join(names, ", "))
	}
	return detail, l, nil
}

// prepareEntries checks each entry and normalizes it. Lists have no types
// (§3.2): an entry is a site or URL entry (normalized so it reaches
// /etc/hosts and the firewall and is found again when removed) or else a
// rendered term, whatever the list looks at, so changing the scope never
// leaves an entry that doesn't fit.
func prepareEntries(where db.Where, entries []string) ([]string, error) {
	if !where.Any() {
		return nil, fmt.Errorf("the list doesn't look anywhere; tick url, title or text first")
	}
	out := make([]string, 0, len(entries))
	for _, raw := range entries {
		e := strings.TrimSpace(raw)
		if e == "" {
			continue
		}
		if urlEntry(e) {
			e = normalizeURLEntry(e)
		} else if checkPageKeywords([]string{e}) == nil {
			e = normalizeCombination(e)
		} else {
			return nil, fmt.Errorf("invalid entry %q: %s", raw, entryHelp)
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no entries given")
	}
	return out, nil
}

// urlEntry reports whether an entry is a site or URL entry rather than a
// rendered term (blocker.IsAddressEntry).
func urlEntry(e string) bool {
	return blocker.IsAddressEntry(normalizeURLEntry(e))
}

// normalizeURLEntry keeps a lone domain normalized (lower case, no scheme,
// www. or trailing slash), so it reaches /etc/hosts and the firewall and is
// found again when removed.
func normalizeURLEntry(e string) string {
	// A YouTube playlist's address, however pasted, is kept one way, so it
	// is found again when removed (§3.2 Exception list).
	if id, ok := db.PlaylistInEntry(e); ok {
		return db.PlaylistException(id)
	}
	bare := e
	if i := strings.Index(bare, "://"); i > 0 {
		bare = bare[i+3:]
	}
	bare = strings.TrimSuffix(bare, "/")
	if bare != "" && !strings.ContainsAny(bare, "/*") {
		if host := blocker.NormalizeDomain(bare); blocker.IsValidDomain(host) {
			return host
		}
	}
	return e
}

var entryHelp = fmt.Sprintf("an entry is a site (example.com), a URL entry (example.com/x), a pattern with * (*.xxx, *casino*), or a rendered term: a word or words on one line of at most %d characters, not only *", maxPageKeyword)

// settle re-applies the layers after an edit, and deactivates a block the
// edit left with nothing to enforce.
func settle(database *db.DB, name string) error {
	detail, err := database.GetBlock(name)
	if err != nil {
		return err
	}
	if detail.Active && detail.Empty() {
		if err := database.DeactivateBlock(name); err != nil {
			return err
		}
	}
	return reapplyIfActive(database, name)
}

func listEdit(action string, l db.WebList, values ...string) db.Edit {
	return db.Edit{Action: action, List: l.ID, ListName: l.Name, Values: values}
}

// listAdd adds entries to a list: stricter for a blacklist, looser for an
// exception list.
func listAdd(cmd *cobra.Command, database *db.DB, block, list string, entries []string) error {
	_, l, err := findList(database, block, list)
	if err != nil {
		return err
	}
	if err := refuseCombinations(entries); err != nil {
		return err
	}
	entries, err = prepareEntries(l.Where, entries)
	if err != nil {
		return err
	}
	proceed, err := gateEdit(cmd, database, block, !l.IsExceptions(), listEdit(db.EditListAdd, l, entries...))
	if err != nil || !proceed {
		return err
	}
	if err := database.AddListEntries(l.ID, entries); err != nil {
		return err
	}
	notePlaylists(block, l, entries)
	return settle(database, block)
}

// notePlaylists says what a YouTube playlist's address does in an exception
// list (§3.2): it lets that playlist's videos through, and only them.
func notePlaylists(block string, l db.WebList, entries []string) {
	if !l.IsExceptions() || !l.Where.URL {
		return
	}
	for _, e := range entries {
		if id, ok := db.PlaylistInEntry(e); ok {
			fmt.Printf("Block '%s' lets the playlist %s through: its page and its videos, no other video.\n", block, id)
		}
	}
}

// listRemove removes entries from a list: looser for a blacklist, stricter
// for an exception list.
func listRemove(cmd *cobra.Command, database *db.DB, block, list string, entries []string) error {
	_, l, err := findList(database, block, list)
	if err != nil {
		return err
	}
	entries = storedEntries(l.Entries, entries)
	if err := requireAll("entry", block, l.Entries, entries); err != nil {
		return err
	}
	proceed, err := gateEdit(cmd, database, block, l.IsExceptions(), listEdit(db.EditListRemove, l, entries...))
	if err != nil || !proceed {
		return err
	}
	if err := database.RemoveListEntries(l.ID, entries); err != nil {
		return err
	}
	return settle(database, block)
}

// storedEntries returns entries as the list or cluster stores them: domains
// and old word combinations are kept normalized, so look them up the same
// way.
func storedEntries(have, entries []string) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		switch {
		case containsString(have, e):
			out[i] = e
		case containsString(have, normalizeURLEntry(e)):
			out[i] = normalizeURLEntry(e)
		default:
			out[i] = normalizeCombination(e)
		}
	}
	return out
}

// refuseCombinations refuses word combinations typed as entries of a list
// or cluster: they are phrases now, in phrase sets (old files are still
// read, and moved). A site or URL entry may have brackets.
func refuseCombinations(entries []string) error {
	for _, e := range entries {
		if urlEntry(e) {
			continue
		}
		if boxes, ok := db.ParseCombination(e); ok {
			hint := ""
			if phrases, err := db.CombinationPhrases(boxes); err == nil && len(phrases) > 0 {
				var texts []string
				for _, p := range phrases {
					texts = append(texts, fmt.Sprintf("%q", p.String()))
				}
				hint = ", as " + strings.Join(texts, " ")
			}
			return fmt.Errorf("%q is a word combination; these are phrases now: add it to a phrase set with 'kc9k phrase-set add'%s", e, hint)
		}
	}
	return nil
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// listWhere ticks or unticks a place.
func listWhere(cmd *cobra.Command, database *db.DB, block, list, place string, on bool) error {
	if !db.IsPlace(place) {
		return fmt.Errorf("unknown place %q (use %s)", place, strings.Join(db.Places, ", "))
	}
	_, l, err := findList(database, block, list)
	if err != nil {
		return err
	}
	if l.Where.Has(place) == on {
		return nil
	}
	if place == db.PlaceAds && on && l.IsExceptions() {
		return fmt.Errorf("exception lists don't look at ads: the ad trigger blocks pages, it never spares them")
	}
	next := l.Where.With(place, on)
	if !next.Any() {
		return fmt.Errorf("list '%s' needs to look somewhere; tick another place first", list)
	}
	value := "off"
	if on {
		value = "on"
	}
	// Looking in more places makes a blacklist stricter and an exception
	// list looser.
	proceed, err := gateEdit(cmd, database, block, on != l.IsExceptions(), listEdit(db.EditListWhere, l, place, value))
	if err != nil || !proceed {
		return err
	}
	if err := database.SetListWhere(l.ID, next); err != nil {
		return err
	}
	return settle(database, block)
}

// actionSettings are what an action needs: an address (redirect), an image
// (image) or minutes (monk, and optionally logout and shutdown).
type actionSettings struct {
	param   string
	minutes int
}

// readActionSettings reads --url, --image and --for for an action. An image
// file is stored (see storeImage); its name becomes the parameter.
func readActionSettings(cmd *cobra.Command, action string) (actionSettings, error) {
	var s actionSettings
	url, _ := cmd.Flags().GetString("url")
	image, _ := cmd.Flags().GetString("image")
	length, _ := cmd.Flags().GetString("for")
	switch {
	case url != "" && action != db.ActionRedirect:
		return s, fmt.Errorf("--url goes with the redirect action")
	case image != "" && action != db.ActionImage:
		return s, fmt.Errorf("--image goes with the image action")
	}
	s.param = url
	if image != "" {
		name, err := storeImage(image)
		if err != nil {
			return s, err
		}
		s.param = name
	}
	if length != "" {
		d, err := parseUserDuration(length)
		if err != nil || d < 0 || d%time.Minute != 0 {
			return s, fmt.Errorf("invalid --for %q: give whole minutes, like 30m or 2h", length)
		}
		s.minutes = int(d / time.Minute)
	}
	return s, db.CheckActionParams(action, s.param, s.minutes)
}

// listAction turns a blacklist's action on or off, or changes what it needs
// (set, for actions that take an address, an image or minutes). A setting
// that can only make the action stricter (a longer time, another image) is
// always allowed; another address loosens it.
func listAction(cmd *cobra.Command, database *db.DB, block, list, action string, on bool, set *actionSettings) error {
	if !db.IsAction(action) {
		return fmt.Errorf("unknown action %q (use %s)", action, strings.Join(db.ActionNames, ", "))
	}
	_, l, err := findList(database, block, list)
	if err != nil {
		return err
	}
	if l.IsExceptions() {
		return fmt.Errorf("'%s' is an exception list; only blacklists have actions", list)
	}
	if on && action == db.ActionMonk {
		if err := requireMonkAck(cmd, "a list's monk action"); err != nil {
			return err
		}
	}
	current, has := l.Action(action)
	switch {
	case on && has && set != nil && (set.param != current.Param || set.minutes != current.Minutes):
		strengthens := db.ParamsStricter(action, current.Param, set.param, current.Minutes, set.minutes)
		proceed, err := gateEdit(cmd, database, block, strengthens,
			listEdit(db.EditListSetParams, l, action, set.param, strconv.Itoa(set.minutes)))
		if err != nil || !proceed {
			return err
		}
		if err := database.SetListActionParams(l.ID, action, set.param, set.minutes); err != nil {
			return err
		}
		return settle(database, block)
	case has == on:
		return nil
	case !on:
		proceed, err := gateEdit(cmd, database, block, false, listEdit(db.EditListUnsetAction, l, action))
		if err != nil || !proceed {
			return err
		}
		err = database.SetListAction(l.ID, action, false)
		if err != nil {
			return err
		}
		return settle(database, block)
	}
	if set == nil {
		set = &actionSettings{}
	}
	if err := database.SetListActionParams(l.ID, action, set.param, set.minutes); err != nil {
		return err
	}
	return settle(database, block)
}

// maxImageSize bounds a picture for the image action.
const maxImageSize = 4 << 20

// storeImage copies a picture into db.ImagesDir under its SHA-256 and type,
// for the daemon to serve, and returns that name.
func storeImage(path string) (string, error) {
	data, err := readAsCaller(path)
	if err != nil {
		return "", err
	}
	if len(data) > maxImageSize {
		return "", fmt.Errorf("%s is larger than %d MB", path, maxImageSize>>20)
	}
	ext := map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/gif": "gif", "image/webp": "webp"}[http.DetectContentType(data)]
	if ext == "" {
		return "", fmt.Errorf("%s isn't a PNG, JPEG, GIF or WebP picture", path)
	}
	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:]) + "." + ext
	dir := db.ImagesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return "", fmt.Errorf("failed to store the image: %w", err)
	}
	return name, nil
}

// listThreshold sets how many different entries a page's text needs for an
// action. Raising it loosens the list.
func listThreshold(cmd *cobra.Command, database *db.DB, block, list, action string, n int) error {
	return listCount(cmd, database, block, list, action, n, false)
}

// listHits sets how many times in all a list's entries must occur in a
// page's text for an action. Raising it loosens the list.
func listHits(cmd *cobra.Command, database *db.DB, block, list, action string, n int) error {
	return listCount(cmd, database, block, list, action, n, true)
}

func listCount(cmd *cobra.Command, database *db.DB, block, list, action string, n int, hits bool) error {
	if !db.IsAction(action) {
		return fmt.Errorf("unknown action %q (use %s)", action, strings.Join(db.ActionNames, ", "))
	}
	if n < 1 || n > db.MaxThreshold {
		return fmt.Errorf("a threshold is a number from 1 to %d", db.MaxThreshold)
	}
	_, l, err := findList(database, block, list)
	if err != nil {
		return err
	}
	a, ok := l.Action(action)
	if !ok {
		return fmt.Errorf("list '%s' doesn't take the %s action; turn it on first", list, action)
	}
	current, edit, set := a.Distinct, db.EditListSetThreshold, database.SetListThreshold
	if hits {
		current, edit, set = a.Hits, db.EditListSetHits, database.SetListHits
	}
	if n == current {
		return nil
	}
	proceed, err := gateEdit(cmd, database, block, n < current, listEdit(edit, l, action, strconv.Itoa(n)))
	if err != nil || !proceed {
		return err
	}
	if err := set(l.ID, action, n); err != nil {
		return err
	}
	return settle(database, block)
}

// listDelete removes a list: looser for a blacklist with entries.
func listDelete(cmd *cobra.Command, database *db.DB, block, list string) error {
	_, l, err := findList(database, block, list)
	if err != nil {
		return err
	}
	strengthens := l.IsExceptions() || len(l.Entries) == 0
	proceed, err := gateEdit(cmd, database, block, strengthens, listEdit(db.EditListDelete, l))
	if err != nil || !proceed {
		return err
	}
	if err := database.DeleteList(l.ID); err != nil {
		return err
	}
	return settle(database, block)
}

// readEntryFile reads a file with one entry per line; # starts a comment.
func readEntryFile(path string) ([]string, error) {
	data, err := readAsCaller(path)
	if err != nil {
		return nil, err
	}
	var entries []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			entries = append(entries, line)
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("%s has no entries", path)
	}
	return entries, nil
}

func parseOnOff(value string) (bool, error) {
	switch strings.ToLower(value) {
	case "on":
		return true, nil
	case "off":
		return false, nil
	}
	return false, fmt.Errorf("use 'on' or 'off'")
}

func describeList(l db.WebList) string {
	kind := "blacklist"
	if l.IsExceptions() {
		kind = "exceptions"
	}
	s := fmt.Sprintf("%s (%s) looks at: %s", l.Name, kind, l.Where)
	if !l.IsExceptions() {
		var acts []string
		for _, a := range l.Actions {
			text := a.Name
			if a.Param != "" {
				text += " " + a.Param
			}
			if a.Minutes > 0 {
				text += fmt.Sprintf(" for %s", time.Duration(a.Minutes)*time.Minute)
			}
			if l.Where.Text {
				text += fmt.Sprintf(" at %d different, %d in all", a.Distinct, a.Hits)
			}
			acts = append(acts, text)
		}
		s += "; actions: " + strings.Join(acts, ", ")
	}
	return s
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Manage a block's website lists (blacklists and exceptions)",
	Long: `A block has blacklists and exception lists. Each list ticks where it looks
(its scope): the page's address (url), its title (the tab title) and its
text. Lists have no types: sites, URL entries and rendered terms share a
list, and each entry is matched by the list's scope.

Sites are domains (reddit.com), URL entries paths (reddit.com/r/funny) or
patterns (*.xxx, *casino*, *.* for every site). With url ticked they match
the whole address and also go to /etc/hosts, the firewall and the browser
policy. Anything else is a rendered term (porn, long cat): with url it
matches words in the address. With title or text every entry is a rendered
term: a plain word counts only on its own ("porn" misses "pornhub"); a *
matches inside words ("*porn*" hits "pornhub").

Words that must occur together are phrases, kept in phrase sets
('kc9k phrase-set').

A blacklist's actions (block, greyscale, no-images) apply to the pages it
matches. A match in the address or the title takes every action; in the
text, each action needs its number of different entries (threshold) and of
times they occur in all (hits). An exception
list spares the pages it matches from all of its block's blacklists.

A block with exceptions by title or text is enforced by the extension only,
since only it reads a page before deciding.`,
}

var listShowCmd = &cobra.Command{
	Use:   "show [block] [list]",
	Short: "Show a block's lists, or one list's entries",
	Args:  cobra.RangeArgs(1, 2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 2 {
			if err := revealBlock(database, args[0]); err != nil {
				return err
			}
			_, l, err := findList(database, args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Println(describeList(l))
			for _, e := range l.Entries {
				fmt.Println("  " + e)
			}
			return nil
		}
		detail, err := database.GetBlock(args[0])
		if err != nil {
			return err
		}
		for _, l := range detail.Lists {
			fmt.Printf("%s; %d entries\n", describeList(l), len(l.Entries))
		}
		return nil
	}),
}

var listCreateCmd = &cobra.Command{
	Use:   "create [block] [list]",
	Short: "Add a blacklist (or, with --exceptions, an exception list) to a block",
	Long: `Adds an empty list that looks where --url, --title and --text say (any
mix; --url alone if none). A list holds sites, URL entries and rendered
terms together; see 'kc9k list --help'. A new blacklist blocks what it
matches; choose other actions with 'kc9k list action'.

--terms is an old alias kept for scripts: lists have no types any more, so
it changes nothing.`,
	Args: cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		exceptions, _ := cmd.Flags().GetBool("exceptions")
		var where db.Where
		where.URL, _ = cmd.Flags().GetBool("url")
		where.Title, _ = cmd.Flags().GetBool("title")
		where.Text, _ = cmd.Flags().GetBool("text")
		where.Ads, _ = cmd.Flags().GetBool("ads")
		if !where.Any() {
			where.URL = true
		}
		kind := db.ListBlacklist
		if exceptions {
			kind = db.ListExceptions
			if where.Ads {
				return fmt.Errorf("exception lists don't look at ads: the ad trigger blocks pages, it never spares them")
			}
		}
		if _, err := database.CreateList(args[0], kind, args[1], where); err != nil {
			return err
		}
		fmt.Printf("List '%s' added to block '%s' (%s; looks at: %s).\n", args[1], args[0], kind, where)
		return nil
	}),
}

var listRenameCmd = &cobra.Command{
	Use:   "rename [block] [list] [new name]",
	Short: "Rename a list",
	Args:  cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		_, l, err := findList(database, args[0], args[1])
		if err != nil {
			return err
		}
		return database.RenameList(l.ID, args[2])
	}),
}

var listDeleteCmd = &cobra.Command{
	Use:   "delete [block] [list]",
	Short: "Remove a list with its entries",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		return listDelete(cmd, database, args[0], args[1])
	}),
}

var listAddCmd = &cobra.Command{
	Use:   "add [block] [list] [entries...]",
	Short: "Add entries to a list",
	Args:  cobra.MinimumNArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		return listAdd(cmd, database, args[0], args[1], args[2:])
	}),
}

var listRemoveCmd = &cobra.Command{
	Use:   "remove [block] [list] [entries...]",
	Short: "Remove entries from a list",
	Args:  cobra.MinimumNArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		return listRemove(cmd, database, args[0], args[1], args[2:])
	}),
}

var listWhereCmd = &cobra.Command{
	Use:     "scope [block] [list] [url|title|text] [on|off]",
	Aliases: []string{"where"},
	Short:   "Choose where a list looks: the address, the page title, the page text",
	Args:    cobra.ExactArgs(4),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		on, err := parseOnOff(args[3])
		if err != nil {
			return err
		}
		return listWhere(cmd, database, args[0], args[1], strings.ToLower(args[2]), on)
	}),
}

var listActionCmd = &cobra.Command{
	Use:   "action [block] [list] [" + strings.Join(db.ActionNames, "|") + "] [on|off]",
	Short: "Choose what a blacklist does to the pages it matches",
	Long: `Actions, in any combination:

  block          show the block page
  greyscale      show the page in greyscale
  no-images      hide the page's images
  mute           switch the tab's sound off
  no-video       stop and hide videos and players
  dark           show a light page dark
  redirect       open another address instead (--url https://...)
  image          show a picture instead (--image FILE: PNG, JPEG, GIF or WebP)
  close-tab      close the tab
  close-browser  close the browser
  logout         close the browser and log out (--for 1h: stay logged out)
  shutdown       close the browser and shut down (--for 1h: stay off)
  monk           close the browser and start monk mode (--for 1h, required)

When several apply, the strongest page action wins (close-tab, then image,
redirect, block). Device actions only follow from the page itself, never
from something embedded in another page. A one-off logout or shutdown
happens at most every 10 minutes; staying off or logged out goes through
Frozen Chicken (at most a day). Monk mode blocks every website and cuts the
computer's internet connection for its time (see 'kc9k monk').

Run 'on' again with other settings to change them: a longer time is always
allowed, another address weakens the list.`,
	Args: cobra.ExactArgs(4),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		on, err := parseOnOff(args[3])
		if err != nil {
			return err
		}
		action := strings.ToLower(args[2])
		var set *actionSettings
		if on {
			settings, err := readActionSettings(cmd, action)
			if err != nil {
				return err
			}
			set = &settings
		} else if cmd.Flags().Changed("url") || cmd.Flags().Changed("image") || cmd.Flags().Changed("for") {
			return fmt.Errorf("--url, --image and --for only go with 'on'")
		}
		return listAction(cmd, database, args[0], args[1], action, on, set)
	}),
}

var listThresholdCmd = &cobra.Command{
	Use:   "threshold [block] [list] [action] [count]",
	Short: "How many different entries a page's text needs before an action applies",
	Long: `For a blacklist that looks at page text: how many of its different entries
a page must contain before the action applies (repeats count once). For
example greyscale at 1 and block at 3. Lowering it is always allowed; raising
it weakens the list.`,
	Args: cobra.ExactArgs(4),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		n, err := strconv.Atoi(args[3])
		if err != nil {
			return fmt.Errorf("a threshold is a number from 1 to %d", db.MaxThreshold)
		}
		return listThreshold(cmd, database, args[0], args[1], strings.ToLower(args[2]), n)
	}),
}

var listHitsCmd = &cobra.Command{
	Use:   "hits [block] [list] [action] [count]",
	Short: "How many times in all a page's text must contain the list's entries",
	Long: `For a blacklist that looks at page text: how many times its entries must
occur in a page's text, repeats counted, before the action applies. It works
together with the threshold of different entries: both must be met. For
example threshold 1 and hits 5: one word five times, or five words once.
Lowering it is always allowed; raising it weakens the list.`,
	Args: cobra.ExactArgs(4),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		n, err := strconv.Atoi(args[3])
		if err != nil {
			return fmt.Errorf("a count is a number from 1 to %d", db.MaxThreshold)
		}
		return listHits(cmd, database, args[0], args[1], strings.ToLower(args[2]), n)
	}),
}

var listRateCmd = &cobra.Command{
	Use:   "rate [block] [list] [action] [percent|off]",
	Short: "What share of a page's words must be the list's entries (pages of 200 words or more)",
	Long: `For a blacklist that looks at page text: matches divided by the page's
words, in percent (0.1 to 100), that the action also needs. Pages under 200
words never meet a rate. Threshold, hit count and rate must all be met
(R-LIST-6). Lowering it or 'off' is always allowed; setting or raising it
weakens the list.`,
	Args: cobra.ExactArgs(4),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		n := 0
		if args[3] != "off" {
			pct, err := strconv.ParseFloat(strings.TrimSuffix(args[3], "%"), 64)
			if err != nil || pct < 0.1 || pct > 100 {
				return fmt.Errorf("a rate is a percent from 0.1 to 100, or off")
			}
			n = int(pct*10 + 0.5)
		}
		return listRate(cmd, database, args[0], args[1], strings.ToLower(args[2]), n)
	}),
}

// listRate sets an action's rate (per 1,000 words, 0: none). A new or
// higher rate weakens; a lower one or none strengthens (§3.5).
func listRate(cmd *cobra.Command, database *db.DB, block, list, action string, n int) error {
	_, l, err := findList(database, block, list)
	if err != nil {
		return err
	}
	a, ok := l.Action(action)
	if !ok {
		return fmt.Errorf("list '%s' doesn't take the %s action; turn it on first", list, action)
	}
	if n == a.Rate {
		return nil
	}
	strengthens := n == 0 || (a.Rate != 0 && n < a.Rate)
	proceed, err := gateEdit(cmd, database, block, strengthens, listEdit(db.EditListSetRate, l, action, strconv.Itoa(n)))
	if err != nil || !proceed {
		return err
	}
	if err := database.SetListRate(l.ID, action, n); err != nil {
		return err
	}
	return settle(database, block)
}

var listCopyCmd = &cobra.Command{
	Use:   "copy [from block] [from list] [to block] [to list]",
	Short: "Add another list's entries to a list (§4.3 import from KC9K)",
	Args:  cobra.ExactArgs(4),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if err := revealBlock(database, args[0]); err != nil { // §3.11
			return err
		}
		_, from, err := findList(database, args[0], args[1])
		if err != nil {
			return err
		}
		if len(from.Entries) == 0 {
			return fmt.Errorf("list '%s' of '%s' is empty", args[1], args[0])
		}
		return listAdd(cmd, database, args[2], args[3], from.Entries)
	}),
}

var listExportCmd = &cobra.Command{
	Use:   "export [block] [list]",
	Short: "Print a list's entries, one per line",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if err := revealBlock(database, args[0]); err != nil {
			return err
		}
		_, l, err := findList(database, args[0], args[1])
		if err != nil {
			return err
		}
		for _, e := range l.Entries {
			fmt.Println(e)
		}
		return nil
	}),
}

var listImportCmd = &cobra.Command{
	Use:   "import [block] [list] [file]",
	Short: "Add entries from a text file with one entry per line",
	Long: `Adds a file's entries to a list: one per line, # starts a comment. Hosts
files ("0.0.0.0 example.com") and adblock rules ("||example.com^") are
read as the sites they name. Phrases ("[a, b] [c]") belong in a phrase set
('kc9k phrase-set import'): they are left out and named.`,
	Args: cobra.ExactArgs(3),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		f, err := readEntries(args[2])
		if err != nil {
			return err
		}
		if len(f.Entries) == 0 {
			return fmt.Errorf("%s holds %s, no entries for a list; phrases go in a phrase set ('kc9k phrase-set import')", args[2], describeEntryFile(f))
		}
		if err := listAdd(cmd, database, args[0], args[1], f.Entries); err != nil {
			return err
		}
		fmt.Printf("Imported %d %s into '%s' of '%s'.\n", len(f.Entries), plural(len(f.Entries), "entry", "entries"), args[1], args[0])
		noteLeftOut(f, false)
		return nil
	}),
}

// plural picks the word for n things.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// noteLeftOut names what an import didn't take: phrases (into a list) or
// entries (into a phrase set), and lines KC9K can't use.
func noteLeftOut(f entryFile, phrases bool) {
	if !phrases && len(f.Phrases) > 0 {
		fmt.Printf("Left out %d %s (they go in a phrase set: 'kc9k phrase-set import').\n", len(f.Phrases), plural(len(f.Phrases), "phrase", "phrases"))
	}
	if phrases && len(f.Entries) > 0 {
		fmt.Printf("Left out %d %s that %s phrases (they go in a list: 'kc9k list import').\n", len(f.Entries), plural(len(f.Entries), "entry", "entries"), plural(len(f.Entries), "isn't", "aren't"))
	}
	if len(f.Skipped) > 0 {
		fmt.Printf("Left out %d %s KC9K can't use, such as %q.\n", len(f.Skipped), plural(len(f.Skipped), "line", "lines"), f.Skipped[0])
	}
}

func init() {
	listActionCmd.Flags().String("url", "", "For redirect: the address to open instead")
	listActionCmd.Flags().String("image", "", "For image: the picture to show instead")
	addMonkAckFlag(listActionCmd)
	listActionCmd.Flags().String("for", "", "For monk, logout and shutdown: how long (e.g. 30m, 2h)")
	listCreateCmd.Flags().Bool("exceptions", false, "Make it an exception list")
	listCreateCmd.Flags().Bool("terms", false, "Old alias; changes nothing (lists have no types)")
	listCreateCmd.Flags().Bool("url", false, "Look at the address (the default)")
	listCreateCmd.Flags().Bool("title", false, "Look at the page title")
	listCreateCmd.Flags().Bool("text", false, "Look at the page text")
	listCreateCmd.Flags().Bool("ads", false, "Look at the ad networks a page loads (§3.9 ad trigger): entries are ad network domains or industries ("+strings.Join(db.AdIndustryNames(), ", ")+")")
	addNowFlag(listDeleteCmd, listAddCmd, listRemoveCmd, listWhereCmd, listActionCmd, listThresholdCmd, listHitsCmd, listRateCmd, listImportCmd)
	listCmd.AddCommand(listShowCmd, listCreateCmd, listRenameCmd, listDeleteCmd, listAddCmd, listRemoveCmd,
		listWhereCmd, listActionCmd, listThresholdCmd, listHitsCmd, listRateCmd, listExportCmd, listImportCmd, listCopyCmd)
	rootCmd.AddCommand(listCmd)
}
