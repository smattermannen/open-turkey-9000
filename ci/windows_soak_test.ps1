# Soak test of KC9K on Windows (ROADMAP W50-W52): runs phases for some
# minutes each and measures what froze the owner's laptop: Group Policy
# refreshes, PowerShell and kc9k.exe started by the service, agent
# restarts, tamper events, CPU, handles and memory, display driver resets,
# crashes, the gamma ramp, and browsers "closed" that are Windows' own.
#   pwsh ci/windows_soak_test.ps1 -Kc9k kc9k.exe [-Minutes 3]
#        [-Phases idle,circadian,play,settings] [-Gui kc9k-gui.exe] [-Out dir]
# Runs in Windows PowerShell 5.1 and pwsh 7, as an administrator, in the
# signed-in session. It never touches a lock, monk mode, Frozen Chicken,
# strict mode, no-escape, factory reset protection, the firmware lock, any
# password, Wi-Fi or network adapters, logout, shutdown or the lock screen,
# task-manager blocking or bypass prevention. It switches active blocks off
# for its run and every setting it changes back afterwards, and puts back
# the plain gamma ramp, dark mode and the wallpaper. "settings" switches
# "block unsupported browsers" and "close browsers if the extension stops"
# on and off: browsers without the extension may be closed meanwhile.
# Installs KC9K when it isn't (and removes it again at the end).
# Stops and puts everything back at once on: CPU over 90 % for 30 s, more
# than 5 powershell.exe at once, a display driver reset (Display 4101) or
# LiveKernelEvent, the GUI ending or not answering, an agent restart.
# Prints one table; exits 1 when any threshold is crossed.
param(
    [Parameter(Mandatory = $true)][string]$Kc9k,
    [double]$Minutes = 3,
    [string[]]$Phases = @('idle', 'circadian', 'play', 'settings'),
    [string]$Gui = '',
    [string]$Out = ''
)
$ErrorActionPreference = 'Continue'
$Phases = @($Phases | ForEach-Object { $_ -split ',' } | ForEach-Object { $_.Trim().ToLower() } | Where-Object { $_ })
foreach ($p in $Phases) {
    if (@('idle', 'circadian', 'play', 'settings') -notcontains $p) { Write-Host "unknown phase $p"; exit 2 }
}
if (-not $Out) { $Out = Join-Path $env:TEMP ("kc9k-soak-" + (Get-Date -Format 'yyyyMMdd-HHmmss')) }
New-Item -ItemType Directory -Force -Path $Out | Out-Null
$Kc9k = (Resolve-Path $Kc9k).Path
$sampleSeconds = 5

function Note([string]$text) {
    $line = "[$(Get-Date -Format HH:mm:ss)] $text"
    Write-Host $line
    Add-Content -Path (Join-Path $Out 'soak.log') -Value $line
}

# --- KC9K itself -----------------------------------------------------------

$installedHere = $false
if (-not (Get-Service kc9k -ErrorAction SilentlyContinue)) {
    Note 'installing KC9K'
    & $Kc9k install | Out-Host
    if ($LASTEXITCODE -ne 0) { Write-Host '::error::install failed'; exit 1 }
    $installedHere = $true
    Start-Sleep -Seconds 20
}
$K = Join-Path $env:ProgramFiles 'KC9K\kc9k.exe'
if (-not (Test-Path $K)) { $K = $Kc9k }

function Kc([string[]]$a) {
    $text = (& $K @a 2>&1 | Out-String).Trim()
    $code = $LASTEXITCODE
    Add-Content -Path (Join-Path $Out 'commands.log') -Value ("kc9k " + ($a -join ' ') + " -> $code`n$text")
    return $code
}
function KcOut([string[]]$a) { return (& $K @a 2>$null | Out-String) }
function GuiState { try { return (KcOut @('gui-state')) | ConvertFrom-Json } catch { return $null } }
function CircadianState { try { return (KcOut @('circadian', 'show', '--json')) | ConvertFrom-Json } catch { return $null } }
function OnOff([bool]$v) { if ($v) { return 'on' } else { return 'off' } }

