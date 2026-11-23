# KC9K account, cross-device sync, accountability partner

- Covers: `internal/cloud/` (client, `partner.go`, `cloudtest/` fake), `cli/sync.go`, `cli/partner.go`, `db/syncclear.go`, `db/partner.go`, `cloud/supabase/schema.sql`, `ci/cloudfake/`
- SPEC: §4.14, §4.15, principle 6, §6 2026-11-23 (Supabase)
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

## Accountability partner (`cli/partner.go`, `cloud/partner.go`, §4.15)

- Cloud tables (schema.sql, RLS): `partners` (owner, partner_email,
  partner_id, invited/active; the user reads own, the invited email reads
  theirs, only that email accepts as itself; either side deletes),
  `partner_messages` (report/alert/password; sent by the user; read by
  the user except passwords, and by active partners: `active_partner()`),
  `unlock_requests` (pending → approved/denied by a partner → used by the
  user, secret cleared), `heartbeats` (user, device, seen_at). Email: the
  cloud's job (owner step), `cloud.Mailer`; `cloudtest` records `Mail`.
- Local: `db/partner.go` outbox `partner_outbox` (≤ 500), queued only
  while setting `partner.active` (refreshed from the cloud by `partner`,
  every 30 min by the daemon); `PartnerAlert` drops the same alert within
  10 min. Alerts: `stop`, `unlock` (and an unlock break), `block remove`,
  bypass prevention step (`enforce/nepsis.go`), every `EventTamper`
  (`db.RecordEventFor`), `prepare-uninstall` (sent at once).
- `partnerFlush`: heartbeat (10 min), partners (30 min), report when due
  (`partner.reports` daily/weekly(default)/off, `partner.report.last`;
  first report a period after the first send; `partnerReport`: top sites
  and apps, times blocked, refused attempts and tamper events), outbox.
  Daemon: `daemon.EveryMinute` (`partnerTick`, quiet when signed out).
- Held passwords (`--partner-holds` on `start`, `password set`):
  `partnerHeldPassword` makes 16 letters, sends them (kind password,
  target `block:<name>` / `settings`) before locking, needs an active
  partner. `partner request` → partner `approve` (the newest password the
  user sent for that target) / `deny` → `partner unlock`: block via
  `unlock` with it on stdin (settings password asked first), settings
  password removed if it matches; request marked used, alert.
- Weakening (`gatePartnerWeakening`: settings password, `refused()` while
  `AnyLock`): `partner remove` of an active partner (told first),
  `partner reports` less often. Free: invite, accept, leave, more reports.
- GTK: Account page section ([gui](gui.md)). Tests:
  `TestAccountabilityPartner`, weakening audit, `ci/gui_account_smoke.py`.
