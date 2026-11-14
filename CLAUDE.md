# Killer Chicken 9000 (KC9K)

A command-line (CLI) productivity blocker for Linux, backed by a `systemd` service.

## Documents

There are four, and each fact lives in exactly one of them:

| File | Holds | Who changes it |
|---|---|---|
| `SPEC.md` | What KC9K should be: vision, principles, glossary, functions, rules, decisions | Only with the owner's agreement |
| `ROADMAP.md` | Where the code stands against `SPEC.md`, and what comes next. The only file with status | Agents, in the same commit as the code |
| `README.md` | How to use what is built | Agents, in the same commit as the code |
| `CLAUDE.md` | How the code is laid out, how to build and test | Agents |

**Read `SPEC.md` and `ROADMAP.md` first.** Use the glossary's words exactly.
Where code or docs disagree with `SPEC.md`, `SPEC.md` is what was meant:
note the gap in `ROADMAP.md`. If `SPEC.md` itself seems wrong or silent,
ask the owner; don't decide alone and don't invent features. **Fix current
functionality before adding new features** (`ROADMAP.md` section 0). Don't
create other planning or status files.

Blocking is organized into **blocks** (named groups) of **sites** (domains) and
**apps** (process names). When a block is activated, a systemd daemon
continuously ensures the blocking layers stay applied.

## Build

```bash
export PATH="/usr/local/go/bin:$PATH"
make build            # CGO_ENABLED=0 go build -o kc9k ./cmd/kc9k
sudo ./install.sh     # compiles + installs binary, wrapper, sudoers, systemd and DB
```

Built without CGo because the SQLite driver is `modernc.org/sqlite` (pure Go). To
check that everything compiles: `CGO_ENABLED=0 go build ./...`. Run tests
as an ordinary user, never with `sudo`: as root `KC9K_DB` is ignored, so the
`internal/cli` tests would write to the real database (they refuse to run).

### The test laptop

The owner's laptop (Mint 22.3, dual boot with Windows) runs a `TAGS=devmaster`
build and is **used for real**. On it:

- Tests add and remove blocks, restart the daemon (a tamper event) and get
  pages blocked, which all count for the owner's bypass prevention ladder.
  `ci/extras_test.py` once tripped it and started monk mode; it now paces
  itself when a global ladder is on. Don't leave test blocks with schedules
  behind: a schedule holds its block before each window.
- Browser tests: `GECKOWEBDRIVER=~/.local/share/kc9k-browsertest
  ~/.local/share/kc9k-browsertest/venv/bin/python ci/<name>.py`
  (`browser_test.py`, `extras_test.py`, `actions_test.py`, `quota_test.py`).
  There is no Node: `cjs ci/match_test_cjs.js` runs the matcher tests.
- GUI smoke tests: build a binary, point a wrapper at it with `KC9K_DB` set to
  a scratch file, load `gui/kc9k-gui.py` as a module with `CLI` replaced, give
  the app another application ID, and never `show()` the window. A database
  opened through `KC9K_DB` never applies layers.
- The browser policy stops many blocked pages before the extension sees a
  request, so the extension then learns of them from the tab (every 3 s).
  Anything needing the request itself needs a fallback (`trail` in
  `background.js`).

## Architecture

- **Entrypoint** (`cmd/kc9k/main.go`): lean, just delegates to `cli.Execute()`.
- **CLI** (`internal/cli/`): Cobra commands. `block.go` (block CRUD),
  `control.go` (`start`/`stop`/`unlock`/`status`).
