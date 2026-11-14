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
sudo ./install.sh     # binary, wrapper, sudoers, systemd unit, DB, LibreWolf
```

- No CGo (`modernc.org/sqlite`). Check: `CGO_ENABLED=0 go build ./...`.
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
  (`browser_test`, `extras_test`, `actions_test`, `quota_test`). No Node
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
  (`block.go`, `control.go`: start/stop/unlock/status). `kc9k --help` lists them.
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
    user-owned exe, minus Electron). LibreWolf: `block-devtools` policy;
    five prefs locked in librewolf.cfg, user overrides replaced by a cleaned
    copy (`librewolfprefs.go`).
  - `sockets.go`: strict mode closes docker/lxd/incus/libvirt sockets.
    Strict lockdown: `/etc/sudoers.d/zz-kc9k-lockdown`,
    `/etc/polkit-1/rules.d/00-kc9k-lockdown.rules`; no exec from `/home`,
    `/tmp`, `/var/tmp`, `/dev/shm`. Boot param `kc9k.nofreeze` skips freeze
    and lockdown for that boot.
  - `titles.go`/`x11.go`: asks matching windows to close, kills after
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
  durations count real hours. Freeze schedules: `db.FreezeSchedule.LockLead`;
  daemon `reconcileFreezeSchedules`.
- **Locks** (`internal/cli/`): `blockHold` decides lock state (text, timer,
  password, restart, delay, schedule lock `db/schedulelock.go`).
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
  block, blacklists and exception lists, each typed (`web_lists.type`,
  `WebList.IsAddress`; to be removed, ROADMAP 0.1) with scope
  (`db.CheckScope`). `migrateListTypes` splits only untyped rows;
  `fillListTypes` types everything. Term-list url terms → `url_terms`,
  matched as words (`kc9kUrlWords`). Only address blacklists with `block`
  feed hosts/firewall/policy (`enforce.Inputs`), and not for blocks with
  title/text exceptions. Everything goes to the extension as `lists` rules
  (`enforce.BuildRules`; `kc9kListActions` in `match.js`: plain words
  whole, `*` inside words; a list waits if its exceptions need unread page
  parts). Per action: threshold (distinct entries) and hit count.
  Strengthening free; else gated (`EditList*` by id). Shortcuts (`block add`,
  `add-exception`, `add-keyword`, `threshold`, `action`) use default lists
  Blocked / Exceptions / Page words (`migrateWebLists`). Old combinations
  `(a, b) (c):2`: `migrateCombinations` (`db/combinations.go`, every open
  and after imports) turns blacklist/term-cluster ones into phrase sets;
  `refuseCombinations` refuses new typed ones; leftovers (one box, or in
  exception lists) use `kc9kCombination`. Case-insensitive lookup.
- **Actions** (`db/actions.go`, Param/Minutes): block, greyscale,
  no-images, mute (`applyMute`), no-video, dark, declutter (to remove),
  redirect, image (`db.ImagesDir`, served `/image/`), close-tab;
  device: close-browser, logout, shutdown, monk. Extension `kc9kOutcome`
  picks the strongest, closes tab/browser itself, POSTs `/hit`
  (`enforce/hits.go`, header required) so the daemon logs and runs
  logout/shutdown/monk.
- **Phrase sets** (`db/phrases.go`, `cli/phrases.go`, `kc9k phrase-set`):
  `db.Phrase` JSON, typed `[a, b] [c]? :N` (`ParsePhrase`, `String`);
  `Check` refuses one-ended bridges (mirrored `kc9kPhraseCheck`). Rules
  with negative IDs (`phraseRules`), matched by `kc9kPhraseMatcher`.
  `HasPhrases` makes a block non-empty. Names unique case-insensitively.
  GUI: `PhraseSetCard` (a `ListCard` keyed by −id), `PhraseDialog`. Files:
  `cli/phraseexport.go`; `db.AddPhraseSet`. Phrase clusters (`phrases`
  set): `cluster copy` → phrase set, `cluster save` from one; no recording.
- **Clusters** (`db/clusters.go`, `cli/clusters.go`): lists outside blocks,
  never enforced. `cluster copy` goes through `listAdd` (gating applies).
- **Link recording** (`db/recording.go`, `enforce/recording.go`,
  `cli/record.go`): `rules.recording` → extension POSTs `/visit`; daemon
  buffers; `record stop` adds via `listAdd`/`clusterAdd`.
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
  them. Only `force-librewolf` is also held by block locks (`GuardsBlocks`).
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
- **Options** (`enforce/options.go`): `polite-close`, `block-page-details`;
  never held.
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
