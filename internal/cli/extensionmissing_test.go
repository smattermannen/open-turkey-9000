package cli

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/enforce"
)

// fakeRunningBrowsers makes these supported browsers run, for so long.
func fakeRunningBrowsers(t *testing.T, running map[string]time.Duration) {
	t.Helper()
	old := runningSupportedBrowsers
	runningSupportedBrowsers = func() map[string]time.Duration { return running }
	t.Cleanup(func() { runningSupportedBrowsers = old })
}

// Owner 2026-11-17: the GUI said the extension wasn't running at every
// start, and now and then while it was. It says so only when no supported
// browser is installed, or one has run past the grace (and a minute) while
// the extension is needed without its extension checking in.
func TestExtensionMissing(t *testing.T) {
	d := testDB(t)
	fakeSupportedBrowser(t, true)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "start", "b")

	// At sign-in: no browser open yet, no check-in for hours.
	fakeRunningBrowsers(t, map[string]time.Duration{})
	old := strconv.FormatInt(time.Now().Add(-10*time.Hour).Unix(), 10)
	if err := d.SetSetting(enforce.SettingExtensionCheckIn, old); err != nil {
		t.Fatal(err)
	}
	if extensionMissing(d) {
		t.Error("missing with no browser open")
	}
	// LibreWolf just opened: it has the grace to check in.
	fakeRunningBrowsers(t, map[string]time.Duration{"librewolf": 30 * time.Second})
	if extensionMissing(d) {
		t.Error("missing in a browser that just started")
	}
	// It runs and checks in.
	fakeRunningBrowsers(t, map[string]time.Duration{"librewolf": 3 * time.Hour})
	if err := d.SetSetting(enforce.SettingExtensionCheckIn+".librewolf", strconv.FormatInt(time.Now().Unix(), 10)); err != nil {
		t.Fatal(err)
	}
	if extensionMissing(d) {
		t.Error("missing while it checks in")
	}
	// Chrome runs past the grace without a check-in: say so, naming it.
	fakeRunningBrowsers(t, map[string]time.Duration{"librewolf": 3 * time.Hour, "chrome": 5 * time.Minute})
	if !extensionMissing(d) || !slices.Equal(browsersWithoutExtension(d), []string{"Google Chrome"}) {
		t.Errorf("not missing in Chrome: %v", browsersWithoutExtension(d))
	}
	// Not needed (the block is off): nothing to say.
	mustRun(t, "stop", "b")
	if extensionMissing(d) {
		t.Error("missing while not needed")
	}
	// No supported browser installed at all.
	fakeSupportedBrowser(t, false)
	if !extensionMissing(d) {
		t.Error("not missing with no supported browser")
	}
}
