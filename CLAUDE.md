# Killer Chicken 9000 (KC9K)

Linux productivity blocker: CLI + systemd daemon + browser extension + GTK GUI.

## Documents

| File | Holds | Changes |
|---|---|---|
| `SPEC.md` | What KC9K should be: principles, glossary, rules, decisions | Only with the owner |
| `ROADMAP.md` | Status against `SPEC.md`; next work. Only status file | Same commit as the code |
| `CLAUDE.md` | Code layout, build, test | Agents |
| `android/CLAUDE.md` | Android app | Agents |

Read `SPEC.md` and `ROADMAP.md` first; use glossary words exactly. Code vs
`SPEC.md`: `SPEC.md` wins; log the gap in `ROADMAP.md`. If `SPEC.md` seems
wrong or silent, ask the owner. Fix before adding (`ROADMAP.md` §0). No
other planning/status files. No user-facing docs.

## Build and test

```bash
export PATH="/usr/local/go/bin:$PATH"
make build            # CGO_ENABLED=0 go build -o kc9k ./cmd/kc9k
make deb              # packaging/build-deb.sh: kc9k_<version>_<arch>.deb (TAGS=devmaster for tests)
sudo ./install.sh     # binary, wrapper, sudoers, systemd unit, DB, LibreWolf
```

- No CGo (`modernc.org/sqlite`). Check: `CGO_ENABLED=0 go build ./...`.
- Race check (tests only, needs gcc): `CGO_ENABLED=1 go test -race ./internal/...`.
- Run tests as a normal user, never `sudo`: as root `KC9K_DB` is ignored and
  `internal/cli` tests would hit the real DB (they refuse).
- Install: `/usr/local/bin/kc9k-bin` (binary), `kc9k` wrapper (`exec sudo`),
  `/etc/sudoers.d/kc9k`, `kc9k.service`, `/var/lib/kc9k/kc9k.db`; LibreWolf
  via `extrepo` (skip: `KC9K_NO_LIBREWOLF=1`; retry: `kc9k browser
  install-librewolf`). Independent of the source dir.
- `make uninstall` runs hidden `prepare-uninstall`: refused while locked;
  else removes all layers, SafeSearch/search engine policies, unsigned-
  extension permission, no-escape boot lock; leaves a policy telling
  browsers to remove the extension. LibreWolf stays.
- `TAGS=devmaster`: `kc9k master-set` / `kc9k master` (stdin) lift every
  lock, freeze, strict mode, no-escape. With root-owned
  `/var/lib/kc9k/allow-remote-control`, remote-controlled browsers aren't
  closed and the devtools policy is dropped (Marionette needs it).
- `./try-challenge.sh [len]` (`cmd/try-challenge/`): unlock challenge alone.
- Root Chrome test: `go test -tags roottest -run Chrome ./internal/enforce`.

### Test laptop (owner's, Mint 22.3, `TAGS=devmaster`, used for real)

- Tests add/remove blocks, restart the daemon (tamper) and trip blocks: all
  count on the owner's bypass ladder. `ci/extras_test.py` paces itself when
  a global ladder is on. Never leave test blocks with schedules (schedule
  locks hold them before windows).
- Browser tests: `GECKOWEBDRIVER=~/.local/share/kc9k-browsertest
  ~/.local/share/kc9k-browsertest/venv/bin/python ci/<name>.py`
  (`browser_test`, `extras_test`, `actions_test`, `quota_test`,
  `download_test`, `flash_test`: R-PAGE-1, records per animation frame
  whether a blocked word was painted). No Node
  there: `cjs ci/match_test_cjs.js`.
- GUI smoke tests: own binary, wrapper with `KC9K_DB` = scratch file, load
  `gui/kc9k-gui.py` as module with `CLI` replaced, other application ID,
  never `show()`. `KC9K_DB` never applies layers nor sets desktop hotkeys.
  As a normal user (as root `KC9K_DB` is ignored: they would use the real
  DB). `ci/gui_click_everything_smoke.py` clicks every button and switch
  of every page, tab and ⋯ menu, once with dialogs cancelled and once
  answered OK as a person could (a greyed-out button isn't pressed); fails
  on a handler exception or a command the CLI can't read. Examples: `ci/gui_phrase_smoke.py`, `ci/gui_phrase_edit_smoke.py`,
  `ci/gui_duration_smoke.py`, `ci/gui_hotkey_smoke.py`,
  `ci/gui_circadian_smoke.py` (`xvfb-run -a`).
- Weakening audit: `cli/weakening_audit_test.go` tries every command that
  could weaken a locked block; add a new weakening command there.
- Browser policy stops many pages before the extension sees the request; it
  then learns from the tab every 3 s. Request-dependent features need a
  fallback (`trail` in `background.js`).
- Judging is resumable (`kc9kListMatchesSteps` and the other `*Steps`
  generators in `match.js`; `kc9kDrain` runs them to the end): the content
  script runs it READ_SLICE_MS at a time (`judgeOn`/`judged`; principle 9).
  Frames judge their own text (`scanFrame`, R-LIST-4) and ask the
  background (`frame-hits` → `frameHit`) for the block page.
- No flashing (R-PAGE-1, `content.js`): while a list could still act on the
  top page by its title or text (`mayActByText`), it stays at opacity 0
  (`hidePage`) until judged and settled (`SETTLE_MS` after DOMContentLoaded
  or an address change seen in the MutationObserver), never over
  `HIDE_CAP_MS`. Don't hide with `visibility`: `innerText` skips it.
- `no-images` pictures drawn as CSS content: `content.js` marks elements
  (`data-kc9k-noimg` before/after/self, idle-time `scanPseudoImages`) and
  both sheets empty them; shadow roots get an adopted sheet. Page script
  harness (Node + Playwright, cloud only): `node ci/content_harness_test.js`.
- State the Chrome service worker must keep across restarts goes in
  `storage.session` (e.g. `kc9kDownloadPages`).
- LibreWolf's policy file is rewritten only by the daemon (Firefox's at once
  by the CLI): wait ~12 s before starting a browser after a change.

