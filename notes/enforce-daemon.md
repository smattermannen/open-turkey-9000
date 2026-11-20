# Enforce, daemon, affected users

- Covers: `internal/enforce/` (`enforce.go`, `enabled.go`), `internal/daemon/`, `cmd/kc9k/main.go`, `cli/root.go`, `cli/daemon.go`, `db/users.go`, `cli/blockusers.go`
- SPEC: §3.2 (affected users), §3.3
- Connects to: all notes; most often [automation](automation.md), [locks](locks.md), [layers-net](layers-net.md)

## Code

- **Enforce** (`internal/enforce/`): active blocks → layer inputs (unioned).
  `Apply` (CLI, on change) and `Tick` (daemon: due old queued edits (`applyDueEditsAt`; nothing queues now), schedule
  reconciliation, drift reapply). Single source of truth.
- **Daemon** (`internal/daemon/`): `Tick` every 5 s; foreground-app
  sampling every 5 s; screen relock every 0.5 s during a lock freeze.
- **Affected users** (`db/users.go`, `cli/blockusers.go`, §3.2):
  `blocks.users`; `enforce.forEveryone` keeps partial blocks out of shared
  layers, `blockUIDs` → `blocker.KillBlockedFor`, `BuildRulesFor(user)`
  with `clientUser` (socket uid, `blocker.ClientUser`); `regularUsers` /
  cli `accounts` faked in tests. Also per user (2026-11-21): window titles
  and folder entries (`titleTargets` → `blocker.TitlesByUser`, each
  desktop user's windows judged by every account's targets and their own;
  `titleHitBlocks` blames only blocks for that user), fetch tools
  (`blockDomains` → `blocker.KillNamingFor`), the extension guard
  (`extensionUsers` → `ExtensionGuard.EnforceFor`: options, locks
  (R-LOCK-9), a block for everyone and block unsupported browsers need it
  in every account). The GUI's "isn't running" dialog still looks at every
  account's browsers.
