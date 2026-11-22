# Automation, schedules, modes, places, autostart

- Covers: `internal/schedule/`, `db/automation.go`, `db/modes.go`, `db/moderun.go`, `db/places.go`, `db/autostart.go`, `db/schedulelock.go`, `db/enabledmigrate.go`, `enforce/automation.go`, `modes.go`, `places.go`, `autostart.go`, `cli/automation.go`, `cli/modes.go`, `cli/schedule.go`, `cli/places.go`, `cli/autostart.go`, `cli/projectmodes.go`
- SPEC: §3.2 Mode/Schedule, §3.3, §3.6, §4.5, §4.7, R-SCHED-1–2, R-MODE-1–3, R-MONK-3
- Connects to: [locks](locks.md) (schedule lock, gates), [device](device.md) (monk mode), [circadian](circadian.md) (mode wallpapers), [gui](gui.md)

## Code

- **Schedule** (`internal/schedule/`): windows and `--allow` windows. Wall
  clock: DST-skipped windows shrink/vanish, repeated hour covered twice;
  durations count real hours. Active vs enabled (§3.3): `enforce.enabledBlocks`
  drops active blocks lifted by breaks/pauses and by `scheduleLifted`
  (automation) and open allowance sessions; the daemon never switches
  blocks on or off (`reconcileQuotas` neither). Freeze schedules:
  `db.FreezeSchedule.LockLead`; daemon `reconcileFreezeSchedules`.
