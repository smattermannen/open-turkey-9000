# Locks, weakening, passwords, access

- Covers: `internal/lock/`, `db/locks.go`, `db/layers.go`, `cli/lockflags.go`, `layers.go`, `confirm.go`, `loosen.go`, `control.go`, `toggle.go`, `extend.go`, `textlock.go`, `settingspassword.go`, `noweakening.go`, `access.go`, `blockpassword.go`, `enforce/lock.go`, `enforce/clockcheck.go`
- SPEC: §3.4, §3.5, §3.11, §4.6, R-LOCK-1–12, R-WEAK-1–5
- Connects to: everything gated: [lists](lists.md), [automation](automation.md), [browsers](browsers.md), [hotkeys](hotkeys.md), [device](device.md); [testing](testing.md) (weakening audit)

## Code

- **Locks** (`internal/cli/`): `blockHold` decides lock state (text, timer,
  password, restart, delay, schedule lock `db/schedulelock.go`). Layered
  (R-LOCK-6/7, `db/layers.go`, `cli/layers.go`): `LockExtras.Order` from
  the lock flags' order (`trackLockFlagOrder`), `LayersOn`, `unlock` lifts
  only the first; `TimerAfter`/`RestartPending` wait behind others;
  `AdvanceLayers` in `blockHold` and the daemon (`finishDelayedStops`).
  Several of one kind (§3.4, owner 2026-11-20): a kind already there waits
  in `LockExtras.Queue` (`lock_queue`, JSON `QueuedLock`) after all placed
  locks (`PlacedLayers`; `Layers`/`LayersOn` append it); `AdvanceLayers`
  `promote`s the first once no placed lock is on. Once anything is queued,
  every new lock queues (order kept); every kind queues, range included
  (`QueuedLock.Range*`). Setting locks: still one per kind.
- **Re-lock** (§3.4, owner 2026-11-20): a block keeps every lock it was
  given, in order and as given (a duration, not an end), in
  `LockExtras.Config` (`lock_config`; `start` and `strengthenLocks` append
  via `queuedLock`; a block without one gets `db.Snapshot` of its locks).
  With `Relock` and a `Config` (`relocksConfig`), unlocking removes a lock
  as without re-lock; `AdvanceLayers` starts `LiftedUntil` once no lock is
  on, clears it if one is added meanwhile, and when it is over
  `restoreConfig` empties every place and queues `Config`, so the first
  lock is on and the rest wait (durations count from then). `--relock`
  goes with any lock. Without a `Config` (setting locks, blocks locked
  before this) the old lift of text/password locks (`LiftedKinds`) stays.
- **Extend** (`cli/extend.go`, §4.6: its own action): `kc9k extend B --for D`
  adds D, `--until T` moves the end later; to the running timer lock, else
  the waiting one (`TimerAfter`, `--for` only), else the first queued.
  Confirmed (`--yes`). `Config` keeps the duration as first given.
  `ensureEditable` refuses only weakening of locked blocks; `start` on an
  active block only strengthens. `gateEdit` (`loosen.go`) no longer queues;
  daemon still applies old queued edits (`pending.go`). `unlock` lifts
  text/password locks and starts delay countdown (`delay_until`).
- **Lock confirmation** (`cli/confirm.go`, R-LOCK-12): every lock path builds
  `lockToSet` items and calls `confirmLocks` before writing; terminal asks
  "yes", otherwise needs `--yes` (`addYesFlag`). Tests `confirm_test.go`
  (`lockCases`). GUI: `confirm_lock` then `--yes`. Hotkeys: confirmed at
  `hotkey add`, and at each press via `blocker.ConfirmDesktop` (zenity as
  session user; none = no) unless `no_confirm`. Automatic monk mode needs
  `--no-confirm-ok` (`requireMonkAck`).
- **Text lock** (`internal/lock/`): raw-mode, paste-resistant; 60% letters,
  35% digits, 5% symbols; 98% accuracy; default 300 chars;
  `--lock-words`, `--lock-text`.