function ServicePid {
    $s = Get-CimInstance Win32_Service -Filter "Name='kc9k'" -ErrorAction SilentlyContinue
    if ($s) { return [int]$s.ProcessId } else { return 0 }
}

# The service's log, copied by a SYSTEM task (the data folder is SYSTEM's).
function ServiceLog {
    $log = Join-Path $env:ProgramData 'kc9k\kc9k.log'
    # Paths without spaces: quotes inside /TR don't survive Windows
    # PowerShell 5.1's argument passing.
    $copy = Join-Path $env:SystemRoot 'Temp\kc9k-soak-service.log'
    Remove-Item $copy -ErrorAction SilentlyContinue
    schtasks.exe /Create /TN kc9ksoaklog /RU SYSTEM /SC ONCE /ST 23:59 /F /TR "cmd /c copy /y $log $copy && icacls $copy /grant Everyone:R" 2>$null | Out-Null
    schtasks.exe /Run /TN kc9ksoaklog 2>$null | Out-Null
    for ($i = 0; $i -lt 20 -and -not (Test-Path $copy); $i++) { Start-Sleep -Milliseconds 500 }
    Start-Sleep -Milliseconds 500
    schtasks.exe /Delete /TN kc9ksoaklog /F 2>$null | Out-Null
    if (Test-Path $copy) { return @(Get-Content $copy) } else { return @() }
}
function LogSince([string[]]$lines, [datetime]$since) {
    $out = @()
    foreach ($l in $lines) {
        if ($l -match '^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})') {
            $t = [datetime]::ParseExact($Matches[1], 'yyyy/MM/dd HH:mm:ss', $null)
            if ($t -ge $since) { $out += $l }
        }
    }
    return $out
}

# --- The screen: gamma ramp, dark mode, wallpaper --------------------------

Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class Kc9kSoakScreen {
    [DllImport("user32.dll")] static extern IntPtr GetDC(IntPtr hwnd);
    [DllImport("user32.dll")] static extern int ReleaseDC(IntPtr hwnd, IntPtr dc);
    [DllImport("gdi32.dll")] static extern bool GetDeviceGammaRamp(IntPtr dc, ushort[] ramp);
    [DllImport("gdi32.dll")] static extern bool SetDeviceGammaRamp(IntPtr dc, ushort[] ramp);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] static extern bool SystemParametersInfo(uint action, uint param, string value, uint flags);
    public static ushort[] Ramp() {
        IntPtr dc = GetDC(IntPtr.Zero);
        ushort[] r = new ushort[768];
        bool ok = GetDeviceGammaRamp(dc, r);
        ReleaseDC(IntPtr.Zero, dc);
        return ok ? r : null;
    }
    public static bool Plain() {
        ushort[] r = new ushort[768];
        for (int i = 0; i < 256; i++) { r[i] = r[256 + i] = r[512 + i] = (ushort)(i * 257); }
        IntPtr dc = GetDC(IntPtr.Zero);
        bool ok = SetDeviceGammaRamp(dc, r);
        ReleaseDC(IntPtr.Zero, dc);
        return ok;
    }
    public static bool Wallpaper(string path) { return SystemParametersInfo(0x14, 0, path, 3); }
}
'@

# GammaSane: 256 rising values per colour, not darker than the clips allow
# (10 % software brightness at the warmest colour), "" when sane.
function GammaProblem {
    $r = [Kc9kSoakScreen]::Ramp()
    if ($null -eq $r) { return '' } # no screen to read (a service session)
    $floor = @(4000, 1000, 20)
    for ($c = 0; $c -lt 3; $c++) {
        for ($i = 1; $i -lt 256; $i++) {
            if ($r[$c * 256 + $i] -lt $r[$c * 256 + $i - 1]) { return "colour $c falls at $i" }
        }
        if ($r[$c * 256 + 255] -lt $floor[$c]) { return "colour $c tops at $($r[$c * 256 + 255])" }
    }
    return ''
}

