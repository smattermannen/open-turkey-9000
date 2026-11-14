// confirm.go — every lock asks to be confirmed when it is set (R-LOCK-12).
//
// confirmLocks is the one place a lock is confirmed. Every command that sets,
// adds to or extends a lock (a block's, a setting's, a browser setting's, a
// freeze schedule's) calls it before it writes anything. On a terminal it
// shows what will be locked and waits for "yes"; anywhere else (the graphical
// interface, scripts) the command is refused unless --yes says the user has
// already confirmed. The interface shows its own confirmation first and then
// passes --yes.
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/lock"
	"github.com/spf13/cobra"
)

// lockToSet is one lock about to be set: what it locks, its kind, and how
// long it is on where it has a length.
type lockToSet struct {
	Target string // e.g. "block 'work'", "browser setting 'block-devtools'"
	Kind   string // e.g. "timer lock", "text lock (300 random characters)"
	Length string // e.g. "2h0m, until 2026-11-14 18:00"; "" when it has none
}

func (l lockToSet) String() string {
	s := l.Target + ": " + l.Kind
	if l.Length != "" {
		s += ", " + l.Length
	}
	return s
}

// errNotConfirmed is returned when the user didn't confirm: nothing is locked.
var errNotConfirmed = fmt.Errorf("not confirmed: nothing was locked")

// addYesFlag gives commands that set locks the --yes flag.
func addYesFlag(cmds ...*cobra.Command) {
	for _, c := range cmds {
		if c.Flags().Lookup("yes") == nil {
			c.Flags().Bool("yes", false, "Confirm the locks this sets (for scripts; otherwise asked on the terminal)")
		}
	}
}

// confirmLocks asks the user to confirm locks before they are set (R-LOCK-12).
// It returns nil when there is nothing to confirm, when --yes was given, or
// when the user typed yes; otherwise an error, and the caller sets nothing.
func confirmLocks(cmd *cobra.Command, locks ...lockToSet) error {
	if len(locks) == 0 {
		return nil
	}
	if f := cmd.Flags().Lookup("yes"); f != nil && f.Value.String() == "true" {
		return nil
	}
	var b strings.Builder
	for _, l := range locks {
		b.WriteString("  " + l.String() + "\n")
	}
	if !isTerminalInput() {
		return fmt.Errorf("this sets a lock; add --yes to confirm:\n%s", strings.TrimRight(b.String(), "\n"))
	}
	fmt.Printf("This sets a lock:\n%sWhile it is on, this can't be weakened. Type yes to confirm: ", b.String())
	answer, _ := stdinLines.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(answer)) != "yes" {
		return errNotConfirmed
	}
	return nil
}

// timerLength describes a timer lock ending at until (Unix seconds), seen
// from now (Unix seconds, trusted time).
func timerLength(now, until int64) string {
	left := time.Duration(until-now) * time.Second
	if left < 0 {
		left = 0
	}
	return fmt.Sprintf("%s, until %s", left.Round(time.Minute), time.Unix(until, 0).Format("2006-01-02 15:04"))
}

// textLockKind names a text lock with its challenge.
func textLockKind(style string, chars int) string {
	return "text lock (" + lock.DescribeStyle(style, chars) + ")"
}

// leadLength describes a schedule lock's lead time in minutes.
func leadLength(minutes int) string {
	if minutes == 0 {
		return "while each window is open"
	}
	return fmt.Sprintf("while each window is open and %s before it", time.Duration(minutes)*time.Minute)
}

// requireMonkAck refuses to set up something that starts monk mode by itself
// (a bypass prevention step, a list's or a phrase set's monk action) unless
// the user ticked "I know this will not ask for confirmation"
// (--no-confirm-ok): monk mode started that way asks nothing (R-LOCK-12).
func requireMonkAck(cmd *cobra.Command, what string) error {
	if f := cmd.Flags().Lookup("no-confirm-ok"); f != nil && f.Value.String() == "true" {
		return nil
	}
	return fmt.Errorf("%s starts monk mode by itself, and monk mode started that way will not ask for confirmation; add --no-confirm-ok to say you know", what)
}

// addMonkAckFlag gives commands that can set up an automatic monk mode the
// --no-confirm-ok flag.
func addMonkAckFlag(cmds ...*cobra.Command) {
	for _, c := range cmds {
		c.Flags().Bool("no-confirm-ok", false, "I know monk mode started by this will not ask for confirmation")
	}
}
