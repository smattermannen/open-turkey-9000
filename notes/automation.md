# Automation, schedules, modes, places, autostart

- Covers: `internal/schedule/`, `db/automation.go`, `db/modes.go`, `db/moderun.go`, `db/places.go`, `db/autostart.go`, `db/schedulelock.go`, `db/enabledmigrate.go`, `enforce/automation.go`, `modes.go`, `places.go`, `autostart.go`, `cli/automation.go`, `cli/modes.go`, `cli/schedule.go`, `cli/places.go`, `cli/autostart.go`, `cli/projectmodes.go`
- SPEC: §3.2 Mode/Schedule, §3.3, §3.6, §4.5, §4.7, R-SCHED-1–2, R-MODE-1–3, R-MONK-3
- Connects to: [locks](locks.md) (schedule lock, gates), [device](device.md) (monk mode), [circadian](circadian.md) (mode wallpapers), [gui](gui.md)

## Windows (`internal/schedule/`)

- Windows and `--allow` windows; same-day, 5-min steps, `--to 24:00` =
  midnight; overnight = two windows; overlap refused. Numbered per block
  1..n, renumbered on removal. Wall clock: DST-skipped windows
  shrink/vanish, repeated hour covered twice; durations count real hours.
- Active vs enabled (§3.3): `enforce.enabledBlocks` drops active blocks
  lifted by breaks/pauses, `scheduleLifted` (automation) and open allowance
  sessions; the daemon never switches blocks on/off (`reconcileQuotas`
  neither). `migrateScheduledSwitchOn` (one-off upgrade) in `db/enabledmigrate.go`.
- Schedule lock: a block is locked while its automation has it enabled
  (`ScheduleActiveNow`); then nothing that lifts it can be removed. Lead:
  longer always, shorter/off weakens. Old queued edits: `kc9k pending`,
  `pending cancel N`.
- Freeze schedules: `db.FreezeSchedule.LockLead`; daemon `reconcileFreezeSchedules`.

## Automation (§3.6; `schedule/automation.go`, `db/`, `enforce/`, `cli/automation.go`)

- `schedule.Automation` (windows, place ids, `DefaultOn`, `NeedBoth`):
  `Enabled(inWindow, atPlace)`; no network seen → place counts as whatever
  keeps the block enabled (`UnknownPlace`). Table `automation`; windows'
  `allow` follows the default (`SetAutomation`, `AutomationOf`).
- Every change of windows, place rules or settings goes through
  `automationStrengthens`: free only when `AutomationWeakens` finds no time
  of week and location where the block goes enabled → lifted, and no
  schedule-locked block is left continuous.
- `kc9k automation B [default enabled|disabled | needs both|either]`. GUI:
  Automation section atop the automation view (`open_automation`,
  `update_automation`, `window_flags`).

## Schedules and modes (§3.2, R-MODE-1/2, R-MONK-1–3; `db/modes.go`, `cli/modes.go`, `enforce/modes.go`)

- `db.AutomationSpec` (own windows, schedule ids, places, settings) = a
  block's or mode's automation. `SetBlockAutomation` writes it into the
  block's tables as a diff (`schedules.source`: 0 own, else schedule id),
  schedule windows clipped by `ResolveWindows` (rows never overlap), so
  readers of a block's windows are unchanged.
- Tables `saved_schedules(_windows)`, `block_schedules`, `modes` (monk row,
  `MonkModeName`), `mode_windows`, `mode_schedules`, `mode_places`,
  `mode_blocks` (one mode per block). Names per kind (`NameTaken(kind,
  name)`; a block and a mode may share one).
- Block's own automation kept while in a mode (`mode_blocks.own`,
  `KeepOwnAutomation`), restored on take-out or mode delete (`takeOut`,
  gated as a whole, R-MODE-1). A block in a mode refuses own automation
  changes; windows from a schedule can't be removed on the block.
- CLI: `resolveTarget` (block, mode, "monk", schedule) in `schedule
  add|list|remove|create|rename|delete|apply|take-off`, `place rule`,
  `automation`; every change via `changeAutomation`/`changeSchedule` →
  `gateAutomation` (all affected blocks, refused as a whole). `kc9k mode
  create|rename|delete|add|take-out|on|off|show` (switch runs
  `startCmd`/`stopCmd` per block).
- Monk mode: never enabled by default, no blocks, switched on with
  `--yes`, can then be switched off and its automation changed
  (`monkComparable`; settings password to weaken; confirmation when time
  is added); only `kc9k monk --for` is locked and sets `monk.until`.
  Daemon `enforceMonk` asks `MonkAutomationOn` each cycle;
  `monkKeepsWifi`. Announced 5 and 1 min before its windows.
- GUI: modes above block cards, mode view, Schedules and Places pages,
  Overview "Mode: X" and "Monk mode today …".

## Autostart (§3.6; `db/`, `enforce/`, `cli/autostart.go`)

once/at/sign-in/next-window; daemon `runAutostarts` in `Tick` (first look
only records; sign-in = new loginctl session or wake by BOOTTIME−MONOTONIC).

## Places (`db/places.go`, `enforce/places.go`, `wifi_*.go`, `cli/places.go`)

- BSSIDs from `nmcli` (Windows: Native Wifi API, `ScanWifi`). `AtPlace`
  needs `Place.Needed()` in range (`places.need`, default 2, `place need P
  N`, `place add --need`); ≤ what the place knows. More needed weakens,
  fewer free. Re-adding a name adds BSSIDs now seen.
- `seenNetworks` every 10 s (none seen: every 2 s, scan every 15 s). None
  seen: unknown (see Automation); Wi-Fi switched back on (`keepWifiOn` via
  `wifiRadio`/`setWifiRadio`, faked in tests).
- A rule can go on any block; adding/removing one gated like any
  automation change (unticking the last place = continuous = stronger).
  `place need` more and `place remove` in the weakening audit.
- GUI: "Add this place", "Save where I am now as…", networks needed per
  place (`ci/gui_places_smoke.py`, fake `nmcli`).
