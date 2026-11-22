package cli

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/blocker"
)

// W30: gui-state names a browser that is on the computer as found, even
// where KC9K can't give it the extension yet (Chrome and Edge on Windows
// before the store listing), so the GUI can say why it isn't supported.
func TestBrowsersFound(t *testing.T) {
	d := testDB(t)
	if len(blocker.SupportedBrowsers) == 0 {
		t.Skip("no supported browsers on this platform")
	}
	want := blocker.SupportedBrowsers[0]
	old := browserFileStat
	browserFileStat = func(name string) (os.FileInfo, error) {
		for _, exe := range want.Exes {
			if name == exe {
				return nil, nil
			}
		}
		return nil, &fs.PathError{Op: "stat", Path: name, Err: errors.New("not there")}
	}
	t.Cleanup(func() { browserFileStat = old })

	for _, b := range supportedBrowsers(d) {
		if b.ID == want.ID && !b.Found {
			t.Errorf("%s not found", b.ID)
		}
		if b.Installed && !b.Found {
			t.Errorf("%s installed but not found", b.ID)
		}
	}
	out, err := json.Marshal(supportedBrowsers(d)[0])
	if err != nil || !strings.Contains(string(out), `"found":true`) {
		t.Errorf("gui-state browser: %s %v", out, err)
	}
}
