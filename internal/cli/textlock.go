// textlock.go — the flags that choose what a text lock asks for (§3.4):
// random characters (the default, --lock-chars long), random words
// (--lock-words, about --lock-chars long) or the user's own text
// (--lock-text). Any of them, or --text-lock, sets a text lock.
package cli

import (
	"github.com/smattermannen/kc9k/internal/lock"
	"github.com/spf13/cobra"
)

// textLockFlags are the flags addTextLockFlags adds.
var textLockFlags = []string{"text-lock", "lock-words", "lock-text"}

// addTextLockFlags adds the text lock's style flags to commands that take
// --lock and --lock-chars.
func addTextLockFlags(cmds ...*cobra.Command) {
	for _, c := range cmds {
		c.Flags().Bool("text-lock", false, "Text lock: a challenge to type before unlocking (the same as --lock)")
		c.Flags().Bool("lock-words", false, "Text lock of random words (about --lock-chars characters) instead of random characters")
		c.Flags().String("lock-text", "", "Text lock of your own text, typed exactly (plain characters, up to 5000)")
	}
}

// textLockRequested reads the text lock flags: whether a text lock is asked
// for, its style and its length in characters.
func textLockRequested(cmd *cobra.Command) (want bool, style string, length int, err error) {
	useLock, _ := cmd.Flags().GetBool("lock")
	textLock, _ := cmd.Flags().GetBool("text-lock")
	words, _ := cmd.Flags().GetBool("lock-words")
	custom, _ := cmd.Flags().GetString("lock-text")
	length, _ = cmd.Flags().GetInt("lock-chars")
	want = useLock || textLock || words || custom != ""
	if !want {
		return false, "", 0, nil
	}
	switch {
	case custom != "":
		if style, err = lock.CustomStyle(custom); err != nil {
			return false, "", 0, err
		}
		return true, style, len(style) - len(lock.StyleCustom), nil
	case words:
		style = lock.StyleWords
	}
	return true, style, length, lock.CheckLockChars(length)
}
