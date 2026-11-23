# Locks, weakening, passwords, access

- Covers: `internal/lock/`, `db/locks.go`, `db/layers.go`, `cli/lockflags.go`, `layers.go`, `confirm.go`, `loosen.go`, `control.go`, `toggle.go`, `extend.go`, `textlock.go`, `settingspassword.go`, `noweakening.go`, `access.go`, `blockpassword.go`, `enforce/lock.go`, `enforce/clockcheck.go`
- SPEC: §3.4, §3.5, §3.11, §4.6, R-LOCK-1–12, R-WEAK-1–5
- Connects to: everything gated: [lists](lists.md), [automation](automation.md), [browsers](browsers.md), [hotkeys](hotkeys.md), [device](device.md); [testing](testing.md) (weakening audit)

## Lock kinds and flags

- Flags: `--for`, `--until`, `--lock` (`--text-lock`), `--password-stdin`,
  `--restart`, `--delay`, `--range "DAYS HH:MM-HH:MM"` (`--range-invert`;
  `LockExtras.InRange`), `--relock D` (with any lock). Each flag given twice
  = two locks. `blockHold` (`internal/cli/`) decides lock state, incl.
  schedule lock (`db/schedulelock.go`).
- Browser setting locks: same kinds minus delay; also strict mode, global
  ladder, tamper. `kc9k browser unlock O` unlocks and turns off. While a
  browser setting is locked: strict mode and no-escape hold, uninstall
  refused. `block-devtools` can't go off while a block is locked or a
  freeze runs. Block unsupported browsers (`kc9k tamper
  unsupported-browsers on [lock flags]`, `tamper unlock
  unsupported-browsers`; hidden alias `browser force-librewolf`): own lock;
  off refused while anything is locked; on refused with no supported
  browser installed.

## Layered locks (R-LOCK-6/7, `db/layers.go`, `cli/layers.go`)

- `LockExtras.Order` from flag order (`trackLockFlagOrder`), `LayersOn`;
  `unlock` lifts only the first; `TimerAfter`/`RestartPending` wait behind
  others; `AdvanceLayers` in `blockHold` and daemon (`finishDelayedStops`).
- Several of one kind (§3.4): a kind already there waits in
  `LockExtras.Queue` (`lock_queue`, JSON `QueuedLock`, range included)
  after all placed locks (`PlacedLayers`); `AdvanceLayers` `promote`s the
  first once no placed lock is on. Once anything is queued, every new lock
  queues. Setting locks the same (`SettingLock.Queue`/`Config`, via
  `asBlock`/`fromBlock`).
- Repeated flags: `orderedValue` keeps every `Set`; `lockOccurrences` →
  `lockSpecs` (one `QueuedLock` per lock; `--lock-chars`, `--lock-words`,
  `--range-invert` apply to all; one password read per password lock) →
  `splitSpecs` places each kind's first until the first repeat, queues the
  rest. `lockArgs` keeps every occurrence (hotkeys); `parseStartLocks`
  resets `startCmd` lock flags before stored ones parse. Hotkeys and mode
  locks: at most one password lock.
- Start on an active block only adds locks, last in unlock order; a kind it
  already has (also from a hotkey) queues last; a set lock is never
  changed. A second range lock replaces the first only once the first is
  off with nothing before it on.

## Re-lock (§3.4)

- A block keeps every lock as given (duration, not end), in order, in
  `LockExtras.Config` (`lock_config`; `start`/`strengthenLocks` append; a
  block without one gets `db.Snapshot`).
- With `Relock` + `Config` (`relocksConfig`): unlock removes a lock
  normally; once no lock is on `AdvanceLayers` starts `LiftedUntil`
  (cleared if a lock is added meanwhile); when over, `restoreConfig` empties
  every place and queues `Config` (first lock on, durations count from
  then). Without `Config` (older data): old lift of text/password locks
  (`LiftedKinds`); a new lock gives a `Config`.
- `SettingLock.PendingRelock`/`RelockAt`: unlocked, re-lock time running.

## Extend, editing, unlock

- `kc9k extend B --for D | --until T` (§4.6, own action, confirmed): to the
  running timer lock, else waiting one (`TimerAfter`, `--for` only), else
  first queued. `Config` keeps the original duration.
- `ensureEditable` refuses only weakening of locked blocks. `gateEdit`
  (`loosen.go`) no longer queues; `Tick` still applies old queued edits
  (`db/pending.go`). `unlock` lifts text/password locks, starts delay
  countdown (`delay_until`).