## Architecture

- `cmd/kc9k/main.go` → `cli.Execute()`. Cobra commands in `internal/cli/`
  (`block.go`, `control.go`: start/stop/unlock/status; `toggle.go`). `kc9k --help` lists them.
- **Layers** (`internal/blocker/`):
  - `/etc/hosts` (atomic; `validate.go` blocks injection).
  - Firewall: nft table `inet kc9k` (address sets, output+forward chains,
    `nft -f`); fallback iptables/ip6tables chain jumped first in OUTPUT and
    FORWARD, re-topped on drift. Real IPs from a built-in DNS client
    bypassing hosts: network server, then Quad9, or `kc9k dns` choice
    (`SetDNSChoice`); loopback answers dropped. DoH resolvers blocked.
    `resolveAll`: 16 parallel, 30 min cache.
  - Browser policies (Firefox, Chrome, Chromium, Brave, Edge, Opera,
    Vivaldi): blocklist, allowlist, wildcards, DoH off.
    `file://` entries (`BlockFiles`, `FilePatterns`) only with tamper
    `file-urls` on (`enforce.SettingFileURLs`), minus the block's file exceptions.
  - Process kill: exact name, Flatpak ID (`app-flatpak-<ID>-<n>.scope` or
    `/.flatpak-info`), desktop units `app-[<launcher>-]<ID>-<n>.scope`/`@….service`
    with ID equal to or ending in `.<name>`; descendants too, except under
    shells/session processes. `scripts.go`: Python by script
    (`pythonScript`, `essentialProcess`), copies by hash (`HashEntry`
    `sha256:<hex>:<size>:<path>`, hidden by `visibleApps`; system programs
    never match), `KillNaming` (fetch tools given a blocked domain).
  - `extguard.go`: extension mandatory while `enforce.ExtensionRequired` and
    a supported browser is installed; closes browsers whose extension hasn't
    fetched `/rules` with `X-KC9K-Extension` within 60 s (traced via
    `/proc/net/tcp`), at once if remote-controlled (`remotecontrol.go`) or
    started before the last protecting settings change
    (`noteSettingsChange`). Browsers known by name or engine
    (`browserdetect.go`: libxul.so or icudtl.dat/V8 snapshot mapped by a
    user-owned exe, minus Electron). Decision in `closeReason`. With block
    unsupported browsers on, runs always and closes at once every browser
    whose exe isn't a root-owned package file of `SupportedBrowsers`
    (`supported.go`: LibreWolf, Chrome, Chromium; `InstallBrowser`, fakes
    `statFile`/`runInstall`). LibreWolf: `block-devtools` policy;
    five prefs locked in librewolf.cfg, user overrides replaced by a cleaned
    copy (`librewolfprefs.go`).
  - `sockets.go`: strict mode closes docker/lxd/incus/libvirt sockets.
    Strict lockdown: `/etc/sudoers.d/zz-kc9k-lockdown`,
    `/etc/polkit-1/rules.d/00-kc9k-lockdown.rules`; no exec from `/home`,
    `/tmp`, `/var/tmp`, `/dev/shm`. Boot param `kc9k.nofreeze` skips freeze
    and lockdown for that boot.
  - `titles.go`/`x11.go`: asks matching windows to close (also file manager
    windows showing a folder entry: `SetFolders`, `folderWindow`), kills after
    `TitleCloseGrace`; cgo-free X11 client (user's own Xauthority only,
    cookie only to root/user-owned socket, capped replies, 3 s budget, PID
    trusted only for the session user). Runs beside the cycle
    (`enforceTitles`). Browsers left to the extension.
  - `polite.go`: `polite-close` asks windows to close, kills a cycle later.
  - `hotkeys.go`: Cinnamon/GNOME dconf keybindings `kc9k<id>` →
    `kc9k hotkey run <id>`: starts in-process with the hotkey's lock flags
    (`hotkeys.lock_args`, start's flags in unlock order, `cli/lockflags.go`:
    `addStartLockFlags`, `parseLockFlags`, `parseLockArgs`; old rows only
    `lock_for`, `Hotkey.Args`); a password lock's hash is taken at `hotkey
    add` (`presetPasswordHash` at the press; never in gui-state, backups
    `hotkey_password_hashes`); settings password asked at `hotkey add`.
- **Enforce** (`internal/enforce/`): active blocks → layer inputs (unioned).
  `Apply` (CLI, on change) and `Tick` (daemon: due queued edits, schedule
  reconciliation, drift reapply). Single source of truth.
- **Daemon** (`internal/daemon/`): `Tick` every 5 s; foreground-app
  sampling every 5 s; screen relock every 0.5 s during a lock freeze.
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
  whole; monk: `monkComparable`, refused while `MonkAutomationOn`,
  confirmation when time is added). `kc9k mode create|rename|delete|add|
  take-out|on|off|show` (group switch runs `startCmd`/`stopCmd` per block).
  Daemon: `runMonkAutomation` before `enforceMonk` keeps `monk.until` ahead
  (window end; `monkPlaceStep` while places decide); `monkKeepsWifi` leaves
  the radio on. Tests `TestSavedSchedules`, `TestModeBlocksShareAutomation`,
  `TestMonkModeAutomation`, `TestMonkAutomation*`, `db/modes_test.go`.
- **Locks** (`internal/cli/`): `blockHold` decides lock state (text, timer,
  password, restart, delay, schedule lock `db/schedulelock.go`). Layered
  (R-LOCK-6/7, `db/layers.go`, `cli/layers.go`): `LockExtras.Order` from
  the lock flags' order (`trackLockFlagOrder`), `LayersOn`, `unlock` lifts
  only the first; `TimerAfter`/`RestartPending` wait behind others;
  `AdvanceLayers` in `blockHold` and the daemon (`finishDelayedStops`).
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
- **Website lists** (`db/weblists.go`, `cli/weblists.go`, `kc9k list`): per
  block, blacklists and exception lists with scope (url/title/text), no
  types (§3.2): an entry is a site/URL entry if `blocker.IsAddressEntry`
  (domain, path, or pattern with `*`/`/`, e.g. `*hot*girl*`), else a
  rendered term; validity doesn't depend on scope (`prepareEntries`).
  With url: `WebList.Addresses` → `url` (and layers), `URLTerms` →
  `url_terms`, matched as words (`kc9kUrlWords`); title/text get every
  entry. Only blacklists with url and `block` feed their addresses to
  hosts/firewall/policy (`enforce.Inputs`), and not for blocks with
  title/text/url-term exceptions. Old `web_lists.type`/`clusters.type`
  columns may remain; nothing reads them. `migrateJoinLists`
  (`db/listjoin.go`, every open) joins split pairs `<name>` (url) +
  `<name> terms` when identical (entries, actions, no test mode, no
  queued edit/recording); clusters too. `--terms` is a no-op alias; old
  `type` fields in block/project files are ignored. GUI create dialog:
  name plus scope boxes (`list_dialog`, `ci/gui_lists_smoke.py`).
  Everything goes to the extension as `lists` rules
  (`enforce.BuildRules`; `kc9kListActions` in `match.js`: plain words
  whole, `*` inside words; a list waits if its exceptions need unread page
  parts). Per action: threshold (distinct entries), hit count and rate
  (`list rate`, percent of the page's words, pages ≥ 200 words).
  Strengthening free; else gated (`EditList*` by id). Shortcuts (`block add`,
  `add-exception`, `add-term` (alias `add-keyword`), `threshold`, `action`)
  use default lists Blocked / Exceptions / Rendered terms on page
  (`migrateWebLists`; named Page words until `migrateWordsListName`).
  User-facing words follow SPEC §3.14 (`rendered term`, `window title
  entry`, `text lock`, `allowance`, `weakening`). Old combinations
  `(a, b) (c):2`: `migrateCombinations` (`db/combinations.go`, every open
  and after imports) turns blacklist/cluster ones (rendered terms only)
  into phrase sets; `refuseCombinations` refuses new typed ones; leftovers
  (one box, or in exception lists) use `kc9kCombination`.
  Case-insensitive lookup.
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
  expands industries; rules `ads`; background `noteAdRequest` →
  `kc9kAdMatches` → `pageHit`.
- **Phrase sets** (`db/phrases.go`, `cli/phrases.go`, `kc9k phrase-set`):
  `db.Phrase` JSON, typed `[a, b] [c]? :N` (`ParsePhrase`, `String`);
  `Check` refuses one-ended bridges (mirrored `kc9kPhraseCheck`). Rules
  with negative IDs (`phraseRules`), matched by `kc9kPhraseMatcher`.
  `HasPhrases` makes a block non-empty. Names unique case-insensitively.
  GUI: `PhraseSetCard` (a `ListCard` keyed by −id) with the in-card
  `PhraseEditor` (double-click loads a phrase); word sets bounded only by
  `db.MaxWordSets` (100). Files:
  `cli/phraseexport.go`; `db.AddPhraseSet`. Phrase clusters (`phrases`
  set): `cluster copy` → phrase set, `cluster save` from one; no recording.
- **Import files** (`cli/entryfile.go`, §4.3): `readEntries` sorts lines
  into phrases, sites (hosts files, adblock `||x^` read as sites), rendered
  terms, unusable; `list import`, `block import-list`, `phrase-set import`,
  `cluster import` take what fits and name the rest (`noteLeftOut`).
  Hidden `kc9k list read FILE` (JSON) lets the GUI's one Import route a
  file (`import_into_phrase_set`, `choose_phrase_set`;
  `ci/gui_import_smoke.py`).
- **Clusters** (`db/clusters.go`, `cli/clusters.go`): lists outside blocks,
  never enforced. `cluster copy` goes through `listAdd` (gating applies).
- **Presets** (§3.2 Preset, §4.4; owner 2026-11-17; `db/presets.go`,
  `cli/presets.go`, GUI page "Presets" = page `clusters`): block presets
  are templates, list and phrase set presets clusters; table `presets`
  (kind `lists` = `db.ListsPreset`, `automation` = `AutomationSpec`,
  `schedule` = windows, `hotkey`) for the rest. `kc9k preset save|use|
  rename|delete` (`--kind`, `--mode`/`--schedule`), use goes through the
  list and automation gates (`useListsPreset`, `changeAutomation`).
  gui-state `presets` (`guiPreset`, `describePreset`); GUI `save_preset`,
  `use_preset`, `add_from_preset`, `update_presets`.
- **Link recording** (`db/recording.go`, `enforce/recording.go`,
  `cli/record.go`): `rules.recording` → extension POSTs `/visit`; daemon
  buffers; `record stop` adds via `listAdd`/`clusterAdd`. Word recording
  (`--words`, `db/words.go`, `enforce/words.go`): `rules.words`
  record|baseline → content.js `kc9kPageWords` → `/words`;
  `RecordedWords` ranks by `Specificity`; `--cut N`; `record baseline`.
- **Monk mode** (`enforce/monk.go`, `db/monk.go`, `blocker/netcut.go`,
  `ApplyFirewallCut`): settings `monk.until` (trusted time), `monk.net`
  (`db.MonkNet`, saved once, `SaveMonkNetIfNone`). `monkBlock()` built in
  memory (`*.*`, `file://*`, `MonkListID`), added by `withMonk`, never
  stored. `enforceMonk` in `Tick` holds the three cuts (nmcli radio, `ip
  link set down` on real Ethernet from /sys/class/net, nft/iptables drop
  all but loopback), restores at end. Device vars (`wifiRadio`, `setLink`,
  `cutFirewall`) faked in `TestMain`; `KC9K_DB` cuts nothing
  (`monkDeviceless`). `migrateMonkMode` removed the old block.
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
- **Allowance** (`internal/quota/`, `enforce/quota.go`): `Spend` only on
  use; daemon sampler adds apps; extension reports focused/playing seconds
  from `rules.quota` (`/usage`). Without LibreWolf, website sessions spend
  wall time. `quota.Cycle` long break; `PomodoroWorking` keeps sessions
  closed in work periods.
