# Monk mode, Frozen Chicken, strict mode, no-escape

- Covers: `enforce/monk.go`, `lockdown.go`, `resetguard.go`, `db/monk.go`, `freeze*.go`, `internal/freeze/`, `blocker/netcut.go`, `sockets.go`, `lockdown.go`, `noexec.go`, `bootlock.go`, `freeze.go`, `allowedapps.go`, `cli/freeze*.go`, `strict*.go`, `resetguard.go`
- SPEC: §3.8, §4.8, §4.10 Device protection, R-MONK-1–3, R-FREEZE-1–3, R-STRICT-1, R-ESCAPE-1
- Connects to: [automation](automation.md) (monk automation), [locks](locks.md), [layers-net](layers-net.md), [install](install.md)

## Code

- `sockets.go`: strict mode closes docker/lxd/incus/libvirt sockets.
  Strict lockdown: `/etc/sudoers.d/zz-kc9k-lockdown`,
  `/etc/polkit-1/rules.d/00-kc9k-lockdown.rules`; no exec from `/home`,
  `/tmp`, `/var/tmp`, `/dev/shm`. Boot param `kc9k.nofreeze` skips freeze
  and lockdown for that boot.
- **Monk mode** (`enforce/monk.go`, `db/monk.go`, `blocker/netcut.go`,
  `ApplyFirewallCut`): settings `monk.until` (trusted time), `monk.net`
  (`db.MonkNet`, saved once, `SaveMonkNetIfNone`). `monkBlock()` built in
  memory (`*.*`, `file://*`, `MonkListID`), added by `withMonk`, never
  stored. `enforceMonk` in `Tick` holds the three cuts (nmcli radio, `ip
  link set down` on real Ethernet from /sys/class/net, nft/iptables drop
  all but loopback), restores at end. Device vars (`wifiRadio`, `setLink`,
  `cutFirewall`) faked in `TestMain`; `KC9K_DB` cuts nothing
  (`monkDeviceless`). `migrateMonkMode` removed the old block.

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **Monk mode**: `kc9k monk --for D` (no `--for` shows state); longer time
  extends; reapplies cuts within seconds; Wi-Fi that was off stays off.
- **Frozen Chicken** (a mode, owner 2026-11-17: `db.FrozenChickenName`, kept in
  the freeze tables; gui-state `modes` entry `freeze` after monk mode,
  `guiFrozenChicken`; `mode on|off|show freeze` say why/what; no sidebar page,
  its sections `build_device` in the mode view): `freeze --for D|--until T --mode lock|logout|shutdown|apps --yes`
  (`apps`: allowed apps only, `freeze apps add|remove`);
  `freeze schedule add --days … --from --to [--mode] [--lock-lead D]`,
  `list`, `remove N`, `lock N on --lead D`. Back-to-back windows run on; a
  longer running freeze is never shortened; ≤ a week; time counts while off;
  only extendable; shutdown mode powers off at each boot. Logout mode
  (`enforceFreeze`): `HoldLogins` writes `/run/nologin` (marked "Killer
  Chicken 9000", until when) so pam_nologin refuses every login but root's
  (LightDM greeter: "Failed to authenticate"; ssh prints the text), then
  `LogoutUsers` ends each user not already "closing"; the file goes
  (`ReleaseLogins`, only ours) once the freeze ends or isn't logout. Ending
  sessions that came in during the freeze, again every cycle, left
  systemd 255's logind spinning at 100% CPU: no login after it (B2).
  Devmaster `master` works from a TTY in lock mode only.
- **Strict mode** triggers on any lock, a running freeze or an on schedule
  lock (not allowances or unlocked schedules). Doesn't stop recovery mode,
  other OSes or a separately set root password. Warns if the user is in
  group `disk`. `strict unlock` lifts its text/password lock, stays on.
- **No-escape** (`no-escape on --yes`): disables `kc9k.nofreeze`, hides
  recovery, random GRUB password. Can't stop USB boot without a firmware
  password.
