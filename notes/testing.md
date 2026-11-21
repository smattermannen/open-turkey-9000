# Testing

- Covers: `ci/`, `*_test.go`, `cmd/try-challenge/`, `try-challenge.sh`, `ci/autopilot.sh`, `ci/autopilot-team.sh`, `ci/autopilot-desktop.sh`
- SPEC: §5 (tests name the rule they prove)
- Connects to: every note; first read [locks](locks.md) before a test that locks anything

## Commands

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
- `ci/autopilot.sh`: unattended loop of `claude -p` sessions working down
  `ROADMAP.md`. The script, not the session, picks the item: the next
  Index row whose Needs match `AUTOPILOT_NEEDS` (default `agent`), not in
  `skip.txt` and not claimed by another worker (`claims.txt`, dead
  workers' claims dropped). The prompt names the other workers' items and
  notes to keep out of. Landing is one worker at a time (`land.lock`):
  rebase onto the shared hub (`hub.git`; a short session resolves
  conflicts), gate (build, vet, tests, GUI smoke, SPEC.md untouched,
  ROADMAP.md changed), fast-forward `autopilot`. STUCK or failing twice
  → `skip.txt`. A usage limit retries the round after the reset (the
  stream's `rate_limit_event` lines are not one); a session that ends
  without a result logs why (`session_error`);
  runs until `STOP` (`AUTOPILOT_ROUNDS=N`: N item rounds). State in
  `~/.local/state/kc9k-autopilot/`. Run as a non-sudo user, never on the
  test laptop, unless in box mode.
- Box mode (`AUTOPILOT_BOX=1`, on the test machine): sessions may use
  sudo, `install.sh`, `kc9k-bin`, the daemon, real browsers and X11 (only
  `git push` stays blocked: the script lands after the gate); Needs
  default `agent|box`; the gate adds every `roottest` test via
  `sudo -n go test -tags roottest` (`root_tests`). Needs passwordless
  sudo (checked at start). Several box workers may run (`autopilot-team.sh`);
  only one holds a box-only item (Needs without `agent`) at a time. The
  prompt sends network cuts, logout, shutdown, reboot and freezing to a VM
  (STUCK when none is set) and asks the session to remove its test blocks.
- `ci/autopilot-desktop.sh` (the owner's desktop, one command): box mode;
  passwordless sudo set up if missing; accounts are the logged-in
  `~/.claude` and `~/.claude-*` (`.credentials.json`), else it makes
  `~/.claude-a`/`-b` and opens `claude` in a window for `/login`
  (`AUTOPILOT_ACCOUNTS` overrides); `AUTOPILOT_VM=kc9k-vm` when its
  snapshot exists (empty `AUTOPILOT_VM`: none). Starts `autopilot-team.sh`
  under `setsid` (session id in `desktop.sid`) and opens one terminal
  window per worker running `watch <worker>`: the worker's `out.txt` and
  its newest session log rendered (text, tool calls, errors, result);
  closing a window stops nothing. `status`; `stop` sets `STOP`;
  `stop --now` kills the whole session (`pkill -s`). Push and merge to
  main stay on.
- VM (`AUTOPILOT_VM=<libvirt domain>`, box mode only): Needs default
  `agent|box|vm`. Checked at start: the snapshot (`AUTOPILOT_VM_SNAPSHOT`,
  default `clean`) exists on `AUTOPILOT_VM_URI` (default
  `qemu:///system`). Before a round on an item whose Needs has `vm`,
  `vm_reset start` reverts the VM running and waits for ssh
  (`AUTOPILOT_VM_SSH`); after it, `vm_reset` reverts it again. The prompt
  gives the session the VM (rsync the tree, build, install, test there).
  One vm item at a time; a vm item does not hold up box items.
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
- Cycles and the planner (`planner_round`): each cycle starts with one
  worker (claim `PLANNER`) running a planner session
  (`AUTOPILOT_PLAN_MODEL`/`_EFFORT`, default opus xhigh), told why it runs
  and what landed since the last `Planner:` commit; it orders, splits,
  marks done and adds Index rows (top `AUTOPILOT_CYCLE` rows, default 4,
  are the next batch, taken by the workers in parallel; the others wait
  while a planner runs). The gate allows
  only ROADMAP.md, then review and land as any round. A new cycle starts
  when `cycle.txt` (rounds ended by all workers since the last planner,
  `cycle_bump`) reaches `AUTOPILOT_CYCLE`, at NONE, or after
  `AUTOPILOT_MAX_FAILS` failed rounds in a row (pause `IDLE_WAIT`, the
  planner is told to re-scope). `STATUS: FINISHED`, or two NONE plans in a
  row: `finished_wait` sleeps `AUTOPILOT_FINISHED_WAIT` (6h) and plans
  again; `AUTOPILOT_END_WHEN_FINISHED=1` ends instead. `AUTOPILOT_PLAN=0`:
  no planner, ends at NONE. `AUTOPILOT_ROUNDS` defaults to 0 (until `STOP`).
- Notes gate (`notes_check`, item rounds): each changed code file
  (`.go .js .py .sh .json .service .desktop`, `Makefile`; not `*_test.*`,
  `testdata/`, `.md`) needs a changed note whose Covers line names it
  (basename, directory token or glob); a file no note covers needs some
  note changed. `AUTOPILOT_NOTES_GATE=0` turns it off.
- Following main (`follow_main`, under `land.lock`, each round): GitHub's
  `$AUTOPILOT_FROM` into the hub's `autopilot`, fast-forward or a merge; a
  conflict is logged and left for the owner. `AUTOPILOT_FOLLOW=0` turns it
  off. `GIT_TERMINAL_PROMPT=0`: a missing GitHub login fails, never waits.
- Resolver round (`resolver_round`): when the open questions (§5 entries
  not `DECIDED`/`FOR THE OWNER`, SPEC §7, `skip.txt`) differ from the
  last run's (`resolver-seen.txt`), one worker (claim `RESOLVER`) runs a
  session on `RESOLVER_MODEL`, no subagents. It decides into SPEC.md
  tagged `(By Opus)` or marks `FOR THE OWNER`; `UNSKIP:` IDs leave
  `skip.txt`. The gate: only SPEC.md and ROADMAP.md, every SPEC hunk
  tagged, nothing above SPEC's Index, no untagged §6 line removed
  (`spec_check`), every turn of the session and of a conflict session on
  that model (`model_check`). `AUTOPILOT_RESOLVE=0` turns it off.
- Lead model and effort per item (`pick_lead`): a read-only triage
  session (`AUTOPILOT_TRIAGE_MODEL`, effort low) gives size S/M/L and
  risk low/high, cached in `triage.txt` until the item's text changes
  (unsure: M high). Tier 1 small and low risk, 2 medium or high risk, 3
  large (`AUTOPILOT_TIER1..3`, "model effort"); each earlier failure one
  tier up; `AUTOPILOT_MODEL`/`AUTOPILOT_EFFORT` force one. The prompt
  sets subagents' model and effort by job (search haiku, edits sonnet,
  plan and the review before committing opus). Each round logs the
  result's `modelUsage` (lead and subagents) and cost. The resolver runs
  at `AUTOPILOT_RESOLVER_EFFORT` (default `max`); `model_check` reads
  `modelUsage` too.
- Merge review (`review`): after the gate (item, resolver and planner rounds), a `RESOLVER_MODEL` session
  with no edit tools reads `base..HEAD` and writes `MERGE: yes|no` and
  reasons; `model_check` on its log. Yes: `land` rebases (a conflict
  session's result is reviewed again), gates again only if `$BRANCH`
  moved, and fast-forwards `autopilot` and `main` (hub and GitHub;
  `AUTOPILOT_PUSH=0`: hub only; `gh_push` retries network errors). No: a failed try with the reasons. `sync_github`
  first builds on the owner's pushes to `main`; a conflict pauses
  merging to `main` for that landing. `AUTOPILOT_MERGE_MAIN=0` keeps
  `main` untouched. The clone gets a local git identity when the user
  has none (rebases commit).
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
  whether a blocked word was painted). Needs a `TAGS=devmaster` install
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