- **Events and bypass prevention** (`db/events.go`, `db/nepsis.go`,
  `enforce/nepsis.go`, `cli/nepsis.go`): events kept a day; refused
  weakening logged via `refused()`, never counted. `db.Nepsis`: detectors,
  ladder, quiet time; global lockable as setting lock `nepsis`; per block
  gated; state in `nepsis.ladder[.<block>]`; old one-step settings read as
  ladders (`normalize`). `/hit` judges at once, answers `close_tab`.
  `search` = `page` from a search origin (`fromSearch`). Multi-block events
  (`RecordEventFor`) get a row per block, `extra` on all but the first.
- **Workarounds**: `kc9kUnwrap` reads addresses in query/path; `checkMirror`
  (content.js) reports canonical addresses/front-ends; `/hit` with `via`
  logs `EventWorkaround`.
- **Browser setting locks** (`db/settinglocks.go`, `cli/browser.go`): per
  option (time, challenge, password, restart); `enforce.AnyLock` counts
  them. The same code serves the tamper setting block unsupported browsers
  (`db.UnsupportedBrowsers`, stored `tamper.unsupported-browsers`,
  `enforce.IsLockableSetting`/`SettingKey`, `cli/tamper.go`), formerly the
  browser setting `force-librewolf` (`migrateForceLibreWolf` moves state
  and lock row; hidden alias in `kc9k browser`; old project files read).
  `GuardsBlocks`: it and `block-devtools`.
