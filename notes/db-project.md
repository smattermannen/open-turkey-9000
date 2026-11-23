# Database, project files, backup

- Covers: `internal/db/` (`db.go`, `openlock.go`, `features.go`, `pending.go`, `clock.go`), `cli/project.go`, `projectbackup.go`, `projectlinux.go`, `blocker/linuxonly.go`, `testdata/`
- SPEC: §4.2 export/import, §4.10 Project
- Connects to: every note (each table belongs to an area); [lists](lists.md) and [automation](automation.md) for imports

## DB (`internal/db/`)

SQLite `/var/lib/kc9k/kc9k.db`, WAL, busy_timeout and foreign_keys per
connection via `_pragma`. `KC9K_DB` overrides (tests only). Schema setup in
`OpenDB` runs under an flock on `<db>.setup-lock` (`openlock.go`; ≤ 30 s):
on a fresh install daemon and `install-librewolf` open at once and
busy_timeout doesn't cover concurrent column adds
(`TestFreshDatabaseOpenedByManyAtOnce`). `DB.entries` caches list entries
([lists](lists.md)).

## Project files (`cli/project.go`)

- Versioned JSON; import validates all, then only adds (existing
  untouched, options only strengthened); `--dry-run` lists additions and
  locks. Schedules/modes: `cli/projectmodes.go` (`copy_of`, `in_mode`,
  `applied_schedules`).
- Holds blocks (lists, apps, titles, schedules, schedule lock, social,
  allowance, Pomodoro, breaks, ladders), browser and behaviour settings,
  global ladder, freeze schedules, list images. Clashing block → `name-2`;
  browser settings only on; ladder replaced only by a stricter one.
- Presets (`exportPresets`): always in a backup; in `--share` only with
  `--presets` (terminal asks on stderr; GUI Share… asks). Import: same
  name+content skipped, taken name → `name-2` (`freeName`).
- Backup (`cli/projectbackup.go`) adds places and place rules (existing
  kept), running locks (range, queued, re-lock config), password hashes,
  hotkeys, statistics: import adds only where nothing is here, confirms
  the locks, starts blocks that come with running locks.

## Linux project on Windows (W56)

`projectText` strips a UTF-8 BOM. On Windows (`projectOS` seam)
`sayLinuxOnly` lists `linuxOnlyParts` as "Not used on Windows" (dry run and
import); `checkBlock` skips `ValidAppEntry` for them, stored as is.
Linux-only = `blocker.LinuxOnlyApp`/`LinuxOnlyCommand` (`linuxonly.go`);
Windows app matcher skips them (`newAppMatcher`). Fixture
`testdata/linux-project.json` + `-crlf`/`-bom` copies (`.gitattributes`
`-text`). Tests `projectlinux_test.go`, `projectlinux_windows_test.go`.
