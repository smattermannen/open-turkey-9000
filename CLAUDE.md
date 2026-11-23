# Killer Chicken 9000 (KC9K)

Productivity blocker: CLI + daemon + browser extension + GUI. Linux: systemd
daemon, GTK GUI. Windows: in progress (`ROADMAP.md` §4).

## Documents

- `SPEC.md`: vision, principles, glossary, rules. Changed with the owner;
  agents' changes tagged *(By claude)*.
- `ROADMAP.md`: only status file (status vs `SPEC.md`, next work).
- `notes/*.md`: built behaviour per area. `ROADMAP.md` and notes change in
  the same commit as the code; a fact lives in one note.

Read only what your work needs: `ROADMAP.md` Index → your item; `SPEC.md`
Index → cited sections/rule IDs (`grep -n 'R-LOCK-6' SPEC.md`); glossary
words (§3) exactly, §3.14 = words to avoid; notes for files you touch, plus
their "Connects to" notes your change reaches (unsure: read one more).

Code vs `SPEC.md`: `SPEC.md` wins; log the gap in `ROADMAP.md`. `SPEC.md`
wrong or silent: ask the owner (unattended: `ROADMAP.md` §5 entry). Fix
before adding (`ROADMAP.md` §0). No other planning/status files, no
user-facing docs.

## Build and test

```bash
export PATH="/usr/local/go/bin:$PATH"
make build            # CGO_ENABLED=0 go build -o kc9k ./cmd/kc9k
make check            # go test, prints only failures (PKG=./internal/cli/... ARGS="-run TestX")
make deb              # packaging/build-deb.sh (TAGS=devmaster for tests)
sudo ./install.sh     # binary, wrapper, sudoers, systemd unit, DB, LibreWolf
```

- No CGo (`modernc.org/sqlite`): `CGO_ENABLED=0 go build ./...`.
- Race (tests only, gcc): `CGO_ENABLED=1 go test -race ./internal/...`.
- Tests as a normal user, never `sudo` (root ignores `KC9K_DB`; `internal/cli` tests refuse).
- Testing anything that locks, blocks or restarts the daemon: read
  [notes/testing.md](notes/testing.md) first; the owner's laptop is used for real.

## CI and long tasks

- Local gate first: `make check`, and `windows/gui/check.sh` for
  `windows/` (compiles the GUI and its tests on Linux, ~45 s). Never use
  CI to find compile errors.
- While fixing, run only the tests you changed: `-run` / `--filter`
  locally, in CI a `workflow_dispatch` run with the workflow's `filter`
  input (5–10 min; the full Windows GUI run is ~35 min). Push for a full
  run only once the filtered run is green; one full run at the end.
- Before CI on Windows, read "CI runners" in
  [notes/testing.md](notes/testing.md): don't rediscover the runner.
- Cancel your own superseded runs; one live run per branch.
- One step per merge: each step merged to main as it goes green (Git
  below), never held on the branch for one final merge.
- Status messages only at milestones (a step merged, blocked, done).
- Keep each failing UI test's last screenshot and the real CLI's logged
  answers; read them before rerunning.
- Never drop, skip or weaken a test to save time.

## Shared rules

- `enforce` is the single source of truth: active blocks → unioned layer
  inputs; `Apply` (CLI, on change), `Tick` (daemon).
- Active vs enabled (§3.3): only the user switches blocks on/off; windows,
  place rules, breaks, pauses, allowance sessions only lift an active block;
  a lock keeps it on, not enabled.
- Strengthening free; weakening a locked block refused (R-WEAK-1/2, CLI only:
  `ensureEditable`, `gateEdit`, `gateAutomation`), logged via `refused()`,
  never a bypass. Every possibly-weakening command goes in
  `cli/weakening_audit_test.go`.
- Every lock confirmed before written (R-LOCK-12, `confirmLocks`, `--yes`).
- GUI decides nothing: every change runs a `kc9k` command.
- Device effects (network cuts, logout, shutdown, Wi-Fi, desktop, circadian
  hardware) via replaceable vars faked in tests; `KC9K_DB` never applies layers.

## Git

- One git command per Bash call, no `&&` chains: allow rules in
  `.claude/settings.json` match a whole command only; a chain goes to the
  permission check and may be stopped.
- Merge to main: `git fetch -q origin main`, check it is an ancestor, then
  `git push -q origin HEAD:main` on its own.

## Architecture

`cmd/kc9k/main.go` → `cli.Execute()`; Cobra commands in `internal/cli/`.
Android: `android/CLAUDE.md`. Each note opens with files covered, SPEC
sections, connected notes.

| Note | Area |
|---|---|
| [testing](notes/testing.md) | Test laptop, browser/GUI/harness tests, weakening audit |
| [install](notes/install.md) | Install, uninstall, devmaster, packaging |
| [enforce-daemon](notes/enforce-daemon.md) | `enforce`, `daemon`, affected users |
| [layers-net](notes/layers-net.md) | hosts, firewall, DNS |
| [browsers](notes/browsers.md) | Policies, extension guard, browsers, browser setting locks, tamper protection |
| [extension](notes/extension.md) | `internal/webext/files/` |
| [apps-titles](notes/apps-titles.md) | Process kill, scripts, window titles, polite close |
| [lists](notes/lists.md) | Blocks, website lists, phrase sets, import, presets, recording, drafts, test mode |
| [actions](notes/actions.md) | Actions, triggers, consequences, bypass prevention |
| [locks](notes/locks.md) | Locks, confirmation, text lock, passwords, no weakening |
| [automation](notes/automation.md) | Schedules, modes, places, autostart |
| [device](notes/device.md) | Monk mode, Frozen Chicken, allowed apps, strict mode, no-escape, firmware lock, guided setup, sockets |
| [allowance](notes/allowance.md) | Allowance, Pomodoro, breaks, pauses, statistics |
| [circadian](notes/circadian.md) | Circadian rhythm, mode wallpapers, sleep hook |
| [hotkeys](notes/hotkeys.md) | Hotkeys, notifications |
| [gui](notes/gui.md) | GTK GUI, design layer, inputs |
| [gui-windows](notes/gui-windows.md) | Windows GUI (C#, WinUI 3), UI Automation tests |
| [install-windows](notes/install-windows.md) | Windows MSI (WiX), signing |
| [db-project](notes/db-project.md) | SQLite DB, project files, backup |
| [sync](notes/sync.md) | KC9K account, cross-device sync, accountability partner (Supabase) |
| [windows](notes/windows.md) | Windows Go side: platform split, CI, service, session agent, pipe |
| [windows-layers](notes/windows-layers.md) | Windows web layers, apps, desktop, places, circadian |
| [windows-tamper](notes/windows-tamper.md) | Windows tamper protection, strict mode, no-escape, firmware lock |