- **Chrome** (`webext/chrome.go`, `blocker/chromesetup.go`): `PackChrome`
  builds MV3 manifest + service worker (`chrome-worker.js`), signs CRX3 with
  a per-computer key (`webext.ChromeKeyPath`); daemon serves
  `/chrome/update.xml` and `.crx`; `kc9k-setup.json` force-installs, disables
  incognito/guest/dev mode. `compat.js` aliases `browser`; keepalive;
  `rules.extension_version` triggers updates. Chrome for Testing reads
  `/etc/opt/chrome_for_testing/policies`.
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
- **Behaviour settings** (`enforce/options.go`, `kc9k behaviour`, alias
  `option`): `polite-close`, `block-page-details`; never locked.
- **Circadian** (`internal/circadian/`, `enforce/circadian.go`,
  `cli/circadian.go`; GUI `CircadianPage`, `ClipEditor`): JSON settings
  `circadian` (`circadian.Config`: three clips of `Point{Minute, Value,
  Tension}`, wallpapers, dark mode, dark pages), `.paused`, `.preview`,
  `.applied`, `.resume`, `.status`. `RunCircadian` goroutine every 5 s,
  never in `Tick`; `ddcutil` and night light every 30 s. Foreign levels
  pause a clip (`PauseBacklight`, `PauseExternal`, `PauseGamma`; brightness
  and kelvin share one gamma ramp), except right after start. Wallpaper and
  dark mode applied once per key change (`WallpaperKey`, `DarkKey`), also
  into signed-out users' dconf (`blocker.Reach`, `dbus-run-session`); who
  got it is in `circadian.applied` (`WallpaperUsers`), a later sign-in
  without it gets it at once.
  `PreviewCircadian` drives hardware; engine adopts it. `circadian play`
  (`PlayCircadian`: 24 h as previews in `--seconds`, wallpaper/dark mode at
  their times, `ReadDesktopLook`/`RestoreDesktopLook` put the desktop back;
  GUI "Play the day" with a playhead). GUI wallpapers: `WallpaperTimeline`
  (drag bar/ends, 5-min steps, Shift minutes; covered parts faint). No
  Circadian section in Settings. Hardware vars
  `circadian*` (`fakeCircadian`); `KC9K_DB` → `circadianDry`. Wallpapers in
  `<db dir>/wallpapers` (0644). `rules.dark` (`CircadianDarkRule`) →
  `darkPage`; the popup's "Exempt from dark pages" → background
  `darkExempt` → daemon `/dark-exempt` (`darkExemptHandler`, refused
  behind an app/settings access password). `circadian.Shape` mirrored by
  GUI `clip_shape`.
  `ci/circadian_test.py` tests dark pages.
