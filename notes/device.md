# Monk mode, Frozen Chicken, strict mode, no-escape

- Covers: `enforce/monk.go`, `lockdown.go`, `resetguard.go`, `db/monk.go`, `freeze*.go`, `internal/freeze/`, `blocker/netcut.go`, `sockets.go`, `lockdown.go`, `noexec.go`, `bootlock.go`, `freeze.go`, `allowedapps.go`, `cli/freeze*.go`, `strict*.go`, `resetguard.go`
- SPEC: §3.8, §4.8, §4.10 Device protection, R-MONK-1–3, R-FREEZE-1–3, R-STRICT-1, R-ESCAPE-1
- Connects to: [automation](automation.md) (monk automation), [locks](locks.md), [layers-net](layers-net.md), [install](install.md)

## Code

- `sockets.go`: strict mode closes docker/lxd/incus/libvirt sockets.
  Strict lockdown: `/etc/sudoers.d/zz-kc9k-lockdown`,
  `/etc/polkit-1/rules.d/00-kc9k-lockdown.rules`; no exec from `/home`,
  `/tmp`, `/var/tmp`, `/dev/shm` (`noexec.go`: each `--rbind` onto itself,
  remounted noexec; mounts inside, made before or after, such as a guest's
  home, and people's `/run/user/<uid>` made noexec in place,
  `noexecSubmounts`, `remountExec`; the runtime ones listed in
  `/run/kc9k-noexec-mounts` to give back). `mountOptions`/`visibleMounts`
  judge only the mount in view at a point (stacked and propagated copies).
  `RemoveNoexec(exempt)` (`removeNoexecPlan`): only our binds (`ourBind`:
  bound onto itself; a folder noexec by fstab stays), exempt binds first
  (their noexec copy beneath goes with them), then `--make-rprivate` and
  `umount -l`, so mounts made meanwhile stay. `ApplyLockdown` sets
  `kernel.apparmor_restrict_unprivileged_userns` to 1 (old value in
  `/run/kc9k-userns-restrict`, put back by `RemoveLockdown`).
  `CloseNoexecPrograms` each cycle: kills people's Flatpaks whose
  `.flatpak-info` app or runtime path is in a noexec folder, and, in the
  daemon's mount namespace, processes whose exe a person owns in a noexec
  or runtime folder or below a world-writable folder (`forbiddenPlace`,
  `worldWritableAbove`), exempt folders excepted. Boot param
  `kc9k.nofreeze` skips freeze and lockdown for that boot.
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
  Holes closed (B3, VM 2026-11-20): `unshare -Urm` mounting an exec tmpfs
  (Mint turns AppArmor's userns restriction off; strict turns it on, as
  Ubuntu has it); programs run from `/run/user/<uid>`; a `--user` Flatpak
  or a program from `/home` started before the lockdown (closed within a
  cycle); a copy in `/var/crash` (Ubuntu) and other world-writable folders
  outside the noexec folders (closed within a cycle, not stopped: a
  program that ends at once still runs). A person's file elsewhere (a
  tarball in `/opt` keeping its owner) still runs. With the restriction on,
  an Electron/Chromium app without an AppArmor profile in an exempt folder
  aborts ("SUID sandbox helper ... not configured correctly") unless run
  with `--no-sandbox` (ROADMAP §5).
- **No-escape** (`no-escape on --yes`): disables `kc9k.nofreeze`, hides
  recovery, random GRUB password. Can't stop USB boot without a firmware
  password.