- Strengthening allowed while locked: add apps/titles/blacklist entries,
  `list create`, blacklist scope/action on, lower threshold/hits, remove
  exceptions or exception scopes, delete an exception list, copy a cluster
  into a blacklist, record into a blacklist. Everything else weakens.

## Confirmation (R-LOCK-12, `cli/confirm.go`)

Every lock path (firmware lock too) builds `lockToSet` items →
`confirmLocks` before writing; terminal asks "yes", else `--yes`
(`addYesFlag`). Tests `confirm_test.go` (`lockCases`). GUI: `confirm_lock`
then `--yes`. Hotkeys: at `hotkey add`, and each press via
`blocker.ConfirmDesktop` (zenity as session user; none = no) unless
`no_confirm`. Automatic monk mode needs `--no-confirm-ok` (`requireMonkAck`).

## Text lock (`internal/lock/`)

- Raw-mode, paste-resistant; 60% letters, 35% digits, 5% symbols; 98%
  accuracy over all; default 300 chars; `--lock-words`, `--lock-text`.
  Longer than the terminal: paged (Enter per page).
- No terminal (GUI, scripts): own window (`lock/window.go`, R-LOCK-5):
  embedded `challenge_window.py` via `python3 -I -c`, as `SUDO_UID` on the
  kept `DISPLAY`/`XAUTHORITY` (never root), `GDK_BACKEND=x11`. No text
  field; reports keys, pastes/drops, synthetic (XSendEvent) keys;
  `readWindowKeys` rebuilds text, refuses bursts (`pasteBurst` keys within
  5 ms). Stdin never read: no window → refused. GUI Unlock
  runs the command (`run_challenge`, 6 h timeout). Test seam
  `lock.StartWindow`; real window `TestChallengeWindowOnX`.

## Trusted time (`db/clock.go`, `enforce/clockcheck.go`)

- With `tamper clock` on and a timer lock running (blocks, setting locks,
  monk mode): only the boot clock moves trusted time; powered-off time not
  counted.
- Clock check (§4.10; daemon every 5 min, once per boot while that holds):
  two of google/cloudflare/microsoft over HTTPS must agree (Date header,
  no-cache, earlier taken; DoH resolvers' IPs firewalled); RTC read
  (`/sys/class/rtc/rtc0`; Windows: [windows](windows.md)). `CheckClock`
  keeps an anchor (real, trusted, boot clock, RTC); next anchor raises
  trusted to anchor + real time passed, capped by RTC count; system clock
  ignored. Anchor is a floor in `TrustedNow` for its boot, dropped when no
  timer lock runs (`TestClockAnchorDoesNotShortenNextLock`). Offline:
  nothing given back. Journal: "clock check: … counted".

## Passwords

- **Settings password** (`cli/settingspassword.go`): needed for weakening,
  `stop`, `unlock`, schedules, new/larger allowance, browser settings off,
  strict off, no-escape, uninstall (`requireSettingsPassword` in
  `gateEdit`, `ensureEditable`, …). Free: reading, starting, adding
  entries, actions on, opening a session, less/no allowance.
  `requireLockPassword` only with `password locks on`. Passwords one per
  line on stdin (`stdinLines`, settings password first); GUI sends its
  remembered one (until close) via hidden `--settings-line`
  (`readSettingsLine`: always read, never mistaken for a lock password);
  retries once after asking. Lost: `kc9k-bin recover` as root in a
  recovery boot (not with no-escape) removes it and password locks;
  devmaster `master` clears it.
- **No weakening** (`cli/noweakening.go`, R-WEAK-4):
  `refuseSettingWeakening` in every setting's weakening path.
  `enforce.StrictLockName` counts in `AnySettingLocked` but doesn't start lockdown.
- **Access passwords** (`cli/access.go`): per area, `access.password.<area>`
  (shared hash when set together). `accessGate` (root `PersistentPreRunE`,
  `commandAreas`) needs app password + one area; `daemon`, `hotkey run`,
  `gui-state` exempt. Areas: `block`, `list`, `schedule`, `quota`… →
  blocks; `freeze`, `strict`, `no-escape` → device; `browser`, `hotkey`,
  `password`, `access` → settings; `status`, `start`, `stop`, `unlock` →
  dashboard or blocks. Passwords first on stdin (hidden `--access-lines
  N`). Wrong: 1 s delay + `access` event. Changing one asks for it (and
  settings password). GUI unlock lasts until close, Lock or idle timeout;
  `hideLocked` empties locked pages. Root can remove all.
- **Block password** (`cli/blockpassword.go`, `blocks.view_password`,
  §3.11): `revealBlock` before showing contents; gui-state `--reveal B`
  (password on stdin) else `hidden`, entries emptied; GUI `block_passwords`.
