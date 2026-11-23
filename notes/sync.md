# KC9K account and cross-device sync

- Covers: `internal/cloud/` (client, `cloudtest/` fake), `cli/sync.go`, `cloud/supabase/schema.sql`
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
- Not built: `sync download` (replace, keep device items, not refused by
  locks: R-WEAK-1 exception), GUIs, Android.
- Tests: `cloud/cloud_test.go`, `cli/sync_test.go`.
