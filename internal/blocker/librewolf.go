//go:build linux

package blocker

import (
	"os"
	"os/exec"
	"strings"
)

// LibreWolfBinary is where the LibreWolf package installs the browser.
const LibreWolfBinary = "/usr/bin/librewolf"

// LibreWolfVersion returns the installed LibreWolf package version, or "" if
// LibreWolf isn't installed.
func LibreWolfVersion() string {
	if _, err := os.Stat(LibreWolfBinary); err != nil {
		return ""
	}
	out, err := exec.Command("dpkg-query", "-W", "-f=${Version}", "librewolf").Output()
	if err != nil || len(out) == 0 {
		return "installed"
	}
	return strings.TrimSpace(string(out))
}

// InstallLibreWolf installs LibreWolf from its official repository, enabled
// through extrepo (Debian, Ubuntu and Linux Mint).
func InstallLibreWolf() error { return InstallBrowser("librewolf") }
