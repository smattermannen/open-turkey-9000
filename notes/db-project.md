# Database, project files, backup

- Covers: `internal/db/` (`db.go`, `openlock.go`, `features.go`, `pending.go`, `clock.go`), `cli/project.go`, `projectbackup.go`
- SPEC: §4.2 export/import, §4.10 Project
- Connects to: every note (each table belongs to an area); [lists](lists.md) and [automation](automation.md) for imports

## Code

- **Project** (`cli/project.go`): versioned JSON; import validates all, then
  only adds (existing untouched, options only strengthened). Schedules and
  modes: `cli/projectmodes.go` (`copy_of` on a block's window, `in_mode`,
  `applied_schedules`).
- **DB** (`internal/db/`): SQLite `/var/lib/kc9k/kc9k.db`, WAL, busy_timeout
  and foreign_keys per connection via `_pragma`. `KC9K_DB` overrides (tests
  only). `db.go`, `features.go` (schedules, time locks, upgrades),
  `weblists.go`, `pending.go`. Schema setup in `OpenDB` (WAL switch,
  `CREATE … IF NOT EXISTS`, upgrades) runs under an flock on
  `<db>.setup-lock` (`openlock.go`; waits ≤ 30 s, then unlocked): on a
  fresh install the daemon and `install-librewolf` create the DB at once,
  and busy_timeout doesn't cover a stale-snapshot write or two processes
  adding one column (`TestFreshDatabaseOpenedByManyAtOnce`).

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **Project**: holds blocks (lists, apps, titles, schedules, schedule lock,
  social, allowance, Pomodoro, breaks, ladders), browser and
  behaviour settings, global ladder, freeze schedules, list images. Presets
  of every type (`exportPresets`: `clusters`, `templates`, `presets` table
  rows) go in a backup always and in a `--share` only by choice
  (`--presets`; without it a terminal asks on stderr, elsewhere none; the
  GUI's Share… asks). Import adds presets like blocks: one with the same
  name and content is skipped, a different one whose name is taken comes
  in as `name-2` (`freeName`, `importPresets`). Backup
  adds places and place rules, and (`cli/projectbackup.go`) running locks
  (with range lock, queued locks and the configuration re-lock restores),
  password hashes, hotkeys, statistics: import adds only where nothing is
  here, confirms the locks, starts blocks that come with running locks. Clashing block → `name-2`; existing
  places kept; browser settings only on; ladder replaced only by a
  stricter one. `--dry-run` lists additions and locks.