$personalize = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize'
$lookBefore = Get-ItemProperty $personalize -ErrorAction SilentlyContinue
$wallpaperBefore = (Get-ItemProperty 'HKCU:\Control Panel\Desktop' -ErrorAction SilentlyContinue).WallPaper

# --- What the run changes, to put back -------------------------------------

$state = GuiState
$circ = CircadianState
if ($null -eq $state -or $null -eq $circ) { Write-Host '::error::kc9k gui-state or circadian show --json failed'; exit 1 }
$notifyBefore = @{}
foreach ($line in ((KcOut @('notifications')) -split "`n")) {
    if ($line -match '^\s*([a-z-]+)\s+(on|off)\s*$') { $notifyBefore[$Matches[1]] = $Matches[2] }
}
$tamperNames = @('clock', 'installer', 'guest', 'extension-stop', 'unsupported-browsers')
$tamperBefore = @{
    'clock' = [bool]$state.tamper_clock; 'installer' = [bool]$state.tamper_installer; 'guest' = [bool]$state.tamper_guest
    'extension-stop' = [bool]$state.tamper_extension_stop; 'unsupported-browsers' = [bool]$state.unsupported_browsers
}
$browserNames = @('safesearch-google', 'safesearch-bing', 'safesearch-duckduckgo', 'safesearch-youtube', 'force-search', 'block-devtools', 'no-private')
$browserBefore = @{}
foreach ($n in $browserNames) { $browserBefore[$n] = [bool]$state.browser.$n }
$optionsBefore = @{}
foreach ($p in $state.options.PSObject.Properties) { $optionsBefore[$p.Name] = [bool]$p.Value }
$statsBefore = [bool]$state.stats
$stopped = @()
$restored = $false

function PointsText($lane) {
    $parts = @()
    foreach ($pt in $lane.points) {
        $m = [int][math]::Round([double]$pt.minute)
        $text = '{0:D2}:{1:D2}={2}' -f [int][math]::Floor($m / 60), ($m % 60), ([double]$pt.value).ToString([Globalization.CultureInfo]::InvariantCulture)
        if ([double]$pt.tension -ne 0) { $text += '~' + ([double]$pt.tension).ToString([Globalization.CultureInfo]::InvariantCulture) }
        $parts += $text
    }
    return ($parts -join ' ')
}

function RestoreAll {
    if ($script:restored) { return }
    $script:restored = $true
    Note 'putting everything back'
    foreach ($lane in $circ.lanes) {
        $points = PointsText $lane
        if ($points) {
            Kc @('circadian', 'lane', $lane.name, (OnOff ([bool]$lane.on)), '--points', $points) | Out-Null
        } else {
            Kc @('circadian', 'lane', $lane.name, (OnOff ([bool]$lane.on))) | Out-Null
        }
    }
    Kc @('circadian', 'dark-pages', (OnOff ([bool]$circ.dark_pages))) | Out-Null
    Kc @('circadian', (OnOff ([bool]$circ.on))) | Out-Null
    foreach ($n in $tamperNames) { Kc @('tamper', $n, (OnOff $tamperBefore[$n])) | Out-Null }
    foreach ($n in $browserNames) { Kc @('browser', $n, (OnOff $browserBefore[$n])) | Out-Null }
    foreach ($n in $optionsBefore.Keys) { Kc @('behaviour', $n, (OnOff $optionsBefore[$n])) | Out-Null }
    foreach ($n in $notifyBefore.Keys) { Kc @('notifications', $n, $notifyBefore[$n]) | Out-Null }
    Kc @('stats', (OnOff $statsBefore)) | Out-Null
    foreach ($b in $script:stopped) { Kc @('start', $b) | Out-Null }
    if ($lookBefore) {
        foreach ($v in @('AppsUseLightTheme', 'SystemUsesLightTheme')) {
            if ($null -ne $lookBefore.$v) { Set-ItemProperty $personalize $v $lookBefore.$v -ErrorAction SilentlyContinue }
        }
    }
    if ($wallpaperBefore) { [Kc9kSoakScreen]::Wallpaper($wallpaperBefore) | Out-Null }
    if (-not [bool]$circ.on) {
        Start-Sleep -Seconds 6 # the engine gives its own ramp back first
        [Kc9kSoakScreen]::Plain() | Out-Null
    }
}

