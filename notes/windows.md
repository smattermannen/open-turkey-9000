# Windows (Core: the Go program)

- Covers: platform split (`*_windows.go`/`*_linux.go` pairs, `blocker/shared.go`, `internal/platform/`, `internal/winapi/`), the manifest (`cmd/kc9k/kc9k.manifest`, `*.syso`, `internal/winres/`), the service and agent (`internal/winsvc/`, `internal/winagent/` but `look_windows.go`, `challenge_windows.go`), `ci/windows_*.ps1`, `.github/workflows/windows.yml`. Layers, desktop, circadian: [windows-layers](windows-layers.md); tamper protection, strict mode, no-escape: [windows-tamper](windows-tamper.md)
- SPEC: §6 2026-11-21 (Windows decisions), §3.13
- Connects to: [windows-layers](windows-layers.md), [windows-tamper](windows-tamper.md), [enforce-daemon](enforce-daemon.md), [install](install.md), [testing](testing.md); the GUI's own notes are `gui-windows.md`, `install-windows.md`

## Platform split (W0)

- Linux-only files: `//go:build linux` (older, most of `internal/blocker/`)
  or `_linux.go`; Windows counterpart `<name>_windows.go`.
- Shared: `blocker/shared.go` (layer input/report types),
  `hotkeys_shared.go`, `hashentry.go`, `browser.go`, `hosts.go` (only
  `hostsPath` differs), `resolve.go` (`GetAdaptersAddresses`).
  `firmware.go` (sysfs) finds nothing on Windows (Windows: WMI, [windows-tamper](windows-tamper.md)).
- Per-system: `db/bootclock_*.go` (boot ID `PrefetchParameters\BootId`;
  `GetTickCount64` counts sleep), `db/openlock_*.go`, `enforce/sys_*.go`
  (`LockFileEx`; time asleep via `QueryUnbiasedInterruptTime`),
  `lock/term_*.go`, `cli/term_*.go` (VT input: bracketed paste),
  `lock/window_*.go`, `cli/readcaller_*.go`.
- Hardware clock (W17, `enforce/rtc_*.go`): newest Kernel-General event 12
  (clock read at boot) + boot clock since (`hardwareClockAt`); cleared
  log → internet time alone.

## CI

- `windows.yml`: job `system` (build + `kc9k.exe` artifact, vet, "Layers
  for real" `-run 'WFP|RegPolicy|WinProc|…'` with `KC9K_TEST_WFP=1`,
  desktop and soak scripts, manifest test (`go test ./cmd/kc9k`), a WSL 1
  Ubuntu by `Vampire/setup-wsl`, service script) and `unit` (`go test
  ./...`, slow).
  Linux-only tests carry their file's tag. GitHub's provisioner runs from
  `AppData\Local\Temp`: exempted before strict mode or the job dies
  silently.
- `ci/windows_service_test.ps1`: install, forwarding, exit codes,
  caller-named files, data DACL, uninstall refused while locked.
- `ci/windows_desktop_test.ps1` (W27, signed-in runner): agent, hosts and
  firewall on a real site, Edge policy, app and title closed; the challenge
  window from `kc9k unlock` with no console (a `SendInput` key refused as
  injected; posted `WM_CHAR` keys of a `--lock-text` unlock it); a hotkey
  pressed (`SendInput`) starting its block; gamma (warm preview, plain
  after; a warning where the runner has no ramp); wallpaper default
  custom reaching HKCU; dark mode; `browsers install librewolf` (winget as
  SYSTEM through the service, W18).
- `ci/windows_service_test.ps1` also starts a real WSL distribution (where
  the workflow installed one) and checks strict mode closes it, and again.