- **Blocking layers** (`internal/blocker/`): `/etc/hosts` (atomic writes),
  iptables + ip6tables firewall (real-IP resolution via a small built-in DNS
  client that bypasses `/etc/hosts` and asks the network's own server, then
  Quad9, or the user's choice: `kc9k dns`, `SetDNSChoice`, loopback answers
  dropped; expanded DoH blocklist), browser policies
  for Firefox/Chrome/Chromium/Brave/Edge/Opera/Vivaldi (blocklist, allowlist,
  wildcard patterns, DoH disabled), and exact-name process killing (plus Flatpak app IDs, matched via the
  `app-flatpak-<ID>-<n>.scope` cgroup or `/.flatpak-info`, and desktop app units
  `app-[<launcher>-]<ID>-<n>.scope`/`@….service` whose ID is or ends in `.<name>`); every
  descendant of a matched process is killed too, except under shells/session processes. Domain
  validation (`validate.go`) guards against `/etc/hosts` injection.
  The firewall is an nftables table `inet kc9k` (sets of addresses, base
  chains on output and forward, loaded atomically with `nft -f`); without
  nft, an iptables chain whose jump comes first in OUTPUT and FORWARD and
  which the drift check re-tops. The extension guard (`extguard.go`) knows
  browsers by name, or by engine (`browserdetect.go`: libxul.so or Chromium's
  icudtl.dat/V8 snapshot mapped by a user-owned executable, minus Electron).
  Strict mode also closes docker/lxd/incus/libvirt sockets (`sockets.go`).
  Window titles: `titles.go` asks matching windows to close, then kills the
  program after `TitleCloseGrace`, via a cgo-free X11 client (`x11.go`:
  Xauthority cookie, _NET_CLIENT_LIST or QueryTree, _NET_WM_NAME/_PID,
  _NET_CLOSE_WINDOW then a sync). The client trusts nothing a user can
  plant: it reads only the user's own regular Xauthority file, sends the
  cookie only to a root- or user-owned socket, caps replies and gives each
  connection one 3 s budget; a window's PID is believed only for the
  session user's own processes. It runs beside the daemon cycle
  (`enforceTitles`), never inside it. Browsers are left to the extension
  (tab titles).
- **Enforce** (`internal/enforce/`): turns the active blocks into layer inputs
  (everything, allowlists included, unioned across active blocks); `Apply`
  (CLI, on state change) and `Tick` (daemon, per cycle: due queued edits +
  schedule reconciliation + drift-only reapply). Single source of truth so CLI
  and daemon agree.
- **Schedule** (`internal/schedule/`): pure time logic for recurring windows
  (block windows, or `--allow` windows that keep a block on except while open);
  `EditFreeze` (2 h) is no longer used as a built-in protection; freeze
  schedules have their own lock lead time (`db.FreezeSchedule.LockLead`).
- **Locks and weakening** (`internal/cli/`): `blockHold` decides whether a
  block is locked (text, timer, password, restart, delay, or its schedule
  lock: `db/schedulelock.go`, `kc9k schedule lock`, on in its windows and
  places and for its own lead time); `ensureEditable` refuses only edits that
  would weaken a locked block, and `start` on an active block can only
  strengthen its locks. `gateEdit` (`loosen.go`) no longer queues anything;
  the daemon still applies edits older versions queued. `unlock` lifts text
  and password locks and starts the delay lock's countdown (`delay_until`);
  the block stays on.
