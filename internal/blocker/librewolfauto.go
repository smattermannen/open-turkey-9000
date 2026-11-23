//go:build linux

// librewolfauto.go — the daemon installs LibreWolf by itself (§4.11, 2.11).
// A .deb's postinst can't run apt (dpkg holds its lock while it runs), so
// the one-click installer leaves LibreWolf to the daemon: a minute after it
// starts, and every half hour after a failure (offline, another package
// manager busy), until LibreWolf is installed, once. The marker file next to
// the database records that it is done (or skipped: KC9K_NO_LIBREWOLF=1 at
// install); install.sh and the .deb remove it at every install, as
// install.sh itself tries LibreWolf at every run. LibreWolf removed later
// by the user isn't put back until the next install.
//
// The apt commands run in a transient systemd unit of their own
// (LibreWolfInstallUnit), not as the daemon's children: a daemon restart
// (an upgrade, install.sh) stops every process in the daemon's cgroup, and
// an apt-get stopped halfway leaves dpkg needing `dpkg --configure -a`. A
// restarted daemon finds the unit still running and waits for it; the unit
// name also keeps two tries from running at once.
package blocker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// LibreWolfAutoFile is the marker, in the database's folder: present, the
// daemon leaves LibreWolf alone. install.sh and the .deb's postinst write or
// remove it (keep the name in step with them).
const LibreWolfAutoFile = "librewolf-auto"

// LibreWolfInstallUnit is the transient unit running the installation; its
// output is in `journalctl -u kc9k-librewolf-install`.
const LibreWolfInstallUnit = "kc9k-librewolf-install"

// aptLockWait is how long each apt-get waits for the package lock: right
// after a .deb install, Mint's package installer (or apt) still holds it
// while our postinst restarts the daemon.
const aptLockWait = "DPkg::Lock::Timeout=600"

var (
	// libreWolfAutoDelay is the wait before the first try: the network
	// after boot, the package installer finishing, install.sh's own try.
	libreWolfAutoDelay = time.Minute
	// libreWolfAutoPoll is the wait while the unit runs.
	libreWolfAutoPoll = 30 * time.Second
	// libreWolfAutoRetry is the wait after a failed try.
	libreWolfAutoRetry = 30 * time.Minute

	// libreWolfPresent reports whether LibreWolf is installed.
	libreWolfPresent = func() bool {
		b, _ := SupportedBrowserByID("librewolf")
		return b.Installed()
	}
	// libreWolfUnitActive reports whether the unit is running.
	libreWolfUnitActive = func() bool {
		return exec.Command("systemctl", "is-active", "--quiet", LibreWolfInstallUnit).Run() == nil
	}
	// libreWolfUnitStart starts the unit running script and returns once
	// it has started.
	libreWolfUnitStart = func(script string) error {
		out, err := exec.Command("systemd-run", "--unit="+LibreWolfInstallUnit, "--collect", "--quiet",
			"--description=Killer Chicken 9000: installing LibreWolf",
			"--setenv=DEBIAN_FRONTEND=noninteractive", "--", "/bin/sh", "-c", script).CombinedOutput()
		if err != nil {
			return fmt.Errorf("systemd-run: %v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
)

// libreWolfScript is installSteps("librewolf") as one shell command, every
// apt-get waiting for the package lock instead of failing at once.
func libreWolfScript() (string, error) {
	steps, err := installSteps("librewolf")
	if err != nil {
		return "", err
	}
	cmds := make([]string, 0, len(steps))
	for _, step := range steps {
		if step[0] == "apt-get" {
			step = append([]string{"apt-get", "-o", aptLockWait}, step[1:]...)
		}
		quoted := make([]string, len(step))
		for i, arg := range step {
			quoted[i] = "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
		}
		cmds = append(cmds, strings.Join(quoted, " "))
	}
	return strings.Join(cmds, " && "), nil
}

// AutoInstallLibreWolf installs LibreWolf once if it is missing (the
// daemon, §4.11): first after libreWolfAutoDelay, again every
// libreWolfAutoRetry after a failure, until it is installed. dir is the
// database's folder, where the marker lives. Returns when done or when stop
// closes; logf reports the start, each failure and the outcome.
func AutoInstallLibreWolf(dir string, stop <-chan struct{}, logf func(format string, args ...any)) {
	marker := filepath.Join(dir, LibreWolfAutoFile)
	wait := libreWolfAutoDelay
	started := false // this daemon started the unit, which hasn't ended yet
	for {
		select {
		case <-stop:
			return
		case <-time.After(wait):
		}
		if _, err := os.Stat(marker); err == nil {
			return
		}
		if libreWolfPresent() {
			note := "LibreWolf found installed"
			if started {
				note = "LibreWolf installed by the daemon"
				logf("daemon: LibreWolf installed; the extension is added the next time it starts")
			}
			line := fmt.Sprintf("%s, %s\n", note, time.Now().Format(time.RFC3339))
			if err := atomicWriteFile(marker, []byte(line), 0644); err != nil {
				logf("daemon: %s, but the marker %s: %v", note, marker, err)
			}
			return
		}
		if libreWolfUnitActive() {
			wait = libreWolfAutoPoll
			continue
		}
		if started {
			started = false
			logf("daemon: LibreWolf not installed (journalctl -u %s); trying again in %s ('kc9k browser install-librewolf' tries now)", LibreWolfInstallUnit, libreWolfAutoRetry)
			wait = libreWolfAutoRetry
			continue
		}
		script, err := libreWolfScript()
		if err == nil {
			err = libreWolfUnitStart(script)
		}
		if err != nil {
			logf("daemon: can't install LibreWolf: %v; trying again in %s", err, libreWolfAutoRetry)
			wait = libreWolfAutoRetry
			continue
		}
		logf("daemon: installing LibreWolf, the browser that runs the extension (journalctl -u %s)", LibreWolfInstallUnit)
		started = true
		wait = libreWolfAutoPoll
	}
}