- `ci/windows_soak_test.ps1 -Kc9k kc9k.exe [-Minutes N] [-Phases
  idle,circadian,play,settings] [-Gui …] [-Out …]` (W50–W52; CI 1 min/phase,
  must parse in PowerShell 5.1): per phase counts policy refreshes,
  process starts, agent restarts, tamper, resources, display resets,
  crashes, gamma, %SystemRoot% programs closed. Aborts and restores on CPU > 90 %
  30 s, > 5 powershell.exe, display reset, GUI dead/hung, agent restart;
  exit 1 over a threshold (idle: no refresh, PowerShell or restart,
  handles ±5 %). PowerShell counts as the service's when its parent is the
  service or a `kc9k.exe` the service started. `-IdleMinutes N`; `-Strict`
  (CI): the process start trace must exist and see a PowerShell the script
  starts. CI: idle 6 min, then the workflow reads `summary.csv` and fails
  on idle PowerShell or policy refreshes (W50). Never touches locks, passwords, monk mode, Frozen Chicken,
  strict mode, no-escape, firmware, network adapters, logout, lock screen.
- `ci/windows_vm_test.ps1 kc9k.exe [-Reboots]`: checks needing a desktop
  or person (`-Reboots`: Safe Mode, recovery). VM with snapshot only.
- Local: `GOOS=windows CGO_ENABLED=0 go build ./... && GOOS=windows go vet ./...`.
- Manifest (§4.3): `cmd/kc9k/kc9k.manifest` (common controls v6,
  per-monitor v2 DPI aware, asInvoker, Windows 10+) linked through
  `cmd/kc9k/rsrc_windows_{amd64,arm64}.syso`, written by `internal/winres`
  (one RT_MANIFEST resource in a COFF `.rsrc` section, ADDR32NB relocation;
  `UPDATE_SYSO=1 go test ./internal/winres`; checked against the manifest
  and read back out of a cross-built `kc9k.exe`; LF kept by
  `.gitattributes`). `cmd/kc9k/manifest_windows_test.go` runs under it:
  per-monitor v2, comctl32 6, challenge window sized for its DPI, keys,
  close.
  `os.Pipe` capture helpers must read while the command runs (4 KB pipe).

## Service, pipe, agent, install (W1)

- `internal/platform` (data folder, caller from `KC9K_CALLER_*`),
  `winapi` (Win32 x/sys lacks), `winsvc` (command pipe), `winagent`;
  `cli/windows_windows.go` (`service`, `agent`, `install`, `uninstall`,
  `finish-uninstall`; `forwardIfNeeded` first in `Execute`).
