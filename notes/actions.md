# Actions, trigger actions, consequences, bypass prevention

- Covers: `db/actions.go`, `adnetworks.go`, `triggers.go`, `custom.go`, `events.go`, `nepsis.go`, `declutter.go`, `enforce/hits.go`, `trigger.go`, `consequence.go`, `moderun.go`, `nepsis.go`, `blocker/custom.go`, `confirm.go`, `cli/triggers.go`, `nepsis.go`
- SPEC: §3.9, §3.10, R-CUSTOM-1–2, R-WARN-1–2, R-EVENT-1, R-MODE-4
- Connects to: [extension](extension.md), [device](device.md), [automation](automation.md) (mode runs), [lists](lists.md)

## Code

- **Actions** (`db/actions.go`, Param/Minutes): block, greyscale,
  no-images, mute (`applyMute`), no-video, dark, no-downloads (background
  `downloadBlocked`, `downloads` permission; `ci/download_test.py`), redirect, image
  (`db.ImagesDir`, served `/image/`), close-tab; device: close-browser, logout, shutdown, monk. Extension `kc9kOutcome`
  picks the strongest, closes tab/browser itself, POSTs `/hit`
  (`enforce/hits.go`, header required) so the daemon logs and runs
  logout/shutdown/monk. `no-video`/`no-images` reach every frame:
  background `tellFrames` pushes `tab-looks`, new frames ask for it; mute
  is tab-wide (`applyMute`). Removed actions (declutter):
  `migrateRemovedActions` drops them on open (emptied → block), imports
  skip them (`db.IsRemovedAction`).
- **Ad trigger** (§3.9, `db/adnetworks.go`): scope `ads` (`Where.Ads`,
  `in_ads`; not exception lists or phrase sets); `WebList.AdNetworks`
  expands industries (`AdIndustries`: adult, gambling, crypto, dating;
  `TestAdIndustries`: plain domains, one industry each); rules `ads`; background `noteAdRequest` →
  `kc9kAdMatches` → `pageHit`.
- **Trigger actions** (`db/triggers.go`, `enforce/trigger.go`,
  `cli/triggers.go`; owner 2026-11-16): table `trigger_actions` (block,
  group `apps`|`titles`, JSON actions); no row = default (`close`). `kc9k
  block trigger B apps|titles ACTION on|off [--for D]` (on/longer free,
  off/shorter weakens; monk needs `--no-confirm-ok`). Daemon:
  `appsTrigger` (close or `RunningApps`), titles via
  `TitleGuard.EnforceWatch` (watch without closing); `fireTrigger` ≤ once
  a minute per block+group → `runAnnounced`. GUI `TriggerEditor`,
  gui-state `triggers`, projects `triggers`. A place rule has **no
  actions** (§3.6, owner 2026-11-16: they existed for a day;
  `migratePlaceTriggers` drops stored `place:` sets, project files' are
  ignored). `ci/gui_trigger_smoke.py`.
- **Consequences** (`enforce/consequence.go`): close browsers, lock
  screen/logout/shutdown (via Frozen Chicken, only extended; ≤ once per 10
  min), monk, mode (`StartModeRun`, `enforce/moderun.go`: `db.ModeRun` in
  setting `mode.runs`; `modeRunning` in `enabledBlocks`, `ModeRunUntil` in
  `blockHold`, `endModeRuns` in `Tick`), custom (`runCustom` →
  `blocker.RunCustom`, `customDue` once a minute; a mode's own:
  `runModeCustoms`, column `modes.custom`). `Consequence.Param`,
  `TriggerAction.Param`, `NepsisAction.Param` carry the mode or the custom
  items (`db.CustomItem`, `CustomParam`); CLI `--target`/`--custom`
  (`addActionParamFlags`), ladder `--step-json`. Device effects via
  replaceable vars.
- **Events and bypass prevention** (`db/events.go`, `db/nepsis.go`,
  `enforce/nepsis.go`, `cli/nepsis.go`): events kept a day; refused
  weakening logged via `refused()`, never counted. `db.Nepsis`: detectors,
  ladder, quiet time; global lockable as setting lock `nepsis`; per block
  gated; state in `nepsis.ladder[.<block>]`; old one-step settings read as
  ladders (`normalize`). `/hit` judges at once, answers `close_tab`.
  `search` = `page` from a search origin (`fromSearch`). Multi-block events
  (`RecordEventFor`) get a row per block, `extra` on all but the first.

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **Actions**: strongest page action close-tab > image > redirect > block;
  redirect to a blocked page shows the block page. Only the top page triggers
  close-tab/device actions; embedded content gets the block page. Image:
  PNG/JPEG/GIF/WebP. Action edits: longer time or another picture free; new
  redirect address weakens. List logout/shutdown/lock-screen notified 60 s
  ahead; one-off logout/shutdown ≤ every 10 min; `--for` goes through
  Frozen Chicken (≤ a week).
- **Bypass prevention**: `bypass set --step "3 in 10m: warning, close-tab,
  logout 1m" … --quiet 2h [--block B] [--detect …] [lock flags]`, short
  form `--count --within --then [--then-for]`. Actions: `warning`,
  `close-tab`, `close-browser`, `lock-screen D`, `logout [D]`,
  `shutdown [D]`, `monk D`. Past the last step repeats the last. `page`
  excludes look-only actions; `search` = page opened from search results
  (Google, Bing, DDG, Brave, Startpage…, any `/search?q=`); `tamper` =
  daemon restart within a boot (not upgrade or self-reported crash) or clock
  moved > 2 min against the boot clock. Global counts all; per block counts
  while on; both apply. Stricter = more detectors, later fall-back, steps
  tripping on fewer events over longer windows with ≥ actions, more steps
  at the end; free. Global lockable (`--for`).