- **Settings password** (`cli/settingspassword.go`):
  `requireSettingsPassword` in `gateEdit`, `ensureEditable`, stop/unlock,
  settings off, strict off, no-escape, prepare-uninstall;
  `requireLockPassword` only with `password locks on`. Passwords one per
  line via `stdinLines` (settings password first). The GUI sends the one it
  remembers with hidden `--settings-line` (`readSettingsLine`: read whether
  needed or not, so it is never taken for a lock's password). GUI retries
  once after asking.
- **No weakening** (`cli/noweakening.go`, R-WEAK-4):
  `refuseSettingWeakening` in every setting's weakening path. Strict mode's
  lock `enforce.StrictLockName` counts in `AnySettingLocked` but doesn't
  start lockdown.
- **Access passwords** (`cli/access.go`): per area, setting
  `access.password.<area>` (shared hash when set together). `accessGate`
  (root `PersistentPreRunE`, `commandAreas`) needs the app password plus one
  area; `daemon`, `hotkey run`, `gui-state` exempt. Passwords first on
  stdin, counted by hidden `--access-lines N`. `gui-state` reports
  `access`/`access_unused`, `hideLocked` empties locked pages.
- **Block password** (`cli/blockpassword.go`, `blocks.view_password`, §3.11):
  `revealBlock` before showing contents; gui-state `--reveal B` (password on
  stdin) else `hidden` with entries emptied; GUI `block_passwords`.

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **Starting on an active block** only adds locks: any lock kind can be
  added, last in the unlock order (a duration timer or restart lock waits
  behind locks on). A kind it already has, of any kind and also from a
  hotkey press, is a second lock of it, queued last (`queuedLock`); a lock
  already set is never changed. A second range lock replaces the first
  only once the first is off (out of its times) with nothing before it on.
  A challenge longer than the
  terminal is paged (Enter per page), accuracy over all.
- **Trusted time** (`db/clock.go`): the clock; with `tamper clock` on and a
  timer lock running (blocks, setting locks, monk mode), only the boot
  clock moves it, powered-off time not counted. **Clock check** (§4.10,
  SPEC §6 2026-11-18; `enforce/clockcheck.go`, daemon every 5 min): once
  per boot while that holds, two of google/cloudflare/microsoft over HTTPS
  must agree (Date header, no-cache, the earlier taken; the DoH resolvers'
  IPs are firewalled) and the RTC is read (`/sys/class/rtc/rtc0`);
  `CheckClock` keeps an anchor (real, trusted, boot clock, RTC). The next
  anchor raises trusted to anchor trusted + real time passed, capped by
  the RTC's count; the system clock plays no part. The anchor is a floor
  in `TrustedNow` for its boot, dropped whenever no timer lock runs (else
  a clock set back before the next lock would shorten it,
  `TestClockAnchorDoesNotShortenNextLock`). Offline: nothing given back. Journal:
  "clock check: … counted".
- **Strengthening allowed while locked**: add apps/titles/blacklist entries,
  `list create`, blacklist scope/action on, lower threshold/hits, remove
  exceptions or exception scopes, delete an exception list, copy a cluster
  into a blacklist, record into a blacklist. Everything else weakens.
- **Locks**: flags `--for`, `--until`, `--lock` (`--text-lock`),
  `--password-stdin`, `--restart`, `--delay`, `--range "DAYS HH:MM-HH:MM"`
  (`--range-invert`; `LockExtras.InRange`), `--relock D` (text/password
  only; re-locks unless stopped meanwhile). Browser setting locks: same kinds
  minus delay (re-lock too: `SettingLock.LiftedUntil`, `unlockSetting`; also
  strict mode, global ladder, tamper); added later only stronger; password can't be replaced;
  `kc9k browser unlock O` unlocks and turns off. While a browser setting is
  locked, strict mode and no-escape hold and uninstall is refused.
  `block-devtools` also can't go off while a block is locked or a freeze
  runs. Block unsupported browsers (`kc9k tamper unsupported-browsers on
  [lock flags]`, `tamper unlock unsupported-browsers`; old `browser
  force-librewolf` hidden): own lock as a browser setting's; off refused
  while anything is locked; on refused with no supported browser installed.
- **Settings password**: needed for weakening, `stop`, `unlock`, schedules,
  a new or larger allowance, browser settings off, strict off, no-escape,
  uninstall. Free: reading, starting, adding entries, actions on, opening a
  session, less allowance or removing it. GUI
  remembers it until closed. Lost: `kc9k-bin recover` as root in a recovery boot
  (not with no-escape) removes it and password locks;
  devmaster `master` clears it.
- **Access passwords**: command areas — `block`, `list`, `schedule`,
  `quota`… → blocks; `freeze`, `strict`, `no-escape` → device; `browser`,
  `hotkey`, `password`, `access` → settings; `status`, `start`, `stop`,
  `unlock` → dashboard or blocks. Wrong password: 1 s delay + `access`
  event. Changing one asks for it (and the settings password). GUI unlock
  lasts until close, Lock or idle timeout (own settings file). Root can
  remove all.
