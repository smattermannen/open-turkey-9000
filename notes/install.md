# Install, packaging, uninstall, devmaster

- Covers: `Makefile`, `install.sh`, `packaging/`, `systemd/`, `cli/uninstall.go`, `cli/installguard.go`, `cli/master.go`, `cli/recover.go`, `blocker/remotecontrol_*.go`
- SPEC: §4.11, R-LOCK-9, R-ESCAPE-1
- Connects to: [device](device.md), [browsers](browsers.md), [locks](locks.md)

- Install: `/usr/local/bin/kc9k-bin`, `kc9k` wrapper (`exec sudo`),
  `/etc/sudoers.d/kc9k`, `kc9k.service`, `/var/lib/kc9k/kc9k.db`; LibreWolf
  via `extrepo` (skip `KC9K_NO_LIBREWOLF=1`; retry `kc9k browser
  install-librewolf`). Independent of the source dir.
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
