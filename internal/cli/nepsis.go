// nepsis.go — monk mode and bypass prevention.
//
//	kc9k monk --for 1h                       # monk mode: the monk-mode block, time-locked
//	kc9k bypass                              # show bypass prevention
//	kc9k bypass set --step "3 in 10m: close-tab, logout 1m" \
//	                --step "3 in 10m: close-browser, monk 5m" \
//	                --step "2 in 30m: shutdown 15m" [--detect page,app] [--quiet 2h] [--block B]
//	kc9k bypass set --count 10 --within 5m --then monk --then-for 1h   # one step
//	kc9k bypass set ... --for 7d --lock      # global only: lock it like a browser option
//	kc9k bypass off [--block B]
//	kc9k bypass unlock                       # global: challenge or password
//
// Bypass prevention counts what KC9K stops (see db/nepsis.go) and climbs a
// ladder of steps. The global one counts every block's events; a block's own
// counts that block's while it is on. Making either stricter is always
// allowed. Loosening a block's is gated like other edits to the block;
// loosening the global one is refused while its lock holds.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/smattermannen/kc9k/internal/lock"
	"github.com/spf13/cobra"
)

var monkCmd = &cobra.Command{
	Use:   "monk --for DURATION",
	Short: "Monk mode: block every website for a while, locked",
	Long: `Starts the block 'monk-mode' with a time lock that ends it. It is made the
first time with every website blocked (*.* in its Blocked list); add
exceptions, apps or anything else to it like any block. On a monk mode that
is already on, a longer time extends it.`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		length, _ := cmd.Flags().GetString("for")
		d, err := time.ParseDuration(length)
		if err != nil || d < time.Minute {
			return fmt.Errorf("give --for, at least 1m (e.g. 45m, 2h)")
		}
		if err := requireLockPassword(database); err != nil {
			return err
		}
		if err := enforce.StartMonk(database, d); err != nil {
			return err
		}
		if err := enforce.ApplyFromDB(database); err != nil {
			return err
		}
		b, err := database.GetBlock(enforce.MonkBlock)
		if err != nil {
			return err
		}
		fmt.Printf("Monk mode is on until %s.\n", time.Unix(b.LockedUntil, 0).Format("2006-01-02 15:04"))
		return nil
	}),
}

// describeNepsis renders a bypass prevention, one line per step.
func describeNepsis(n db.Nepsis) string {
	if !n.On() {
		return "off"
	}
	var lines []string
	head := "counts every detector"
	if len(n.Detectors) > 0 {
		head = "counts " + strings.Join(n.Detectors, ", ")
	}
	if n.QuietMinutes > 0 {
		head += "; falls back a step after " + (time.Duration(n.QuietMinutes) * time.Minute).String() + " quiet"
	}
	lines = append(lines, head)
	for i, step := range n.Steps {
		var actions []string
		for _, a := range step.Actions {
			actions = append(actions, enforce.Consequence{Kind: a.Kind, Minutes: a.Minutes}.String())
		}
		lines = append(lines, fmt.Sprintf("  step %d: %d within %d min: %s", i+1, step.Count, step.Minutes,
			strings.Join(actions, ", ")))
	}
	return strings.Join(lines, "\n")
}

var stepPattern = regexp.MustCompile(`^\s*(\d+)\s+in\s+(\S+)\s*:\s*(.+)$`)

// parseMinutes reads a duration in whole minutes ("5m", "2h").
func parseMinutes(text, what string) (int, error) {
	d, err := time.ParseDuration(strings.TrimSpace(text))
	if err != nil || d < 0 || d%time.Minute != 0 {
		return 0, fmt.Errorf("invalid %s %q: give whole minutes, like 5m or 2h", what, text)
	}
	return int(d / time.Minute), nil
}

