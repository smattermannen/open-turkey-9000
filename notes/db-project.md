# Database, project files, backup

- Covers: `internal/db/` (`db.go`, `features.go`, `pending.go`, `clock.go`), `cli/project.go`, `projectbackup.go`
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
  `weblists.go`, `pending.go`.

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **Project**: holds blocks (lists, apps, titles, schedules, schedule lock,
  social, allowance, Pomodoro, breaks, ladders), clusters, browser and
  behaviour settings, global ladder, freeze schedules, list images; backup
  adds places and place rules, and (`cli/projectbackup.go`) running locks,
  password hashes, hotkeys, statistics: import adds only where nothing is
  here, confirms the locks, starts blocks that come with running locks. Clashing block → `name-2`; existing
  clusters/places kept; browser settings only on; ladder replaced only by a
  stricter one. `--dry-run` lists additions and locks.
