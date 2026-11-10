// access.go — optional passwords to open Killer Chicken 9000 and each of its
// main pages.
//
// The settings password decides who may loosen; access passwords decide who
// may look and work at all. There is one for opening the program ("app") and
// one for each page of the interface: dashboard, blocks, device, stats and
// settings. Each is optional, and they can be the same or differ.
//
// The interface runs as the user, so a check there could be edited out. The
// check sits here instead: every command belongs to a page (commandAreas),
// and runs only when the app password and that page's password, where set,
// were given. The interface passes the passwords it was given as the first
// lines of standard input and says how many with --access-lines; on a
// terminal the command asks.
//
//	kc9k access                        # which are set
//	kc9k access set blocks stats       # one password for both
//	kc9k access set --all
//	kc9k access remove stats
package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

// The areas an access password can guard.
const (
	areaApp       = "app"
	areaDashboard = "dashboard"
	areaBlocks    = "blocks"
	areaDevice    = "device"
	areaStats     = "stats"
	areaSettings  = "settings"
)

var accessAreas = []string{areaApp, areaDashboard, areaBlocks, areaDevice, areaStats, areaSettings}

// settingAccessPrefix + area holds that area's salted hash. Areas given one
// password in one command share the same hash, so it is checked once.
const settingAccessPrefix = db.AccessPasswordSetting

// commandAreas maps a command (its path below kc9k; the longest match wins)
// to the pages it belongs to. A command on several pages runs when any of
// them is open.
var commandAreas = map[string][]string{
	"status":      {areaDashboard, areaBlocks},
	"start":       {areaDashboard, areaBlocks},
	"stop":        {areaDashboard, areaBlocks},
	"unlock":      {areaDashboard, areaBlocks},
	"monk":        {areaDashboard},
	"break":       {areaDashboard, areaBlocks},
	"block":       {areaBlocks},
	"list":        {areaBlocks},
	"cluster":     {areaBlocks},
	"social":      {areaBlocks},
	"project":     {areaSettings},
	"record":      {areaBlocks},
	"schedule":    {areaBlocks},
	"pending":     {areaBlocks},
	"quota":       {areaBlocks},
	"pomodoro":    {areaBlocks},
	"nepsis":      {areaBlocks, areaSettings},
	"freeze":      {areaDevice},
	"strict":      {areaDevice},
	"no-escape":   {areaDevice},
	"stats":       {areaStats, areaDashboard},
	"stats on":    {areaStats, areaSettings},
	"stats off":   {areaStats, areaSettings},
	"stats clear": {areaStats, areaSettings},
	"browser":     {areaSettings},
	"hotkey":      {areaSettings},
	"option":      {areaSettings},
	"password":    {areaSettings},
	"master":      {areaSettings},
	"master-set":  {areaSettings},
	"access":      {areaSettings},
}

// accessExempt are the commands that never ask: the daemon, a hotkey press
// (its password was asked when the hotkey was made), the interface's state
// (it filters what it prints itself) and uninstalling (root's business,
// guarded by the settings password).
var accessExempt = []string{"daemon", "hotkey run", "gui-state", "prepare-uninstall", "help", "completion"}

// accessGiven are the access passwords given so far in this command.
var (
	accessGiven    []string
	accessLinesRed bool
)

// readAccessLines reads the lines --access-lines announces, once.
func readAccessLines(cmd *cobra.Command) {
	if accessLinesRed {
		return
	}
	accessLinesRed = true
	n, _ := cmd.Root().PersistentFlags().GetInt("access-lines")
	for i := 0; i < n && i < 2*len(accessAreas); i++ {
		line, err := stdinLines.ReadString('\n')
		if line = strings.TrimRight(line, "\r\n"); line != "" && !slices.Contains(accessGiven, line) {
			accessGiven = append(accessGiven, line)
		}
		if err != nil {
			return
		}
	}
}

// accessHashes returns the hash of every area that has a password.
func accessHashes(database *db.DB) (map[string]string, error) {
	hashes := map[string]string{}
	for _, area := range accessAreas {
		hash, err := database.GetSetting(settingAccessPrefix + area)
		if err != nil {
			return nil, err
		}
		if hash != "" {
			hashes[area] = hash
		}
	}
	return hashes, nil
}

// accessChecker remembers which hashes the given passwords open, so areas
// sharing a hash cost one check.
type accessChecker struct {
	hashes map[string]string
	opened map[string]bool // by hash
	used   map[string]bool // given passwords that opened something
}

func newAccessChecker(database *db.DB) (*accessChecker, error) {
	hashes, err := accessHashes(database)
	return &accessChecker{hashes: hashes, opened: map[string]bool{}, used: map[string]bool{}}, err
}

// has reports whether the area has a password.
func (c *accessChecker) has(area string) bool { return c.hashes[area] != "" }

// opens reports whether the area has no password or one that was given.
func (c *accessChecker) opens(area string) bool {
	hash := c.hashes[area]
	if hash == "" {
		return true
	}
	if open, done := c.opened[hash]; done {
		return open
	}
	c.opened[hash] = false
	for _, password := range accessGiven {
		if db.CheckPassword(hash, password) {
			c.opened[hash], c.used[password] = true, true
			break
		}
	}
	return c.opened[hash]
}

