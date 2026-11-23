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

// 2.12: the GUI reads why a found browser isn't supported yet (additive
// "waiting"; left out when nothing is awaited).
func TestBrowsersWaitingJSON(t *testing.T) {
	out, _ := json.Marshal(guiBrowser{ID: "firefox", Waiting: blocker.WaitingMozillaSignature, Found: true})
	if !strings.Contains(string(out), `"waiting":"mozilla-signature"`) {
		t.Errorf("waiting missing: %s", out)
	}
	out, _ = json.Marshal(guiBrowser{ID: "librewolf"})
	if strings.Contains(string(out), "waiting") {
		t.Errorf("nothing awaited but waiting shown: %s", out)
	}
	for _, b := range supportedBrowsers(testDB(t)) {
		sb, _ := blocker.SupportedBrowserByID(b.ID)
		if b.Waiting != sb.Waiting() || (b.Waiting != "" && b.Installed) {
			t.Errorf("%s: waiting %q, installed %v", b.ID, b.Waiting, b.Installed)
		}
	}
	if text := (guiBrowser{Found: true, Waiting: blocker.WaitingChromeWebStore}).extensionText(); !strings.Contains(text, "Chrome Web Store") {
		t.Errorf("text %q", text)
	}
}
