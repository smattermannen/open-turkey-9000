# Install, packaging, uninstall, devmaster

- Covers: `Makefile`, `install.sh`, `packaging/`, `systemd/`, `cli/uninstall.go`, `cli/master.go`, `cli/recover.go`, `blocker/remotecontrol_*.go`
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
