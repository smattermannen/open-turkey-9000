# Windows: tamper protection, strict mode, no-escape

- Covers: `blocker/tamper_windows.go`, `strict_windows.go`, `noexec_windows.go`, `clockguard_windows.go`, `firmware_windows.go`, `firmware_wmi.go`, `enforce/sys_windows.go`, `cli/guidedsetup_windows.go`
- SPEC: §3.8, §3.10, §4.10 Device protection, R-ESCAPE-1, §6 2026-11-21
- Connects to: [windows](windows.md) (service), [windows-layers](windows-layers.md), [device](device.md), [locks](locks.md), [browsers](browsers.md)

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
  parts and service DACLs at once, the rest after the running pass, which
  stops between steps once ended (`startHeavyUndo`, `undoHeavy`, marker
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
