# Testing

- Covers: `ci/`, `*_test.go`, `cmd/try-challenge/`, `try-challenge.sh`
- SPEC: §5 (tests name the rule they prove)
- Connects to: every note; first read [locks](locks.md) before a test that locks anything

## Test laptop (owner's, Mint 22.3, `TAGS=devmaster`, used for real)

- Tests add/remove blocks, restart the daemon (tamper) and trip blocks: all
  count on the owner's bypass ladder. `ci/extras_test.py` paces itself when
  a global ladder is on. Never leave test blocks with schedules (schedule
  locks hold them before windows).
- Browser tests: `GECKOWEBDRIVER=~/.local/share/kc9k-browsertest
  ~/.local/share/kc9k-browsertest/venv/bin/python ci/<name>.py`
  (`browser_test`, `extras_test`, `actions_test`, `quota_test`,
  `download_test`, `flash_test`: R-PAGE-1, per animation frame whether a
  blocked word was painted, dark page unstyled). Needs a `TAGS=devmaster`
  install (else the guard closes the remote-controlled LibreWolf). Run each
  suite alone (back to back, later suites see local pages "blockedByPolicy").
  No Node there: `cjs ci/match_test_cjs.js`.
- `ci/allowance_notify_test.sh` (real daemon, desktop unlocked, ~4 min):
  allowance advance notice on the session bus, session ends on time (B1).
- `ci/lockedhint_check.sh` (as the desktop user from its terminal; never
  run yet): locks the screen, checks logind `LockedHint` before/while/after
  (B8: `blocker.lockedByLogind` relies on it).

## Windows laptop (owner's, Windows 11 Home, 200 % scaling, used for real)

- Claude Code must run elevated: data folder is SYSTEM-only; MSI, `kc9k
  uninstall`, SYSTEM one-shot tasks need it. Read SYSTEM-only files via a
  one-shot `schtasks /RU SYSTEM` task (create, run, delete); never change
  the data folder's security.
