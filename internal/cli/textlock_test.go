package cli

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/lock"
)

// R-LOCK-5, §4.6 (§6 2026-11-20): without a terminal, unlock takes the
// text lock's challenge from the window the command opens; an answer on
// standard input is never read.
func TestTextLockUnlocksOnlyFromItsWindow(t *testing.T) {
	notTerminal(t)
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "start", "b", "--lock", "--lock-chars", "20", "--yes")
	var shown []string
	old := lock.StartWindow
	lock.StartWindow = func(pages []string) (io.Reader, func() error, error) {
		shown = pages
		var b strings.Builder
		for i, p := range pages {
			for j := 0; j < len(p); j++ {
				fmt.Fprintf(&b, "k %d %d\n", p[j], 1000*(i+1)+100*j)
			}
			b.WriteString("n 0\n")
		}
		return strings.NewReader(b.String()), func() error { return nil }, nil
	}
	t.Cleanup(func() { lock.StartWindow = old })
	if err := runWith(t, 0, []string{"a piped answer, never read"}, "unlock", "b"); err != nil {
		t.Fatal(err)
	}
	if len(shown) != 1 || len(shown[0]) != 20 {
		t.Errorf("the window showed %q", shown)
	}
	if b, _ := d.GetBlock("b"); b.Locked {
		t.Errorf("still locked after the challenge was typed in its window: %+v", b)
	}
}
