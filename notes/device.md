# Monk mode, Frozen Chicken, strict mode, no-escape

- Covers: `enforce/monk.go`, `lockdown.go`, `resetguard.go`, `db/monk.go`, `freeze*.go`, `internal/freeze/`, `blocker/netcut.go`, `sockets.go`, `lockdown.go`, `pamguard.go`, `firmware.go`, `noexec.go`, `bootlock.go`, `freeze.go`, `allowedapps.go`, `cli/freeze*.go`, `strict*.go`, `resetguard.go`, `firmware.go`, `guidedsetup.go`
- SPEC: §3.8, §4.8, §4.10 Device protection, R-MONK-1–3, R-FREEZE-1–3, R-STRICT-1, R-ESCAPE-1
- Connects to: [automation](automation.md) (monk automation), [locks](locks.md), [layers-net](layers-net.md), [install](install.md)

## Code

- `sockets.go`: strict mode closes docker/lxd/incus/libvirt sockets.
  Strict lockdown: `/etc/sudoers.d/zz-kc9k-lockdown`,
  `/etc/polkit-1/rules.d/00-kc9k-lockdown.rules`, the PAM root guard
  (`pamguard.go`: a marked `auth requisite pam_succeed_if.so uid ne 0`
  in `login`, `su`, `su-l` under `PAMDir`, after `pam_rootok` so root
  itself keeps su; `UnguardPAMConfig` takes out only the marked lines);
  no exec from `/home`, `/tmp`, `/var/tmp`, `/dev/shm`. Boot param
  `kc9k.nofreeze` skips freeze and lockdown for that boot.
- **Firmware lock** (`blocker/firmware.go`, `cli/firmware.go`): everything
  through `FirmwareRoot` (`/sys/class/firmware-attributes`; tests point it
  at a generated tree; a `KC9K_DB` database with the real root reads
  nothing and changes nothing, `firmwareDeviceless`). Settings are found by
  folded name (`firmwarePurposes`: USB boot, network boot, one-time boot
  menu off; boot order lock on) and set from `possible_values`; then the
  admin password (`authentication/Admin` or `Setup Password`,
  `new_password`). With a password already set each change writes
  `current_password` first. Settings `device.firmware-lock`,
  `device.firmware-before` (what it changed, put back by `off`).
- **Guided setup** (`cli/guidedsetup.go`): `guidedSetup` builds the steps
  (`setupStep`: id, state done/todo/by-hand/info/unknown, command); reads
  `shadowPath` (root password) and `grubCfgPath` (other systems), both
  replaced in tests. GUI: Device protection's Firmware lock… and Guided
  setup… (`show_guided_setup`, Do it… → `guided_action`, the same handler
  as the step's own switch).
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
  only extendable; shutdown mode powers off at each boot. Devmaster
  `master` works from a TTY in lock mode only.
- **Strict mode** triggers on any lock, a running freeze or an on schedule
  lock (not allowances or unlocked schedules). Doesn't stop recovery mode,
  other OSes or a separately set root password. Warns if the user is in
  group `disk`. `strict unlock` lifts its text/password lock, stays on.
- **No-escape** (`no-escape on --yes`): disables `kc9k.nofreeze`, hides
  recovery, random GRUB password. Can't stop USB boot without a firmware
  password. `off` while locked is refused and logged.
- **Firmware lock**: `kc9k firmware-lock [--json]` shows it; `on` (lock
  confirmation, `--yes`; password on stdin, twice on a terminal; settings
  password first if `password locks on`); `off` (settings password,
  refused and logged while anything is locked, firmware password). It is
  no KC9K lock: it never holds strict mode or anything else. Live on real
  firmware: Unchecked.
- **Guided setup**: `kc9k guided-setup [--json]`, read only: settings
  password, no weakening, tamper protection, strict mode, its lock
  (optional), root-equivalent groups, root password, installer media
  guard, firmware lock, other systems (info), no-escape last.
