# Windows (Core: the Go program)

- Covers: every `*_windows.go` and `*_linux.go`, `blocker/shared.go`, `internal/platform/`, `internal/winapi/`, `internal/winsvc/`, `internal/winagent/`, `ci/windows_*.ps1`, `.github/workflows/windows.yml`
- SPEC: §6 2026-11-21 (Windows decisions), §3.13
- Connects to: [enforce-daemon](enforce-daemon.md), [install](install.md), [layers-net](layers-net.md), [browsers](browsers.md), [apps-titles](apps-titles.md), [device](device.md), [circadian](circadian.md), [testing](testing.md); the GUI's own notes are `gui-windows.md`, `install-windows.md`

## Platform split (W0)

- Linux-only files: `//go:build linux` (older, most of `internal/blocker/`)
  or `_linux.go`; Windows counterpart `<name>_windows.go`.
- Shared: `blocker/shared.go` (layer input/report types),
  `hotkeys_shared.go`, `hashentry.go`, `browser.go`, `hosts.go` (only
  `hostsPath` differs), `resolve.go` (`GetAdaptersAddresses`).
  `firmware.go` (sysfs) finds nothing on Windows (Windows: WMI, below).
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
  for real" `-run 'WFP|RegPolicy|WinProc'` with `KC9K_TEST_WFP=1`,
  service, desktop and soak scripts) and `unit` (`go test ./...`, slow).
  Linux-only tests carry their file's tag. GitHub's provisioner runs from
  `AppData\Local\Temp`: exempted before strict mode or the job dies
  silently.
- `ci/windows_service_test.ps1`: install, forwarding, exit codes,
  caller-named files, data DACL, uninstall refused while locked.
- `ci/windows_desktop_test.ps1` (W27, signed-in runner): agent, hosts and
  firewall on a real site, Edge policy, app and title closed, dark mode.
- `ci/windows_soak_test.ps1 -Kc9k kc9k.exe [-Minutes N] [-Phases
  idle,circadian,play,settings] [-Gui …] [-Out …]` (W50–W52; CI 1 min/phase,
  must parse in PowerShell 5.1): per phase counts policy refreshes,
  process starts, agent restarts, tamper, resources, display resets,
  crashes, gamma, %SystemRoot% programs closed. Aborts and restores on CPU > 90 %
  30 s, > 5 powershell.exe, display reset, GUI dead/hung, agent restart;
  exit 1 over a threshold (idle: no refresh, PowerShell or restart,
  handles ±5 %). Never touches locks, passwords, monk mode, Frozen Chicken,
  strict mode, no-escape, firmware, network adapters, logout, lock screen.
- `ci/windows_vm_test.ps1 kc9k.exe [-Reboots]`: checks needing a desktop
  or person (`-Reboots`: Safe Mode, recovery). VM with snapshot only.
- Local: `GOOS=windows CGO_ENABLED=0 go build ./... && GOOS=windows go vet ./...`.
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
  nonce; input; input end; Ctrl-C; output; errors; exit code).
