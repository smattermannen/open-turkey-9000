# Killer Chicken 9000 (KC9K)

A command-line (CLI) productivity blocker for Linux, backed by a `systemd` service.

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
check that everything compiles: `CGO_ENABLED=0 go build ./...`.

## Architecture

- **Entrypoint** (`cmd/kc9k/main.go`): lean, just delegates to `cli.Execute()`.
- **CLI** (`internal/cli/`): Cobra commands. `block.go` (block CRUD),
  `control.go` (`start`/`stop`/`unlock`/`status`).
- **Blocking layers** (`internal/blocker/`): `/etc/hosts` (atomic writes),
  iptables + ip6tables firewall (real-IP resolution via a small built-in DNS
  client that bypasses `/etc/hosts`; expanded DoH blocklist), browser policies
  for Firefox/Chrome/Chromium/Brave/Edge/Opera/Vivaldi (blocklist, allowlist,
  wildcard patterns, DoH disabled), and exact-name process killing (plus Flatpak app IDs, matched via the
  `app-flatpak-<ID>-<n>.scope` cgroup or `/.flatpak-info`, and desktop app units
  `app-[<launcher>-]<ID>-<n>.scope`/`@….service` whose ID is or ends in `.<name>`); every
  descendant of a matched process is killed too, except under shells/session processes. Domain
  validation (`validate.go`) guards against `/etc/hosts` injection.
  The firewall's jump comes first in both OUTPUT and FORWARD (containers/VMs)
  and the drift check re-tops it. The extension guard (`extguard.go`) knows
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
  `EditFreeze` (2 h) is the lead time before a window during which the CLI
  refuses weakening edits/removals of that block or schedule.
- **Holds** (`internal/cli/`): `blockHold` decides whether a block is held;
  `ensureEditable` refuses only edits that would weaken a held block, and
  `start` on an active block can only strengthen its locks. A block with allow
  windows is never loosened directly: `gateEdit` (`loosen.go`) queues the edit
  for `LoosenDelay` (24h, applied by the daemon) or, with `--now`, runs the
  typing challenge first.
- **Daemon** (`internal/daemon/`): systemd service, calls `enforce.Tick` every 5s; also samples the foreground app for statistics every 5s and relocks the screen every 0.5s during a lock freeze.
- **GUI** (`gui/kc9k-gui.py`): GTK 3 Python, English; Dashboard, Blocks, Device Blocks, Statistics, Settings. Reads `kc9k gui-state` and `kc9k stats --json`; every change runs a `kc9k` command.
- **Website lists** (`internal/db/weblists.go`, `internal/cli/weblists.go`,
  `kc9k list …`): each block has named blacklists and exception lists. Each
  list ticks where it looks (url / title / text); each blacklist ticks its
  actions (block / greyscale / no-images, `actions.go`) with, per action, two
  counts for page text: distinct entries (threshold) and total hits. An exception list spares pages
  from its own block's blacklists only. Only url blacklists with `block` feed
  hosts, firewall and browser policy (`enforce.Inputs`), and not at all for a
  block with title/text exceptions; everything goes to the extension as
  `lists` rules (`enforce.BuildRules`, `kc9kListActions` in `match.js`: plain
  words are whole words, `*` matches inside words; a list whose exceptions
  look at a part of the page not read yet waits). What strengthens is free;
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
- **Consequences** (`internal/enforce/consequence.go`): one engine for list
  device actions, bypass prevention and monk mode: close browsers, lock
  screen/logout/shutdown (for a while through Frozen Chicken, only
  extended; once at most every 10 min), monk mode (`StartMonk`: the ordinary
  block `monk-mode`, `*.*` by default, time-locked with StopAt ending it).
  Device effects go through replaceable vars so tests never touch the machine.
- **Events and bypass prevention** (`db/events.go`, `db/nepsis.go`,
  `enforce/nepsis.go`, `cli/nepsis.go`): what KC9K stops is logged (pages
  via `/hit`, app kills per block, title closes, browsers closed, refused
  loosening via `refused()` in the CLI), kept a day. Nepsis (global setting,
  lockable as setting lock `nepsis`; per block, gated) fires a consequence
  when Count events fall within Minutes; only newer events count after.
