# Install, packaging, uninstall, devmaster

- Covers: `Makefile`, `install.sh`, `packaging/`, `systemd/`, `cli/uninstall.go`, `cli/installguard.go`, `cli/master.go`, `cli/recover.go`, `blocker/remotecontrol_*.go`
- SPEC: §4.11, R-LOCK-9, R-ESCAPE-1
- Connects to: [device](device.md), [browsers](browsers.md), [locks](locks.md)

## Code

- Install: `/usr/local/bin/kc9k-bin` (binary), `kc9k` wrapper (`exec sudo`),
  `/etc/sudoers.d/kc9k`, `kc9k.service`, `/var/lib/kc9k/kc9k.db`; LibreWolf
  via `extrepo` (skip: `KC9K_NO_LIBREWOLF=1`; retry: `kc9k browser
  install-librewolf`). Independent of the source dir.
- `make uninstall` runs hidden `prepare-uninstall`: refused while locked;
  else removes all layers, SafeSearch/search engine policies, unsigned-
  extension permission, no-escape boot lock, a strict mode lockdown a
  crash left (`LockdownInstalled`: sudoers, polkit, PAM guard), the sleep hook; leaves a
  policy telling browsers to remove the extension. LibreWolf stays.
- `TAGS=devmaster`: `kc9k master-set` / `kc9k master` (stdin) lift every
  lock, freeze, strict mode, no-escape. With root-owned
  `/var/lib/kc9k/allow-remote-control`, remote-controlled browsers aren't
  closed and the devtools policy is dropped (Marionette needs it).
- Installer guard (tamper `installer`, §4.10): hidden `prepare-install`
  (`cli/installguard.go`) refuses (logged) while the setting is on and
  anything is locked. `make install` (so `install.sh`) runs the installed
  binary's `prepare-install` before copying; the `.deb`'s prerm runs it on
  `upgrade` to the same or an older version (`@VERSION@` filled in by
  `build-deb.sh`; a newer version passes). An installed binary without the
  command ("unknown command") doesn't refuse. Live: Unchecked (vm).
