# Windows: layers, apps, desktop, circadian

- Covers: `firewall_windows.go`, `winapi/wfp_windows.go`, `browser_windows.go`, `firefoxreg_windows.go`, `procs_windows.go`, `extguard_windows.go`, `apps_windows.go`, `desktop_windows.go`, `enforce/wifi_windows.go`, `netcut_windows.go`, `lock/window_windows.go`, `winagent/challenge_windows.go`, `blocker/circadian_windows.go`, `winagent/look_windows.go`
- SPEC: §3.1, §3.13, §4.10, §6 2026-11-21
- Connects to: [windows](windows.md) (service, agent, pipe), [windows-tamper](windows-tamper.md), [layers-net](layers-net.md), [browsers](browsers.md), [apps-titles](apps-titles.md), [device](device.md), [circadian](circadian.md)

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
- Supported browsers (`extguard_windows.go`, 2.12): LibreWolf (bundled
  `.xpi`); Chrome and Edge once `ChromeStoreExtensionID` is set (the only
  step left, 2.17: `ExtensionSettings` force-install from the Chrome Web
  Store update URL, pinned by Chrome's `toolbar_pin` / Edge's
  `toolbar_state`, plus `ExtensionInstallForcelist`); Firefox in Program
  Files and the Store's (`WindowsApps\Mozilla.Firefox_*\VFS\ProgramFiles\
  Firefox Package Root\firefox.exe`) once the signed `.xpi` exists
  ([browsers](browsers.md)). `Waiting()` names what is missing; until then
  `Installed()` is false and the guard treats it as unsupported. Per-user
  AppData Firefox gets the policies but is no supported browser. The
  Microsoft Bing app is Edge on Windows. `supportedExeOf` matches the path
  only (`*` = one folder), no folder read per process.
- One-click install (W18, `InstallBrowser`): winget as SYSTEM (newest
  `winget.exe` in WindowsApps), `--scope machine --silent`; skipped if
  present; librewolf, chrome, edge, firefox (a found browser still waiting
  for its extension says so).
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
  fixed frame, close only = cancel, system window brush), 900×560 and
  Consolas 16 px at 100 %, scaled to its screen's DPI and again on
  `WM_DPICHANGED` (kc9k.exe is per-monitor aware, [windows](windows.md)), no text field, opened by the caller's agent (`OpChallenge`, ≤ 4 h); GTK's
  events (`k`, `b`, `n`, `p`, `s`, `c`). Raw Input: key with no device
  (SendInput, on-screen keyboard) → `s`; Ctrl+V/Shift+Insert → `p`.
  Read by `lock/window_windows.go`. The asking command ending (killed,
  Ctrl-C) closes it: the service sees the command's internal pipe close
  and sends `OpChallengeCancel` (else the window stayed and refused every
  later unlock: "open already"). Another process finds it by title, not
  class. Gamma a driver refuses without an error code: "the display driver
  refused the colour ramp" (GitHub's runner does).

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