// parseStep reads "3 in 10m: close-tab, logout 1m".
func parseStep(text string) (db.NepsisStep, error) {
	var step db.NepsisStep
	m := stepPattern.FindStringSubmatch(text)
	if m == nil {
		return step, fmt.Errorf("invalid step %q: write it like \"3 in 10m: close-tab, logout 1m\"", text)
	}
	step.Count, _ = strconv.Atoi(m[1])
	var err error
	if step.Minutes, err = parseMinutes(m[2], "window"); err != nil {
		return step, err
	}
	for _, part := range strings.Split(m[3], ",") {
		fields := strings.Fields(part)
		if len(fields) == 0 || len(fields) > 2 {
			return step, fmt.Errorf("invalid action %q in step %q", strings.TrimSpace(part), text)
		}
		a := db.NepsisAction{Kind: strings.ToLower(fields[0])}
		if len(fields) == 2 {
			if a.Minutes, err = parseMinutes(fields[1], "duration"); err != nil {
				return step, err
			}
		}
		step.Actions = append(step.Actions, a)
	}
	return step, nil
}

// readNepsis reads the steps (--step, or the one-step shorthand --count,
// --within, --then, --then-for), --detect and --quiet.
func readNepsis(cmd *cobra.Command) (db.Nepsis, error) {
	var n db.Nepsis
	steps, _ := cmd.Flags().GetStringArray("step")
	for _, text := range steps {
		step, err := parseStep(text)
		if err != nil {
			return n, err
		}
		n.Steps = append(n.Steps, step)
	}
	if cmd.Flags().Changed("count") || cmd.Flags().Changed("then") {
		if len(steps) > 0 {
			return n, fmt.Errorf("give --step, or --count, --within and --then, not both")
		}
		var step db.NepsisStep
		var err error
		step.Count, _ = cmd.Flags().GetInt("count")
		within, _ := cmd.Flags().GetString("within")
		if step.Minutes, err = parseMinutes(within, "--within"); err != nil {
			return n, err
		}
		a := db.NepsisAction{}
		a.Kind, _ = cmd.Flags().GetString("then")
		if thenFor, _ := cmd.Flags().GetString("then-for"); thenFor != "" {
			if a.Minutes, err = parseMinutes(thenFor, "--then-for"); err != nil {
				return n, err
			}
		}
		step.Actions = []db.NepsisAction{a}
		n.Steps = []db.NepsisStep{step}
	}
	if len(n.Steps) == 0 {
		return n, fmt.Errorf("give at least one --step (or --count, --within and --then)")
	}
	n.Detectors, _ = cmd.Flags().GetStringSlice("detect")
	if quiet, _ := cmd.Flags().GetString("quiet"); quiet != "" {
		var err error
		if n.QuietMinutes, err = parseMinutes(quiet, "--quiet"); err != nil {
			return n, err
		}
	}
	return n, n.Check()
}

func nepsisEdit(n db.Nepsis) db.Edit {
	value := ""
	if n.On() {
		data, _ := json.Marshal(n)
		value = string(data)
	}
	return db.Edit{Action: db.EditSetNepsis, Values: []string{value}}
}

// setBlockNepsis changes a block's bypass prevention; loosening is gated.
func setBlockNepsis(cmd *cobra.Command, database *db.DB, block string, n db.Nepsis) error {
	old, err := database.BlockNepsis(block)
	if err != nil {
		return err
	}
	if reflect.DeepEqual(old, n) {
		return nil
	}
	proceed, err := gateEdit(cmd, database, block, n.StricterThan(old), nepsisEdit(n))
	if err != nil || !proceed {
		return err
	}
	if err := database.SetBlockNepsis(block, n); err != nil {
		return err
	}
	fmt.Printf("Bypass prevention for block '%s': %s\n", block, describeNepsis(n))
	return nil
}

