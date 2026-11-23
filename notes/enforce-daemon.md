# Enforce, daemon, affected users

- Covers: `internal/enforce/` (`enforce.go`, `enabled.go`), `internal/daemon/`, `cmd/kc9k/main.go`, `cli/root.go`, `cli/daemon.go`, `db/users.go`, `cli/blockusers.go`
- SPEC: §3.2 (affected users), §3.3
- Connects to: all notes; most often [automation](automation.md), [locks](locks.md), [layers-net](layers-net.md)

## Enforce (`internal/enforce/`)

- Active blocks → unioned layer inputs. `Apply` (CLI, on change); `Tick`
  (daemon: old queued edits `applyDueEditsAt`, schedule reconciliation,
  drift reapply).
- `Inputs` reads tamper settings file:// blocking and block embedded
  content from package vars loaded before each apply (`loadDNSChoice`);
  keeps its results per blocks (`keptInputs`, ≤ 32, keyed by `inputsKey`:
  maphash via `blocker.ListsKey`, those settings included).
- Firewall filled only by `Tick` (`deferFirewall`), never waiting on
  lookups (`blocker.ApplyFirewallKnown`), reapplied when the background run
  ends (`firewallLookups`): waiting held the apply lock and every `kc9k`
  command.

## Daemon (`internal/daemon/`)

`Tick` every 5 s; foreground-app sampling every 5 s; screen relock every
0.5 s during a lock freeze.

## Affected users (§3.2; `db/users.go`, `cli/blockusers.go`)

- `blocks.users`; `enforce.forEveryone` keeps partial blocks out of shared
  layers. Per user: kills (`blockUIDs` → `blocker.KillBlockedFor`), rules
  (`BuildRulesFor(user)`, `clientUser` = socket uid), window titles and
  folder entries (`titleTargets` → `blocker.TitlesByUser`; `titleHitBlocks`
  blames only that user's blocks), fetch tools (`blockDomains` →
  `KillNamingFor`), extension guard (`extensionUsers` →
  `ExtensionGuard.EnforceFor`; options, locks (R-LOCK-9), a block for
  everyone and block unsupported browsers need it in every account).
- GUI "isn't running" dialog: `ExtensionRequiredNowFor(callerUID)`,
  `RunningSupportedBrowsersFor`; check-ins per browser, not per account.
- `regularUsers` / cli `accounts` faked in tests.
