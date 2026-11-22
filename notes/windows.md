# Windows (Core: the Go program)

- Covers: every `*_windows.go` and `*_linux.go`, `blocker/shared.go`, `internal/platform/`, `internal/winapi/`, `internal/winsvc/`, `internal/winagent/`, `ci/windows_*.ps1`, `.github/workflows/windows.yml`
- SPEC: §6 2026-11-21 (Windows decisions), §3.13
- Connects to: [enforce-daemon](enforce-daemon.md), [install](install.md), [layers-net](layers-net.md), [browsers](browsers.md), [apps-titles](apps-titles.md), [device](device.md), [circadian](circadian.md), [testing](testing.md); the GUI's own notes are `gui-windows.md`, `install-windows.md`

## Platform split (W0)

- One Go program for both systems. A file that only makes sense on Linux
  starts with `//go:build linux` (most of `internal/blocker/`); its
  Windows counterpart is `<name>_windows.go`. New platform files use the
  suffix (`sys_linux.go`/`sys_windows.go`), old ones keep their names
  and the tag, so notes and history stay valid.
- What enforce hands a layer and what it reports back is shared on both:
  `blocker/shared.go` (policy and setup types, `Reach`, `DesktopLook`,
  `GammaReading`, title targets, firmware status, `PercentToRaw`),
  `hotkeys_shared.go` (`ParseAccel`), `hashentry.go` (contents entries),
  `browser.go` (policy sets and patterns; writing them is
  `browser_linux.go`), `hosts.go` (only `hostsPath` differs:
  `System32\drivers\etc\hosts`), `resolve.go` (the DNS client;
  network servers from resolv.conf or `GetAdaptersAddresses`).