// setGlobalNepsis changes the global bypass prevention, with any locks the
// flags ask for. Loosening is refused while a lock holds.
func setGlobalNepsis(cmd *cobra.Command, database *db.DB, n db.Nepsis) error {
	old, err := database.GlobalNepsis()
	if err != nil {
		return err
	}
	if !n.StricterThan(old) {
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
		s, err := loadSettingLock(database, db.NepsisLockName)
		if err != nil {
			return err
		}
		if err := s.refuseWhileHeld("bypass prevention"); err != nil {
			return refused(database, "", err)
		}
		if s.lock.LockChars > 0 || s.lock.PasswordHash != "" {
			return refused(database, "", fmt.Errorf("bypass prevention is locked. Use 'kc9k bypass unlock' first"))
		}
		if err := refuseSettingWeakening(database, "bypass prevention"); err != nil {
			return err
		}
		if err := database.ClearSettingLock(db.NepsisLockName); err != nil { // locks that ran out
			return err
		}
	}
	if lockFlagsGiven(cmd) {
		if !n.On() {
			return fmt.Errorf("lock options only go with 'set'")
		}
		if err := requireLockPassword(database); err != nil {
			return err
		}
		want, err := requestedSettingLock(cmd, database, db.NepsisLockName)
		if err != nil {
			return err
		}
		cur, err := loadSettingLock(database, db.NepsisLockName)
		if err != nil {
			return err
		}
		next, changes, err := strengthenSettingLock(cur, want, cmd.Flags().Changed("lock-chars"))
		if err != nil {
			return err
		}
		if err := database.SetSettingLock(next); err != nil {
			return err
		}
		if len(changes) > 0 {
			fmt.Printf("Bypass prevention %s\n", strings.Join(changes, ", "))
		}
	}
	if err := database.SetGlobalNepsis(n); err != nil {
		return err
	}
	fmt.Printf("Bypass prevention for the whole computer: %s\n", describeNepsis(n))
	return nil
}

var nepsisCmd = &cobra.Command{
	Use:     "bypass",
	Aliases: []string{"nepsis"},
	Short:   "Bypass prevention: act when KC9K stops many things in a short time",
	Long: `Bypass prevention counts what KC9K stops and climbs a ladder of steps. The
detectors (--detect; default all) are the kinds of events that count:

  page     a blacklist took a page away       browser  a browser without the extension was closed
  app      a blocked app was closed           tamper   the daemon restarted in a boot (not for an upgrade or its own crash), or the clock jumped
  title    a window was closed for its title  access   a wrong access password
  search   a page taken away that was opened from a search engine's results
           (page counts these too; alone, it counts only pages looked for)

Each --step trips when its number of events happens within its window, then
takes its actions; the next time, the next step; past the last, the last
again. After --quiet without tripping, it falls back a step. For example:

  kc9k bypass set --step "3 in 10m: warning, close-tab, logout 1m" \
                  --step "3 in 10m: close-browser, monk 5m" \
                  --step "2 in 30m: shutdown 15m" --quiet 2h

Actions: warning (a notification), close-tab (the page that tripped it),
close-browser, lock-screen D, logout [D], shutdown [D] (with D: stay so that
long, through Frozen Chicken, at most a day; without: once, at most every
10 minutes), monk D. --count, --within, --then and --then-for are a short
way to give one step with one action.

The whole computer has one (it counts every block's events), and each block
can have its own (--block: that block's events, while it is on; a closed
browser counts for every active block with websites, tampering for every
active block); when both are set, both apply. Without a subcommand, shows them and where they stand.

Making one stricter is always allowed: more detectors, a later fall-back,
steps that trip on fewer events over longer windows and keep every action
for at least as long, more steps at the end. Anything else loosens it: a
block's is gated like other edits to the block, and the global one can be
locked like a browser option (--for, --until, --lock, --password-stdin,
--restart on 'set').`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		now := time.Now()
		show := func(title, scope string, n db.Nepsis) {
			fmt.Printf("%s: %s\n", title, describeNepsis(n))
			if n.On() {
				level, count, _ := enforce.NepsisState(database, now, scope, n)
				fmt.Printf("  now at step %d of %d, %d of %d events so far\n", level+1, len(n.Steps), count, n.Step(level).Count)
			}
		}
		n, err := database.GlobalNepsis()
		if err != nil {
			return err
		}
		title := "whole computer"
		if s, err := loadSettingLock(database, db.NepsisLockName); err == nil && s.describe() != "" {
			title += " (" + s.describe() + ")"
		}
		show(title, "", n)
		blocks, err := database.ListBlocks()
		if err != nil {
			return err
		}
		for _, b := range blocks {
			bn, err := database.BlockNepsis(b.Name)
			if err != nil {
				return err
			}
			if bn.On() {
				show("block "+b.Name, b.Name, bn)
			}
		}
		return nil
	}),
}