// wrongAccess slows guessing down, logs the try and returns the error.
func wrongAccess(database *db.DB, area string) error {
	time.Sleep(time.Second)
	if err := database.RecordEvent(db.EventAccess, "", "wrong access password ("+area+")"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	return fmt.Errorf("wrong access password (%s)", area)
}

// need makes sure the area is open, asking on a terminal.
func (c *accessChecker) need(database *db.DB, area string) error {
	if c.opens(area) {
		return nil
	}
	if !isTerminalInput() {
		if len(accessGiven) > 0 {
			return wrongAccess(database, area)
		}
		return fmt.Errorf("this needs the access password (%s)", area)
	}
	password, err := readPasswordStdin(fmt.Sprintf("Access password (%s)", area))
	if err != nil {
		return fmt.Errorf("this needs the access password (%s)", area)
	}
	if !db.CheckPassword(c.hashes[area], password) {
		return wrongAccess(database, area)
	}
	accessGiven = append(accessGiven, password)
	c.opened[c.hashes[area]] = true
	return nil
}

// requireAccess fails unless the app is open and one of the areas is.
func requireAccess(cmd *cobra.Command, database *db.DB, areas []string) error {
	readAccessLines(cmd)
	c, err := newAccessChecker(database)
	if err != nil {
		return err
	}
	if err := c.need(database, areaApp); err != nil {
		return err
	}
	if len(areas) == 0 || slices.ContainsFunc(areas, c.opens) {
		return nil
	}
	return c.need(database, areas[0])
}

// areasOf returns the pages a command belongs to, and whether it asks at all.
func areasOf(cmd *cobra.Command) ([]string, bool) {
	path := strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name())
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false
	}
	for _, exempt := range accessExempt {
		if path == exempt || strings.HasPrefix(path, exempt+" ") || strings.HasPrefix(path, "__") {
			return nil, false
		}
	}
	for p := path; p != ""; {
		if areas, ok := commandAreas[p]; ok {
			return areas, true
		}
		i := strings.LastIndexByte(p, ' ')
		if i < 0 {
			break
		}
		p = p[:i]
	}
	return nil, true // an unlisted command still needs the app password
}

// accessGate runs before every command.
func accessGate(cmd *cobra.Command, _ []string) error {
	areas, asks := areasOf(cmd)
	if !asks {
		return nil
	}
	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()
	return requireAccess(cmd, database, areas)
}

// accessStatus is what gui-state says about each area: "none" (no
// password), "open" or "locked". With the app locked, every page is.
func accessStatus(cmd *cobra.Command, database *db.DB) (status map[string]string, unused []int, err error) {
	readAccessLines(cmd)
	c, err := newAccessChecker(database)
	if err != nil {
		return nil, nil, err
	}
	status = map[string]string{}
	appOpen := c.opens(areaApp)
	for _, area := range accessAreas {
		switch {
		case !c.opens(area) || !appOpen:
			status[area] = "locked"
		case c.has(area):
			status[area] = "open"
		default:
			status[area] = "none"
		}
	}
	unused = []int{}
	for i, password := range accessGiven {
		if !c.used[password] {
			unused = append(unused, i)
		}
	}
	if len(unused) > 0 {
		time.Sleep(time.Second) // gui-state must not be a fast way to guess
		if err := database.RecordEvent(db.EventAccess, "", "wrong access password"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
	}
	return status, unused, nil
}

// accessTargets turns the arguments of set/remove into areas.
func accessTargets(cmd *cobra.Command, args []string) ([]string, error) {
	if all, _ := cmd.Flags().GetBool("all"); all {
		if len(args) > 0 {
			return nil, fmt.Errorf("give areas or --all, not both")
		}
		return accessAreas, nil
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("name at least one of: %s (or --all)", strings.Join(accessAreas, ", "))
	}
	var areas []string
	for _, a := range args {
		a = strings.ToLower(a)
		if !slices.Contains(accessAreas, a) {
			return nil, fmt.Errorf("unknown area %q (use %s)", a, strings.Join(accessAreas, ", "))
		}
		if !slices.Contains(areas, a) {
			areas = append(areas, a)
		}
	}
	return areas, nil
}

// authorizeAccessChange asks for the settings password and for the current
// password of every target that has one.
func authorizeAccessChange(database *db.DB, areas []string) error {
	if err := requireSettingsPassword(database); err != nil {
		return err
	}
	c, err := newAccessChecker(database)
	if err != nil {
		return err
	}
	for _, area := range areas {
		if err := c.need(database, area); err != nil {
			return err
		}
	}
	return nil
}

var accessCmd = &cobra.Command{
	Use:   "access",
	Short: "Passwords to open the program and its pages",
	Long: `Access passwords decide who may open Killer Chicken 9000. Each is optional:

  app        opening the program at all
  dashboard  the Dashboard: status, starting and stopping
  blocks     blocks and everything in them
  device     Frozen Chicken, strict mode, no-escape
  stats      statistics
  settings   browser options, hotkeys, passwords

Name several areas in one "set" to give them the same password. Commands ask
for the app password and for the password of the page they belong to; the
graphical interface asks once and remembers until it is closed or locked.

Changing or removing one asks for its current password (and the settings
password, when set). New passwords are read from standard input, hidden on a
terminal.

If one is lost, root can remove them all:
  sudo python3 -c "import sqlite3; c=sqlite3.connect('/var/lib/kc9k/kc9k.db'); c.execute(\"DELETE FROM settings WHERE key LIKE 'access.password.%'\"); c.commit()"`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		hashes, err := accessHashes(database)
		if err != nil {
			return err
		}
		for _, area := range accessAreas {
			state := "no password"
			if hashes[area] != "" {
				state = "password set"
				for _, other := range accessAreas {
					if other != area && hashes[other] == hashes[area] {
						state = "password set (shared)"
						break
					}
				}
			}
			fmt.Printf("%-10s %s\n", area, state)
		}
		return nil
	}),
}