- Local commands (`service`, `agent`, `install`, `finish-uninstall`,
  `watchdog`) skip access passwords (they can't read the SYSTEM-only DB).
- Wording (W16): `rewordForWindows` swaps Linux names via
  `cli/wording.go` (hosts file, service, Safe Mode, Task Manager, Guest);
  `strict`, `no-escape` have Windows help. `TestWindowsWording`.
- Forwarding: all commands but `service`, `agent`, `install`, `help`,
  `completion` go to `\\.\pipe\kc9k` unless LocalSystem or `KC9K_DB`
  set; client first checks the server PID is `ServicePID`. Frames: type,
  4-byte length, payload (request JSON with args, cwd, console size,
  nonce; input; input end; Ctrl-C; output; errors; exit code). After
  "input end" the server keeps reading frames: Ctrl-C or the client's end
  still kills the command.
- Server: pipe DACL SYSTEM all, signed-in users read/write (no new
  instances), network denied; `FILE_FLAG_FIRST_PIPE_INSTANCE`, no remote
  clients, ≤ 32 commands. A failure never ends `Serve` (W55): pipe
  held by another process (an upgrade's old one) or a failed accept is
  logged once per run of failures and retried after 250 ms doubling to
  5 s (`retry.go`) until stop; `ci/windows_service_test.ps1` holds the
  pipe across install, then lets go. Caller via `ImpersonateNamedPipeClient` (SID,
  RID as uid, name, session; Administrators only). Child `kc9k.exe <args>`
  as LocalSystem, env minus `KC9K_*` plus
  `KC9K_CALLER_SID/_UID/_USER/_SESSION/_TOKEN` (inherited impersonation
  token: `readcaller_windows.go` opens caller-named files as the caller)
  and `KC9K_TERMINAL` (cols,rows,nonce). Ctrl-C/closed client kills it.
- Consoles: VT output, cooked input; for raw input (challenge) or no echo
  a command writes `ESC ]7799;kc9k;<nonce>;raw|cooked|echo-off|echo-on BEL`,
  which the client strips and obeys (`ControlFilter`); the nonce stops
  printed text switching the console. Client waits for input before each
  read, so raw applies to the next key.
- Agents (`manager_windows.go`): every 3 s each session with a user token
  gets `kc9k agent` (`CreateProcessAsUser`, `winsta0\default`), restarted
  if ended. Agent pipe `\\.\pipe\kc9k-agent` accepts only the installed
  `kc9k.exe` image, in the session its hello names, own PID. JSON lines
  (requests with IDs, replies, events ID 0). Service-run commands reach
  agents via `\\.\pipe\kc9k-agents-internal` (SYSTEM only):
  `winagent.Call`, `winagent.Sessions`.
- Agent stability: `dispatch.go` runs each request in a goroutine under
  `recover()` (panic → error reply, logged). Screen ops one at a time (one
  mutex); a waiting gamma set is replaced by a newer one. `CheckRamp`
  rejects bad ramps (256 values/colour, never falling, top ≥ the
  1000–10000 K / 10 % floor less a tenth). Exiting agent restores the
  plain ramp if it set another. Win32 callbacks made once
  (`displays()`: Go caps `NewCallback` at 2000; `TestWinDisplaysManyCalls`).
- Crash vs tamper (§3.10): agent output on a pipe only it inherits
  (`spawnWithOutput`); first 200 lines to the service log; last 16 KB
  decide (`classifyEnd`): exit 2 with Go's "panic: "/"fatal error: " →
  crash (`Manager.Crashes`, not tamper); its own marker line → ended by
  itself; else (TerminateProcess, taskkill) from outside
  (`Manager.Restarts`, tamper via `guardPlatform`). `TestAgentEndCrashVsKill`.
- Install (`install_windows.go`): `Program Files\KC9K` (`ProgramSDDL`:
  owner SYSTEM, Users read+run, protected), `ProgramData\kc9k`
  (`DataSDDL`: SYSTEM only), `kc9k.exe` copied (running one renamed
  `.old`, deleted at reboot), Machine PATH, service `kc9k` (automatic;
  restarts 0/5/10 s, also on non-crash failures; waits ≤ 30 s for a
  just-removed one). Service exits as failed when installed `kc9k.exe`
  changes, so Windows starts the new one. Log `ProgramData\kc9k\kc9k.log`
  (10 MB + one old).
- Uninstall: `prepareUninstall`, then a temp copy `finish-uninstall <pid>`
  waits, removes service, program folder, PATH entry. Data stays.

## Process starts and broadcasts from the service cycle (W50 audit)

What `Tick`, the circadian engine and the agent manager start or
broadcast, and the allowed rate:

| What | Where | Runs |
|---|---|---|
| `RefreshPolicyEx` (forced) | `noexec_windows.go` `refreshPolicy` | after SRP rules changed |
| PowerShell brightness | `circadian_windows.go` `panel` | look once (again after wake/failure); read ≤ 1/min; set ≤ 1/s (newest kept); one at a time |
| PowerShell `Get-NetAdapter` | `netcut_windows.go` | ≤ 1/min, while monk mode cuts or places keep Wi-Fi on |
| PowerShell `Enable/Disable-NetAdapter` | `netcut_windows.go` | on change |
| `schtasks /Query`, `/Create` | `GuardService` | task file checked each cycle, put back at once; after a failure waits a minute |
| `schtasks /Delete`; PowerShell (services, tasks, WMI) | `strict_windows.go` | strict mode: on change; first pass then ≤ 1/5 min |
| `reagentc`, `bcdedit` | `SyncRecoveryGuard` | on change; failure retried next cycle |
| PowerShell (firmware) | `firmware_windows.go` | `firmware-lock` only |
| `wevtutil` | `rtc_windows.go` | once per service run |
| `kc9k hotkey run`; `kc9k wallpaper before-sleep` | service | on hotkey; on suspend |
| `WM_SETTINGCHANGE`, `SPIF_SENDCHANGE` | agent `look_windows.go` | on change |
| `kc9k agent` | `manager_windows.go` | per session without one; not within 2 s |
| `TerminateProcess` | `procs_windows.go` | only via the deny-list kill paths |