- **Pauses and breaks** (`db/pauses.go`, `enforce/soon.go`, `db/breaks.go`,
  `cli/breaks.go`): `block_pauses` until trusted time; `pausedFor` checked
  by `enabledBlocks`. `rules.soon` feeds toolbar badge/popup (`timeLeftFor`,
  `popup.js`); `/add` adds the tab's site to Blocked (refused with a blocks
  access password). Reward (`break reward`, alias `allowance`) earned in `earnBreaks`;
  `break text`/`relock` makes `unlock` pause instead (second unlock ends).
- **Drafts** (`cli/draft.go`, §4.3; owner 2026-11-17: an opened block is
  always a draft): `block draft B` (`makeDraft`) = `DuplicateBlock` into
  "B (draft)" (setting `db.SettingDraftOf`) plus its social media
  settings, trigger actions and lists' test mode, and keeps the block as it
  was (`draftContent` JSON, setting `draft.base.<draft>`); never runs
  (`db.ActivateBlock` refuses, autostart skips). `saveDraft` applies only
  what changed in the draft since (base → draft) onto the block as it is
  now (an entry added meanwhile stays) via `listAdd`/`listWhere`/
  `listAction`…, `saveDraftPhraseSets`, `saveDraftTriggers` (each through
  its gate) and names refusals; commands for anything else refuse a draft
  (`draftRefused`, `refuseDraftEdit` in the root PreRun). GUI: no Edit as
  draft; `Window.run` sends the commands of `DRAFT_EDITS` for the block
  opened on the Blocks page to its draft (`to_draft`, made at the first
  edit, `start_draft`), `DRAFT_READS` read it; `self.selected` stays the
  block; Websites and Apps show the draft (`show_block`: `content`), the
  bar says Unsaved edits with Save edits / Discard, leaving asks
  (`leave_block`, `ask_unsaved`), the card shows "Unsaved edits"; list
  cards keep their widget by name when the draft takes over.
- **Affected users** (`db/users.go`, `cli/blockusers.go`, §3.2):
  `blocks.users`; `enforce.forEveryone` keeps partial blocks out of shared
  layers, `blockUIDs` → `blocker.KillBlockedFor`, `BuildRulesFor(user)`
  with `clientUser` (socket uid, `blocker.ClientUser`); `regularUsers` /
  cli `accounts` faked in tests.
- **Block password** (`cli/blockpassword.go`, `blocks.view_password`, §3.11):
  `revealBlock` before showing contents; gui-state `--reveal B` (password on
  stdin) else `hidden` with entries emptied; GUI `block_passwords`.
- **Autostart** (`db/autostart.go`, `enforce/autostart.go`, `cli/autostart.go`, §3.6):
  once/at/sign-in/next-window; daemon `runAutostarts` in `Tick` (first look
  only records; sign-in = new loginctl session or wake by BOOTTIME−MONOTONIC).
- **Project** (`cli/project.go`): versioned JSON; import validates all, then
  only adds (existing untouched, options only strengthened). Schedules and
  modes: `cli/projectmodes.go` (`copy_of` on a block's window, `in_mode`,
  `applied_schedules`).
- **Social** (`db/social.go`, `cli/social.go`): `block_social`,
  `BlockDetail.Social`; `rules.social` → `webext/files/social.js`
  (`KC9K_SOCIAL`). Playlist = preset `youtube:playlist=<id>` plus
  exceptions `youtube.com/playlist?list=<id>`, `youtube.com/watch`; GUI:
  `PlaylistBox` in the Exceptions card (`ci/gui_playlist_smoke.py`);
  `checkPlaylistVideo`.