- **Hotkeys** (`internal/db/hotkeys.go`, `internal/cli/hotkeys.go`,
  `internal/blocker/hotkeys.go`): Cinnamon custom keybindings in dconf named
  `kc9k<id>`, running `kc9k hotkey run <id>` (starts the block in-process with
  the preset `--for`; the settings password is asked at `hotkey add`).
- **Browser setting locks** (`internal/db/settinglocks.go`, `internal/cli/browser.go`):
  each browser option locks on its own (time, typing challenge, password,
  restart), independently of blocks; `enforce.AnyLock` counts them (strict
  mode, no-escape) and trusted time freezes for their time locks. Only
  `force-librewolf` is also held by block locks (`GuardsBlocks`).
- **Extension guard** (`internal/blocker/extguard.go`): the extension is
  mandatory. While `enforce.ExtensionRequired` (an active block with
  websites, or web-only/force-search on) and LibreWolf is installed, the
  daemon closes browsers whose extension hasn't fetched `/rules` with the
  `X-KC9K-Extension` header within 60 s (connection traced via
  `/proc/net/tcp` to the PID), and at once those started for remote control
  (`remotecontrol.go`), or started before KC9K last changed the settings
  protecting the extension (`noteSettingsChange`). The last check-in is kept
  for `kc9k status`. LibreWolf hardening: `block-devtools` (default on,
  DisableDeveloperTools policy); five prefs locked at the end of
  librewolf.cfg, with the user's overrides file replaced by a cleaned copy
  (`librewolfprefs.go`).
- **Settings password** (`internal/cli/settingspassword.go`): when set,
  `requireSettingsPassword` guards every loosening (it sits in `gateEdit` and
  `ensureEditable`), stop/unlock, anything creating a lock, browser options
  off, strict, no-escape and prepare-uninstall. Passwords come one per line
  from `stdinLines` (settings password first). The GUI retries a refused
  command once after asking for it.
- **DB** (`internal/db/`): SQLite at `/var/lib/kc9k/kc9k.db` (WAL; every
  pooled connection gets busy_timeout and foreign_keys via `_pragma`);
  `KC9K_DB` overrides the path (tests only; the unit never sets it).
  `db.go` core + `features.go` (schedules, time locks, schema upgrades) +
  `weblists.go` (website lists) + `pending.go` (queued loosening edits).
- **Lock** (`internal/lock/`): raw-mode, paste-resistant typing challenge (random
  characters: 60% letters, 35% digits, 5% symbols) that gates a char-locked
  block's deactivation.

## System installation

`make install` copies everything outside the source directory: the real binary to
`/usr/local/bin/kc9k-bin`, the `kc9k` wrapper (does `exec sudo`), a rule
in `/etc/sudoers.d/kc9k`, the `kc9k.service` unit and the DB in
`/var/lib/kc9k/`. The installed program doesn't depend on the project folder.

## Commands

Full usage and examples in `README.md`. Summary:
`list show | create [--exceptions --url --title --text] | rename | delete | add | remove | where | action |
threshold | hits | export | import` (`action … on --url/--image/--for`), `monk --for D`,
`nepsis [set --count N --within D --then C [--then-for D] [--block B] [lock flags] | off | unlock]`,
`block create [--allowlist] [--template T] | add | add-exception | add-app | add-title | remove-title |
remove-app | action | export | import | export-list | import-list | list |
info | remove`, `start [--lock] [--lock-chars N] [--for D | --until HH:MM]`,
`stop`, `unlock`, `status`, `schedule add [--allow]|list|remove`,
`pending [cancel N]` (loosening edits take `--now`), `block add-keyword | remove-keyword | threshold`, `hotkey add [--for D] | list | remove`, `password set|remove`, `freeze [--for D --mode M --yes] | freeze schedule add|list|remove` (the daemon's `reconcileFreezeSchedules` starts/extends scheduled freezes), `browser [<option> on|off [lock flags] | unlock <option>]`, `stats [on|off|clear] [--days N] [--json]`, and the hidden
`prepare-uninstall` (run by `make uninstall`).

`./try-challenge.sh [length]` (source: `cmd/try-challenge/`) builds and runs the
unlock challenge on its own for practice/testing; it touches no blocks.
