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
  `download_test`). No Node
  there: `cjs ci/match_test_cjs.js`.
- GUI smoke tests: own binary, wrapper with `KC9K_DB` = scratch file, load
  `gui/kc9k-gui.py` as module with `CLI` replaced, other application ID,
  never `show()`. `KC9K_DB` never applies layers. Examples:
  `ci/gui_phrase_smoke.py`, `ci/gui_circadian_smoke.py` (`xvfb-run -a`).
- Browser policy stops many pages before the extension sees the request; it
  then learns from the tab every 3 s. Request-dependent features need a
  fallback (`trail` in `background.js`).
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
    `kc9k hotkey run <id>` (starts in-process with preset `--for`; settings password asked at `hotkey add`).
- **Enforce** (`internal/enforce/`): active blocks → layer inputs (unioned).
  `Apply` (CLI, on change) and `Tick` (daemon: due queued edits, schedule
  reconciliation, drift reapply). Single source of truth.
- **Daemon** (`internal/daemon/`): `Tick` every 5 s; foreground-app
  sampling every 5 s; screen relock every 0.5 s during a lock freeze.
- **Schedule** (`internal/schedule/`): windows and `--allow` windows. Wall
  clock: DST-skipped windows shrink/vanish, repeated hour covered twice;
  durations count real hours. Active vs enabled (§3.3): `enforce.enabledBlocks`
  drops active blocks lifted by breaks/pauses and by `scheduleLifted`
  (windows, place rules) and open allowance sessions; the daemon never
  switches blocks off (`reconcileQuotas` keeps allowance blocks on). Freeze schedules:
  `db.FreezeSchedule.LockLead`; daemon `reconcileFreezeSchedules`.
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
  parts). Per action: threshold (distinct entries) and hit count.
  Strengthening free; else gated (`EditList*` by id). Shortcuts (`block add`,
  `add-exception`, `add-keyword`, `threshold`, `action`) use default lists
  Blocked / Exceptions / Page words (`migrateWebLists`). Old combinations
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
- **Clusters** (`db/clusters.go`, `cli/clusters.go`): lists outside blocks,
  never enforced. `cluster copy` goes through `listAdd` (gating applies).
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
- **Consequences** (`enforce/consequence.go`): close browsers, lock
  screen/logout/shutdown (via Frozen Chicken, only extended; ≤ once per 10
  min), monk. Device effects via replaceable vars.
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
  line via `stdinLines` (settings password first). GUI retries once after
  asking.
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
  dark mode applied once per key change (`WallpaperKey`, `DarkKey`).
  `PreviewCircadian` drives hardware; engine adopts it. Hardware vars
  `circadian*` (`fakeCircadian`); `KC9K_DB` → `circadianDry`. Wallpapers in
  `<db dir>/wallpapers` (0644). `rules.dark` (`CircadianDarkRule`) →
  `darkPage`. `circadian.Shape` mirrored by GUI `clip_shape`.
  `ci/circadian_test.py` tests dark pages.
- **Pauses and breaks** (`db/pauses.go`, `enforce/soon.go`, `db/breaks.go`,
  `cli/breaks.go`): `block_pauses` until trusted time; `pausedFor` checked
  by `withoutBreaks`. `rules.soon` feeds toolbar badge/popup (`timeLeftFor`,
  `popup.js`); `/add` adds the tab's site to Blocked (refused with a blocks
  access password). Reward (`break allowance`) earned in `earnBreaks`;
  `break text`/`relock` makes `unlock` pause instead (second unlock ends).
- **Block password** (`cli/blockpassword.go`, `blocks.view_password`, §3.11):
  `revealBlock` before showing contents; gui-state `--reveal B` (password on
  stdin) else `hidden` with entries emptied; GUI `block_passwords`.
- **Autostart** (`db/autostart.go`, `enforce/autostart.go`, `cli/autostart.go`, §3.6):
  once/at/sign-in/next-window; daemon `runAutostarts` in `Tick` (first look
  only records; sign-in = new loginctl session or wake by BOOTTIME−MONOTONIC).
- **Project** (`cli/project.go`): versioned JSON; import validates all, then
  only adds (existing untouched, options only strengthened).
- **Social** (`db/social.go`, `cli/social.go`): `block_social`,
  `BlockDetail.Social`; `rules.social` → `webext/files/social.js`
  (`KC9K_SOCIAL`). Playlist = preset `youtube:playlist=<id>` plus
  exceptions `youtube.com/playlist?list=<id>`, `youtube.com/watch`;
  `checkPlaylistVideo`.
- **Places** (`db/places.go`, `enforce/places.go`, `cli/places.go`): BSSIDs
  from `nmcli` (`ScanWifi`); `AtPlace` needs two in range; empty scan =
  there. `PlaceBlocks` feeds `reconcileSchedulesAt`, `ScheduleActiveNow`;
  `keepWifiOn`.