- **Daemon** (`internal/daemon/`): systemd service, calls `enforce.Tick` every 5s; also samples the foreground app for statistics every 5s and relocks the screen every 0.5s during a lock freeze.
- **GUI** (`gui/kc9k-gui.py`): GTK 3 Python, English; Dashboard, Blocks, Device Blocks, Statistics, Settings. Reads `kc9k gui-state` and `kc9k stats --json`; every change runs a `kc9k` command.
- **Website lists** (`internal/db/weblists.go`, `internal/cli/weblists.go`,
  `kc9k list …`): each block has named blacklists and exception lists. Each
  list is an address list (url) or a term list (title and/or text;
  `db.CheckWhere`, mixed lists split once by `migrateListTypes`); each blacklist ticks its
  actions (block / greyscale / no-images, `actions.go`) with, per action, two
  counts for page text: distinct entries (threshold) and total hits. An exception list spares pages
  from its own block's blacklists only. Only url blacklists with `block` feed
  hosts, firewall and browser policy (`enforce.Inputs`), and not at all for a
  block with title/text exceptions; everything goes to the extension as
  `lists` rules (`enforce.BuildRules`, `kc9kListActions` in `match.js`: plain
  words are whole words, `*` matches inside words; a list whose exceptions
  look at a part of the page not read yet waits; word combinations such as
  `(car, lambo*) (big, small, nice):2` count runs of directly following words
  that give each box its minimum of different words: `kc9kCombination` /
  `kc9kKeywordMatcher`, mirrored by `pageCombination` in `cli/keywords.go`,
  which stores them normalized, and `parse_combination` plus
  `CombinationDialog` in the GUI). What strengthens is free;
  the rest is gated/queueable (`EditList*` edits name lists by id). Old
  commands (`block add`, `add-exception`, `add-keyword`, `threshold`,
  `action`) are shortcuts to the default lists Blocked / Exceptions / Page
  words; `migrateWebLists` turned pre-list blocks (sites, patterns,
  exceptions, page keywords, allowlist mode) into those lists once.
  More actions (`db/actions.go`, with Param/Minutes): redirect, image
  (stored in `db.ImagesDir`, served at `/image/`), close-tab, and device
  actions close-browser/logout/shutdown/monk. The extension (`kc9kOutcome`)
  picks the strongest page action, closes the tab and browser itself, and
  POSTs `/hit` (`enforce/hits.go`, extension header required) so the daemon
  logs the event and runs logout/shutdown/monk.
- **Clusters** (`internal/db/clusters.go`, `internal/cli/clusters.go`,
  `kc9k cluster …`): website lists outside any block (name, where, entries;
  no kind, no actions, never enforced, always editable). `cluster copy`
  copies one into a block as a new list (blacklist, or `--exceptions`) or
  `--into` an existing list, through `listAdd`, so the block's gating
  applies; `cluster save` makes one from a block's list.
- **Link recording** (`db/recording.go`, `enforce/recording.go`,
  `cli/record.go`, `kc9k record [start <block> <list> | start --cluster C |
  stop [--discard]]`): while on, `rules.recording` makes the extension POST
  each opened page/frame address to `/visit` (extension header required);
  the daemon only buffers sites. `record stop` adds them through
  `listAdd`/`clusterAdd`, so gating happens there, in the CLI.
- **Consequences** (`internal/enforce/consequence.go`): one engine for list
  device actions, bypass prevention and monk mode: close browsers, lock
  screen/logout/shutdown (for a while through Frozen Chicken, only
  extended; once at most every 10 min), monk mode (`StartMonk`: the system-owned
  block `monk-mode`, fixed while on (`refuseMonkEdit`), never removed or renamed, `*.*` by default, time-locked with StopAt ending it).
  Device effects go through replaceable vars so tests never touch the machine.
- **Quota by use** (`internal/quota/`, `enforce/quota.go`): a session spends
  only time in use (`Spend`; `Step` never spends): the daemon's foreground
  sampler adds the block's apps, and the extension reports, from
  `rules.quota` (lists of blocks in a session), seconds a site was in the
  focused tab or played media (`/usage` `quota`). Without LibreWolf a session
  with websites spends wall time; while one is open the extension is required.