- Safe: UI suite against the fake kc9k; `ci/windows_soak_test.ps1` with a
  freeze guard beside it. Not without the owner: RealTests or any block
  with a website (extension guard and browser-setting changes close
  LibreWolf, the owner's browser, even mid-dialog).
- At 200 %, `gui.json` window sizes are physical pixels. The Accessibility
  test once cleared the owner's High Contrast flags (restore value: 126).
- Needs a person: Narrator, reboot-only checks, anything judged by eye.

## Owner's VM (`kc9k-vm` on the test machine)

- Ubuntu 24.04, Cinnamon autologin on X11 as `tester` (passwordless sudo),
  kc9k + LibreWolf installed, repo `~/open-turkey-9000`; ssh alias
  `kc9k-vm` (key `~/.ssh/kc9k-vm`), IP 192.168.122.80, snapshot `clean`
  (shut off).
- `ci/monk_live_test.sh host [vm]` (R-MONK-2, VM only): ssh goes with the
  cut, so the VM checks itself (`inside 1`, oneshot unit runs `inside 2`
  after restart) into `/var/lib/kc9k-monk-live`; host reads `verdict`.
  `MONK_MINUTES` (default 6).
- `ci/<name>.sh host [domain] [ssh-alias]` via `ci/vm_check_lib.sh`: revert
  to `clean`, rsync tree, build and install `.deb` (`9000.N~vmcheck`), run
  `inside 1` as root via `systemd-run` (outlives ssh and lockdown's loss of
  sudo), wait through restarts for `/var/lib/kc9k-vmcheck-*/verdict`, print
  PASS/FAIL, revert shut off. kc9k runs as the VM user via the wrapper
  (strict mode records `SUDO_USER`); root status checks call `kc9k-bin`.
  `ci/strict_pam_vm_test.sh` (2.6: root password, account `kc9kother`, root
  login/su on a pty, one boot with `kc9k.nofreeze`), `ci/tamper_vm_test.sh`
  (2.9: installer guard via dpkg and `install.sh`, needs Go in the VM;
  LightDM `HasGuestAccount`). Both never run yet.
- `ci/deb_vm_test.sh host [domain] [ssh-alias]` (2.11; never run; needs a
  clean Mint VM, default `kc9k-mint`, snapshot `clean`: no KC9K, no
  LibreWolf, user with passwordless sudo and ssh key): reverts, builds a
  release `.deb` (`9000.1~vmcheck`), runs `ci/deb_install_check.sh` in the
  VM through the user's sudo (apt install), reverts shut off. `MANUAL=1`:
  the `.deb` goes to `~/Downloads`, a person double-clicks it and presses
  Install, then only the checks run.

## CI runners (GitHub)

- Time: Windows GUI full run ~35 min (one job, ~130 UI tests serially,
  then the display-scale passes); a `filter` run 5–10 min. Iterate with
  `filter`, full run once at the end (CLAUDE.md "CI and long tasks").
  Faster split: ROADMAP `W55`.
- `windows-latest` quirks (handled in `GuiApp` and `windows-gui.yml`;
  details in [gui-windows](gui-windows.md)):
  - screen is 1024×768 (the workflow sets 1920×1080);
  - a console window steals focus: bring the GUI forward (`Front()`);
  - elements out of view have no rectangle: scroll them in (`Show()`);
  - FlaUI's mouse moves are not drags: `GuiApp.Drag`;
  - a fresh database shows the extension-missing dialog first.

## Other commands

- `ci/integration.sh` (CI `Integration tests`, after `install.sh`, on
  `ubuntu-latest` = Ubuntu 26.04: uutils coreutils, sudo-rs; changes the
  clock, firewall, hosts: never on a computer in use): `kc9k` runs
  through `$KC9K_BIN` under a 120 s `timeout`; the app checks run a sleeper
  built from Go (uutils' multicall `sleep`, Ubuntu 25.10+, refuses to run
  under another name); on GitHub the runner's folders (`/home/runner/…`)
  are made strict mode exempt folders first, else strict mode closes the
  runner. Locking commands take `--yes` (R-LOCK-12); protections are set
  by the test, never assumed (R-LOCK-10: strict mode's own lock, tamper
  `clock`, `--lock-lead`); the firewall is checked as nftables' `inet
  kc9k` table where `nft` exists, else the iptables `KC9K` jump.
- `ci/deb_install_check.sh <deb> [version]` (root, disposable machine: CI
  job `deb` and `linux-release.yml` on `ubuntu-24.04`): installs the `.deb`
  with apt, checks what `install.sh` sets up (daemon, sudo rule, wrapper,
  GUI, extension policies), waits for the daemon's LibreWolf
  (`LIBREWOLF_MINUTES`, default 15), removes.

- Timing checks on shared runners (A17): a time judged by its fastest of
  several rounds (a pause hits one round, too much work every round);
  servers in unit tests bind port 0 (fixed ports in 32768–60999 get
  taken: Android `RulesServerTest`).
- `test.yml` runs, before KC9K is installed: `ci/background_test.js`,
  `ci/search_results_test.js` (Playwright Chromium from npm, `NODE_PATH`),
  `ci/gui_account_smoke.py` (fake cloud `ci/cloudfake`), then LibreWolf
  with `ci/fake_daemon.py` on 47613: `ci/search_results_librewolf_test.py`
  (waits ≤ 10 s for each state, not a fixed sleep), `ci/dark_browser_test.py`.
- Extension, Node: `node ci/match_test.js` (huge page: each step's fastest
  of 3 timed rounds ≤ 16 ms), `node ci/background_test.js`
  (APIs stubbed: frame reports, ad trigger), `node
  ci/android_background_test.js`; Playwright Chromium (cloud only): `node
  ci/content_harness_test.js` (principle 9: each extension callback ≤ 16 ms
  on a huge page; whole-page delay printed, not checked).
- GUI smoke tests: own binary, wrapper with `KC9K_DB` = scratch file, load
  `gui/kc9k-gui.py` as module with `CLI` replaced, other application ID,
  never `show()`; normal user only. `ci/gui_click_everything_smoke.py`
  clicks every button/switch of every page, tab and ⋯ menu, dialogs
  cancelled then answered OK (greyed-out not pressed); fails on handler
  exception or unreadable command. Others `ci/gui_*_smoke.py` (`xvfb-run -a`).
  Cloud container (root, Python without `gi`): `apt-get install -y
  gir1.2-gtk-3.0 python3-gi-cairo python3-cairo dbus-x11`, a user
  (`useradd -m t`), then per test `runuser -u t -- env HOME=/home/t
  PATH=/usr/local/go/bin:/usr/bin:/bin xvfb-run -a dbus-run-session --
  /usr/bin/python3.12 ci/gui_X_smoke.py` (the Python `gi` was built for).
- Weakening audit: `cli/weakening_audit_test.go`; add every new weakening command.
- Race: `CGO_ENABLED=1 go test -race -timeout 30m ./internal/...`
  (`internal/cli` ~14 min). Spinning disk: `TMPDIR=/dev/shm/…` (else ~16 min).
- X11 tests need `/tmp/.X11-unix` root-owned and sticky (`trustedX11Socket`
  refuses else). Fresh container: as root `chown root:root /tmp/.X11-unix
  && chmod 1777 /tmp/.X11-unix`.
- Text lock window: `KC9K_TEST_PYTHON=<python3 with GTK 3> xvfb-run -a go
  test -run Window ./internal/lock/` (xdotool, xterm).
- Root tests (daemon stopped): `go test -tags roottest -run Chrome
  ./internal/enforce` (Playwright Chromium in `/opt/pw-browsers/`);
  `sudo KC9K_TEST_APPIMAGE=<AppImage> go test -tags roottest -run AppImage
  ./internal/blocker` (AppImage browser never supported).
- `./try-challenge.sh [len]`: unlock challenge alone.
