# Actions, trigger actions, consequences, bypass prevention

- Covers: `db/actions.go`, `adnetworks.go`, `triggers.go`, `custom.go`, `events.go`, `nepsis.go`, `declutter.go`, `enforce/hits.go`, `trigger.go`, `consequence.go`, `moderun.go`, `nepsis.go`, `blocker/custom.go`, `confirm.go`, `cli/triggers.go`, `nepsis.go`
- SPEC: §3.9, §3.10, R-CUSTOM-1–2, R-WARN-1–2, R-EVENT-1, R-MODE-4
- Connects to: [extension](extension.md), [device](device.md), [automation](automation.md) (mode runs), [lists](lists.md)

## Actions (`db/actions.go`, Param/Minutes)

- Page: block, greyscale, no-images, mute, no-video, dark, no-downloads
  ([extension](extension.md)), redirect, image (`db.ImagesDir`, served
  `/image/`; PNG/JPEG/GIF/WebP), close-tab. Device: close-browser, logout,
  shutdown, monk.
- Extension `kc9kOutcome` picks the strongest (close-tab > image > redirect
  > block; redirect to a blocked page shows the block page), closes
  tab/browser itself, POSTs `/hit` (`enforce/hits.go`, header required);
  daemon logs and runs logout/shutdown/monk. Only the top page triggers
  close-tab/device actions; embedded content gets the block page.
- `no-video`/`no-images` reach every frame (`tellFrames` → `tab-looks`);
  mute tab-wide (`applyMute`).
- Removed actions: `migrateRemovedActions` drops them (emptied → block);
  imports skip them (`db.IsRemovedAction`).
- Edits: longer time or another picture free; new redirect address weakens.
  List logout/shutdown/lock-screen notified 60 s ahead; one-off
  logout/shutdown ≤ every 10 min; `--for` via Frozen Chicken (≤ a week).

## Ad trigger (§3.9, `db/adnetworks.go`)

Scope `ads` (`Where.Ads`; not exception lists or phrase sets);
`WebList.AdNetworks` expands industries (`AdIndustries`: adult, gambling,
crypto, dating; `TestAdIndustries`: plain domains, one industry each,
only domains ad filter lists name or seen live). Background
`noteAdRequest` → `kc9kAdMatches` → `pageHit`, once per page and list
(`adSeen`); address/URL-term exceptions spare the page. Block with
title/text exceptions (`kc9kAdsByPage`): content script collects loaded
hosts (`noteResource`, `adHosts`), matched like an address after every
exception. Browser test: `ci/extras_test.py` block `extraads` (ad hosts
via `network.dns.localDomains`).

## Trigger actions (`db/triggers.go`, `enforce/trigger.go`, `cli/triggers.go`)

Table `trigger_actions` (block, group `apps`|`titles`, JSON actions); no
row = `close`. `kc9k block trigger B apps|titles ACTION on|off [--for D]`
(on/longer free, off/shorter weakens; monk needs `--no-confirm-ok`).
Daemon: `appsTrigger` (close or `RunningApps`), titles via
`TitleGuard.EnforceUsers` (`TitleTargets.Watch`: watch without closing);
`fireTrigger` ≤ once a minute per block+group → `runAnnounced`. GUI
`TriggerEditor`. Place rules have **no actions** (§3.6;
`migratePlaceTriggers` drops old `place:` sets).

## Consequences (`enforce/consequence.go`)

Close browsers; lock screen/logout/shutdown (via Frozen Chicken, only
extended, ≤ once per 10 min); monk; mode (`StartModeRun`,
`enforce/moderun.go`: `db.ModeRun` in `mode.runs`; `modeRunning` in
`enabledBlocks`, `ModeRunUntil` in `blockHold`, `endModeRuns` in `Tick`);
custom (`runCustom` → `blocker.RunCustom`, `customDue` once a minute; a
mode's own: `runModeCustoms`, `modes.custom`). `*.Param` carries the mode
or custom items (`db.CustomItem`); CLI `--target`/`--custom`
(`addActionParamFlags`), ladder `--step-json`. Device effects via
replaceable vars.

## Events and bypass prevention (`db/events.go`, `db/nepsis.go`, `enforce/nepsis.go`, `cli/nepsis.go`)

- Events kept a day. Refused weakening logged via `refused()`, never
  counted. Multi-block events (`RecordEventFor`): a row per block, `extra`
  on all but the first. `/hit` judges at once, answers `close_tab`.
- `db.Nepsis`: detectors, ladder, quiet time; state in
  `nepsis.ladder[.<block>]`; old one-step settings read as ladders
  (`normalize`). Global lockable as setting lock `nepsis` (`--for`); per
  block gated. Global counts all; per block counts while on; both apply.
- `bypass set --step "3 in 10m: warning, close-tab, logout 1m" … --quiet 2h
  [--block B] [--detect …] [lock flags]`; short form `--count --within
  --then [--then-for]`. Actions: `warning`, `close-tab`, `close-browser`,
  `lock-screen D`, `logout [D]`, `shutdown [D]`, `monk D`. Past the last
  step repeats the last.
- Detectors: `page` (excludes look-only actions); `search` = page from a
  search origin (`fromSearch`: Google, Bing, DDG, Brave, Startpage…, any
  `/search?q=`); `tamper` = daemon restart within a boot (not upgrade or
  self-reported crash) or clock moved > 2 min vs the boot clock.
- Stricter (free): more detectors, later fall-back, steps tripping on fewer
  events over longer windows with ≥ actions, more steps at the end.
