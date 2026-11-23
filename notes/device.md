# Monk mode, Frozen Chicken, strict mode, no-escape

- Covers: `enforce/monk.go`, `lockdown.go`, `resetguard.go`, `db/monk.go`, `freeze*.go`, `internal/freeze/`, `blocker/netcut.go`, `sockets.go`, `lockdown.go`, `pamguard.go`, `firmware.go`, `noexec.go`, `bootlock.go`, `freeze.go`, `allowedapps.go`, `cli/freeze*.go`, `strict*.go`, `resetguard.go`, `firmware.go`, `guidedsetup.go`
- SPEC: §3.8, §4.8, §4.10 Device protection, R-MONK-1–3, R-FREEZE-1–3, R-STRICT-1, R-ESCAPE-1
- Connects to: [automation](automation.md) (monk automation), [locks](locks.md), [layers-net](layers-net.md), [install](install.md)

## Monk mode (`enforce/monk.go`, `db/monk.go`, `blocker/netcut.go`)

- `kc9k monk --for D` (no `--for`: state); longer time extends. Settings
  `monk.until` (trusted time), `monk.net` (`db.MonkNet`, saved once).
  `monkBlock()` (`*.*`, `file://*`, `MonkListID`) added in memory by
  `withMonk`, never stored.
- `enforceMonk` in `Tick` holds three cuts within seconds (nmcli radio,
  `ip link set down` on real Ethernet, nft/iptables drop all but loopback;
  `ApplyFirewallCut`), restores at end; Wi-Fi that was off stays off.
  Vars `wifiRadio`, `setLink`, `cutFirewall` faked in `TestMain`;
  `KC9K_DB` cuts nothing (`monkDeviceless`).

## Frozen Chicken (`db.FrozenChickenName`, freeze tables)

- A mode: gui-state `modes` entry `freeze` after monk mode
  (`guiFrozenChicken`); `mode on|off|show freeze`; no sidebar page.
- `freeze --for D|--until T --mode lock|logout|shutdown|apps --yes`
  (`apps`: allowed apps only, `freeze apps add|remove`); `freeze schedule
  add --days … --from --to [--mode] [--lock-lead D]`, `list`, `remove N`,
  `lock N on --lead D`. Confirmed as a lock (`confirmLocks`).
- Back-to-back windows run on; running freeze never shortened, only
  extended; ≤ a week; time counts while off; shutdown mode powers off at
  each boot. `freeze.ExtendMode`: a running lock/logout/shutdown freeze
  keeps its mode when extended to `apps` (refused, logged).
- Logout mode (`enforceFreeze`): `HoldLogins` writes `/run/nologin`
  (marked, until when; pam_nologin refuses all but root), then
  `LogoutUsers` ends each user not already "closing"; `ReleaseLogins`
  removes only ours. Never re-end sessions every cycle: systemd 255's
  logind spins at 100% CPU, no login after (B2).
- Devmaster `master` works from a TTY in lock mode only.
- Allowed apps (`blocker/allowedapps.go`): `appsToClose` (pure) keeps
  system/desktop programs, allowed ones and descendants (≤ 64 parents),
  leaves root's, other system users' and unreadable ones. Vars
  `readAppProcesses`, `killApp`, `freezeLogout`, `freezePowerOff`,
  `freezeLockScreen`, `freezeCloseApps` faked in `TestMain`.

## Strict mode and lockdown

- Triggers on any lock, a running freeze or an on schedule lock (not
  allowances or unlocked schedules). Doesn't stop recovery mode, other OSes
  or a separately set root password. Warns if user is in group `disk`.
  `strict unlock` lifts its text/password lock, stays on.
- Lockdown: `/etc/sudoers.d/zz-kc9k-lockdown`,
  `/etc/polkit-1/rules.d/00-kc9k-lockdown.rules`; `sockets.go` closes
  docker/lxd/incus/libvirt sockets; PAM root guard (`pamguard.go`: marked
  `auth requisite pam_succeed_if.so uid ne 0` in `login`, `su`, `su-l`,
  after `pam_rootok` so root keeps su; `UnguardPAMConfig` removes only
  marked lines).
- No exec (`noexec.go`) from `/home`, `/tmp`, `/var/tmp`, `/dev/shm`: each
  `--rbind` onto itself, remounted noexec; submounts and
  `/run/user/<uid>` made noexec in place (`noexecSubmounts`; runtime ones
  in `/run/kc9k-noexec-mounts`). `visibleMounts` judges only the mount in
  view. `RemoveNoexec(exempt)` removes only our binds (fstab noexec stays),
  exempt first, then `--make-rprivate` + `umount -l`.
- `ApplyLockdown` sets `kernel.apparmor_restrict_unprivileged_userns=1`
  (blocks `unshare -Urm` exec tmpfs; old value in
  `/run/kc9k-userns-restrict`, restored). Side effect: Electron/Chromium
  apps without AppArmor profile in an exempt folder abort unless
  `--no-sandbox` (ROADMAP §5).
- `CloseNoexecPrograms` each cycle: kills Flatpaks whose app/runtime path
  is noexec, and processes whose exe a person owns in a noexec/runtime
  folder or below a world-writable one (`forbiddenPlace`,
  `worldWritableAbove`, e.g. `/var/crash`), exempt folders excepted.
  Closed within a cycle, not prevented (B3). Still open: a person's file
  elsewhere (e.g. `/opt` tarball keeping its owner).
- Boot param `kc9k.nofreeze` skips freeze and lockdown for that boot
  (Windows: Safe Mode, [windows](windows.md)); `kc9k recover` boot:
  `recoveryBoot`, `cli/recover_*.go`.
- `enforceLockdown` uses `enforce` vars `applyLockdown`, `removeLockdown`,
  `applyNoexec`, `removeNoexec`, `closeNoexecPrograms`, `kernelCmdline`,
  faked in `TestMain` (`TestEnforceLockdownWhen`).

## No-escape

`no-escape on --yes`: disables `kc9k.nofreeze`, hides recovery, random
GRUB password. Can't stop USB boot without a firmware password. `off`
while locked refused and logged.

## Firmware lock (`blocker/firmware.go`, `cli/firmware.go`)

- `kc9k firmware-lock [--json]`; `on` (lock confirmation; password on
  stdin, twice on a terminal; settings password first if `password locks
  on`); `off` (settings password, firmware password; refused and logged
  while anything is locked). Not a KC9K lock: holds nothing. Untested on
  real firmware.
- Via `FirmwareRoot` (`/sys/class/firmware-attributes`; tests generate a
  tree; `KC9K_DB` touches nothing, `firmwareDeviceless`). Settings found by
  folded name (`firmwarePurposes`: USB boot, network boot, one-time boot
  menu off; boot order lock on), set from `possible_values`; then admin
  password (`authentication/Admin` or `Setup Password`); with one set, each
  change writes `current_password` first. Settings `device.firmware-lock`,
  `device.firmware-before` (restored by `off`). No `FirmwareRoot`:
  `platformFirmware` (Windows vendor WMI, W19).

## Guided setup (`cli/guidedsetup.go`)

`kc9k guided-setup [--json]`, read only. Steps (`setupStep`: id,
done/todo/by-hand/info/unknown, command): settings password, no weakening,
tamper protection, strict mode, its lock (optional), root-equivalent
groups, root password, installer media guard, firmware lock, other systems
(info), no-escape last. Per system: `guidedsetup_linux.go` (`shadowPath`,
`grubCfgPath`, replaced in tests), `guidedsetup_windows.go`. GUI: Device
protection's Firmware lock… and Guided setup… (`show_guided_setup`, Do it…
→ `guided_action`).