var nepsisSetCmd = &cobra.Command{
	Use:   "set --step \"N in D: action [D], ...\" ... [--detect kinds] [--quiet D] [--block B]",
	Short: "Turn bypass prevention on, or change it",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		n, err := readNepsis(cmd)
		if err != nil {
			return err
		}
		if block, _ := cmd.Flags().GetString("block"); block != "" {
			if lockFlagsGiven(cmd) {
				return fmt.Errorf("a block's bypass prevention is held by the block's own locks")
			}
			return setBlockNepsis(cmd, database, block, n)
		}
		return setGlobalNepsis(cmd, database, n)
	}),
}

var nepsisOffCmd = &cobra.Command{
	Use:   "off [--block B]",
	Short: "Turn bypass prevention off (loosens it)",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if block, _ := cmd.Flags().GetString("block"); block != "" {
			return setBlockNepsis(cmd, database, block, db.Nepsis{})
		}
		return setGlobalNepsis(cmd, database, db.Nepsis{})
	}),
}

var nepsisUnlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Remove the challenge or password lock on the global bypass prevention",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
		s, err := loadSettingLock(database, db.NepsisLockName)
		if err != nil {
			return err
		}
		if err := s.refuseWhileHeld("bypass prevention"); err != nil {
			return refused(database, "", err)
		}
		if s.lock.LockChars == 0 && s.lock.PasswordHash == "" {
			return fmt.Errorf("bypass prevention is not locked")
		}
		if s.lock.PasswordHash != "" {
			password, err := readPasswordStdin("Setting password")
			if err != nil {
				return err
			}
			if !db.CheckPassword(s.lock.PasswordHash, password) {
				return refused(database, "", fmt.Errorf("wrong password; bypass prevention stays locked"))
			}
		}
		if s.lock.LockChars > 0 {
			ok, err := lock.RunStyled(s.lock.Style, s.lock.LockChars)
			if err != nil {
				return fmt.Errorf("failed to run the typing challenge: %w", err)
			}
			if !ok {
				_ = refused(database, "", fmt.Errorf("failed the unlock challenge"))
				fmt.Println("Unlock failed. Bypass prevention stays locked.")
				os.Exit(1)
			}
		}
		if err := database.ClearSettingLock(db.NepsisLockName); err != nil {
			return err
		}
		fmt.Println("Bypass prevention is unlocked; it stays on until you change it.")
		return nil
	}),
}

func init() {
	monkCmd.Flags().String("for", "", "How long (e.g. 45m, 2h)")
	nepsisSetCmd.Flags().StringArray("step", nil, `A step of the ladder, e.g. "3 in 10m: close-tab, logout 1m" (repeat for more)`)
	nepsisSetCmd.Flags().StringSlice("detect", nil, "Which events count (default all): "+strings.Join(db.Detectors, ", "))
	nepsisSetCmd.Flags().String("quiet", "", "Fall back a step after this long without firing (e.g. 2h)")
	nepsisSetCmd.Flags().Int("count", 0, "One step: how many events")
	nepsisSetCmd.Flags().String("within", "", "One step: within how long (e.g. 5m)")
	nepsisSetCmd.Flags().String("then", "", "One step: the action: "+strings.Join(db.Consequences, ", "))
	nepsisSetCmd.Flags().String("then-for", "", "One step: how long the action lasts (e.g. 1h)")
	for _, c := range []*cobra.Command{nepsisSetCmd, nepsisOffCmd} {
		c.Flags().String("block", "", "A block's own bypass prevention, instead of the global one")
	}
	nepsisSetCmd.Flags().String("for", "", "Global only. Time lock: can't be loosened for this long")
	nepsisSetCmd.Flags().String("until", "", "Global only. Time lock until this clock time (HH:MM)")
	nepsisSetCmd.Flags().Bool("lock", false, "Global only. Typing challenge lock ('kc9k bypass unlock')")
	nepsisSetCmd.Flags().Int("lock-chars", lock.DefaultLockChars, "Number of characters in the unlock challenge")
	nepsisSetCmd.Flags().Bool("password-stdin", false, "Global only. Password lock, read from standard input")
	nepsisSetCmd.Flags().Bool("restart", false, "Global only. Can't be loosened until the computer restarts")
	addTextLockFlags(nepsisSetCmd)
	addNowFlag(nepsisSetCmd, nepsisOffCmd)
	nepsisCmd.AddCommand(nepsisSetCmd, nepsisOffCmd, nepsisUnlockCmd)
	rootCmd.AddCommand(monkCmd, nepsisCmd)
}
