# Install, packaging, uninstall, devmaster

- Covers: `Makefile`, `install.sh`, `packaging/`, `systemd/`, `cli/uninstall.go`, `cli/installguard.go`, `cli/master.go`, `cli/recover.go`, `blocker/remotecontrol_*.go`, `blocker/librewolfauto.go`, `.github/workflows/linux-release.yml`, `ci/deb_install_check.sh`
- SPEC: §4.11, R-LOCK-9, R-ESCAPE-1
- Connects to: [device](device.md), [browsers](browsers.md), [locks](locks.md), [enforce-daemon](enforce-daemon.md), [testing](testing.md)

- Install: `/usr/local/bin/kc9k-bin`, `kc9k` wrapper (`exec sudo`),
  `/etc/sudoers.d/kc9k`, `kc9k.service`, `/var/lib/kc9k/kc9k.db`; LibreWolf
  via `extrepo` (skip `KC9K_NO_LIBREWOLF=1`; retry `kc9k browser
  install-librewolf`). Independent of the source dir.
- `.deb` (`packaging/build-deb.sh [version]`, `make deb`; the one-click
  installer, §4.11): same files as `make install`; postinst writes the
  sudo rule (visudo-checked, sudo-rs fallback), the DB folder, enables and
  restarts the service. `kc9k version` = the package version (ldflags); xz
  compressed. Files in `/usr/local` (paths shared with `install.sh`).
- LibreWolf without apt in postinst (dpkg holds the lock): the daemon
  installs it (`blocker.AutoInstallLibreWolf`, started in `daemon.RunUntil`,
  Linux only, never with `KC9K_DB`): first try 1 min after start, then
  every 30 min until installed. Steps = `installSteps("librewolf")`, each
  `apt-get -o DPkg::Lock::Timeout=600` (waits for the package installer
  still running the postinst), in a transient unit `kc9k-librewolf-install`
  (`systemd-run --collect`): a daemon restart doesn't kill apt halfway; a
  restarted daemon waits for a running unit. Marker
  `/var/lib/kc9k/librewolf-auto`: present = leave LibreWolf alone; written
  when LibreWolf is found or installed. `install.sh` and postinst remove it
  at every install (or write "skipped" with `KC9K_NO_LIBREWOLF=1`), so each
  install makes sure of LibreWolf once; one removed by the user later stays
  removed until the next install. `install.sh` still installs it itself at
  once (the daemon's first try finds it).
- Release (`.github/workflows/linux-release.yml`): tag `v1.2.3` →
  `KillerChicken9000-1.2.3-amd64.deb` (Debian version: first `-` → `~`, so
  a pre-release sorts older) + `.sha256`, checked by
  `ci/deb_install_check.sh` on `ubuntu-24.04`, attached to the release;
  waits for `windows-release.yml` on the same commit (its `gh release
  create` fails on an existing release), then uploads to its release (notes
  appended) or creates it. `workflow_dispatch`: build and check only.
- `make uninstall` → hidden `prepare-uninstall`: refused while locked; else
  removes all layers, SafeSearch/search engine policies, unsigned-extension
  permission, no-escape boot lock, leftover strict lockdown
  (`LockdownInstalled`: sudoers, polkit, PAM guard), sleep hook; leaves a
  policy telling browsers to remove the extension. LibreWolf stays.
- `TAGS=devmaster`: `kc9k master-set` / `kc9k master` (stdin) lift every
  lock, freeze, strict mode, no-escape. Root-owned
  `/var/lib/kc9k/allow-remote-control`: remote-controlled browsers not
  closed, devtools policy dropped (Marionette needs it).
- Installer guard (tamper `installer`, §4.10): hidden `prepare-install`
  (`cli/installguard.go`) refuses (logged) while on and anything locked.
  `install.sh` runs the installed binary's `prepare-install` first; the
  `.deb` prerm runs it on `upgrade` to same/older version (`@VERSION@` from
  `build-deb.sh`; newer passes) and on `failed-upgrade` (dpkg asks the
  incoming prerm when the installed one refused: must refuse too). An
  installed binary without the command doesn't refuse. Untested live
  (`ci/tamper_vm_test.sh`).
