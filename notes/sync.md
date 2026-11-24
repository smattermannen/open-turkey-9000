# KC9K account and cross-device sync

- Covers: `internal/cloud/` (client, `cloudtest/` fake), `cmd/fake-cloud`, `cli/sync.go`, `db/syncclear.go`, `cloud/supabase/schema.sql`, `gui/kc9k_gui/win_account.py`, `ci/gui_account_smoke.py`
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
  setting locks but strict/no-escape/firmware-lock, synced settings, every
  `tamper.*` setting included; `tamper.clock` set to a fresh install's
  "off", else a reopen's `migrateTamperClock` turns it on over the file);
  `importProject(confirmed)`; `restore`; remember the locks; apply. Not
  one transaction end to end: a failure after the clear leaves part of the
  file (the check pass makes that unlikely; said in the error).
- Access area Settings (`commandAreas` `account`, `sync`).
- GTK Account page (`gui/kc9k_gui/win_account.py`, sidebar "Account",
  covered by Settings' access password): status from `account status
  --json` when shown (in the background); Sign up… (email, password twice,
  ≥ 8), Log in…, Log out (asks), Refresh; Upload all (asks before replacing
  an upload); Download all… runs `sync download --dry-run` (monk
  acknowledgement asked if the dry run needs `--no-confirm-ok`), shows it,
  confirms the replace and its locks (`confirm_lock`), then `--yes`.
  `ci/gui_account_smoke.py`: two devices on scratch databases against
  `cmd/fake-cloud` (serves `cloudtest`; test only, in `test.yml`).
- Not built: Windows GUI, Android, the real project.
- Tests: `cloud/cloud_test.go`, `cli/sync_test.go` (two devices on two
  databases, one fake), `weakening_audit_test.go` (download over a lock
  set here).

## Accountability partner (§4.15, 2.13)

- Covers also: `internal/cloud/partners.go`, `mail.go`, `cloudtest/partners.go`,
  `cmd/kc9k-mailer`, `db/partners.go`, `cli/partner.go`,
  `gui/kc9k_gui/win_partners.py`, `ci/supabase_schema_test.*`, `ci/supabase_stub.sql`.
- Cloud: tables `partners`, `partner_held`, `partner_messages`,
  `unlock_requests`, `heartbeats`, closed (RLS on, no policies); only
  security-definer functions (`/rest/v1/rpc/…`), each checking the caller:
  owner `partner_invite|list|set|remove|send|hold|release`,
  `unlock_request|status|used`, `heartbeat`; partner `partner_accept`,
  `partner_inbox` (invitations to the caller's address, who they watch with
  last heartbeat, held secrets, messages of 60 days, pending requests),
  `unlock_answer` (approval copies the held secret into the request, which
  the owner reads once: `unlock_used` clears it). Removing a partner drops
  what they hold and denies their pending requests. The fake implements the
  same rules; `ci/supabase_schema_test.sh` runs the real SQL in a
  throwaway Postgres with Supabase's `auth`/`storage` stubbed.
- Email: queued as `partner_messages`; `cmd/kc9k-mailer` (service role
  key, `outbox_take`/`outbox_done`) sends through `cloud.Mailer`
  (`SMTPMailer`); a bounce stays queued. Never sent from a device.
- CLI `kc9k partner` (access area Settings): `invite <email> [--reports
  daily|weekly|off] [--alerts]`, `list [--json]` (with open requests, never
  secrets), `set`, `remove`, `hold settings-password <email>`, `lock <block>
  <email>` (a new 24-character secret, held first, then `start B
  --password-stdin --yes` with it as stdin; both confirmed per R-LOCK-12),
  `request <block>|settings-password [--reason]`, `requests` (an approved
  lock: `unlock B` with the secret, never shown; the settings password:
  shown once, as the partner telling it; then used and released),
  `report [--period] [--print]`, partner side `inbox [--json]`, `accept`,
  `approve`, `deny`. Local `partner.holds` (what → partner).
- Weakening (refused while anything is locked, `refusePartnerWeakening` →
  `refused()`; weakening audit): `remove`, `set` to fewer reports or alerts
  off. `remove` also refused while they hold the settings password set
  here (lockout).
- Alerts: `db.QueuePartnerAlert` (table `partner_alerts`, newest 500), from
  `RecordEvent` (tamper, workaround), bypass prevention firing
  (`enforce/nepsis.go`), and after a command succeeds (`noteForPartners` in
  `Execute`: stop, toggle off, unlock, block remove, master, recover,
  prepare-uninstall, which sends at once). `daemon.PartnerTick` =
  `partnerTick` every 5 min and at a clean stop ("KC9K was stopped"):
  heartbeat every 15 min, alerts (over 20 queued: one message), the
  daily/weekly report once per span (`partner.reports`), held lock secrets
  released once the block is no longer held. Signed out or no accepted
  partner: the queue is dropped unsent (principle 6).
- Report (`partnerReport`): last whole day, or Monday–Sunday week: top 10
  sites and apps by time, times blocked, top blocked, by block.
- GTK (Account page): partners with state, what they get and hold, Change…,
  Remove; Invite…, Lock a block…, Give settings password…, Ask to unlock…,
  Check requests; "You as a partner": Accept, last heard from, held
  passwords (Show), Approve/Deny, recent messages.
- Tests: `cloud/partners_test.go`, `cli/partner_test.go` (two devices),
  `weakening_audit_test.go`, `ci/gui_account_smoke.py` (three devices),
  `ci/supabase_schema_test.sh`.
