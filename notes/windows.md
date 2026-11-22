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
- The hardware clock (`/sys/class/rtc`) has no Windows reader: the clock
  check uses the internet's time only.

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
- `ci/windows_vm_test.ps1 kc9k.exe [-Reboots]`: what needs a desktop or a
  person (agent, titles, apps, notifications, the challenge in a console
  and in its window, the lock freeze, takeown against strict mode, gamma,
  dark mode; with -Reboots the Safe Mode and recovery steps). In a VM
  with a snapshot only.
- Locally: `GOOS=windows CGO_ENABLED=0 go build ./... && GOOS=windows go vet ./...`
  (vet compiles the Windows tests too). Test helpers that capture output
  through `os.Pipe` read it while the command runs: a Windows pipe holds
  only 4 KB.

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
  again (`Manager.Restarts`; W5 counts it). Agents connect to
  `\\.\pipe\kc9k-agent`; accepted only from the installed `kc9k.exe`
  image, in the session its hello names, with its own PID. One JSON line
  per message: requests with IDs, replies, events (ID 0). Commands the
  service runs reach agents through `\\.\pipe\kc9k-agents-internal`
  (SYSTEM only): `winagent.Call`, `winagent.Sessions` work from either.
  Ops so far: `status` (idle ms from `GetLastInputInfo`).
- Install (`install_windows.go`): Program Files\KC9K (`ProgramSDDL`:
  owner SYSTEM, SYSTEM full, Administrators and Users read/run, protected),
  ProgramData\kc9k (`DataSDDL`: SYSTEM only), `kc9k.exe` copied (a running
  one renamed `.old`, removed at reboot), PATH (Machine, broadcast),
  service `kc9k` (`kc9k.exe service`, automatic, recovery: restart 0 s,
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
  (same field: ORed), DoH resolvers + remote port 443 (`doh.go`, shared).
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
- Extension guard (`extguard_windows.go`; `extguard_shared.go` is the
  shared verdict): a browser is a guarded program name, a renamed one by
  its version resource's OriginalFilename, or a program beside `xul.dll`
  or `chrome.dll`/`msedge.dll` (portable browsers); never Thunderbird or
  WebView2. Main process: topmost ancestor with the same program. A
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
  data folders' security (not while strict mode holds them). A session
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
  AppData keep running; nothing is mounted
  (`ApplyNoexec` does nothing).
- No-escape (`ApplyBootLock`): `reagentc /disable` (if it was on),
  `bcdedit /set {current} recoveryenabled No`, `bootstatuspolicy
  IgnoreAllFailures`, `{bootmgr} displaybootmenu No`; ProgramData\kc9k\
  bootlock.json keeps whether recovery was on; `RemoveBootLock` undoes.
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
- The WFP objects keep the engine's default security; firmware lock
  (Dell, Lenovo, HP tools): not built.
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