# --- Measuring --------------------------------------------------------------

try {
    Register-CimIndicationEvent -ClassName Win32_ProcessStartTrace -SourceIdentifier kc9kSoakStart -ErrorAction Stop | Out-Null
    $traced = $true
} catch { $traced = $false; Note "process start events unavailable ($_): polling instead" }
$seenPids = @{}
function NewStarts {
    $out = @()
    if ($traced) {
        foreach ($e in @(Get-Event -SourceIdentifier kc9kSoakStart -ErrorAction SilentlyContinue)) {
            $n = $e.SourceEventArgs.NewEvent
            $out += [pscustomobject]@{ Name = [string]$n.ProcessName; Pid = [int]$n.ProcessID; Parent = [int]$n.ParentProcessID }
            Remove-Event -EventIdentifier $e.EventIdentifier
        }
    } else {
        foreach ($p in @(Get-CimInstance Win32_Process -Filter "Name='powershell.exe' or Name='kc9k.exe'")) {
            if (-not $seenPids.ContainsKey([int]$p.ProcessId)) {
                $seenPids[[int]$p.ProcessId] = $true
                $out += [pscustomobject]@{ Name = [string]$p.Name; Pid = [int]$p.ProcessId; Parent = [int]$p.ParentProcessId }
            }
        }
    }
    return $out
}

function Agents([int]$svc) {
    $map = @{}
    foreach ($p in @(Get-CimInstance Win32_Process -Filter "Name='kc9k.exe'")) {
        if ([int]$p.ParentProcessId -eq $svc -and [int]$p.SessionId -ne 0) { $map[[int]$p.SessionId] = [int]$p.ProcessId }
    }
    return $map
}

function Cpu {
    $c = Get-CimInstance Win32_PerfFormattedData_PerfOS_Processor -Filter "Name='_Total'" -ErrorAction SilentlyContinue
    if ($c) { return [double]$c.PercentProcessorTime } else { return 0 }
}

function ProcStats([int]$id) {
    if ($id -le 0) { return $null }
    $p = Get-Process -Id $id -ErrorAction SilentlyContinue
    if (-not $p) { return $null }
    return [pscustomobject]@{ Handles = [double]$p.HandleCount; Private = [double]$p.PrivateMemorySize64 }
}

# Slope: last third's mean against the first third's, in percent.
function Slope([double[]]$v) {
    if ($v.Count -lt 3) { return 0 }
    $n = [math]::Max([int][math]::Floor($v.Count / 3), 1)
    $a = ($v[0..($n - 1)] | Measure-Object -Average).Average
    $b = ($v[($v.Count - $n)..($v.Count - 1)] | Measure-Object -Average).Average
    if ($a -eq 0) { return 0 }
    return [math]::Round(($b - $a) * 100 / $a, 1)
}

