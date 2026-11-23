# Allowance, Pomodoro, breaks, pauses, statistics

- Covers: `internal/quota/`, `db/quota.go`, `usage.go`, `pomodoro.go`, `pauses.go`, `breaks.go`, `enforce/quota.go`, `pomodoro.go`, `breaks.go`, `soon.go`, `stats.go`, `cli/quota.go`, `pomodoro.go`, `breaks.go`, `stats.go`, `usecounting.go`
- SPEC: §3.6, §3.7, §4.7, §4.9, R-BREAK-1–2, R-SCHED-3
- Connects to: [locks](locks.md), [extension](extension.md) (use, toolbar), [apps-titles](apps-titles.md) (app use), [gui](gui.md)

## Allowance (`internal/quota/`, `enforce/quota.go`)

- `kc9k allowance set|start|end|remove` (`--per
  day|week|month|rolling|window`, `--refill HH:MM`, `--sessions N`; unset
  flags keep values). `set` switches the block on; nothing else does.
- Use (`Spend` only on use) = block's app focused (daemon sampler), or its
  site (address/title lists, not text) in the focused tab with input < 3
  min, or playing (even muted) in any tab (extension `/usage` from
  `rules.quota`). Without LibreWolf, website sessions spend wall time.
  Session with websites closes extension-less browsers.
- Protects nothing by itself (R-LOCK-1, `held`): unlocked, every change at
  once; locked, only less time or fewer sessions; removing always free.
- A lock, window or Pomodoro work period closes a session. Day follows the
  system clock.

## Pomodoro

`pomodoro set B W R [--long D --every N]`, `off` free; refused while
locked. Locks stay; `PomodoroWorking` closes allowance sessions in work
periods; `quota.Cycle` long break.

## Pauses and breaks (`db/pauses.go`, `db/breaks.go`, `cli/breaks.go`)

- `block_pauses` until trusted time; `pausedFor` in `enabledBlocks`.
- Reward (`break reward`, alias `allowance`) earned in `earnBreaks` while
  block on and computer in use (input < 3 min, screen unlocked). `--keep
  one`, `window|day|week|month` (accumulates until it ends) or length ≥ 1
  h (`Breaks.Keep` = seconds; each hour's part in `Chunks` expires that
  long after; a break takes the oldest, `break end` returns the newest,
  `OnBreak`). No cap. Period ↔ length = weakening (`KeepLooser`). More
  breaks weaken (refused while locked); fewer free.
- A break lifts a locked block; it returns with locks; edits stay refused.
- `break text`/`relock`: `unlock` pauses instead (second unlock ends).
- `rules.soon` → toolbar badge/popup (`timeLeftFor`); `/add` adds the tab's
  site to Blocked (refused behind a blocks access password).

## Statistics

Foreground app time (daemon, needs X11) and site time (extension, every 30
s); not counted when locked or after 3 min idle (`idleLimit`); kept a
year. `stats --json` filtered by `--search`/`--block` (block password,
§3.11); `--days N` or `--from D [--to D]`. GUI: `DailyChart.set_stacks`
(top names stacked, rest Other), `assign_series` (stable palette slot),
`SERIES_LIGHT`/`SERIES_DARK`; dashboard `stats_rows`, page `stats_page_rows`.
