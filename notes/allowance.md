# Allowance, Pomodoro, breaks, pauses, statistics

- Covers: `internal/quota/`, `db/quota.go`, `usage.go`, `pomodoro.go`, `pauses.go`, `breaks.go`, `enforce/quota.go`, `pomodoro.go`, `breaks.go`, `soon.go`, `stats.go`, `cli/quota.go`, `pomodoro.go`, `breaks.go`, `stats.go`, `usecounting.go`
- SPEC: §3.6, §3.7, §4.7, §4.9, R-BREAK-1–2, R-SCHED-3
- Connects to: [locks](locks.md), [extension](extension.md) (use, toolbar), [apps-titles](apps-titles.md) (app use), [gui](gui.md)

## Code

- **Allowance** (`internal/quota/`, `enforce/quota.go`): `Spend` only on
  use; daemon sampler adds apps; extension reports focused/playing seconds
  from `rules.quota` (`/usage`). Without LibreWolf, website sessions spend
  wall time. `quota.Cycle` long break; `PomodoroWorking` keeps sessions
  closed in work periods.
- **Pauses and breaks** (`db/pauses.go`, `enforce/soon.go`, `db/breaks.go`,
  `cli/breaks.go`): `block_pauses` until trusted time; `pausedFor` checked
  by `enabledBlocks`. `rules.soon` feeds toolbar badge/popup (`timeLeftFor`,
  `popup.js`); `/add` adds the tab's site to Blocked (refused with a blocks
  access password). Reward (`break reward`, alias `allowance`) earned in `earnBreaks`;
  `break text`/`relock` makes `unlock` pause instead (second unlock ends).
- **Statistics**: foreground app time (daemon) and site time (extension);
  not counted when locked or after 3 min idle (`idleLimit`); kept a year. `stats --json`
  rows are filtered by `--search`/`--block` (block password, §3.11); time
  frame `--days N` or `--from D [--to D]` (GUI Custom…, `DateButton`,
  `stats_range_problem`). GUI:
  `DailyChart.set_stacks` (top names stacked bottom up, the rest Other),
  `Legend`, `assign_series` (a name keeps its palette slot, never two shown
  in one), `SERIES_LIGHT`/`SERIES_DARK`; the dashboard reads `stats_rows`
  (unfiltered), the page `stats_page_rows`. `ci/gui_stats_smoke.py`.

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **Allowance** (`kc9k allowance set|start|end|remove`; `--per
  day|week|month|rolling|window`, `--refill HH:MM`, `--sessions N`; flags
  not given keep what it has): use = block's app focused, or its site
  (address/title lists, not text) in the focused tab with input < 3 min, or
  playing (even muted) in any tab. Session with websites closes
  extension-less browsers. Protects nothing by itself (R-LOCK-1, `held`):
  unlocked, every change at once; locked, only less time or fewer sessions;
  removing always free. `set` switches the block on; nothing else switches
  it. A lock, window or Pomodoro work period closes a session. Day follows
  the system clock.
- **Pomodoro** (`pomodoro set B W R [--long D --every N]`, `off` free;
  refused while locked): locks stay; work periods close allowance sessions.
- **Breaks**: reward earned while block on and computer in use (input < 3
  min, screen unlocked); `--keep one` (up to one reward), `window|day|week|
  month` (accumulates until it ends) or a length ≥ 1 h, no upper limit
  (`Breaks.Keep` = seconds; each hour's part in `Chunks` expires that long
  after its hour; a break takes the oldest, `break end` gives back the
  newest it took, `OnBreak`). No cap on the amount. Period ↔ length counts
  as weakening (`KeepLooser`). A break lifts a locked
  block; it returns with locks; edits stay refused. More breaks weaken
  (refused while locked); fewer always free.
- **Statistics**: app time needs X11; extension posts site time every 30 s.