function EventsSince([hashtable]$filter, [datetime]$since) {
    $filter['StartTime'] = $since
    return @(Get-WinEvent -FilterHashtable $filter -ErrorAction SilentlyContinue)
}
function DisplayResets([datetime]$since) {
    $a = EventsSince @{ LogName = 'System'; ProviderName = 'Display'; Id = 4101 } $since
    $b = EventsSince @{ LogName = 'Application'; Id = 1001 } $since | Where-Object { $_.Message -match 'LiveKernelEvent' }
    $c = EventsSince @{ LogName = 'System'; Id = 1001 } $since | Where-Object { $_.Message -match 'LiveKernelEvent' }
    return @($a).Count + @($b).Count + @($c).Count
}
function AppErrors([datetime]$since) {
    $e = EventsSince @{ LogName = 'Application'; Id = @(1000, 1001, 1002) } $since |
        Where-Object { $_.ProviderName -in @('Application Error', 'Windows Error Reporting', 'Application Hang') -and $_.Message -match 'kc9k|dwm\.exe|explorer\.exe' }
    return @($e).Count
}
function PolicyRefreshes([datetime]$since) {
    return @(EventsSince @{ LogName = 'Microsoft-Windows-GroupPolicy/Operational'; Id = @(4004, 4006, 8004, 8006) } $since).Count
}

$guiProc = $null
if ($Gui) {
    $guiPath = (Resolve-Path $Gui).Path
    $guiProc = Get-Process | Where-Object { $_.Path -eq $guiPath } | Select-Object -First 1
    if (-not $guiProc) { $guiProc = Start-Process $guiPath -PassThru; Start-Sleep -Seconds 10 }
}

# --- Phase actions ----------------------------------------------------------

function SetUpCircadian {
    $hw = (CircadianState).hardware
    Kc @('circadian', 'lane', 'kelvin', 'on', '--points', '00:00=3400 12:00=5000') | Out-Null
    Kc @('circadian', 'lane', 'software', 'on', '--points', '00:00=60 12:00=85') | Out-Null
    if ($hw -and @($hw.backlights).Count -gt 0) {
        Kc @('circadian', 'lane', 'backlight', 'on', '--points', '00:00=50 12:00=80') | Out-Null
    } else {
        Kc @('circadian', 'lane', 'backlight', 'off') | Out-Null
    }
    Kc @('circadian', 'on') | Out-Null
}

$toggles = @()
foreach ($n in $tamperNames) { $toggles += , @('tamper', $n) }
foreach ($n in $browserNames) { $toggles += , @('browser', $n) }
foreach ($n in $optionsBefore.Keys) { $toggles += , @('behaviour', $n) }
foreach ($n in $notifyBefore.Keys) { $toggles += , @('notifications', $n) }
$toggles += , @('stats')
$toggles += , @('circadian', 'dark-pages')

function Act([string]$phase, [int]$tick) {
    switch ($phase) {
        'circadian' {
            # A preview every 20 s, ended 10 s later.
            switch ($tick % 4) {
                0 { Kc @('circadian', 'preview', 'kelvin', '4200') | Out-Null; Kc @('circadian', 'preview', 'software', '70') | Out-Null }
                2 { Kc @('circadian', 'preview', 'kelvin', '--end') | Out-Null; Kc @('circadian', 'preview', 'software', '--end') | Out-Null }
            }
        }
        'play' {
            if ($script:plays -lt 3 -and $tick % 4 -eq 0) {
                $script:plays++
                Kc @('circadian', 'play', '--seconds', '8') | Out-Null
            }
        }
        'settings' {
            # Each setting on and off 5 times, one step a sample.
            $round = [int][math]::Floor($tick / 2)
            if ($round -ge 5) { return }
            $value = OnOff (($tick % 2) -eq 0)
            foreach ($t in $toggles) { Kc ($t + @($value)) | Out-Null }
        }
    }
}

# --- Running ----------------------------------------------------------------

$results = @()
$failed = $false
$stopReason = ''
$runStart = Get-Date
$psLimit = @{ idle = 0; circadian = 4; play = 70; settings = 2 }

