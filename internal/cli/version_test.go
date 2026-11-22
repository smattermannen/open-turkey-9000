package cli

import (
	"strings"
	"testing"
)

// W43: `kc9k version` names the build, with no password asked.
func TestVersion(t *testing.T) {
	testDB(t)
	if err := runWith(t, 0, []string{"pass-one"}, "access", "set", "app"); err != nil {
		t.Fatal(err)
	}
	if out := capture(t, "version"); !strings.HasPrefix(out, "kc9k dev (") {
		t.Errorf("version: %q", out)
	}
}