- **Automation** (§3.6, owner 2026-11-16; `schedule/automation.go`,
  `db/automation.go`, `enforce/automation.go`, `cli/automation.go`):
  `schedule.Automation` (windows, place ids, `DefaultOn`, `NeedBoth`):
  `Enabled(inWindow, atPlace)`; with no network seen a place counts as
  what keeps the block enabled (`UnknownPlace`). Table `automation`
  (default, both); a block's windows' `allow` follows the default
  (`SetAutomation`), so with windows they decide (`AutomationOf`).
  `scheduleLifted` and `ScheduleActiveNow` (schedule lock holds) use it.
  Every change of windows, place rules or settings goes through
  `automationStrengthens`: free only when `AutomationWeakens` finds no
  time of the week and location where the block goes from enabled to
  lifted, and it doesn't leave a schedule-locked block continuous. `kc9k
  automation B [default enabled|disabled | needs both|either]`; GUI
  Automation section atop the automation view (`open_automation`, from the
  block's menu; `update_automation`,
  `window_flags`: new windows follow the default);
  `ci/gui_automation_smoke.py`, `TestAutomationCommand`.
- **Schedules and modes** (§3.2 Mode, §3.6 Schedule, R-MODE-1/2,
  R-MONK-1–3; owner 2026-11-17; `db/modes.go`, `cli/modes.go`,
  `enforce/modes.go`): `db.AutomationSpec` (own windows, schedule ids,
  places, settings) is what a block's or mode's automation is made of;
  `SetBlockAutomation` writes it into the block's old tables as a diff
  (`schedules.source`: 0 own, else the schedule's id; unchanged rows keep
  id and break), schedule windows clipped by `ResolveWindows` so rows never
  overlap: every reader of a block's windows is unchanged. A block's own automation is kept
  while it is in a mode (`mode_blocks.own`, `KeepOwnAutomation`,
  `OwnAutomation`) and comes back on take-out or mode delete (`takeOut`,
  gated as a whole, R-MODE-1). Tables
  `saved_schedules(_windows)`, `block_schedules`, `modes` (monk row made by
  `migrateModes`, `MonkModeName`), `mode_windows`, `mode_schedules`,
  `mode_places`, `mode_blocks` (one mode per block). Blocks, modes and
  schedules each have their own names (`NameTaken(kind, name)`; owner
  2026-11-17: a block and a mode may share one; `--mode`/`--schedule`).
  CLI: `resolveTarget` (block, mode, "monk", schedule; `targetKind`) in `schedule
  add|list|remove`, `schedule create|rename|delete|apply|take-off`, `place
  rule`, `automation`; every change through `changeAutomation` /
  `changeSchedule` → `gateAutomation` (all blocks affected, refused as a
  whole; monk: `monkComparable`, free to change while switched on, a
  settings password to weaken, confirmation when time is added). `kc9k
  mode create|rename|delete|add|take-out|on|off|show` (group switch runs
  `startCmd`/`stopCmd` per block). Daemon: `enforceMonk` asks
  `MonkAutomationOn` each cycle (nothing pushes `monk.until` ahead; only
  `kc9k monk --for` sets it); `monkKeepsWifi` leaves the radio on. Tests `TestSavedSchedules`, `TestModeBlocksShareAutomation`,
  `TestMonkModeAutomation`, `TestMonkAutomation*`, `db/modes_test.go`.
- **Autostart** (`db/autostart.go`, `enforce/autostart.go`, `cli/autostart.go`, §3.6):
  once/at/sign-in/next-window; daemon `runAutostarts` in `Tick` (first look
  only records; sign-in = new loginctl session or wake by BOOTTIME−MONOTONIC).
- **Places** (`db/places.go`, `enforce/places.go`, `wifi_*.go`,
  `cli/places.go`): BSSIDs from `nmcli` on Linux, the Native Wifi API on
  Windows (`ScanWifi`, `rescanWifi`; notes/windows.md); `AtPlace` needs `Place.Needed()` in range
  (`places.need`, default 2, `place need P N`, `place add --need`);
  `placesHere` (none seen: unknown, see Automation); `keepWifiOn` (through
  monk mode's `wifiRadio`/`setWifiRadio`, faked in tests).
  `seenNetworks` reads every 10 s (every 2 s, scan asked every 15 s, while
  none are seen).

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **Schedules**: same-day windows in 5-min steps, `--to 24:00` = midnight;
  overnight = two windows; overlap refused; a block's windows follow its
  default (`--allow` on an enabled-by-default block; on a continuous block
  it sets the default). Numbered per block 1..n, renumbered on removal. Schedule
  lock lead: longer always, shorter/off weakens. Old queued edits:
  `kc9k pending`, `pending cancel N`. Only the user switches blocks on/off;
  windows and place rules only lift an active block (a lock keeps it on, not
  enabled). The 2026-11-15 upgrade switched on every inactive block with
  windows or place rules once (`migrateScheduledSwitchOn`).
- **Schedules outside blocks, modes** (2026-11-17): `kc9k schedule create|
  apply|take-off|rename|delete`, `schedule add|remove|list NAME` for a
  block, mode ("monk" too) or schedule; a block's windows from a schedule
  can't be removed on it (take the schedule off, or change it). `kc9k mode
  create|add|take-out|on|off|rename|delete|show`; a block in a mode refuses
  automation changes of its own. Monk mode: never enabled by default, no
  blocks, switched on with `--yes`; switched on it can be switched off and
  its automation changed; only a run for a set time (`kc9k monk --for`) is
  locked; announced 5 and 1 min before its windows. GUI: modes
  above the block cards, the mode's view, Schedules and Places pages,
  Overview "Mode: X" in Start and "Monk mode today …".
- **Places**: re-adding a name adds the BSSIDs now seen; needs at most what
  the place knows; more needed weakens its blocks, fewer is free. A rule
  can go on any block (it switches nothing on); adding or removing one is
  gated like any automation change (unticking the last place makes a
  block continuous: stronger). `place need` more and `place remove` are
  in the weakening audit (`TestLockedBlockRefusesPlaceWeakening`). With a schedule lock, a block is
  locked while its automation has it enabled; then nothing that lifts it
  can be removed. GUI: "Add this place", "Save where I am now as…"
  (ticked: add it to the block), networks needed per place (`ci/gui_places_smoke.py`, fake
  `nmcli`). No networks seen: what keeps each block enabled; Wi-Fi is
  switched back on.
