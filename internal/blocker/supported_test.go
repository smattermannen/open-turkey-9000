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

	if err := InstallBrowser("firefox"); err == nil || !strings.Contains(err.Error(), "Mozilla has signed") {
		t.Errorf("installed Firefox before it can get the extension: %v", err)
	}
	if err := InstallBrowser("brave"); err == nil {
		t.Error("installed an unknown browser")
	}
	if steps, err := installSteps("chrome"); err != nil || steps[len(steps)-1][3] != "google-chrome-stable" {
		t.Errorf("chrome steps = %v, %v", steps, err)
	}
}

// 2.12 on Linux: Edge and Firefox are found and named with what they wait
// for (the Chrome Web Store listing, Mozilla's signature); until then they
// are no supported browser (closed like any other with block unsupported
// browsers on), afterwards they are.
func TestEdgeAndFirefoxWait(t *testing.T) {
	fakeInstalled(t, "/opt/microsoft/msedge/msedge", "/usr/lib/firefox/firefox")
	oldStore, oldSigned := chromeStoreReady, mozillaSigned
	t.Cleanup(func() { chromeStoreReady, mozillaSigned = oldStore, oldSigned })
	chromeStoreReady = func() bool { return false }
	mozillaSigned = func() bool { return false }
	for id, want := range map[string]string{"edge": WaitingChromeWebStore, "firefox": WaitingMozillaSignature,
		"chrome": "", "chromium": "", "librewolf": ""} {
		b, _ := SupportedBrowserByID(id)
		if got := b.Waiting(); got != want {
			t.Errorf("%s waits for %q, want %q", id, got, want)
		}
	}
	edge, _ := SupportedBrowserByID("edge")
	firefox, _ := SupportedBrowserByID("firefox")
	if !edge.Found() || !firefox.Found() || edge.Installed() || firefox.Installed() || ExtensionBrowserInstalled() {
		t.Error("a waiting browser counts as a supported one")
	}
	for _, exe := range []string{"/opt/microsoft/msedge/msedge", "/usr/lib/firefox/firefox", "/snap/firefox/current/usr/lib/firefox/firefox"} {
		if id := supportedExePath(exe); id != "" {
			t.Errorf("%s supported while waiting: %q", exe, id)
		}
	}
	chromeStoreReady = func() bool { return true }
	mozillaSigned = func() bool { return true }
	if supportedExePath("/usr/lib/firefox/firefox") != "firefox" || supportedExePath("/opt/microsoft/msedge/msedge") != "edge" ||
		!firefox.Installed() || !ExtensionBrowserInstalled() || ChromeInstalled() {
		t.Error("Firefox and Edge not supported once they can run the extension")
	}
}
