# Handoff: branch `monk` (R-MONK-1, R-MONK-2), stopped mid-work 2026-11-14

Work in `~/kc9k-monk` on the second computer (test machine). Not merged into main.
`CGO_ENABLED=0 go build ./...` passes (also `-tags devmaster`). **`go test` does not compile yet** (see 1).

## Done (committed)
- `internal/db/monk.go`: monk mode is `monk.until` (trusted time) + `monk.net` (saved network state).
  `ExtendMonk` (only grows), `EndMonkIfOver`, `SaveMonkNetIfNone` (first state wins), `ForgetMonkNetIfOff`,
  `migrateMonkMode` (removes the `monk-mode` block once, a running one keeps its end). Tests: `monk_test.go` (pass).
- `internal/blocker/firewall.go`: `ApplyFirewallCut`, `IsFirewallCut` (nft `inet kc9k` input/output/forward, iptables fallback). Tests pass.
- `internal/blocker/netcut.go`: `WifiRadio`, `SetWifiRadio` (nmcli), `EthernetLinks` (/sys/class/net), `SetLink`. Test passes.
- `internal/enforce/monk.go`: `StartMonk`, `enforceMonk` (called in `Tick`), `cutNetwork`, `restoreNetwork`, `monkBlock`
  (in-memory `*.*` + `file://*`, list id `MonkListID`), `withMonk`, `MonkEnd`/`MonkOn`. Device effects are replaceable vars.
- Wired in: `Tick`/`apply` (firewall stays cut, re-resolve 45 s after restore), `BuildRules` (+ `rules.Ends["Monk mode"]`),
  `findActiveList` (monk hits are global events), `ExtensionRequiredNow`, `keepWifiOn` (off during monk),
  `BlocksLocked` (monk counts as a lock), uninstall refused, `master` ends it.
- CLI: every `enforce.MonkBlock` special case and `refuseMonkEdit` removed; `kc9k monk` without `--for` shows the state;
  `kc9k status` has a monk line; gui-state has `monk_seconds` instead of `monk_block`.

## Left, in order
1. Fix tests that still use `MonkBlock`: `internal/enforce/consequence_test.go` (`TestStartMonk`) and
   `internal/cli/settings_test.go` (`TestMonkBlockIsSystemOwned`). Replace with `TestR_MONK_1_*` / `TestR_MONK_2_*`.
   **Add a `TestMain` in `internal/enforce` that replaces the network vars in monk.go with fakes first**:
   enforce tests don't set `KC9K_DB`, so `StartMonk` would cut the real network.
   Planned cases: no block exists after `StartMonk`; rules carry `*.*`; started by `Run` and by a ladder step;
   three cuts + state saved once; put back when switched on; restore at the end, after a restart, after a crash
   (`until` 0 with a saved state); Wi-Fi that was off stays off; `monkDeviceless` never cuts.
2. GUI (`gui/kc9k-gui.py`): lines 883-888, 1062, 1731, 2529-2533 still use `monk_block` / mention the block.
   Show `monk_seconds` on the Dashboard; fix the texts.
3. `ci/actions_test.py` lines 261-279: check `kc9k monk` output instead of `block info monk-mode`.
4. `ci/monk_live_test.sh` and the live test (take `/tmp/kc9k-live.lock`, install, `systemd-run --on-active=20s`), then the
   reboot test (ask the owner first). Nothing was installed or run live; the lock was never taken.
5. ROADMAP rows (lines ~40, 138, 245), README.md, CLAUDE.md (Consequences paragraph: monk is no block) are **not updated yet**.
6. `git fetch origin && git merge origin/main`, build, test, merge into main, push.

## Open questions for the owner
- Pages blocked by monk mode are logged as global `page` events (no block). OK, or should they not count for bypass detection?
- No NetworkManager: the Wi-Fi radio is left alone (only Ethernet + firewall). Add an `rfkill` fallback?
- `kc9k master` (devmaster builds) ends monk mode, as it lifted the old block. Keep?
- Commits here are authored `Claude <noreply@anthropic.com>` via `-c` flags (no git identity on this machine).