try {
    # No blocks: those switched on are switched off for the run (a locked
    # one stays on; nothing here touches a lock).
    foreach ($b in @($state.blocks)) {
        if ($b.active) {
            if ((Kc @('stop', $b.name)) -eq 0) { $script:stopped += $b.name } else { Note "block $($b.name) stays on (locked or held)" }
        }
    }
    Kc @('circadian', 'off') | Out-Null
    Note 'settling for 30 s'
    Start-Sleep -Seconds 30
    NewStarts | Out-Null

    foreach ($phase in $Phases) {
        if ($stopReason) { break }
        Note "phase $phase ($Minutes min)"
        switch ($phase) {
            'idle' { Kc @('circadian', 'off') | Out-Null; Start-Sleep -Seconds 10 }
            'circadian' { SetUpCircadian }
            'play' { if (-not (CircadianState).on) { SetUpCircadian }; $script:plays = 0 }
            'settings' { }
        }
        NewStarts | Out-Null
        $phaseStart = Get-Date
        $svc = ServicePid
        $agents = Agents $svc
        $series = @{ svcH = @(); svcP = @(); agH = @(); agP = @(); guiH = @(); guiP = @() }
        $cpu = @(); $hot = 0; $ps = 0; $kc = 0; $restarts = 0; $mostPs = 0; $gammaBad = ''; $guiDead = 0
        $samples = [int][math]::Max([math]::Ceiling($Minutes * 60 / $sampleSeconds), 2)
        for ($tick = 0; $tick -lt $samples; $tick++) {
            Act $phase $tick
            Start-Sleep -Seconds $sampleSeconds
            $now = Get-Date
            $svcNow = ServicePid
            foreach ($s in NewStarts) {
                if ($s.Parent -ne $svc -and $s.Parent -ne $svcNow) { continue }
                if ($s.Name -ieq 'powershell.exe') { $ps++ }
                if ($s.Name -ieq 'kc9k.exe') { $kc++ }
            }
            $c = Cpu; $cpu += $c
            if ($c -gt 90) { $hot++ } else { $hot = 0 }
            $running = @(Get-CimInstance Win32_Process -Filter "Name='powershell.exe'" | Where-Object { [int]$_.ProcessId -ne $PID }).Count
            $mostPs = [math]::Max($mostPs, $running)
            $nowAgents = Agents $svcNow
            if ($svcNow -eq $svc) {
                foreach ($sess in $agents.Keys) {
                    if ($nowAgents.ContainsKey($sess) -and $nowAgents[$sess] -ne $agents[$sess]) { $restarts++ }
                }
            }
            foreach ($sess in $nowAgents.Keys) { $agents[$sess] = $nowAgents[$sess] }
            $svc = $svcNow
            $st = ProcStats $svc
            if ($st) { $series.svcH += $st.Handles; $series.svcP += $st.Private }
            $first = @($agents.Values) | Select-Object -First 1
            $st = ProcStats ([int]$first)
            if ($st) { $series.agH += $st.Handles; $series.agP += $st.Private }
            if ($guiProc) {
                $guiProc.Refresh()
                if ($guiProc.HasExited) { $guiDead = 99 } elseif (-not $guiProc.Responding) { $guiDead++ } else { $guiDead = 0 }
                $st = ProcStats $guiProc.Id
                if ($st) { $series.guiH += $st.Handles; $series.guiP += $st.Private }
            }
            if ($phase -ne 'idle' -and -not $gammaBad) { $gammaBad = GammaProblem }
            Add-Content -Path (Join-Path $Out 'samples.csv') -Value ("{0},{1},{2:N1},{3},{4},{5},{6}" -f $phase, $now.ToString('HH:mm:ss'), $c, $ps, $kc, $running, $restarts)

            if ($hot * $sampleSeconds -ge 30) { $stopReason = 'CPU over 90 % for 30 s' }
            elseif ($running -gt 5) { $stopReason = "$running powershell.exe at once" }
            elseif ((DisplayResets $phaseStart) -gt 0) { $stopReason = 'a display driver reset (Display 4101) or LiveKernelEvent' }
            elseif ($guiDead -ge 99) { $stopReason = 'the GUI ended' }
            elseif ($guiDead -ge 2) { $stopReason = 'the GUI is not responding' }
            elseif ($restarts -gt 0) { $stopReason = 'a session agent was started again' }
            if ($stopReason) { Note "STOP: $stopReason"; break }
        }
        $phaseEnd = Get-Date
        $mins = [math]::Max(($phaseEnd - $phaseStart).TotalMinutes, 0.1)
        $log = LogSince (ServiceLog) $phaseStart
        $tamper = @($log | Where-Object { $_ -match 'tamper: put back' }).Count
        $outside = @($log | Where-Object { $_ -match 'agent: the agent in session \d+ ended from outside' }).Count
        $sysRoot = [regex]::Escape($env:SystemRoot)
        $badClose = @($log | Where-Object { $_ -match 'browser: closing' -and $_ -match $sysRoot })
        foreach ($l in $badClose) { Note "closed a Windows program: $l" }
        $policy = PolicyRefreshes $phaseStart
        $row = [pscustomobject]@{
            Phase = $phase; Min = [math]::Round($mins, 1)
            'GP/min' = [math]::Round($policy / $mins, 1); 'PS' = $ps; 'PS/min' = [math]::Round($ps / $mins, 1); 'kc9k' = $kc
            Restarts = [math]::Max($restarts, $outside); Tamper = $tamper
            'CPU max' = [math]::Round((($cpu | Measure-Object -Maximum).Maximum), 0)
            'CPU avg' = [math]::Round((($cpu | Measure-Object -Average).Average), 0)
            'svc H%' = Slope $series.svcH; 'svc M%' = Slope $series.svcP
            'agent H%' = Slope $series.agH; 'agent M%' = Slope $series.agP
            'GUI H%' = Slope $series.guiH; 'GUI M%' = Slope $series.guiP
            '4101/LKE' = DisplayResets $phaseStart; AppErr = AppErrors $phaseStart
            Gamma = $(if ($gammaBad) { $gammaBad } else { 'ok' }); Result = 'ok'
        }
        $why = @()
        if ($policy -gt 0) { $why += 'policy refreshed' }
        if ($row.'PS/min' -gt $psLimit[$phase] -or ($phase -eq 'idle' -and $ps -gt 0)) { $why += 'PowerShell by the service' }
        if ($row.Restarts -gt 0) { $why += 'agent restarted' }
        if ($tamper -gt 0) { $why += 'tamper events' }
        if ($row.'4101/LKE' -gt 0) { $why += 'display reset' }
        if ($row.AppErr -gt 0) { $why += 'crash reports' }
        if ($gammaBad) { $why += 'gamma ramp' }
        if ($badClose.Count -gt 0) { $why += 'closed a Windows program' }
        if ($mostPs -gt 5) { $why += 'powershell.exe at once' }
        if ($phase -eq 'idle') {
            foreach ($k in @('svc H%', 'agent H%', 'GUI H%')) { if ([math]::Abs($row.$k) -gt 5) { $why += "$k not flat" } }
        }
        if ($stopReason) { $why += "stopped: $stopReason" }
        if ($why.Count -gt 0) { $row.Result = ($why -join '; '); $failed = $true }
        $results += $row
    }
} finally {
    RestoreAll
    if ($traced) { Unregister-Event -SourceIdentifier kc9kSoakStart -ErrorAction SilentlyContinue }
    $problem = GammaProblem
    if ($problem) { Note "gamma ramp after putting back: $problem"; $failed = $true }
    if ($installedHere) {
        Note 'removing KC9K again'
        & $K uninstall | Out-Host
    }
}

$results | Format-Table -AutoSize | Out-String -Width 400 | Write-Host
$results | Export-Csv -NoTypeInformation -Path (Join-Path $Out 'summary.csv')
Note "results in $Out"
if ($failed -or $stopReason) { Write-Host '::error::soak test: a threshold was crossed'; exit 1 }
Write-Host 'soak test: all thresholds held'
exit 0
