//go:build linux

package blocker

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// Only the packages' own executables are supported browsers: Firefox,
// copies and look-alike paths are not.
func TestSupportedExePath(t *testing.T) {
	old := mozillaSigned
	mozillaSigned = func() bool { return false } // Firefox waits (2.12)
	t.Cleanup(func() { mozillaSigned = old })
	cases := map[string]string{
		"/usr/share/librewolf/librewolf":           "librewolf",
		"/usr/share/librewolf/librewolf-bin":       "librewolf",
		"/usr/lib/librewolf/librewolf (deleted)":   "librewolf", // upgraded under it
		"/opt/google/chrome/chrome":                "chrome",
		"/usr/lib/chromium/chromium":               "chromium",
		"/usr/lib/firefox/firefox":                 "",
		"/home/u/librewolf/librewolf":              "",
		"/home/u/opt/google/chrome/chrome":         "",
		"/usr/lib/chromium-browser/chromium-x":     "",
		"/opt/google/chrome-beta/chrome":           "",
		"/usr/share/librewolf/librewolf.sh":        "",
		"/opt/brave.com/brave/brave":               "",
		"/usr/lib/chromium/chromium/../../../evil": "",
	}
	for exe, want := range cases {
		if got := supportedExePath(exe); got != want {
			t.Errorf("%s: %q; want %q", exe, got, want)
		}
	}
}

// fakeInstalled makes exactly the given files exist for Installed.
func fakeInstalled(t *testing.T, files ...string) {
	t.Helper()
	old := statFile
	statFile = func(name string) (os.FileInfo, error) {
		for _, f := range files {
			if f == name {
				return nil, nil
			}
		}
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() { statFile = old })
}

func TestSupportedInstalled(t *testing.T) {
	fakeInstalled(t, "/usr/lib/chromium/chromium")
	got := map[string]bool{}
	for _, b := range SupportedBrowsers {
		got[b.ID] = b.Installed()
	}
	if !reflect.DeepEqual(got, map[string]bool{"librewolf": false, "chrome": false, "chromium": true, "edge": false, "firefox": false}) {
		t.Errorf("installed = %v", got)
	}
	if !ChromeInstalled() {
		t.Error("ChromeInstalled misses Chromium")
	}
}

// InstallBrowser runs the package steps and checks the browser is there.
func TestInstallBrowser(t *testing.T) {
	var ran [][]string
	old := runInstall
	runInstall = func(steps [][]string) error { ran = append(ran, steps...); return nil }
	t.Cleanup(func() { runInstall = old })

	fakeInstalled(t, "/usr/lib/chromium/chromium")
	if err := InstallBrowser("chromium"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ran, [][]string{{"apt-get", "install", "-y", "chromium"}}) {
		t.Errorf("chromium steps = %v", ran)
	}

	ran = nil
	if err := InstallBrowser("librewolf"); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("librewolf missing after install: %v", err)
	}
	if len(ran) != 4 || ran[1][0] != "extrepo" {
		t.Errorf("librewolf steps = %v", ran)
	}

	if err := InstallBrowser("firefox"); err == nil {
		t.Error("installed an unsupported browser")
	}
	if steps, err := installSteps("chrome"); err != nil || steps[len(steps)-1][3] != "google-chrome-stable" {
		t.Errorf("chrome steps = %v, %v", steps, err)
	}
}

// 2.12: Edge and Firefox are found but wait (the Chrome Web Store listing,
// Mozilla's signature): until then not installed as supported browsers,
// their executables guarded as unsupported, KC9K installing neither. Once
// ready, each is a supported browser like the others.
func TestEdgeFirefoxWaiting(t *testing.T) {
	fakeInstalled(t, "/opt/microsoft/msedge/msedge", "/usr/lib/firefox/firefox")
	oldStore, oldSigned := chromeStoreReady, mozillaSigned
	t.Cleanup(func() { chromeStoreReady, mozillaSigned = oldStore, oldSigned })
	chromeStoreReady, mozillaSigned = func() bool { return false }, func() bool { return false }
	for id, want := range map[string]string{"edge": WaitingChromeWebStore, "firefox": WaitingMozillaSignature, "chromium": ""} {
		b, _ := SupportedBrowserByID(id)
		if b.Waiting() != want {
			t.Errorf("%s waits for %q; want %q", id, b.Waiting(), want)
		}
	}
	for _, id := range []string{"edge", "firefox"} {
		b, _ := SupportedBrowserByID(id)
		if !b.Found() || b.Installed() {
			t.Errorf("%s: found %v, installed %v; want found, not installed", id, b.Found(), b.Installed())
		}
		if _, err := installSteps(id); err == nil {
			t.Errorf("%s: KC9K would install it", id)
		}
	}
	for _, exe := range []string{"/opt/microsoft/msedge/msedge", "/usr/lib/firefox/firefox"} {
		if got := supportedExePath(exe); got != "" {
			t.Errorf("%s waiting, yet a supported browser: %q", exe, got)
		}
	}
	if ExtensionBrowserInstalled() && LibreWolfVersion() == "" {
		t.Error("a waiting browser counts as one that runs the extension")
	}

	chromeStoreReady, mozillaSigned = func() bool { return true }, func() bool { return true }
	if supportedExePath("/usr/lib/firefox/firefox") != "firefox" || supportedExePath("/opt/microsoft/msedge/msedge") != "edge" {
		t.Error("ready Firefox or Edge not a supported browser")
	}
	if b, _ := SupportedBrowserByID("firefox"); !b.Installed() {
		t.Error("ready Firefox not installed")
	}
	if ChromeInstalled() {
		t.Error("Firefox or Edge taken for Chrome")
	}
}