- Server: pipe DACL SYSTEM all, signed-in users read/write (no new
  instances), network denied; `FILE_FLAG_FIRST_PIPE_INSTANCE`, no remote
  clients, ≤ 32 commands. Caller via `ImpersonateNamedPipeClient` (SID,
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

## Web layers (W2)

- Hosts: `System32\drivers\etc\hosts`; every write (and `SyncSafeSearch`)
  ends in `hostsWritten` (`DnsFlushResolverCache`). Defender may flag it;
  KC9K never hides from Defender (SPEC §6).
- Firewall (`firewall_windows.go`, `winapi/wfp_windows.go`): WFP, not
  Windows Firewall. Own persistent provider/sublayer (fixed GUIDs, weight
  0xF000, default engine security). `ALE_AUTH_CONNECT_V4/V6` filters:
  blocked addresses (500 ORed conditions per filter); DoH resolvers + port
  443/853 (W26, `doh.go`). Monk mode's cut: all four ALE connect/accept
  layers, permit loopback (weight 15), block all (1). One transaction
  replaces all filters; persistent (boot, service stopped).
  `IsFirewallApplied`: last apply's filters present by name and count.
- Policies (`browser_windows.go`): Chrome, Chromium, Edge, Brave, Vivaldi
  under `HKLM\SOFTWARE\Policies\<vendor>\<name>` (`writeRegPolicy`).
  Blocklist owns `URLBlocklist`, `URLAllowlist`, `DnsOverHttpsMode`,
  `BuiltInDnsClientEnabled`; setup owns private windows
  (`IncognitoModeAvailability`, `InPrivateModeAvailability`), guest,
  developer mode, devtools, extension, forced search. Firefox family:
  `ProgramData\kc9k\firefox\policies.json` copied to each install's
  `distribution\` (Program Files, App Paths, per-user `AppData\Local`)
  and (W26, `firefoxreg_windows.go`) mirrored to
  `HKLM\SOFTWARE\Policies\Mozilla\Firefox` for Store/portable Firefox;
  only keys in `firefox-registry.json` are removed. LibreWolf: own key +
  distribution file; setting locks (`librewolfprefs.go`) at the end of
  `librewolf.cfg`, per-account overrides in
  `Program Files\KC9K\share\librewolf\<name>.cfg` via `USERNAME`.
- Chrome/Edge are supported browsers (extension by `ExtensionSettings`/
  `ExtensionInstallForcelist`) only once `ChromeStoreExtensionID` is set
  (empty until the owner publishes). LibreWolf gets the bundled `.xpi`.
- One-click install (W18, `InstallBrowser`): winget as SYSTEM (newest
  `winget.exe` in WindowsApps), `--scope machine --silent`; skipped if
  present.
- Processes (`procs_windows.go`): Toolhelp snapshot, full path, owner RID,
  service-account flag, session, start time; a parent counts only if
  started before its child (PID reuse). `tcpOwner`: connection → process.
- Killing (W50): only via `killProcess` (what KC9K found; never under
  %SystemRoot% or an unreadable path) or `killNamedProcess` (a rule names
  it: blocked app and tree, tamper tools, blocked window owner, `vmwp`).
  Both refuse `sessionCritical`: PID 0/4, `essentialWindowsApps` +
  `neverKillExtra` in the Windows folder or unreadable, KC9K's own; logged
  once ("kill refused"). `terminateProcess` (test seam) rechecks the
  handle's path. `TestWinKillDenyList`, `TestWinGuardNeverClosesWindows`.
- Extension guard (`extguard_windows.go`, verdict `extguard_shared.go`):
  a browser = guarded name, OriginalFilename of a renamed one, or a
  program beside `xul.dll`/`chrome.dll`/`msedge.dll` (or in a version
  subfolder). Never Thunderbird, WebView2, Windows' own
  (`windowsOwnProgram`: %SystemRoot%, Microsoft WindowsApps packages),
  unreadable paths. Hazard: a glob `<dir>\*\msedge.dll` matched
  `System32\Microsoft-Edge-WebView`, so the guard closed dwm/svchost… and
  hung the laptop; `TestWinGuardSystem32NotBrowsers`. Main process:
  topmost same-program ancestor. Remote-control starts closed at once
  (W11): `commandLine`, `processEnvironment` (PEB) checked by
  `remoteControlArgs` (also `/switch`, any case).
- Guest: Chromium guest off by setup. "Close guest profiles" also keeps
  Windows' Guest account (RID 501) off (`syncGuestAccount`); `guest-off`
  records it was on, and only then is it re-enabled after (W14).

## Apps and desktop (W3)

- Apps (`apps_windows.go`): entry = program name, full path, folder
  (ends `\` or `/`), or `sha256:…` (cached by path/size/time; never
  Windows' own; Linux-only entries unused, [db-project](db-project.md)).
  Only people's processes (`winProc.person`); never `essentialWindowsApps`
  in the Windows folder, KC9K, or this process and its ancestors; children
  too, except under shells/Explorer (`noDescendWindows`). Protected
  folders: drive root, Windows, System32, SysWOW64, Program Files, Users,
  ProgramData. Polite close: `polite_shared.go`, agent `OpClose`.
- Scripts (W12, `scripthosts.go`, shared): a script entry matches its
  host: Python (`-m` too), PowerShell, cmd, wscript/cscript, mshta, node,
  java `-jar`, AutoHotkey (`hostScript`; cwd from PEB); never a
  Windows-folder script. Matched shell dies with its children. Fetch tools
  known by script/module (`python -m yt_dlp`, `scriptNames`).
- Allowed apps (`CloseAllExcept`): kills people's programs not allowed,
  not started by an allowed one, not Windows' or KC9K's. Tor: Tor Browser
  folders, `tor.exe`. `KillNamingFor` reads command lines via
  `NtQueryInformationProcess`.
- Sessions (`desktop_windows.go`): WTS sessions with a user token; locked
  from `WTSSessionInfoEx`. Agent ops: `OpStatus` (idle), `OpForeground`
  (only if the window's process is the session user's), `OpNotify` (tray
  balloon), `OpConfirm` (MessageBox), `OpLock`, `OpWindows` (visible
  unowned), `OpClose` (`WM_CLOSE`; folder-entry Explorer windows closed,
  never killed). Logout `WTSLogoffSession`; power off `ExitWindowsEx`.
  Logout freeze: `HoldLogins` writes `hold-logins`; the service then signs
  out every session instead of starting its agent
  (`winagent.SessionFilter`).
- Custom actions (`RunCustom`): Explorer opens, `cmd /c` runs, as the first
  signed-in person.
- Hotkeys: `SyncHotkeys` → `hotkeys.json` (`RegisterHotKey` codes) to
  every agent; a press → service runs `kc9k hotkey run <id>` for that
  person.
- Places (W10, `enforce/wifi_windows.go`, `winapi/wlan*.go`): Native Wifi
  (`ParseBSSList` tested on Linux). No WLAN service → at every place.
  Wi-Fi on = enabled adapter + radio switch on (airplane mode keeps the
  adapter enabled); `SetWifiRadio(true)` sets both, so `keepWifiOn` holds.
- Monk mode's physical cut (`netcut_windows.go`): `Get-NetAdapter
  -Physical`, `Enable-/Disable-NetAdapter`.
- Challenge window (`winagent/challenge_windows.go`): Win32 window (topmost,
  fixed frame, close only = cancel, system window brush), no text field, opened by the caller's agent (`OpChallenge`, ≤ 4 h); GTK's
  events (`k`, `b`, `n`, `p`, `s`, `c`). Raw Input: key with no device
  (SendInput, on-screen keyboard) → `s`; Ctrl+V/Shift+Insert → `p`.
  Read by `lock/window_windows.go`.

## Tamper protection against an administrator (W5)

- `blocker/tamper_windows.go`; `guardPlatform` (`enforce/sys_windows.go`,
  each cycle after `enforceLockdown`) puts back anything changed and logs
  tamper (§3.10, counted like a daemon restart). An admin can still win
  (take ownership, rewrite ACLs, stop things); each step is undone within
  a cycle and counted. Nothing hides from Defender.
- Always (`GuardService`): `ServiceSDDL` (Administrators: query, start,
  read only); automatic start, program path, restart on failure; Safe Mode
  registration (`SafeBoot\Minimal|Network\kc9k`); watchdog task
  `\KC9K\Watchdog` (SYSTEM, boot + every minute, `kc9k watchdog`
  starts/recreates the service; admins can delete it → recreated,
  counted); program/data folder security (not while strict mode holds
  them; compared structurally by `sameSecurity`, never as text). An agent
  ended while its session lasts counts too.
- Strict mode (`ApplyLockdown`): deny entries (`denyACEs`) for
  Interactive, Network, Batch (S-1-5-3); never Administrators
  (LocalSystem's token has it) or SYSTEM. Files `0xd0156` (write, delete,
  permissions, owner; compared via `canonACE`), keys `0xd0006`. Targets:
  service key, `Control\SafeBoot`, `BCD00000000`, `Recovery\ReAgent.xml`,
  Chromium policy keys, hosts, program folder, Firefox `distribution`
  folders, SRP key, `DisableMSI` key, `Services` tree. Original security in
  `lockdown.json`, restored by `RemoveLockdown`; removed deny entries put
  back and counted (`LockdownRepairs`).
- Downloaded/portable programs (`userProgram`: profile folders but
  AppData, `AppData\Local\Temp`, removable drives) closed
  (`CloseNoexecPrograms`) unless exempt; AppData installs run. Starts
  blocked too (W25, `noexec_windows.go` `ApplyNoexec`): Software
  Restriction Policies path rules (registry, so Home has them; all users),
  keys `{4B43394B-…}`: profiles, `AppData\Local\Temp`, Public, removable
  drives disallowed; AppData and exempt folders (long paths) allowed; not
  Windows' Temp (SYSTEM uses it). `ensureSRPOn` sets missing SRP values
  (DefaultLevel, TransparentEnabled 1, PolicyScope 0, ExecutableTypes;
  off on Windows Server), noted in `srp-made`; `RemoveNoexec` removes
  exactly KC9K's, first in `RemoveLockdown`. Shells started before the
  rules may keep old policy: `CloseNoexecPrograms` catches their starts.
  Service-run PowerShell: 90 s limit (firmware 5 min).
- `refreshPolicy` (var; forced `RefreshPolicyEx`, needed since rules are
  plain registry): only after a rule really changed.
  `TestStrictSRPRefreshOnlyOnChange`.
- Strict mode's ways to SYSTEM (W20–W22, `strict_windows.go`;
  `applyStrictExtras`/`removeStrictExtras`, the latter first in
  `RemoveLockdown`, failure logged and retried, never keeps deny entries).
  State `strict-extras.json`. Each cycle: service manager DACL denies the
  three logons `0xc0022`; `DisableMSI`=2; people's tokens lose backup,
  restore, take ownership, debug, load driver, firmware vars, manage
  volume, relabel, TCB (`SE_PRIVILEGE_REMOVED`); accounts not enabled at
  start disabled. Once: those LSA rights removed from Administrators and
  Backup Operators, restored after. Heavy pass every 30 s in a goroutine
  (`strictHeavy`; PowerShell checks first pass, then every 5 min): each
  service DACL denies `0xd0002` (`0xd0062` + stop/pause for BFE, Dnscache,
  Winmgmt, Schedule, EventLog, WlanSvc, W32Time), not KC9K's; Task
  Scheduler root denies `0xd0116`; new SYSTEM/service-account tasks
  outside `\Microsoft\Windows\`, `\KC9K\` deleted; new WMI consumers in
  `root\subscription` deleted with bindings. All counted. Ending: quick
  parts at once, heavy after the running pass (`startHeavyUndo`, marker
  `strict-heavy`); `kc9k uninstall` waits (`WaitStrictUndone`).
- VMs (W24, `closeVirtualMachines`): people's VM, WSL, Sandbox, QEMU,
  Docker programs (`vmPrograms`, `isVMProcess`) closed with children,
  `vmwp` ended, `vmServices` stopped; logged, not counted.
- Clock/time zone (W23, `clockguard_windows.go`): tamper clock on and
  anything locked → zone and wall−boot offset kept in `clock-guard.json`;
  zone change set back; clock moved > 2 min set back unless internet time
  (≤ 1/min) confirms it (new reference). Counted.
- No-escape (`ApplyBootLock`): `reagentc /disable`, `bcdedit`
  `recoveryenabled No`, `bootstatuspolicy IgnoreAllFailures`,
  `displaybootmenu No`; `bootlock.json`; `RemoveBootLock` undoes.
- Guided setup (W15, `cli/guidedsetup_windows.go`): `other-admins`
  (`winapi.AdministratorSIDs`), `builtin-admin` (RID 500), fixed by `net
  user … /active:no`; `safe-mode` info step before no-escape; other
  systems from `bcdedit /enum osloader`.
- Recovery path (W13, principle 4, R-ESCAPE-1): Safe Mode is the recovery
  boot (`SafeModeBoot`); `KernelCmdline` then carries `kc9k.nofreeze`
  (freezes, lockdown skipped unless no-escape; blocks stay), `kc9k
  recover` works. Factory reset protection on and no-escape off → F8 menu
  on (`bootmenupolicy Legacy`, `recoveryMenuOn`); else Standard.
- Factory reset protection while anything is locked (`SyncRecoveryGuard`):
  recovery environment off (Reset this PC needs it), restored after
  unless no-escape; re-enabled by hand → put back, counted. Image writers
  (Rufus, Ventoy, Media Creation Tool…) join its apps.
- Task managers: `platformTaskManagers` (Task Manager, regedit, MMC, sc,
  bcdedit, schtasks, taskkill…).
- Firmware lock (W19, `firmware_windows.go`, `firmware_wmi.go`): vendor
  WMI (as Linux's drivers): Dell `root\dcim\sysman` (`biosattributes`,
  `wmisecurity`); Lenovo `root\wmi` (`Lenovo_BiosSetting`,
  `SetBiosSetting` + `SaveBiosSettings`, `SetBiosPassword` "pap";
  passwords without "," or ";"); HP `root\HP\InstrumentedBIOS`
  (`SetBIOSSetting`, "Setup Password", `<utf-16/>`). Read script → JSON
  (`wmiStatus`); write script reads `wmiRequest` on stdin (no passwords on
  command lines). Same purposes and order as Linux.

## Circadian rhythm and wallpapers (W6)

- `blocker/circadian_windows.go`; agent side `winagent/look_windows.go`.
- Gamma: ramps from `RampFunc` set by each agent on every screen
  (`SetDeviceGammaRamp`); HKLM ICM `GdiIcmGammaRange`=256 so warm ramps
  aren't refused.
- Wallpaper copied to `Program Files\KC9K\share\wallpapers` (data folder
  is SYSTEM-only), set via `SPI_SETDESKWALLPAPER`. Dark mode: HKCU
  `Themes\Personalize` `AppsUseLightTheme`/`SystemUsesLightTheme` +
  `WM_SETTINGCHANGE "ImmersiveColorSet"`. Signed-out people unreachable.
- Night light: read only (`bluelightreductionstate` byte 18 = 0x15);
  `TurnOffNightLight` asks the person.
- Brightness: panel via PowerShell WMI (`WmiMonitorBrightness`; WMI over
  COM isn't in x/sys, `\\.\LCD` IOCTLs unprovable in CI; rates in the
  table below, `ForgetBacklights` on wake, `TestWinPanelRateLimits`);
  external screens via DDC/CI (dxva2) in the agent, listed ≤ 1/min,
  `Display` = "session:index".
- Sleep hook: `sleep-hook` file while on; on suspend the service runs
  `kc9k wallpaper before-sleep`.

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
