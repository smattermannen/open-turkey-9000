# KC9K account and cross-device sync

- Covers: `internal/cloud/` (client, `cloudtest/` fake), `cli/sync.go`, `db/syncclear.go`, `cloud/supabase/schema.sql`
- SPEC: §4.14, principle 6, §6 2026-11-23 (Supabase)
- Connects to: [db-project](db-project.md) (the setup file is a backup, `buildProject`), [locks](locks.md), [windows](windows.md) (commands run in the service)

## Cloud (`internal/cloud/`)

- Supabase over plain HTTPS (`net/http`, no SDK). Headers `apikey` (anon
  key) and `Authorization: Bearer` (session's access token, else the anon
  key). Auth: `/auth/v1/signup`, `/auth/v1/token?grant_type=password|
  refresh_token`, `/auth/v1/logout`. `Fresh` refreshes a session within
  60 s of expiry; a 4xx refresh = `ErrSignedOut` (session dropped).
- One setup per account: Storage `setups/<user id>/setup.json.gz`
  (`POST /storage/v1/object/…`, `X-Upsert: true`; read from
  `/object/authenticated/…`), plus row `public.setups` (upload time,
  device, format = `projectVersion`, size) upserted through PostgREST
  (`on_conflict=user_id`). Missing object/row = `ErrNoUpload`.
- `schema.sql`: table, private bucket (gzip, 50 MB), row-level security on
  both so an account touches only its own; run once in the SQL editor.
- `cloudtest.New()`: in-memory Auth/Storage/rows with the same RLS checks;
  `Expire()` stales access tokens.

## CLI (`cli/sync.go`)

- Cloud address: build vars `cloudURL`/`cloudAnonKey` (`-ldflags -X`,
  empty until the owner's project exists: "isn't set up in this build
  yet"); hidden `account server <url> <key>` (settings `cloud.url`,
  `cloud.key`; drops the session).
- Session JSON in setting `cloud.session` (the DB is root/SYSTEM only).
- `account signup|login <email>` (password via `readPasswordStdin`; sign-up
  needing email confirmation says so), `logout` (ends it on the cloud too,
  forgets it anyway), `status [--json]` (`available`, `email`,
  `last_upload`, `error`).
- `sync upload` = `buildProject` backup → `syncSetup` → gzip → `Put`.
  `syncSetup` drops §4.14's device-specific items: places, block places
  and affected users, modes' places and wallpapers, wallpapers and
  circadian wallpapers, statistics, setting locks `strict`, `no-escape`,
  `firmware-lock`, app entries that are paths or `sha256:` fingerprints
  (`deviceApp`). Kind `"sync"`.
- `sync download [--dry-run] [--yes]`: `Latest` + `Get`, gunzip,
  `syncSetup` again (a file never brings device items). Refused (via
  `refused()`, naming them) while a lock runs here that didn't come by
  Download all from this account unchanged (`localLocks`: each running
  lock of `exportBackupExtras` as JSON, compared with setting
  `sync.locks` = account + prints written after each download; ROADMAP §5
  DECIDED 2026-11-23). Then a check pass (`importProject` dry,
  `replacing`, output discarded by `quietly`) validates the file and
  collects its locks; confirm the replace (`confirmReplace`: typed yes or
  `--yes`), monk ack, custom actions, locks (R-LOCK-12); save device items
  by name (`saveDeviceItems`: place rules, affected users, device app
  entries, modes' places and wallpapers, circadian wallpapers);
  `db.ClearForSync` (one transaction: blocks, clusters, templates,
  presets, saved schedules, modes but monk's row, freezes, hotkeys,
  setting locks but strict/no-escape/firmware-lock, synced settings, the
  tamper protection settings and grace among them);
  `importProject(confirmed, oneToOne)` (tamper settings written as the
  file has them, off too; `TestTamperSettingsInProjectAndSync`); `restore`; remember the locks; apply. Not
  one transaction end to end: a failure after the clear leaves part of the
  file (the check pass makes that unlikely; said in the error).
- GTK: Account page ([gui](gui.md)). Not built: Windows GUI, Android,
  the real project.
- Tests: `cloud/cloud_test.go`, `cli/sync_test.go` (two devices on two
  databases, one fake), `weakening_audit_test.go` (download over a lock
  set here).
