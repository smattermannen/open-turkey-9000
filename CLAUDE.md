# Killer Chicken 9000 (KC9K)

Linux productivity blocker: CLI + systemd daemon + browser extension + GTK GUI.

## Documents

| File | Holds | Changes |
|---|---|---|
| `SPEC.md` | What KC9K should be: principles, glossary, rules, decisions | With the owner, or the autopilot's resolver (tagged *(By Opus)*) |
| `ROADMAP.md` | Status against `SPEC.md`; next work. Only status file | Same commit as the code |
| `CLAUDE.md` | Shared rules, build, test, note index | Agents |
| `notes/*.md` | Code layout and built behaviour, one note per area | Agents, same commit as the code |
| `android/CLAUDE.md` | Android app | Agents |

## Reading, not everything

Read only what your work needs:

1. `ROADMAP.md`: its Index, then only your item.
2. `SPEC.md`: its Index, then only the sections and rule IDs your item cites
   or the Index names for your topic (`grep -n 'R-LOCK-6' SPEC.md`, a
   heading such as `### 3.4`, a function `**4.3`). Use glossary words
   (§3) exactly; §3.14 lists words to avoid.
3. `notes/`: the notes for the files you touch (table below), then each
   note in their "Connects to" line that your change reaches. When unsure,
   read one more note rather than guess.

Code vs `SPEC.md`: `SPEC.md` wins; log the gap in `ROADMAP.md`. If `SPEC.md`
seems wrong or silent, ask the owner (unattended: a `ROADMAP.md` §5 entry). Fix before adding (`ROADMAP.md` §0). No
other planning/status files. No user-facing docs. A change to code updates
its note in the same commit; a fact lives in exactly one note.

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

Before testing anything that locks, blocks or restarts the daemon, read
[notes/testing.md](notes/testing.md): the owner's test laptop is used for real.

## Rules shared by every area

- `enforce` is the single source of truth: active blocks → layer inputs
  (unioned); `Apply` (CLI, on change) and `Tick` (daemon).
- Active vs enabled (§3.3): only the user switches blocks on or off;
  windows, place rules, breaks, pauses and allowance sessions only lift an
  active block; a lock keeps it on, not enabled.
- Strengthening is always free; weakening a locked block is refused
  (R-WEAK-1/2: decided in the CLI only, `ensureEditable`, `gateEdit`,
  `gateAutomation`), logged via `refused()`, never counted as a bypass. Every
  command that could weaken goes into `cli/weakening_audit_test.go`.
- Every lock of every kind is confirmed before it is written (R-LOCK-12,
  `confirmLocks`, `--yes`).
- The GUI decides nothing: every change runs a `kc9k` command.
- Device effects (network cuts, logout, shutdown, Wi-Fi, desktop and
  circadian hardware) go through replaceable vars faked in tests; `KC9K_DB`
  never applies layers.

## Architecture

- `cmd/kc9k/main.go` → `cli.Execute()`. Cobra commands in `internal/cli/`
  (`block.go`, `control.go`: start/stop/unlock/status; `toggle.go`). `kc9k --help` lists them.
- **Android**: `android/CLAUDE.md`. Its `RulesServer` serves `/rules`;
  `rules.safesearch` comes only from there (`kc9kSafeSearch` in `match.js`).

| Note | Area (main paths; `blocker/` = `internal/blocker/`, etc.) |
|---|---|
| [testing](notes/testing.md) | Test laptop, browser/GUI/harness tests, weakening audit (`ci/`) |
| [install](notes/install.md) | Install, uninstall, devmaster (`install.sh`, `packaging/`, `cli/uninstall.go`, `cli/master.go`) |
| [enforce-daemon](notes/enforce-daemon.md) | `enforce/enforce.go`, `enabled.go`, `internal/daemon/`, affected users |
| [layers-net](notes/layers-net.md) | hosts, firewall, DNS (`blocker/hosts.go`, `firewall.go`, `resolve.go`) |
| [browsers](notes/browsers.md) | Policies, extension guard, supported browsers, LibreWolf, Chrome, browser setting locks |
| [extension](notes/extension.md) | `webext/files/` (`match.js`, `content.js`, `background.js`, `social.js`) |
| [apps-titles](notes/apps-titles.md) | Process kill, scripts, window titles, X11, polite close, behaviour settings |
| [lists](notes/lists.md) | Blocks, website lists, phrase sets, import, clusters, presets, recording, drafts, test mode |
| [actions](notes/actions.md) | Actions, ad trigger, trigger actions, consequences, bypass prevention, workarounds |
| [locks](notes/locks.md) | Locks, layered locks, confirmation, text lock, settings/access/block passwords, no weakening |
| [automation](notes/automation.md) | Automation, schedules, modes, places, autostart (`internal/schedule/`, `cli/modes.go`) |
| [device](notes/device.md) | Monk mode, Frozen Chicken, strict mode, no-escape, sockets |
| [allowance](notes/allowance.md) | Allowance, Pomodoro, breaks, pauses, statistics (`internal/quota/`) |
| [circadian](notes/circadian.md) | Circadian rhythm, mode wallpapers, sleep hook (`internal/circadian/`) |
| [hotkeys](notes/hotkeys.md) | Hotkeys, notifications |
| [gui](notes/gui.md) | `gui/kc9k-gui.py`, design layer, inputs |
| [db-project](notes/db-project.md) | SQLite DB, project files, backup (`internal/db/db.go`, `cli/project*.go`) |

Each note starts with the files it covers, its SPEC sections and the notes
it connects to.
