//go:build linux

// supported.go — the supported browsers (§3.13): LibreWolf, Google Chrome
// and Chromium, the browsers that run the KC9K extension. With block
// unsupported browsers on (§4.10), every other browser is closed, and these
// too unless their extension checks in (see ExtensionGuard).
//
// A supported browser is known by the executable it runs: one of the
// installed packages' files, owned by root. A copy elsewhere (a portable
// LibreWolf, a renamed Chrome) doesn't read KC9K's policies, so it is an
// unsupported browser.
package blocker

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// SupportedBrowsers lists the supported browsers, in the order shown.
var SupportedBrowsers = []SupportedBrowser{
	{ID: "librewolf", Name: "LibreWolf", Exes: []string{
		"/usr/share/librewolf/librewolf", "/usr/share/librewolf/librewolf-bin",
		"/usr/lib/librewolf/librewolf", "/usr/lib/librewolf/librewolf-bin"}},
	{ID: "chrome", Name: "Google Chrome", Exes: []string{"/opt/google/chrome/chrome"}},
	// Chromium from Linux Mint's own .deb (Debian layout, reads
	// /etc/chromium/policies); Ubuntu's chromium-browser is a snap, which
	// ignores policies.
	{ID: "chromium", Name: "Chromium", Exes: []string{"/usr/lib/chromium/chromium"}},
}

// statFile is os.Stat, replaced in tests.
var statFile = os.Stat

// Installed reports whether the browser is installed.
func (b SupportedBrowser) Installed() bool {
	for _, exe := range b.Exes {
		if _, err := statFile(exe); err == nil {
			return true
		}
	}
	return false
}

// Waiting is what b waits for before KC9K can give it the extension: on
// Linux nothing (each listed browser takes it as it is).
func (b SupportedBrowser) Waiting() string { return "" }

// SupportedBrowserByID returns the supported browser with that ID.
func SupportedBrowserByID(id string) (SupportedBrowser, bool) {
	for _, b := range SupportedBrowsers {
		if b.ID == id {
			return b, true
		}
	}
	return SupportedBrowser{}, false
}

// supportedExePath returns the ID of the supported browser whose package
// installs exe, or "".
func supportedExePath(exe string) string {
	exe = strings.TrimSuffix(exe, " (deleted)")
	for _, b := range SupportedBrowsers {
		for _, path := range b.Exes {
			if exe == path {
				return b.ID
			}
		}
	}
	return ""
}

// supportedExe returns the ID of the supported browser pid runs, or "": its
// executable must be the package's own file, owned by root.
func supportedExe(pid int) string {
	exe, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe")
	if err != nil {
		return ""
	}
	id := supportedExePath(exe)
	if id == "" || exeOwnedByUser(pid) {
		return ""
	}
	return id
}

// installSteps are the commands that install a supported browser.
func installSteps(id string) ([][]string, error) {
	switch id {
	case "librewolf":
		return [][]string{
			{"apt-get", "install", "-y", "extrepo"},
			{"extrepo", "enable", "librewolf"},
			{"apt-get", "update"},
			{"apt-get", "install", "-y", "librewolf"},
		}, nil
	case "chromium":
		// Linux Mint packages Chromium as a .deb in its own repository.
		return [][]string{{"apt-get", "install", "-y", "chromium"}}, nil
	case "chrome":
		// Google's repository, signed with the key installChromeRepo saves;
		// the package keeps its own list file up to date afterwards.
		return [][]string{
			{"apt-get", "update"},
			{"apt-get", "install", "-y", "google-chrome-stable"},
		}, nil
	}
	return nil, fmt.Errorf("unknown browser %q (use librewolf, chrome or chromium)", id)
}

// Google's apt repository for Chrome and the key that signs it.
const (
	chromeKeyURL  = "https://dl.google.com/linux/linux_signing_key.pub"
	chromeKeyPath = "/etc/apt/keyrings/google-chrome.asc"
	chromeList    = "/etc/apt/sources.list.d/google-chrome.list"
	chromeRepo    = "deb [arch=amd64 signed-by=" + chromeKeyPath + "] https://dl.google.com/linux/chrome/deb/ stable main\n"
)

// installChromeRepo saves Google's signing key and adds its repository, so
// apt checks the package's signature.
func installChromeRepo() error {
	if runtime.GOARCH != "amd64" {
		return fmt.Errorf("Google Chrome for Linux is only made for 64-bit PCs (amd64); install Chromium instead")
	}
	client := &http.Client{Timeout: time.Minute}
	resp, err := client.Get(chromeKeyURL)
	if err != nil {
		return fmt.Errorf("failed to download Google's signing key: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download Google's signing key: %s", resp.Status)
	}
	key, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("failed to download Google's signing key: %w", err)
	}
	if !strings.Contains(string(key), "-----BEGIN PGP PUBLIC KEY BLOCK-----") {
		return fmt.Errorf("Google's signing key isn't a PGP key")
	}
	if err := os.MkdirAll(filepath.Dir(chromeKeyPath), 0755); err != nil {
		return err
	}
	if err := atomicWriteFile(chromeKeyPath, key, 0644); err != nil {
		return err
	}
	return atomicWriteFile(chromeList, []byte(chromeRepo), 0644)
}

// runInstall runs commands, for InstallBrowser; replaced in tests.
var runInstall = func(steps [][]string) error {
	env := append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	for _, step := range steps {
		cmd := exec.Command(step[0], step[1:]...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) > 5 {
				lines = lines[len(lines)-5:]
			}
			return fmt.Errorf("%s failed: %w\n%s", strings.Join(step, " "), err, strings.Join(lines, "\n"))
		}
	}
	return nil
}

// InstallBrowser installs a supported browser from its official package
// source: LibreWolf through extrepo, Chromium from Linux Mint's repository,
// Google Chrome from Google's. Root only.
func InstallBrowser(id string) error {
	b, ok := SupportedBrowserByID(id)
	if !ok {
		_, err := installSteps(id)
		return err
	}
	steps, err := installSteps(id)
	if err != nil {
		return err
	}
	if id == "chrome" {
		if err := installChromeRepo(); err != nil {
			return err
		}
	}
	if err := runInstall(steps); err != nil {
		return err
	}
	if !b.Installed() {
		return fmt.Errorf("the installation finished but %s is missing", b.Exes[0])
	}
	return nil
}
