# Testing

- Covers: `ci/`, `*_test.go`, `cmd/try-challenge/`
- SPEC: §5 (tests name the rule they prove)
- Connects to: every note; first read [locks](locks.md) before a test that locks anything

## Commands

- `./try-challenge.sh [len]` (`cmd/try-challenge/`): unlock challenge alone.
- Root Chrome test: `go test -tags roottest -run Chrome ./internal/enforce`.

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