- **Places** (`db/places.go`, `enforce/places.go`, `cli/places.go`): BSSIDs
  from `nmcli` (`ScanWifi`); `AtPlace` needs `Place.Needed()` in range
  (`places.need`, default 2, `place need P N`, `place add --need`);
  `placesHere` (none seen: unknown, see Automation); `keepWifiOn`.
- **Test mode** (`db/testmode.go`): lists go out `test: true`; extension
  shows `kc9kTestReport`, skips them in `kc9kListMatches`.
- **Statistics**: foreground app time (daemon) and site time (extension);
  not counted when locked or after 2 min idle; kept a year. `stats --json`
  rows are filtered by `--search`/`--block` (block password, §3.11); time
  frame `--days N` or `--from D [--to D]` (GUI Custom…, `DateButton`,
  `stats_range_problem`). GUI:
  `DailyChart.set_stacks` (top names stacked bottom up, the rest Other),
  `Legend`, `assign_series` (a name keeps its palette slot, never two shown
  in one), `SERIES_LIGHT`/`SERIES_DARK`; the dashboard reads `stats_rows`
  (unfiltered), the page `stats_page_rows`. `ci/gui_stats_smoke.py`.
- **GUI** (`gui/kc9k-gui.py`, GTK 3): reads `kc9k gui-state`,
  `kc9k stats --json`; every change runs a `kc9k` command. Blocks page
  (§4.2): `blocks_stack` "cards" (`update_block_cards`, `block_card`:
  name, switch, Continuous/Automated, `breaks_words`, `lock_words`) and
  "detail" (the block, `open_block(name, tab)`) and "mode" (`open_mode`,
  `update_mode_view`); modes above the blocks (`mode_card`, drop target
  `BLOCK_TARGET`; block cards' title is the drag source, ⋯ "Move to the
  mode"); `AutomationEditor` (kind mode/monk/schedule) runs the same
  commands with the mode's or schedule's name; sidebar pages Schedules
  (`build_schedules_page`, `open_schedule`) and Places
  (`build_places_page`); a block in a mode greys `block_auto_widgets`.
  `ci/gui_cards_smoke.py`, `ci/gui_modes_smoke.py`. Design layer
  (R-GUI-3, `APP_CSS`): `page_header`, `section` (a card; `help=` puts
  long text behind a ? popover), `card`, `pill`/`block_pills`,
  `flow_row` (a row into a `WrapBox`: laid out like words, natural widths,
  a new line only when the next doesn't fit; GTK 3's FlowBox put one per
  line). `WrapBox` is a `Gtk.Fixed` overriding only size vfuncs: never
  write a container in Python (`do_forall`): GTK calls it on a fresh
  wrapper when a destroyed one is finalized and spins. Inputs (R-GUI-1/2):
  `DurationField` (typed `2d 4h`, says why when wrong, never clamps),
  `NumberField`, `TimerLockField` (none / for / until a calendar date),
  `LockPicker` (every lock kind; `lock_dialog` reads it before closing).
  CLI side: `cli/duration.go` (`parseUserDuration` with d/w,
  `humanDuration`, `untilToday` with dates).
- **DB** (`internal/db/`): SQLite `/var/lib/kc9k/kc9k.db`, WAL, busy_timeout
  and foreign_keys per connection via `_pragma`. `KC9K_DB` overrides (tests
  only). `db.go`, `features.go` (schedules, time locks, upgrades),
  `weblists.go`, `pending.go`.
- **Android**: `android/CLAUDE.md`. Its `RulesServer` serves `/rules`;
  `rules.safesearch` comes only from there (`kc9kSafeSearch` in `match.js`).

## Built behaviour (not fixed by `SPEC.md`; the code is the truth)

- **Blocks**: new block = blacklist `Blocked` + exception list `Exceptions`
  (`file://*`; spares no website); `Rendered terms on page` made on first
  `add-term`.
  `--allowlist` = `*.*` in Blocked. `block action B A on|off` sets all
  blacklists. A blacklist keeps ≥ 1 scope; lists and phrase sets may have no action (they then do nothing). Pre-list blocks
  (sites, patterns, exceptions, keywords, allowlist) were migrated into the
  three lists; `block import` reads pre-list exports. `add-pattern`,
  `remove-site`, `remove-entry` are Blocked shortcuts. List/apps/titles
  files: one entry per line, `#` comments (`block export-list|import-list
  B apps|titles|blocked|exceptions`). Remove block: inactive and unlocked.
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
  blocks, switched on with `--yes`; running by its automation, off and any
  weakening refused; announced 5 and 1 min before its windows. GUI: modes
  above the block cards, the mode's view, Schedules and Places pages,
  Overview "Mode: X" in Start and "Monk mode today …".
- **Starting on an active block** only strengthens: any lock kind can be
  added, last in the unlock order (a duration timer or restart lock waits
  behind locks on); longer `--lock-chars`, later `--for/--until`, longer
  `--delay`; shorter/earlier, another password, another range or text-lock
  kind refused. A challenge longer than the
  terminal is paged (Enter per page), accuracy over all.
- **Strengthening allowed while locked**: add apps/titles/blacklist entries,
  `list create`, blacklist scope/action on, lower threshold/hits, remove
  exceptions or exception scopes, delete an exception list, copy a cluster
  into a blacklist, record into a blacklist. Everything else weakens.
- **Matching**: plain term = whole word (`porn` ∉ "pornhub", `18+` ∉
  "1118+"); `*` inside words; case ignored. Address/title match fires every
  action; text needs threshold (distinct) and hits (total), both default 1;
  once met the page stays acted on until left. Typed phrase `[a, b] [c]? :N`
  (`?` optional, third set on; `:N` length). Exceptions by title/text leave
  that block's sites to the extension only (page flashes before blocking);
  address exceptions remove their domain from hosts/firewall.
- **Actions**: strongest page action close-tab > image > redirect > block;
  redirect to a blocked page shows the block page. Only the top page triggers
  close-tab/device actions; embedded content gets the block page. Image:
  PNG/JPEG/GIF/WebP. Action edits: longer time or another picture free; new
  redirect address weakens. List logout/shutdown/lock-screen notified 60 s
  ahead; one-off logout/shutdown ≤ every 10 min; `--for` goes through
  Frozen Chicken (≤ a week).
- **Clusters**: copies are independent. `--exceptions`, `--as N`,
  `--into L`. Export = plain entry file with a scope header (`list import`
  reads it).
- **Recording**: domains without `www.`, pages and frames, private windows
  and blocked pages included; ≤ 2000 sites; one recording at a time; a
  refused `stop` keeps the recording.
- **Places**: re-adding a name adds the BSSIDs now seen; needs at most what
  the place knows; more needed weakens its blocks, fewer is free. A rule
  can go on any block (it switches nothing on); adding or removing one is
  gated like any automation change. With a schedule lock, a block is
  locked while its automation has it enabled; then nothing that lifts it
  can be removed. GUI: "Add this place", "Save where I am now as…"
  (ticked: add it to the block), networks needed per place (`ci/gui_places_smoke.py`, fake
  `nmcli`). No networks seen: what keeps each block enabled; Wi-Fi is
  switched back on.
- **Allowance** (`kc9k allowance set|start|end|remove`; `--per
  day|week|month|rolling|window`, `--refill HH:MM`, `--sessions N`; flags
  not given keep what it has): use = block's app focused, or its site
  (address/title lists, not text) in the focused tab with input < 3 min, or
  playing (even muted) in any tab. Session with websites closes
  extension-less browsers. Protects nothing by itself (R-LOCK-1, `held`):
  unlocked, every change at once; locked, only less time or fewer sessions;
  removing always free. `set` switches the block on; nothing else switches
  it. A lock, window or Pomodoro work period closes a session. Day follows
  the system clock.
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
- **Extension guard**: the GUI's "isn't running" dialog (`extensionMissing`)
  only with no supported browser, or one running past the grace without a
  check-in (`browsersWithoutExtension`, `runningSupportedBrowsers`).
  Required while an active block has a non-empty
  blacklist, web-only/force-search is on, or anything is locked (R-LOCK-9);
  unlocked app-only blocks leave browsers alone. With block unsupported browsers on, always: other browsers
  (Firefox too) close at once, supported ones after 60 s without a check-in.
  Nothing closed without LibreWolf, Chrome or Chromium installed (`status`
  says so). `kc9k browsers` lists them with per-browser check-in
  (`extension.last_checkin.<id>`); `browsers install librewolf|chromium|chrome`
  (extrepo; Mint's chromium .deb; Google's repo with key in
  `/etc/apt/keyrings/google-chrome.asc`, amd64 only). Firefox needs signed extensions; Brave/Edge/Opera/Vivaldi not set
  up. Installed engine-based apps (Thunderbird, Spotify, Steam, Electron)
  aren't browsers. Chrome flags `--disable-extensions[-except]`,
  `--remote-debugging-pipe` and Firefox `--marionette`/remote-debugging
  close at once. After a protecting change, older browser windows close a
  minute later. LibreWolf locked prefs: `extensions.webextensions.restrictedDomains`,
  two `extensions.quarantinedDomains*`, `devtools.chrome.enabled`,
  `devtools.debugger.remote-enabled`; overrides copy at
  `/var/lib/kc9k/librewolf/<user>.cfg` keeps only `pref`/`defaultPref`/
  `lockPref` lines. Translate proxy (`*.translate.goog`) and archives
  (`web.archive.org`, `archive.ph`) blocked as the site. `web-only` uses
  Google `udm=14`.
- **Chrome**: policy `/etc/opt/chrome/policies/managed/kc9k-setup.json`
  (Chromium `/etc/chromium/…`); key `/var/lib/kc9k/chrome-extension.pem`
  (root only, ID per computer); pinned; devtools never on the extension;
  `force-search` sets Google web results as default engine. A restarted
  service worker may let the first page through the extension (URL policy
  still blocks plain sites) until the page reports in.
- **Pomodoro** (`pomodoro set B W R [--long D --every N]`, `off` free;
  refused while locked): locks stay; work periods close allowance sessions.
- **Breaks**: reward earned while block on and computer in use (input < 3
  min, screen unlocked); `--keep one` (up to one reward), `window|day|week|
  month` (accumulates until it ends) or a length ≥ 1 h, no upper limit
  (`Breaks.Keep` = seconds; each hour's part in `Chunks` expires that long
  after its hour; a break takes the oldest, `break end` gives back the
  newest it took, `OnBreak`). No cap on the amount. Period ↔ length counts
  as weakening (`KeepLooser`). A break lifts a locked
  block; it returns with locks; edits stay refused. More breaks weaken
  (refused while locked); fewer always free.
- **Window titles**: case-insensitive substring; X11 only (Wayland: XWayland
  apps only); asked to close, killed with descendants a few seconds later.
- **Apps**: hash match breaks on rebuild; `python3` itself and desktop
  essentials (`Xorg`, `cinnamon`, `pipewire`…) can't be added; system
  folders (`/usr/bin/`) refused; only regular-user processes are killed.
  Fetch tools killed for blocked addresses: `yt-dlp`, `mpv`, `vlc`, `curl`,
  `wget`, `streamlink`…; never editors/shells.
- **Monk mode**: `kc9k monk --for D` (no `--for` shows state); longer time
  extends; reapplies cuts within seconds; Wi-Fi that was off stays off.
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
- **Hotkeys**: `ctrl|alt|shift|super+key`, F1–F24 alone; shown as
  "KC9K: …" in keyboard settings; notification on press; any lock `start`
  takes, no upper limit; on an active block a press only adds or extends
  locks, as `start`. Older hotkeys ask at each press. Hotkey actions (owner
  2026-11-17; `db.HotkeyAction`, column `hotkeys.actions`, `Hotkey.Steps`;
  `cli/hotkeys.go`: `addHotkey`, `parseHotkeyAction` for `--do`,
  `runHotkeyAction`, `describeHotkey`): start/stop, mode-on/off,
  mode-run, monk, freeze, close-browser, lock-screen, logout, shutdown,
  custom, in order; one desktop confirmation per press for all its locks.
  GUI: Hotkeys page (`build_hotkeys_page`, `update_hotkeys_page`; area
  settings): Add hotkey… (`add_hotkey_actions`, `ask_hotkey_action`),
  Start a block… (`add_hotkey`: `HotkeyFields`, the `LockPicker`), Save as
  preset….
- **Notifications** (`notify-send`, `enforce/notices.go`, `kc9k notifications`):
  user picks kinds (window start/end, timer lock end, Pomodoro, allowance,
  app closed) and advance minutes (default 5; each advance once); windows
  only of blocks that are on; scheduled freeze always (5 and 1 min).
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
- **Social presets**: YouTube (no Shorts, no home feed → Subscriptions, no
  recommendations, no comments); Instagram/Facebook/X (messages only, no
  Reels/Explore); Reddit (no Popular/All, no home feed); TikTok (Following
  only); LinkedIn (no feed, no news). Disabled parts redirect; leftovers
  hidden. Playlist: videos need `list=` and must appear in the shown
  playlist panel.
- **Toolbar button**: time left (allowance, break, schedule within 2 h); red
  in the last 5 min; `!` when the daemon is unreachable; popup lists covering
  blocks and adds the site to Blocked.
- **Test mode**: `list test B [L] on|off`; corner panel shows found entries,
  counts, distinct count and per-action threshold state; address-only lists
  keep working.
- **Workarounds**: proxied addresses in `?url=`, base64, or after a slash;
  searches for an address don't count. Front-ends: Invidious, Piped, Nitter,
  Redlib, ProxiTok….
- **Options**: `polite-close` (on): windowed apps asked, killed ~5 s later;
  windowless or restarted within a minute killed at once.
  `block-page-details` (on): block page names block, list, end time.
- **Circadian**: `kc9k circadian` (`on|off`, `preset [N]`, `lane L on
  --points "07:00=6500 19:00=6500~-0.4"`, `point add L HH:MM V --tension
  T`, `resume [L]`, `wallpaper add F A B | time N A B | remove N`, `dark
  system|light|dark|light-between A B`, `dark-pages`, `exempt`,
  `nightlight off`). Ranges: software 10–100 %, kelvin 1000–10000 (6500
  neutral), tension −1..1 (> 0 bulges up). Wallpaper PNG/JPEG/WebP ≤ 32 MB;
  spans midnight; overlap → latest start wins. Desktop night light conflicts.
- **Project**: holds blocks (lists, apps, titles, schedules, schedule lock,
  social, allowance, Pomodoro, breaks, ladders), clusters, browser and
  behaviour settings, global ladder, freeze schedules, list images; backup
  adds places and place rules, and (`cli/projectbackup.go`) running locks,
  password hashes, hotkeys, statistics: import adds only where nothing is
  here, confirms the locks, starts blocks that come with running locks. Clashing block → `name-2`; existing
  clusters/places kept; browser settings only on; ladder replaced only by a
  stricter one. `--dry-run` lists additions and locks.
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
  password.
- **Statistics**: app time needs X11; extension posts site time every 30 s.
- **Layers**: hosts/policy authoritative; IP rules best effort (CDNs).
  Containers/VMs: firewall only. Snap/Flatpak browsers may ignore policy.
- **GUI**: menu entry / `kc9k-gui` (`python3-gi`); theme in
  `~/.config/kc9k/gui.json`. Schedule grid: drag adds, click removes,
  filled = block window, outlined = allow window, one-click presets.
  ClipEditor: right-click add/remove point, drag (Shift = minute), drag
  curve to bend, double-click to type.