- `blocker/stubs_windows.go` held every layer function not yet built on
  Windows while W2–W6 were built; it is gone (2026-11-21): every function
  has its Windows version. The firmware lock (`firmware.go`) is shared
  file I/O on sysfs: on Windows the folder isn't there, so the firmware
  counts as not available (the vendors' tools: not built).
- Small per-system pieces: `db/bootclock_*.go` (boot ID: `BootId` under
  `PrefetchParameters`, raised at every boot; boot clock:
  `GetTickCount64`, counts sleep like `CLOCK_BOOTTIME`),
  `db/openlock_*.go` and `enforce/sys_*.go` (file locks: `flock` /
  `LockFileEx`; time asleep: tick count minus
  `QueryUnbiasedInterruptTime`), `lock/term_*.go` and `cli/term_*.go`
  (raw console: console modes with VT input, so bracketed paste works in
  Windows Terminal), `lock/window_*.go` (the challenge window),
  `cli/readcaller_*.go`.
- The hardware clock (W17, `enforce/rtc_*.go`): Windows lets programs read
  it no other way, so it is taken from Kernel-General event 12 ("The
  operating system started at system time", the hardware clock read at
  boot), the newest one (every boot logs one), read once per service run,
  plus the boot clock since (`hardwareClockAt`; the wall clock plays no
  part); a cleared log gives none, and the clock check uses internet time
  alone.

## CI

- `.github/workflows/windows.yml` on `windows-latest`: build (also
  `kc9k.exe`, uploaded as an artifact), vet, `go test ./...`. Linux-only
  tests carry the tag of the file they test.
- Jobs: `system` (build, vet, "Layers for real": `-run
  'WFP|RegPolicy|WinProc'` with `KC9K_TEST_WFP=1`, then
  `ci/windows_service_test.ps1`: install, pipe, tamper protection, strict
  mode, uninstall) and `unit` (`go test ./...`, slow: SQLite on the
  runner's disk). The runner is a disposable administrator's machine.
  GitHub's job provisioner runs from `AppData\Local\Temp`: the script
  exempts the folders of such programs before strict mode, or strict mode
  closes them and the job is cancelled without a log.
- `ci/windows_desktop_test.ps1 kc9k.exe` (W27, the `system` job's
  "Desktop checks"): the runner's session is signed in, so the session
  agent, hosts and firewall against a real site, the Edge policy, a
  blocked app and a blocked window title closed through the agent, and
  dark mode written to the signed-in person's settings are checked there.
- `ci/windows_vm_test.ps1 kc9k.exe [-Reboots]`: what needs a desktop or a
  person (agent, titles, apps, notifications, the challenge in a console
  and in its window, the lock freeze, takeown against strict mode, gamma,
  dark mode; with -Reboots the Safe Mode and recovery steps). In a VM
  with a snapshot only.
- Locally: `GOOS=windows CGO_ENABLED=0 go build ./... && GOOS=windows go vet ./...`
  (vet compiles the Windows tests too). Test helpers that capture output
  through `os.Pipe` read it while the command runs: a Windows pipe holds
  only 4 KB.

- Local commands (`service`, `agent`, `install`, `finish-uninstall`,
  `watchdog`) are exempt from access passwords: they can't read the
  database (SYSTEM's alone). Before, the session agent (run as the person)
  ended at once and was started again every 3 s, and a reinstall over
  kept data failed; found by the desktop checks (W27).
- Wording (W16): help texts are written for Linux; on Windows
  `rewordForWindows` (called first in `forwardIfNeeded`) replaces each
  Linux-only name by Windows' own from `cli/wording.go`'s table (the
  hosts file, the service, Safe Mode instead of GRUB and
  `kc9k.nofreeze`, Task Manager, the Guest account); `strict` and
  `no-escape` get Windows texts of their own (`windows_windows.go`).
  `TestWindowsWording` fails on any Linux name left.

## Service, pipe, agent, install (W1)

- Packages: `internal/platform` (data folder, privileged, caller; on
  Windows from `KC9K_CALLER_*`, which only the service sets),
  `internal/winapi` (Win32 calls x/sys lacks; overlapped pipe ends,
  `Listen`/`DialPipe`, server/client process IDs), `internal/winsvc` (the
  command pipe: `proto.go` framing, shared and tested on Linux;
  `server_windows.go`, `client_windows.go`, `install_windows.go`),
  `internal/winagent` (agents: `proto.go`; `manager_windows.go` in the
  service; `agent_windows.go`, `kc9k agent`), `cli/windows_windows.go`
  (`service`, `agent`, `install`, `uninstall`, `finish-uninstall`, and
  `forwardIfNeeded`, called first in `Execute`).
- Forwarding: every command but `service`, `agent`, `install`, `help`,
  `completion` goes to `\\.\pipe\kc9k` unless the process is LocalSystem
  or `KC9K_DB` is set (tests). The client checks the pipe's server process
  is the service's (`ServicePID` from the service manager) before sending.
  Frames: type byte, 4-byte length, payload; request (JSON: args, working
  folder, console size and a random nonce), input, input end, Ctrl-C;
  output, errors, exit code.
- The service (`server_windows.go`): pipe DACL SYSTEM all, signed-in users
  read/write but not create instances, network denied; first instance
  `FILE_FLAG_FIRST_PIPE_INSTANCE`, remote clients rejected; at most 32
  commands at once. Caller from `ImpersonateNamedPipeClient` + thread
  token: SID, RID as uid, name, session, Administrators member (refused
  otherwise). The command runs as `kc9k.exe <args>` (child, LocalSystem)
  with the service's environment minus `KC9K_*`, plus `KC9K_CALLER_SID`,
  `_UID`, `_USER`, `_SESSION`, `_TOKEN` (an inherited impersonation token
  handle: `cli/readcaller_windows.go` opens caller-named files while
  impersonating it) and `KC9K_TERMINAL` (cols,rows,nonce) for a console.
  Ctrl-C or a closed client kills the command.
- Consoles: the client enables VT output and keeps input cooked; a
  command that needs raw input (the text lock's challenge) or no echo
  (passwords) writes `ESC ]7799;kc9k;<nonce>;raw|cooked|echo-off|echo-on
  BEL`, which the client strips and obeys (`lock/term_windows.go`,
  `cli/term_windows.go`; `ControlFilter`). Without the nonce, printed
  text can't switch the console. The client waits for input before each
  read, so a switch to raw applies to the next key.
- Agents (`manager_windows.go`): every 3 s, each session with a signed-in
  user (`WTSQueryUserToken`) gets `kc9k agent` (`CreateProcessAsUser`,
  `winsta0\default`, the user's environment); one that ended is started
  again (`Manager.Restarts` when ended from outside; W5 counts it). Agents connect to
  `\\.\pipe\kc9k-agent`; accepted only from the installed `kc9k.exe`
  image, in the session its hello names, with its own PID. One JSON line
  per message: requests with IDs, replies, events (ID 0). Commands the
  service runs reach agents through `\\.\pipe\kc9k-agents-internal`
  (SYSTEM only): `winagent.Call`, `winagent.Sessions` work from either.
  Ops so far: `status` (idle ms from `GetLastInputInfo`).
- Agent stability (W50–W52) *(By claude)*: `dispatch.go` (shared, tested
  on Linux) answers each request in its own goroutine but every handler
  under `recover()` (a panic is an error reply and a logged stack, never
  the agent's end); screen ops (gamma get/set, monitors, wallpaper, dark
  mode, Night light, look) one at a time under one mutex, a gamma set
  still waiting replaced by a newer one (its reply: 0 screens). Ramps are
  checked before they reach a screen (`CheckRamp`: 256 values per colour,
  never falling, top no lower than 1000–10000 K at 10 % software
  brightness allow, less a tenth). The agent puts the plain ramp back when
  it ends (the service stopped, is uninstalled, closed its connection) if
  it set another. `displays()` uses one package-level callback with its
  results under a mutex (Go keeps every `NewCallback` and stops at 2000:
  one per call ended the agent after about 2000 screen calls);
  `TestWinDisplaysManyCalls` makes 5000 of each. The repo's other
  callbacks (`enumCallback`, the challenge's `wndProc` under a `Once`)
  were already made once.
- Crash vs. tamper (§3.10) *(By claude)*: the service starts each agent
  with its standard output and error on a pipe only that end inherits
  (`spawnWithOutput`, `PROC_THREAD_ATTRIBUTE_HANDLE_LIST`); the first 200
  lines go to the service's log ("agent N: ..."), the last 16 KB tell how
  it ended (`classifyEnd`): exit code 2 with Go's "panic: "/"fatal error: "
  report is a crash (`Manager.Crashes`, logged, not tamper); the agent's
  own marker line (service closed the connection, exit 0; gave up, with an
  error) is ending by itself; anything else (TerminateProcess with any
  code, Task Manager, taskkill) is from outside (`Manager.Restarts`,
  which `guardPlatform` logs as tamper). `TestAgentEndCrashVsKill` runs
  each for real.
- Install (`install_windows.go`): Program Files\KC9K (`ProgramSDDL`:
  owner SYSTEM, SYSTEM full, Administrators and Users read/run, protected),
  ProgramData\kc9k (`DataSDDL`: SYSTEM only), `kc9k.exe` copied (a running
  one renamed `.old`, removed at reboot), PATH (Machine, broadcast),
  service `kc9k` (created once the one an uninstall just removed is gone,
  up to 30 s; `kc9k.exe service`, automatic, recovery: restart 0 s,
  5 s, 10 s, also on non-crash failures), started. The service exits as a
  failure when the installed `kc9k.exe` changes (size/time), so Windows
  starts the new one. Log: ProgramData\kc9k\kc9k.log (10 MB, one old
  copy).
- Uninstall: `kc9k uninstall` (in the service) runs `prepareUninstall`
  (Linux's prepare-uninstall), then starts a copy of itself in the temp
  folder (`finish-uninstall <pid>`), which waits for the command to end,
  deletes and stops the service, removes the program folder and the PATH
  entry. The data stays.
- Checks: `ci/windows_service_test.ps1 kc9k.exe` (install, forwarded
  commands and exit codes, caller-named files, the data folder's DACL,
  uninstall refused while locked, then done) runs on `windows-latest`.
  The agent needs a signed-in desktop: win-vm.

## Web layers (W2)

- Hosts: `hosts.go` (shared) with `hostsPath` =
  `%SystemRoot%\System32\drivers\etc\hosts`; every write ends in
  `hostsWritten` (`DnsFlushResolverCache`: the DNS Client caches hosts
  answers). SafeSearch lines (`SyncSafeSearch`) the same. Windows Defender
  may flag a hosts file that redirects some popular sites; KC9K never hides
  from it (SPEC §6): if it does, the firewall and policies still block.
- Firewall (`firewall_windows.go`, `winapi/wfp_windows.go`): the Windows
  Filtering Platform, not Windows Firewall. Persistent provider and
  sublayer (fixed GUIDs, weight 0xF000) of KC9K's own; filters at
  `ALE_AUTH_CONNECT_V4/V6`: blocked addresses in filters of 500 conditions
  (same field: ORed), DoH resolvers + remote port 443 or 853 (DNS over
  TLS and QUIC, W26; `doh.go`, shared).
  Monk mode's cut: in all four ALE connect/accept layers, permit loopback
  (weight 15) and block everything (weight 1). One transaction deletes
  every filter of the provider and adds the new ones. Persistent: in force
  from boot, before the service, and after it stops. `IsFirewallApplied`:
  the filters of the last apply all there, by name and count (after a
  restart: the two DoH filters). `wfpSecurity` (nil: the engine's default
  security) is W5's.
- Policies (`browser_windows.go`): Chromium browsers (Chrome, Chromium,
  Edge, Brave, Vivaldi) under `HKLM\SOFTWARE\Policies\<vendor>\<name>`:
  lists as subkeys of numbered strings, dictionaries as JSON strings,
  booleans and numbers as DWORDs (`writeRegPolicy`). The blocklist owns
  `URLBlocklist`, `URLAllowlist`, `DnsOverHttpsMode`,
  `BuiltInDnsClientEnabled`; the setup owns private windows
  (`IncognitoModeAvailability`, Edge `InPrivateModeAvailability`), guest,
  developer mode, devtools, the extension, forced search. Firefox family:
  KC9K's copy in ProgramData\kc9k\firefox\policies.json, written to
  `distribution\policies.json` of every installation found (usual Program
  Files folders, App Paths, per-user `AppData\Local\Mozilla Firefox`).
  LibreWolf's setting locks (`librewolfprefs.go`, shared): locks at the end
  of `Program Files\LibreWolf\librewolf.cfg`; cleaned overrides per
  account (ProfileList) in `Program Files\KC9K\share\librewolf\<name>.cfg`
  (users may read, only SYSTEM write), reached through `USERNAME`.
- One-click install (W18, `InstallBrowser`): winget as SYSTEM (the newest
  App Installer's `winget.exe` under WindowsApps, which SYSTEM has on no
  PATH), `--scope machine --silent`, packages `LibreWolf.LibreWolf` and
  `Google.Chrome`; a browser already there isn't installed again (Chrome:
  says it waits for the store listing). CI installs LibreWolf for real on
  manual runs (`workflow_dispatch`).
- Firefox's registry (W26, `firefoxreg_windows.go`): KC9K's
  policies.json is also mirrored into
  `HKLM\SOFTWARE\Policies\Mozilla\Firefox` (objects as keys, lists as
  keys of "1", "2"…, booleans and whole numbers as DWORDs), which the
  Store's Firefox and portable ones read; only the policies KC9K wrote
  (ProgramData\kc9k\firefox-registry.json) are taken out again. Firefox
  reads Group Policy before policies.json, so both say the same.
  LibreWolf reads its own key and keeps its distribution file.
- The extension in Chrome and Edge needs the Chrome Web Store listing's ID
  (`ChromeStoreExtensionID`, empty until the owner publishes it): only
  then are they supported browsers and given the extension by policy
  (`ExtensionSettings`, `ExtensionInstallForcelist` with the store's update
  URL). LibreWolf gets the bundled `.xpi` as on Linux.
- Processes (`procs_windows.go`): Toolhelp snapshot + full path
  (`QueryFullProcessImageName`), owner RID and service-account flag,
  session, start time; a parent counts only if it started before its
  child (IDs are reused). `tcpOwner` (`GetExtendedTcpTable`) maps a
  connection to its process: check-ins, statistics per user.
- Killing (W50) *(By claude)*: every Windows kill path goes through
  `killProcess` (what KC9K found itself: browsers, trees below a blocked
  app, people's programs; never a program under %SystemRoot% or with an
  unreadable path) or `killNamedProcess` (a process a rule names: the
  blocked app itself, a tamper tool such as taskmgr, curl naming a
  blocked site, a blocked window's owner, a VM's vmwp). Both refuse the
  hard deny-list `sessionCritical`: PID 0 and 4, `essentialWindowsApps`
  and `neverKillExtra` (dwm, csrss, winlogon, svchost, fontdrvhost,
  sihost, ctfmon, explorer, ShellHost, RuntimeBroker, dllhost...) in the
  Windows folder or unreadable, KC9K's own programs; a refusal is logged
  once per process ("kill refused"). `terminateProcess` (the seam tests
  replace) checks the deny-list again on the path its handle names (PIDs
  are reused). Tests: `TestWinKillDenyList`, `TestWinGuardNeverClosesWindows`.
- Extension guard (`extguard_windows.go`; `extguard_shared.go` is the
  shared verdict): a browser is a guarded program name, a renamed one by
  its version resource's OriginalFilename, or a program beside `xul.dll`
  or `chrome.dll`/`msedge.dll`, or with one in a version folder beside it
  (`Application\120.0.…\chrome.dll`); never Thunderbird or WebView2,
  never a program of Windows' own (`windowsOwnProgram`: under
  %SystemRoot%, a Microsoft package in WindowsApps) nor one whose path
  can't be read. *(By claude)* Laptop 2026-11-22 (W50): the former glob
  `<dir>\*\msedge.dll` matched `System32\Microsoft-Edge-WebView`, so
  every System32 program was a browser and the guard closed dwm (black
  screens, classic white windows), fontdrvhost, sihost, svchost... every
  5 s after the grace until the computer hung. `TestWinGuardSystem32NotBrowsers`
  lists System32 for real. "browser: closing" lines name the path. Main process: topmost ancestor with the same program. A
  browser started for remote control is closed at once (W11): its command
  line (`commandLine`) and environment (`processEnvironment`: the PEB's
  process parameters, x64 layout) checked by `remoteControlArgs`, which
  also takes Firefox's `/switch` and any case. Supported
  browsers: LibreWolf; Chrome and Edge once the store ID is set.
- Guest: Chromium guest mode is always off by the setup. "Close guest
  profiles" also keeps Windows' built-in Guest account (relative ID 501,
  found by ID since its name is translated) off, each cycle
  (`syncGuestAccount`, `winapi/accounts_windows.go`); ProgramData\kc9k\
  guest-off notes it was on, and only then is it turned on again when the
  setting goes off (W14).
- Checked on `windows-latest` (CI "Layers for real"): WFP blocks a blocked
  address and DoH on 443 with WSAEACCES, leaves others, replaces on
  re-apply, removes fully; registry policies round-trip; process
  snapshot, kill tree, TCP owner, browser recognition.

## Apps and desktop (W3)

- Apps (`apps_windows.go`): an entry is a program name (`steam`,
  `steam.exe`), a full path, a folder (ends in `\` or `/`), or a contents
  entry (`sha256:…`, size checked before hashing, cached by path, size and
  time; never Windows' own programs). Only people's processes
  (`winProc.person`); never `essentialWindowsApps` in the Windows folder,
  KC9K, or this process and its ancestors; children too, except under
  shells and Explorer (`noDescendWindows`). Protected folders: drive root,
  Windows, System32, SysWOW64, Program Files (both), Users, ProgramData.
  Polite: `polite_shared.go`'s state; windows asked through the agent
  (`OpClose` by process).
- Scripts (W12, `scripthosts.go`, shared and tested on Linux): an entry
  naming a script (`bot.py`, `bot`, `game.ps1`, a full path or a folder)
  matches the script host running it: Python (`py`, `pythonw`, `-m`
  modules), PowerShell (`-File`, a `.ps1` first), cmd (`/c` a `.bat` or
  `.cmd`), wscript/cscript, mshta, node, java `-jar`, AutoHotkey
  (`hostScript`, working folder from the PEB, `processCwd`); never a
  script in the Windows folder. A shell matched by its script is killed
  with its children. Fetch tools are known by their script or module too
  (`python -m yt_dlp` is yt-dlp, `scriptNames`).
- Allowed apps (`CloseAllExcept`): kills people's programs that aren't
  allowed, started by an allowed one, Windows' own (Windows folder) or
  KC9K's. Tor: Tor Browser folders and `tor.exe`. Tools naming blocked
  domains (`KillNamingFor`): command lines from
  `NtQueryInformationProcess(ProcessCommandLineInformation)`.
- Sessions (`desktop_windows.go`): WTS sessions with a user token;
  locked from `WTSQuerySessionInformation(WTSSessionInfoEx)` (Windows'
  answer, never the session's programs). Idle and the app in front from the
  agent (`OpStatus`, `OpForeground`; a window's process believed only if
  it is the session user's). Notifications (`OpNotify`: a
  notification-area balloon, shown as a toast), questions (`OpConfirm`:
  MessageBox), locking (`OpLock`: `LockWorkStation`), titles
  (`OpWindows`: `EnumWindows`, visible unowned windows; `OpClose`:
  `WM_CLOSE`), Explorer windows showing a folder entry close (never
  killed). Logout: `WTSLogoffSession`; power off: `ExitWindowsEx` with
  `SeShutdownPrivilege`. Logout freeze: `HoldLogins` writes
  ProgramData\kc9k\hold-logins; while it holds, the service signs out
  every session instead of starting its agent (`winagent.SessionFilter`).
- Custom actions (`RunCustom`): Explorer opens, `cmd /c` runs, as the
  first signed-in person (`CreateProcessAsUser`).
- Hotkeys: `SyncHotkeys` saves ProgramData\kc9k\hotkeys.json (GTK
  accelerators turned into `RegisterHotKey` modifiers and virtual keys)
  and gives them to every agent (and each one that connects); a press is
  an agent event, and the service runs `kc9k hotkey run <id>` for that
  session's person.
- Places (W10, `enforce/wifi_windows.go`, `winapi/wlan*.go`): the
  Native Wifi API (`WlanEnumInterfaces`, `WlanScan`,
  `WlanGetNetworkBssList`; layouts read by `ParseBSSList`, tested on
  Linux). No WLAN service (no Wi-Fi, Windows Server): nothing seen, which
  counts as at every place. Wi-Fi counts as on only with an enabled adapter
  and a radio whose software switch is on (Windows' "Wi-Fi off" and
  airplane mode leave the adapter enabled); `SetWifiRadio(true)` enables
  the adapters and turns the switches on (`WlanSetInterface`), so
  `keepWifiOn` holds against both.
- Monk mode's physical cut (`netcut_windows.go`): `Get-NetAdapter
  -Physical` (PowerShell: its values aren't translated), Wi-Fi by
  physical media type; `Enable-/Disable-NetAdapter`.
- The challenge window (`winagent/challenge_windows.go`): a Win32 window
  (no text field) the caller's session agent opens (`OpChallenge`, up to
  4 h); its events are the GTK window's (`k`, `b`, `n`, `p`, `s`, `c`).
  Raw Input (`RIDEV_INPUTSINK`): a key down with no device is injected
  (SendInput, on-screen keyboards): `s`. Ctrl+V and Shift+Insert: `p`.
  `lock/window_windows.go` reads the events as the Linux window's.

## Tamper protection against an administrator (W5)

- `blocker/tamper_windows.go`; `enforce/sys_windows.go` (`guardPlatform`,
  each cycle after `enforceLockdown`): everything found changed is put
  back and logged as a tamper event (§3.10, counted like a daemon
  restart).
- Always (`GuardService`, service only): the service's security
  descriptor `ServiceSDDL` (Administrators: query, start, read; never stop,
  pause, change, delete, change permissions or owner); automatic start and
  its program path; restart on failure; Safe Mode registration
  (`SafeBoot\Minimal|Network\kc9k` = Service: the service runs in Safe Mode
  too); the watchdog task `\KC9K\Watchdog` (SYSTEM, at boot and every
  minute, `kc9k watchdog` starts or recreates the service; the task's
  XML asks that administrators only read it, but Task Scheduler lets an
  elevated administrator delete it: it is made again within a cycle and
  counted, checked on `windows-latest`); the program and
  data folders' security (not while strict mode holds them; compared by
  owner, protection and explicit entries, `sameSecurity`: Windows prints
  back flags of its own, and a text comparison counted the folders as
  changed every cycle until 2026-11-22). A session
  agent ended while its session lasts counts too.
- Strict mode (`ApplyLockdown`, from `enforceLockdown` as on Linux):
  deny entries (`denyACEs`) for how a person's programs signed in —
  Interactive, Network, Batch (S-1-5-3) — not for Administrators, whose
  group LocalSystem's token holds too (the service would deny itself;
  seen on `windows-latest`). Files and folders `0xd0156` (write, delete,
  permissions, owner; Windows prints it `DCLCRPDTCRSDWDWO`, so entries are
  compared in its own form, `canonACE`), keys `0xd0006` (set, create,
  delete, permissions, owner) on the service's key, `Control\SafeBoot`,
  `BCD00000000` (boot settings), `Recovery\ReAgent.xml`, the Chromium
  policy keys, the hosts file, the program folder and Firefox-family
  `distribution` folders. The security found is kept in
  ProgramData\kc9k\lockdown.json and put back by `RemoveLockdown`; a deny
  entry removed while it holds is put back and counted
  (`LockdownRepairs`). SYSTEM is never denied. People's downloaded and
  portable programs (`userProgram`: a profile's folders other than
  AppData, `AppData\Local\Temp`, removable drives) are closed
  (`CloseNoexecPrograms`), except exempt folders; per-user installs in
  AppData keep running. They don't start either (W25,
  `noexec_windows.go`): `ApplyNoexec` keeps Software Restriction Policies
  path rules (registry only, so Home has them; every user,
  administrators included): each profile disallowed, its AppData allowed
  but `AppData\Local\Temp` disallowed, the Public profile, Windows' Temp,
  removable drives (not Windows' Temp: SYSTEM's programs use it), and
  exempt folders allowed (long paths: SRP doesn't match 8.3 names); rules
  synced each cycle, running programs told to read them again
  (`RefreshPolicyEx`; a shell running before the rules still started a
  program in CI, so processes started earlier may keep the old policy:
  `CloseNoexecPrograms` closes what they start within a cycle),
  under keys named `{4B43394B-…}`. What SRP needs to act and is missing
  is set (`ensureSRPOn`: DefaultLevel unrestricted, TransparentEnabled 1
  so DLLs are left out, PolicyScope 0, ExecutableTypes; Windows Server
  ships the key with only `authenticodeenabled`, which leaves SRP off),
  noted in ProgramData\kc9k\srp-made (the key, or the values added); the
  key is a lockdown target. `RemoveNoexec` removes KC9K's rules and
  exactly what it added; `RemoveLockdown` does
  it first, since putting the rest back runs PowerShell. Every PowerShell
  the service runs has a time limit (90 s; the firmware's 5 min).
- Policy refresh (W50): `refreshPolicy` (a variable; forced
  `RefreshPolicyEx`, which tells every window "Policy" changed) runs only
  after a rule or value really changed. Strict mode not acting (every
  cycle) runs `RemoveLockdown` and `RemoveNoexec`: with nothing of KC9K's
  there they touch nothing and refresh nothing. Before, each cycle
  forced two machine policy refreshes. RP_FORCE stays: the rules are
  plain registry values, not a GPO, so an unforced refresh tells no
  program. `TestStrictSRPRefreshOnlyOnChange` (real registry): 0 calls
  idle, 1 after a real removal. *(By claude)*
- Strict mode's ways to SYSTEM (W20–W22, `strict_windows.go`;
  `applyStrictExtras` from `ApplyLockdown`, `removeStrictExtras` first in
  `RemoveLockdown`, whose failure is logged and retried but never keeps
  the deny entries on). State: ProgramData\kc9k\strict-extras.json
  (start time, DisableMSI before, WMI consumers and accounts at the start,
  accounts turned off, user rights removed). Every cycle: the service
  manager's DACL denies the three logons `0xc0022` (create service, boot
  configuration, permissions, owner); `DisableMSI`=2 (its key is a
  lockdown target); privileges stripped from every person's process token
  (`SE_PRIVILEGE_REMOVED`, once per process: backup, restore, take
  ownership, debug, load driver, firmware variables, manage volume,
  relabel, and the TCB ones); accounts not on, or not there, at the
  start turned off. Once: those rights taken from Administrators' and
  Backup Operators' user rights (LSA; from the next sign-in), given back
  after. Every 30 s, in a goroutine beside the cycle (`strictHeavy`; a
  step over 5 s is logged; what it puts back is reported by the next
  cycle; the PowerShell checks, Task Scheduler's and WMI's, only on the
  first pass and every 5 minutes: on GitHub's runner one takes 30–90 s).
  Ending strict mode undoes the quick parts at once and these beside the
  cycle once a pass running has ended (`startHeavyUndo`, marker
  ProgramData\kc9k\strict-heavy); `kc9k uninstall` waits for it
  (`WaitStrictUndone`): each service's DACL denies `0xd0002` (configuration,
  delete, permissions, owner; `0xd0062` adds stop and pause for BFE,
  Dnscache, Winmgmt, Schedule, EventLog, WlanSvc, W32Time), not KC9K's
  own (GuardService) nor protected ones it can't open; Task Scheduler's
  root folder denies `0xd0116` (registering tasks); task files written
  since the start that run as SYSTEM or a service account, outside
  `\Microsoft\Windows\` and `\KC9K\`, are deleted (`schtasks /Delete`);
  WMI event consumers made since the start (`root\subscription`, listed
  every 5 minutes) are deleted with their bindings (denying the namespace
  itself failed on GitHub's runner, "Invalid parameter", and was dropped:
  a first build's entry is taken out once, marker without "v2"). The `Services` key
  tree is a lockdown target (deny entries inherited by every service's
  key). Each put back or deleted is counted as tamper. Virtual machines
  (W24, every cycle, `closeVirtualMachines`): people's VM, WSL, Sandbox,
  QEMU and Docker programs (`vmPrograms`, `isVMProcess`; WSL's bash only
  in System32) closed with their children, Hyper-V's VM workers (`vmwp`)
  ended, and the services behind them stopped (`vmServices`: LxssManager,
  WSLService, vmcompute, vmms, CmService, docker…); logged, not counted.
- Clock and time zone (W23, `clockguard_windows.go`, from `guardPlatform`):
  with tamper clock on and anything locked, the first cycle keeps the
  time zone (`GetDynamicTimeZoneInformation`) and the wall clock minus
  the boot clock in ProgramData\kc9k\clock-guard.json. A changed zone is
  set back; a clock moved more than 2 minutes against the boot clock is
  set back unless internet time (the clock check's, at most once a
  minute) confirms the new time, which then becomes the reference. Each
  counted as tamper; the file goes when the guard ends.
- No-escape (`ApplyBootLock`): `reagentc /disable` (if it was on),
  `bcdedit /set {current} recoveryenabled No`, `bootstatuspolicy
  IgnoreAllFailures`, `{bootmgr} displaybootmenu No`; ProgramData\kc9k\
  bootlock.json keeps whether recovery was on; `RemoveBootLock` undoes.
- Guided setup (W15, `cli/guidedsetup_windows.go`): instead of Linux's
  groups and root password, `other-admins` (enabled local accounts in
  Administrators, found by SID: `winapi.AdministratorSIDs`) and
  `builtin-admin` (relative ID 500 on), each with its `net user … /active:no`;
  a `safe-mode` info step before no-escape; other systems from `bcdedit
  /enum osloader` (element names aren't translated).
- Recovery path (W13, principle 4, R-ESCAPE-1): Safe Mode is Windows'
  recovery boot (`SafeModeBoot`: `SafeBoot\Option` exists only then).
  `KernelCmdline` carries `kc9k.nofreeze` in Safe Mode, so freezes and
  strict mode's lockdown are skipped for that boot unless no-escape is on
  (blocks stay: the service runs in Safe Mode); `kc9k recover` works there
  (`cli/recover_windows.go`). While factory reset protection keeps the
  recovery environment off and no-escape isn't on, the F8 menu is on
  (`bootmenupolicy Legacy`, `recoveryMenuOn`); no-escape sets it back to
  Standard, and so does the end of factory reset protection.
- Factory reset protection while anything is locked
  (`SyncRecoveryGuard`): the recovery environment off (Reset this PC
  needs it), back on after unless no-escape holds it; turned on by hand
  during a lock: put back and counted. Windows' image writers join its
  apps (Rufus, balenaEtcher, Ventoy, Media Creation Tool…).
- Task managers (tamper setting): `platformTaskManagers` (Task Manager,
  Process Explorer/Hacker, System Informer, MMC consoles such as Services
  and the editors, regedit, sc, bcdedit, reagentc, schtasks, taskkill,
  msconfig…).
- The WFP objects keep the engine's default security.
- Firmware lock (W19, `firmware_windows.go`, `firmware_wmi.go`): the
  firmware's own WMI interfaces, which Linux's firmware-attributes
  drivers use too: Dell `root\dcim\sysman\biosattributes` and
  `wmisecurity`, Lenovo `root\wmi` (`Lenovo_BiosSetting`,
  `GetBiosSelections`, `SetBiosSetting` + `SaveBiosSettings`,
  `SetBiosPassword` "pap"), HP `root\HP\InstrumentedBIOS`
  (`HP_BIOSEnumeration`, `SetBIOSSetting`, "Setup Password",
  `<utf-16/>`). A read script prints JSON (`wmiStatus` reads it, tested on
  Linux; drivers named as Linux's: dell-wmi-sysman, thinklmi,
  hp-bioscfg); a write script takes a `wmiRequest` on stdin (passwords
  never on a command line). Same purposes and order as Linux; Lenovo
  passwords can't hold "," or ";". No vendor tool needed.
- An administrator can still win: take ownership with their privileges,
  rewrite permissions, then stop things; each step is undone within a
  cycle and counted. Nothing hides from Windows Defender.

## Circadian rhythm and wallpapers (W6)

- `blocker/circadian_windows.go`; agent ops in `winagent/look_windows.go`.
- Gamma ramps (colour temperature, software brightness): 256 values per
  colour, computed in the service from `RampFunc`, set by each session's
  agent on every screen (`SetDeviceGammaRamp` on a DC per display);
  `GdiIcmGammaRange`=256 (HKLM ICM) so warm ramps aren't refused.
  `ReadGamma` compares as on Linux.
- Wallpaper: copied to Program Files\KC9K\share\wallpapers (people can't
  read the data folder), set by the agent
  (`SystemParametersInfo(SPI_SETDESKWALLPAPER)`). Dark mode: HKCU
  `Themes\Personalize` `AppsUseLightTheme`, `SystemUsesLightTheme`, then
  `WM_SETTINGCHANGE "ImmersiveColorSet"`. Signed-out people can't be
  reached (their settings aren't loaded). `DesktopLook` keys: wallpaper,
  apps-light, system-light.
- Night light: read from the user's settings store (byte 18 of
  `bluelightreductionstate` is 0x15 while on); not switched
  (`TurnOffNightLight` asks the person).
- Brightness: a laptop panel through WMI (`WmiMonitorBrightness`,
  `WmiSetBrightness`, PowerShell); external screens through DDC/CI in
  the agent (dxva2 `Get/SetMonitorBrightness`), listed at most once a
  minute, `Display` = "session:index".
- Sleep hook: ProgramData\kc9k\sleep-hook while on; the service accepts
  power events and on suspend runs `kc9k wallpaper before-sleep`.
