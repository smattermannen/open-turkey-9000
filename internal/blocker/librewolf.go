package blocker

import (
	"fmt"
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
func InstallLibreWolf() error {
	env := append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	steps := [][]string{
		{"apt-get", "install", "-y", "extrepo"},
		{"extrepo", "enable", "librewolf"},
		{"apt-get", "update"},
		{"apt-get", "install", "-y", "librewolf"},
	}
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
	if LibreWolfVersion() == "" {
		return fmt.Errorf("the installation finished but %s is missing", LibreWolfBinary)
	}
	return nil
}