- **Events and bypass prevention** (`db/events.go`, `db/nepsis.go`,
  `enforce/nepsis.go`, `cli/nepsis.go`): what KC9K stops is logged (pages
  via `/hit`, app kills per block, title closes, browsers closed, refused
  loosening via `refused()` in the CLI (logged only, never counted), tamper: daemon restart, clock jump),
  kept a day. Bypass detection (`db.Nepsis`: detectors, a ladder of steps,
  quiet time; global setting lockable as setting lock `nepsis`; per block,
  gated) climbs a step when a step's Count events of its detectors fall
  within its window (state per scope in setting `nepsis.ladder[.<block>]`);
  `/hit` judges at once and answers `close_tab`. `search` is a kind of `page` (the extension sends the page's origin as `from`; `fromSearch` in `hits.go`); an event blamed on several blocks (`RecordEventFor`: closed browsers, tamper) has a row per block, `extra` on all but the first so it counts once globally. Old one-step settings are
  read as a ladder (`normalize`).
- **Hotkeys** (`internal/db/hotkeys.go`, `internal/cli/hotkeys.go`,
  `internal/blocker/hotkeys.go`): Cinnamon and GNOME custom keybindings in dconf named
  `kc9k<id>`, running `kc9k hotkey run <id>` (starts the block in-process with
  the preset `--for`; the settings password is asked at `hotkey add`).
- **Browser setting locks** (`internal/db/settinglocks.go`, `internal/cli/browser.go`):
  each browser option locks on its own (time, typing challenge, password,
  restart), independently of blocks; `enforce.AnyLock` counts them (strict
  mode, no-escape) and trusted time freezes for their time locks. Only
  `force-librewolf` is also held by block locks (`GuardsBlocks`).
- **Extension guard** (`internal/blocker/extguard.go`): the extension is
  mandatory. While `enforce.ExtensionRequired` (an active block with
  websites, or web-only/force-search on) and LibreWolf or Chrome is installed
  (`blocker.ExtensionBrowserInstalled`), the
  daemon closes browsers whose extension hasn't fetched `/rules` with the
  `X-KC9K-Extension` header within 60 s (connection traced via
  `/proc/net/tcp` to the PID), and at once those started for remote control
  (`remotecontrol.go`), or started before KC9K last changed the settings
  protecting the extension (`noteSettingsChange`). The last check-in is kept
  for `kc9k status`. LibreWolf hardening: `block-devtools` (default on,
  DisableDeveloperTools policy); five prefs locked at the end of
  librewolf.cfg, with the user's overrides file replaced by a cleaned copy
  (`librewolfprefs.go`).
- **Google Chrome** (`webext/chrome.go`, `blocker/chromesetup.go`): the same
  extension for Chrome and Chromium. `PackChrome` swaps in a Manifest V3
  manifest built from Firefox's and a service worker (`chrome-worker.js`,
  `importScripts` of the background scripts), and signs a CRX3 with a key
  made per computer (`webext.ChromeKeyPath`). The daemon serves
  `/chrome/update.xml` and the `.crx`; `kc9k-setup.json` force-installs it
  (`ExtensionSettings`), turns incognito, guest and developer mode off.
  `compat.js` aliases `browser` to `chrome`; `onMessage` answers through
  `sendResponse`; a keepalive holds the service worker; `rules.extension_version`
  makes Chrome's copy ask for an update. Root test: `go test -tags roottest
  -run Chrome ./internal/enforce` (Chrome for Testing reads
  `/etc/opt/chrome_for_testing/policies`).
- **Settings password** (`internal/cli/settingspassword.go`): when set,
  `requireSettingsPassword` guards every loosening (it sits in `gateEdit` and
  `ensureEditable`), stop/unlock, browser settings off, strict off,
  no-escape and prepare-uninstall; `requireLockPassword` asks for it before
  a lock is set only with `password locks on` (R-WEAK-1). Passwords come one
  per line from `stdinLines` (settings password first). The GUI retries a
  refused command once after asking for it.
- **No weakening during active locks** (`internal/cli/noweakening.go`,
  R-WEAK-4): `refuseSettingWeakening` sits in every setting's weakening path
  (strict off, browser settings off, global bypass prevention, settings
  password). Strict mode has its own setting lock (`enforce.StrictLockName`),
  counted by `AnySettingLocked` but not starting the lockdown itself.
- **Access passwords** (`internal/cli/access.go`, `kc9k access [set|remove
  <area…>|--all]`): optional passwords per area (`app`, `dashboard`,
  `blocks`, `device`, `stats`, `settings`; setting `access.password.<area>`,
  areas set together share a hash). `accessGate` (root `PersistentPreRunE`)
  maps each command to its areas (`commandAreas`) and needs the app password
  plus one open area; `daemon`, `hotkey run`, `gui-state` are exempt.
  Passwords come as the first stdin lines, counted by the hidden
  `--access-lines N` (before the settings password), or are asked on a
  terminal. `gui-state` reports `access`/`access_unused` and empties what
  locked pages own (`hideLocked`); the GUI covers locked pages (`guarded`).
- **Options** (`enforce/options.go`, `kc9k option [<name> on|off]`):
  behaviour switches, never held: `polite-close` (`blocker/polite.go`: a
  blocked app's windows are asked to close, killed a cycle later; `Apply`
  leaves kills to the daemon then) and `block-page-details`.
- **Pauses** (`db/pauses.go`, `enforce/soon.go`): `block_pauses` lifts an
  active block until a trusted time; `pausedFor` (Pomodoro break or pause)
  is what `withoutBreaks` checks. `rules.soon` (quota left, break left,
  schedule start within 2 h) feeds the toolbar badge and popup
  (`timeLeftFor`, `popup.js`); `/add` adds the tab's site to a block's
  Blocked list (refused with an access password on blocks).
- **Breaks** (`db/breaks.go`, `cli/breaks.go`, `kc9k break [allowance B D
  --per D | relock B D | start B [D] | end B]`): an allowance earns pause
  time while the block is enforced (`earnBreaks` in `Tick`); `relock` makes
  `unlock` set a pause instead of deactivating (a second unlock during it
  ends the block). Looser settings go through `ensureEditable`. Pomodoro
  has an optional long break (`quota.Cycle`); `PomodoroWorking` keeps a
  quota session closed during a work period.
- **Project** (`cli/project.go`, `kc9k project export | import <file>
  [--dry-run]`): everything except locks, passwords, hotkeys and
  statistics in one versioned JSON; import validates all first, then only
  adds (existing blocks/clusters untouched, options only strengthened).
- **Look actions** (`db/actions.go`): besides greyscale and no-images,
  `mute` (background `applyMute`), `no-video`, `declutter` and `dark`
  (content.js `render`, CSS only; dark skips pages that are dark already).
- **Social media settings** (`db/social.go`, `cli/social.go`, `kc9k social
  [block] [<site:what> on|off | playlist <id|url> on|off]`): presets per
  block in `block_social`, on `BlockDetail.Social`; `rules.social` goes to
  `webext/files/social.js` (`KC9K_SOCIAL`: hosts, redirects, hide CSS).
  A playlist is the preset `youtube:playlist=<id>` plus two exceptions
  (`youtube.com/playlist?list=<id>`, `youtube.com/watch`); the extension
  blocks video pages without an allowed `list=` and checks the playlist
  panel (`checkPlaylistVideo`).
- **Places** (`db/places.go`, `enforce/places.go`, `cli/places.go`, `kc9k
  place [add N | remove N | rule <block> <place> on|off]`): a place is a
  set of BSSIDs from `nmcli` (`ScanWifi`, replaceable in tests); `AtPlace`
  needs two in range, and an empty scan counts as there. `PlaceBlocks`
  feeds `reconcileSchedulesAt` and `ScheduleActiveNow`, so a place rule
  behaves like an open schedule window; `keepWifiOn` re-enables the radio.
- **Keyword test mode** (`db/testmode.go`, `kc9k list test <block> on|off`):
  title/text lists of the block go out with `test: true`; the extension
  shows `kc9kTestReport` in a panel and skips them in `kc9kListMatches`.
- **App matching beyond names** (`blocker/scripts.go`): Python programs by
  script (`pythonScript`, `matchesScript`; `essentialProcess` spares only
  system scripts), copies by contents (`HashEntry`: add-app stores
  `sha256:<hex>:<size>:<path>` beside a path, hidden by `visibleApps`;
  system programs never match), and `KillNaming` (fetch tools given a
  blocked domain, per block in `Tick`, logged as workaround).
- **Workarounds**: `kc9kUnwrap` reads any full address carried in a query or
  path; `checkMirror` (content.js) reports canonical addresses and known
  front-ends; `/hit` with `via` logs `EventWorkaround` (a detector).
- **DB** (`internal/db/`): SQLite at `/var/lib/kc9k/kc9k.db` (WAL; every
  pooled connection gets busy_timeout and foreign_keys via `_pragma`);
  `KC9K_DB` overrides the path (tests only; the unit never sets it).
  `db.go` core + `features.go` (schedules, time locks, schema upgrades) +
  `weblists.go` (website lists) + `pending.go` (queued loosening edits).
- **Lock confirmation** (`internal/cli/confirm.go`, R-LOCK-12): every path
  that sets, adds to or extends a lock builds `lockToSet` items (target, kind,
  length) and calls `confirmLocks` before writing anything: a terminal is
  asked for "yes", anything else is refused without `--yes` (`addYesFlag`).
  Tests: `confirm_test.go` (`lockCases`, one per lock kind and target).
  The GUI's lock dialogs all go through `confirm_lock` and then pass `--yes`.
  A hotkey's lock is confirmed at `hotkey add`; `hotkey run` passes `yes`.
- **Lock** (`internal/lock/`): raw-mode, paste-resistant typing challenge (random
  characters: 60% letters, 35% digits, 5% symbols) that gates a char-locked
  block's deactivation.

## System installation

`make install` copies everything outside the source directory: the real binary to
`/usr/local/bin/kc9k-bin`, the `kc9k` wrapper (does `exec sudo`), a rule
in `/etc/sudoers.d/kc9k`, the `kc9k.service` unit and the DB in
`/var/lib/kc9k/`. The installed program doesn't depend on the project folder.

## Commands

Android companion app: see `android/CLAUDE.md` (it runs this extension in IronFox and serves it `/rules` itself; `rules.safesearch` comes only from there, `kc9kSafeSearch` in `match.js`).

Full usage and examples in `README.md`. Summary:
`cluster list | show | create [--url --title --text] | rename | delete | add | remove | scope | export | import | copy [--into L | --as N] [--exceptions] | save`,
`list show | create [--exceptions --url --title --text] | rename | delete | add | remove | scope | action |
threshold | hits | export | import` (`action … on --url/--image/--for`), `monk --for D`,
`bypass` (old name `nepsis`) `[set --count N --within D --then C [--then-for D] [--block B] [lock flags] | off | unlock]`,
`block create [--allowlist] [--template T] | add | add-exception | add-app | add-title | remove-title |
remove-app | action | export | import | export-list | import-list | list |
info | remove`, `start [--lock | --text-lock] [--lock-chars N] [--lock-words | --lock-text T] [--for D | --until HH:MM]`,
`strict on|off|unlock [lock flags]`, `no-weakening on|off`, `password locks on|off`, `dns [network|quad9|IPs]`,
`stop`, `unlock`, `status`, `schedule add [--allow]|list|remove`,
`pending [cancel N | delay <block> [D]]` (loosening edits take `--now`; the delay is per block, `db.LoosenDelay`), `block add-keyword | remove-keyword | threshold`, `hotkey add [--for D] | list | remove`, `password set|remove`, `access [set|remove <area…>|--all]`, `option [<name> on|off]`, `break …`, `social …`, `place …`, `project export|import`, `pomodoro set B W R [--long D --every N]`, `list test <block> [on|off]`, `freeze [--for D --mode M --yes] | freeze schedule add|list|remove` (the daemon's `reconcileFreezeSchedules` starts/extends scheduled freezes), `browser [<option> on|off [lock flags] | unlock <option>]`, `stats [on|off|clear] [--days N] [--json]`, and the hidden
`prepare-uninstall` (run by `make uninstall`).

`./try-challenge.sh [length]` (source: `cmd/try-challenge/`) builds and runs the
unlock challenge on its own for practice/testing; it touches no blocks.