- **Test mode** (`db/testmode.go`): lists go out `test: true`; extension
  shows `kc9kTestReport`, skips them in `kc9kListMatches`.
- **Statistics**: foreground app time (daemon) and site time (extension);
  not counted when locked or after 2 min idle; kept a year.
- **GUI** (`gui/kc9k-gui.py`, GTK 3): reads `kc9k gui-state`,
  `kc9k stats --json`; every change runs a `kc9k` command.
- **DB** (`internal/db/`): SQLite `/var/lib/kc9k/kc9k.db`, WAL, busy_timeout
  and foreign_keys per connection via `_pragma`. `KC9K_DB` overrides (tests
  only). `db.go`, `features.go` (schedules, time locks, upgrades),
  `weblists.go`, `pending.go`.
- **Android**: `android/CLAUDE.md`. Its `RulesServer` serves `/rules`;
  `rules.safesearch` comes only from there (`kc9kSafeSearch` in `match.js`).

## Built behaviour (not fixed by `SPEC.md`; the code is the truth)

- **Blocks**: new block = blacklist `Blocked` + exception list `Exceptions`
  (`file://*`; spares no website); `Page words` made on first `add-keyword`.
  `--allowlist` = `*.*` in Blocked. `block action B A on|off` sets all
  blacklists. A blacklist keeps ≥ 1 scope and ≥ 1 action. Pre-list blocks
  (sites, patterns, exceptions, keywords, allowlist) were migrated into the
  three lists; `block import` reads pre-list exports. `add-pattern`,
  `remove-site`, `remove-entry` are Blocked shortcuts. List/apps/titles
  files: one entry per line, `#` comments (`block export-list|import-list
  B apps|titles|blocked|exceptions`). Remove block: inactive and unlocked.
- **Schedules**: same-day windows in 5-min steps, `--to 24:00` = midnight;
  overnight = two windows; overlap refused; a block's windows all block or
  all `--allow`. Numbered per block 1..n, renumbered on removal. Schedule
  lock lead: longer always, shorter/off weakens. Old queued edits:
  `kc9k pending`, `pending cancel N`. Only the user switches blocks on/off;
  windows and place rules only lift an active block (a lock keeps it on, not
  enabled). The 2026-11-15 upgrade switched on every inactive block with
  windows or place rules once (`migrateScheduledSwitchOn`).
- **Starting on an active block** only strengthens: longer `--lock-chars`,
  later `--for/--until`; shorter/earlier refused. A challenge longer than the
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
  Frozen Chicken (≤ 24 h).
- **Clusters**: copies are independent. `--exceptions`, `--as N`,
  `--into L`. Export = plain entry file with a scope header (`list import`
  reads it).
- **Recording**: domains without `www.`, pages and frames, private windows
  and blocked pages included; ≤ 2000 sites; one recording at a time; a
  refused `stop` keeps the recording.
- **Places**: re-adding a name adds the BSSIDs now seen; one BSSID needed if
  the place knows only one. A place-ruled block is held while there; rule
  and place can't be removed then. No networks seen = at every ruled place,
  and Wi-Fi is switched back on.
- **Allowance** (`kc9k allowance set|start|end|remove`, daily): use = block's
  app focused, or its site (address/title lists, not text) in the focused
  tab with input < 3 min, or playing (even muted) in any tab. Session with
  websites closes extension-less browsers. Lowering applies at once; raising
  or removing at midnight. A lock or window closes a session. Day follows
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
- **Extension guard**: required while an active block has a non-empty
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
  min, screen unlocked), saved up to one reward. A break lifts a locked
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
  "KC9K: …" in keyboard settings; notification on press; on an active block
  only extends the lock. Older hotkeys ask at each press.
- **Notifications** (`notify-send`, `enforce/notices.go`, `kc9k notifications`):
  user picks kinds (window start/end, timer lock end, Pomodoro, allowance,
  app closed) and advance minutes (default 5; each advance once); windows
  only of blocks that are on; scheduled freeze always (5 and 1 min).
- **Settings password**: needed for weakening, `stop`, `unlock`, schedules,
  allowances, browser settings off, strict off, no-escape, uninstall. Free:
  reading, starting, adding entries, actions on, opening a session. GUI
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
- **Frozen Chicken**: `freeze --for D --mode lock|logout|shutdown --yes`;
  `freeze schedule add --days … --from --to [--mode] [--lock-lead D]`,
  `list`, `remove N`, `lock N on --lead D`. Back-to-back windows run on; a
  longer running freeze is never shortened; ≤ 24 h; time counts while off;
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
