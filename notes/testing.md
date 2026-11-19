# Testing

- Covers: `ci/`, `*_test.go`, `cmd/try-challenge/`, `ci/autopilot.sh`, `ci/autopilot-team.sh`
- SPEC: §5 (tests name the rule they prove)
- Connects to: every note; first read [locks](locks.md) before a test that locks anything

## Commands

- `./try-challenge.sh [len]` (`cmd/try-challenge/`): unlock challenge alone.
- Root Chrome test: `go test -tags roottest -run Chrome ./internal/enforce`.
- `ci/autopilot.sh`: unattended loop of `claude -p` sessions working down
  `ROADMAP.md`. The script, not the session, picks the item: the next
  Index row whose Needs match `AUTOPILOT_NEEDS` (default `agent`), not in
  `skip.txt` and not claimed by another worker (`claims.txt`, dead
  workers' claims dropped). The prompt names the other workers' items and
  notes to keep out of. Landing is one worker at a time (`land.lock`):
  rebase onto the shared hub (`hub.git`; a short session resolves
  conflicts), gate (build, vet, tests, GUI smoke, SPEC.md untouched,
  ROADMAP.md changed), fast-forward `autopilot`. STUCK or failing twice
  → `skip.txt`. A usage limit retries the round after the reset;
  `AUTOPILOT_ROUNDS=0` runs until `STOP`. State in
  `~/.local/state/kc9k-autopilot/`. Run as a non-sudo user, never on the
  test laptop.
- Resolver round (`resolver_round`): when the open questions (§5 entries
  not `DECIDED`/`FOR THE OWNER`, SPEC §7, `skip.txt`) differ from the
  last run's (`resolver-seen.txt`), one worker (claim `RESOLVER`) runs a
  session on `RESOLVER_MODEL`, no subagents. It decides into SPEC.md
  tagged `(By Opus)` or marks `FOR THE OWNER`; `UNSKIP:` IDs leave
  `skip.txt`. The gate: only SPEC.md and ROADMAP.md, every SPEC hunk
  tagged, nothing above SPEC's Index, no untagged §6 line removed
  (`spec_check`), every turn of the session and of a conflict session on
  that model (`model_check`). `AUTOPILOT_RESOLVE=0` turns it off.
- `ci/autopilot-team.sh DIR...`: one worker per Claude account, each
  `DIR` a `CLAUDE_CONFIG_DIR` logged in to its own account.
- Parallel workers are one Unix user: each runs under its own
  `dbus-run-session` (the GUI smoke tests' fixed application IDs would
  meet on one bus); `startX` (`blocker/x11_test.go`) takes a display only
  when its lock file names its own Xvfb. Go tests use port 0 and
  `t.TempDir()`; two full `go test ./...` at once pass.

## Test laptop (owner's, Mint 22.3, `TAGS=devmaster`, used for real)

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