var accessSetCmd = &cobra.Command{
	Use:   "set <area…> | --all",
	Short: "Set or change the password of one or more areas",
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		areas, err := accessTargets(cmd, args)
		if err != nil {
			return err
		}
		if err := authorizeAccessChange(database, areas); err != nil {
			return err
		}
		password, err := readPasswordStdin("New access password")
		if err != nil {
			return err
		}
		if len(password) < 4 {
			return fmt.Errorf("use at least 4 characters")
		}
		if isTerminalInput() {
			again, err := readPasswordStdin("Repeat it")
			if err != nil {
				return err
			}
			if again != password {
				return fmt.Errorf("the two passwords differ; nothing changed")
			}
		}
		hash, err := db.HashPassword(password)
		if err != nil {
			return err
		}
		for _, area := range areas {
			if err := database.SetSetting(settingAccessPrefix+area, hash); err != nil {
				return err
			}
		}
		fmt.Printf("Access password set for: %s.\n", strings.Join(areas, ", "))
		return nil
	}),
}

var accessRemoveCmd = &cobra.Command{
	Use:   "remove <area…> | --all",
	Short: "Remove the password of one or more areas",
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		areas, err := accessTargets(cmd, args)
		if err != nil {
			return err
		}
		if err := authorizeAccessChange(database, areas); err != nil {
			return err
		}
		for _, area := range areas {
			if err := database.SetSetting(settingAccessPrefix+area, ""); err != nil {
				return err
			}
		}
		fmt.Printf("Access password removed for: %s.\n", strings.Join(areas, ", "))
		return nil
	}),
}

func init() {
	rootCmd.PersistentFlags().Int("access-lines", 0, "Internal: this many access passwords come first on standard input")
	_ = rootCmd.PersistentFlags().MarkHidden("access-lines")
	rootCmd.PersistentPreRunE = accessGate
	for _, c := range []*cobra.Command{accessSetCmd, accessRemoveCmd} {
		c.Flags().Bool("all", false, "Every area")
	}
	accessCmd.AddCommand(accessSetCmd, accessRemoveCmd)
	rootCmd.AddCommand(accessCmd)
}

// hideLocked empties what belongs only to pages whose password wasn't given,
// so the interface never receives what it may not show. Blocks appear on the
// Dashboard and the Blocks page, the freeze on the Dashboard and Device
// Blocks; with every page locked nothing but the access map is left.
func (s *guiState) hideLocked() {
	open := func(areas ...string) bool {
		return slices.ContainsFunc(areas, func(a string) bool { return s.Access[a] != "locked" })
	}
	access, unused, settingsPass := s.Access, s.AccessUnused, s.SettingsPass
	if !open(areaDashboard, areaBlocks, areaDevice, areaStats, areaSettings) {
		*s = guiState{}
	}
	if !open(areaDashboard, areaBlocks) {
		s.Blocks = nil
	}
	if !open(areaDashboard, areaDevice) {
		s.FreezeSeconds, s.FreezeMode, s.Strict, s.NoEscape, s.FreezeSched = 0, "", false, false, nil
	}
	if !open(areaDashboard, areaBlocks, areaSettings) {
		s.Nepsis, s.NepsisCount, s.NepsisLock, s.MonkBlock = db.Nepsis{}, 0, guiSettingLock{}, ""
	}
	if !open(areaSettings) {
		s.Browser, s.BrowserLocks, s.Hotkeys = nil, nil, nil
	}
	s.Access, s.AccessUnused, s.SettingsPass = access, unused, settingsPass
	// The interface expects these to be present.
	if s.Blocks == nil {
		s.Blocks = []guiBlock{}
	}
	if s.FreezeSched == nil {
		s.FreezeSched = []string{}
	}
	if s.Browser == nil {
		s.Browser = map[string]bool{}
	}
	if s.BrowserLocks == nil {
		s.BrowserLocks = map[string]guiSettingLock{}
	}
}
