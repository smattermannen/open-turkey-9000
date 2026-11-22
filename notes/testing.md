# Testing

- Covers: `ci/`, `*_test.go`, `cmd/try-challenge/`, `try-challenge.sh`
- SPEC: §5 (tests name the rule they prove)
- Connects to: every note; first read [locks](locks.md) before a test that locks anything

## Commands

- AppImage browser judged by engine, never supported (`blocker`
  roottest, daemon stopped): `sudo KC9K_TEST_APPIMAGE=<browser AppImage>
  go test -tags roottest -run AppImage ./internal/blocker` (runs it as
  `SUDO_UID`; skipped without).
- `./try-challenge.sh [len]` (`cmd/try-challenge/`): unlock challenge alone.
- The text lock's window for real: `KC9K_TEST_PYTHON=<python3 with GTK 3>
  xvfb-run -a go test -run Window ./internal/lock/` (needs xdotool, xterm
  for the focus case; skipped without `DISPLAY`).
- Root Chrome test: `go test -tags roottest -run Chrome ./internal/enforce`,
  with the daemon stopped (it holds the rules port) and Playwright's
  Chromium under `/opt/pw-browsers/chromium-*/` (else skipped).
- On a box with a spinning disk, SQLite's fsyncs make `internal/cli` take
  ~16 min (default `go test` timeout 10 min): give the tests
  `TMPDIR=/dev/shm/…`.
- The owner's VM (2026-11-19): `kc9k-vm` on the test machine, Ubuntu 24.04
  cloud image, Cinnamon autologin on X11 as `tester` (passwordless sudo),
  kc9k and LibreWolf installed, repo at `~/open-turkey-9000`; ssh alias
  `kc9k-vm` (key `~/.ssh/kc9k-vm`), IP 192.168.122.80 pinned, snapshot
  `clean` taken shut off.
- `ci/monk_live_test.sh host [vm]` (R-MONK-2, VM only): ssh goes with the
  cut, so the VM checks itself (`inside 1`, then a oneshot unit runs
  `inside 2` after the restart it makes) into `/var/lib/kc9k-monk-live`;
  the host waits for ssh and reads `verdict`. `MONK_MINUTES` (default 6).
- VM checks with a verdict (`ci/<name>.sh host [domain] [ssh-alias]`,
  default `kc9k-vm`), sharing `ci/vm_check_lib.sh`: the host reverts the
  VM to `clean` (running), rsyncs the tree to `~/open-turkey-9000`, builds
  `.deb`s of it (`9000.N~vmcheck`) and installs the first, starts `inside
  1` as root (`systemd-run`: it outlives ssh and a lockdown's loss of
  sudo), waits through restarts (a oneshot unit runs `inside 2`) for
  `/var/lib/kc9k-vmcheck-*/verdict` (readable without sudo), prints the
  log's PASS/FAIL lines and reverts the VM shut off. kc9k runs as the VM's
  user through the wrapper (strict mode records `SUDO_USER`); status
  checks as root call `kc9k-bin`. `ci/strict_pam_vm_test.sh` (2.6: sets a
  root password and an account `kc9kother`; root login and su driven on a
  pseudo-terminal by an inline Python helper; one boot with
  `kc9k.nofreeze` via a `/etc/default/grub.d` drop-in),
  `ci/tamper_vm_test.sh` (2.9: installer guard with dpkg and `install.sh`,
  which needs Go in the VM; LightDM's `HasGuestAccount` over D-Bus, a
  guest session allowed first if the VM has none). Both never run yet.
## Windows laptop (owner's, Windows 11 Home, 200 % scaling, used for real) *(By claude)*

- Claude Code must run elevated (UAC): the data folder is SYSTEM-only and
  the MSI, `kc9k uninstall` and SYSTEM one-shot tasks need it. Read
  SYSTEM-only files through a one-shot `schtasks /RU SYSTEM` task (create,
  run, delete); never change the data folder's security.
- Safe here: the UI suite against the fake kc9k; `ci/windows_soak_test.ps1`
  with a freeze guard beside it. Not without the owner: RealTests or any
  block with a website (the extension guard and browser-setting changes
  close LibreWolf, the owner's browser, even mid-dialog).
- At 200 %, `gui.json` window sizes are physical pixels and the UI tests'
  coordinates and `Capture.Element` were logical (fixed in the DPI-aware
  tests); the Accessibility test once cleared the owner's High Contrast
  flags (put back: 126).
- Needs a person: Narrator, reboot-only checks, anything judged by eye.

## Test laptop (owner's, Mint 22.3, `TAGS=devmaster`, used for real)

- Tests add/remove blocks, restart the daemon (tamper) and trip blocks: all
  count on the owner's bypass ladder. `ci/extras_test.py` paces itself when
  a global ladder is on. Never leave test blocks with schedules (schedule
  locks hold them before windows).
- Browser tests: `GECKOWEBDRIVER=~/.local/share/kc9k-browsertest
  ~/.local/share/kc9k-browsertest/venv/bin/python ci/<name>.py`
  (`browser_test`, `extras_test`, `actions_test`, `quota_test`,
  `download_test`, `flash_test`: R-PAGE-1, records per animation frame
  whether a blocked word was painted, and a dark page unstyled). Needs a `TAGS=devmaster` install
  (else the guard closes the remote-controlled LibreWolf: "started for
  remote control"). Run each alone: back to back (2026-11-20, desktop)
  the later suites found local pages "blockedByPolicy", probably the
  earlier suites' trips carried over; each passes alone. No Node
  there: `cjs ci/match_test_cjs.js`.
- Extension scripts with Node (cloud or box): `node ci/match_test.js`
  (matching, folding), `node ci/background_test.js` (background script,
  browser APIs stubbed: frame reports, the ad trigger),
  `node ci/android_background_test.js`; with Playwright's Chromium (cloud
  only): `node ci/content_harness_test.js`. Its principle 9 check times
  each of the extension's own callbacks (idle time, timers, observers,
  events) on a huge page, at most 16 ms; how long the page as a whole was
  held up (the browser's own layout of 40,000 words included) is printed,
  not checked.
- `ci/allowance_notify_test.sh` (box, real daemon, desktop unlocked; ~4
  min): an allowance session's advance notice arrives on the session bus
  and the session ends on time (B1).
- `ci/lockedhint_check.sh` (box, as the desktop's user from a terminal in
  it; never run yet): locks the screen with
  `cinnamon-screensaver-command --lock` and checks logind's `LockedHint`
  for the user's x11/wayland sessions before, while locked and after the
  person unlocks (B8: `blocker.lockedByLogind` relies on it).
- X11 tests (`startX`) need `/tmp/.X11-unix` owned by root and sticky
  (`trustedX11Socket` refuses anything else, on purpose). In a fresh
  cloud container the first Xvfb run by the test user creates it as that
  user and `TestX11IdleMillis`, `TestTitleGuardPerUser`,
  `TestX11ListsAndClosesWindows` fail ("has no X server of root's"): as
  root, `chown root:root /tmp/.X11-unix && chmod 1777 /tmp/.X11-unix`.
- Race check: the `internal/cli` package takes ~14 min under `-race`, past
  go test's 10-minute default: `CGO_ENABLED=1 go test -race -timeout 30m
  ./internal/...`. Clean 2026-11-21.
- GUI smoke tests: own binary, wrapper with `KC9K_DB` = scratch file, load
  `gui/kc9k-gui.py` as module with `CLI` replaced, other application ID,
  never `show()`. `KC9K_DB` never applies layers nor sets desktop hotkeys.
  As a normal user (as root `KC9K_DB` is ignored: they would use the real
  DB). `ci/gui_click_everything_smoke.py` clicks every button and switch
  of every page, tab and ⋯ menu, once with dialogs cancelled and once
  answered OK as a person could (a greyed-out button isn't pressed); fails
  on a handler exception or a command the CLI can't read. Examples: `ci/gui_phrase_smoke.py`, `ci/gui_phrase_edit_smoke.py`,
  `ci/gui_duration_smoke.py`, `ci/gui_hotkey_smoke.py`, `ci/gui_extend_smoke.py`,
  `ci/gui_circadian_smoke.py` (`xvfb-run -a`).
- Weakening audit: `cli/weakening_audit_test.go` tries every command that
  could weaken a locked block; add a new weakening command there.
