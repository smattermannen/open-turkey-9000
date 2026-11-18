# Enforce, daemon, affected users

- Covers: `internal/enforce/` (`enforce.go`, `enabled.go`), `internal/daemon/`, `db/users.go`, `cli/blockusers.go`
- SPEC: §3.2 (affected users), §3.3
- Connects to: all notes; most often [automation](automation.md), [locks](locks.md), [layers-net](layers-net.md)

## Code

- **Enforce** (`internal/enforce/`): active blocks → layer inputs (unioned).
  `Apply` (CLI, on change) and `Tick` (daemon: due queued edits, schedule
  reconciliation, drift reapply). Single source of truth.
- **Daemon** (`internal/daemon/`): `Tick` every 5 s; foreground-app
  sampling every 5 s; screen relock every 0.5 s during a lock freeze.
- **Affected users** (`db/users.go`, `cli/blockusers.go`, §3.2):
  `blocks.users`; `enforce.forEveryone` keeps partial blocks out of shared
  layers, `blockUIDs` → `blocker.KillBlockedFor`, `BuildRulesFor(user)`
  with `clientUser` (socket uid, `blocker.ClientUser`); `regularUsers` /
  cli `accounts` faked in tests.
