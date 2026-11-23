# The checks of ci/windows_vm_test.ps1 that need a signed-in desktop but no
# person and no reboot (ROADMAP W27), on GitHub's windows-latest runner,
# whose session has one. Installs KC9K, checks, uninstalls. Never run it on
# a computer you use.
#   pwsh ci/windows_desktop_test.ps1 path\to\kc9k.exe
param([Parameter(Mandatory = $true)][string]$Exe)
$ErrorActionPreference = 'Continue'
$failures = 0
function Check([string]$name, [scriptblock]$test) {
    Write-Host "[$(Get-Date -Format HH:mm:ss)] $name"
    $ok = $false
    try { $ok = & $test } catch { Write-Host "  $_" }
    if ($ok) { Write-Host "PASS: $name" } else { Write-Host "::error::FAIL: $name"; $script:failures++ }
}
function Until([int]$seconds, [scriptblock]$test) {
    for ($i = 0; $i -lt $seconds; $i++) { try { if (& $test) { return $true } } catch { }; Start-Sleep -Seconds 1 }
    return $false
}

Write-Host "session $((Get-Process -Id $PID).SessionId), user $env:USERNAME"
& $Exe install
if ($LASTEXITCODE -ne 0) { Write-Host '::error::install failed'; exit 1 }
$k = Join-Path $env:ProgramFiles 'KC9K\kc9k.exe'

# W1: the session agent.
Check 'a session agent runs in the signed-in session' { Until 30 { Get-Process kc9k -IncludeUserName -ErrorAction SilentlyContinue | Where-Object { $_.UserName -like "*\$env:USERNAME" -and $_.SessionId -eq (Get-Process -Id $PID).SessionId } } }

# W2: web layers.
$real = (Resolve-DnsName example.com -Server 9.9.9.9 -Type A -DnsOnly -ErrorAction SilentlyContinue | Where-Object { $_.IPAddress } | Select-Object -First 1).IPAddress
& $k block create ci-sites | Out-Host
& $k block add ci-sites example.com | Out-Host
& $k start ci-sites | Out-Host
Check 'hosts sends example.com nowhere' { Until 20 { Clear-DnsClientCache; (Resolve-DnsName example.com -ErrorAction SilentlyContinue | Select-Object -First 1).IPAddress -in @('0.0.0.0', '::') } }
if ($real) {
    Check "the firewall refuses example.com's address ($real)" { Until 20 { -not (Test-NetConnection $real -Port 80 -InformationLevel Quiet -WarningAction SilentlyContinue) } }
}
Check "Edge's policy holds the blocklist" { (Get-Item 'HKLM:\SOFTWARE\Policies\Microsoft\Edge\URLBlocklist' -ErrorAction SilentlyContinue).Property.Count -gt 0 }
& $k stop ci-sites | Out-Host

# W3: apps and window titles, closed through the agent.
& $k block create ci-apps | Out-Host
& $k block add-app ci-apps notepad | Out-Host
& $k start ci-apps | Out-Host
Start-Process notepad
Check 'a blocked app is closed' { Until 20 { -not (Get-Process notepad -ErrorAction SilentlyContinue) } }
& $k stop ci-apps | Out-Host
& $k block create ci-titles | Out-Host
& $k block add-title ci-titles 'KC9K title test' | Out-Host
& $k start ci-titles | Out-Host
$cmd = Start-Process cmd '/k title KC9K title test' -PassThru
Check 'a window with a blocked title is closed' { Until 30 { $cmd.HasExited } }
if (-not $cmd.HasExited) { $cmd.Kill() }
& $k stop ci-titles | Out-Host

# W6: dark mode reaches the signed-in person's settings.
$before = (Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize' -ErrorAction SilentlyContinue).AppsUseLightTheme
& $k circadian on 2>&1 | Out-Host
& $k circadian dark dark 2>&1 | Out-Host
Check 'dark mode on reaches your settings (AppsUseLightTheme 0)' { Until 60 { (Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize').AppsUseLightTheme -eq 0 } }
& $k circadian dark system 2>&1 | Out-Host
& $k circadian off 2>&1 | Out-Host
if ($null -ne $before) { Set-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize' AppsUseLightTheme $before }

# W3, W6: what needs the signed-in session's own input and screen.
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class Kc9kDesk {
    [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr hwnd, uint msg, IntPtr w, IntPtr l);
    [DllImport("user32.dll")] static extern IntPtr GetDC(IntPtr hwnd);
    [DllImport("user32.dll")] static extern int ReleaseDC(IntPtr hwnd, IntPtr dc);
    [DllImport("gdi32.dll")] static extern bool GetDeviceGammaRamp(IntPtr dc, ushort[] ramp);
    [DllImport("user32.dll")] static extern uint SendInput(uint n, INPUT[] inputs, int size);
    [StructLayout(LayoutKind.Sequential)] public struct KEYBDINPUT { public ushort vk; public ushort scan; public uint flags; public uint time; public IntPtr extra; }
    [StructLayout(LayoutKind.Explicit, Size = 40)] public struct INPUT { [FieldOffset(0)] public uint type; [FieldOffset(8)] public KEYBDINPUT ki; }
    // Keys down in order, then up in reverse: injected input (no device).
    public static uint Keys(params ushort[] vks) {
        var list = new System.Collections.Generic.List<INPUT>();
        foreach (var vk in vks) { var i = new INPUT(); i.type = 1; i.ki.vk = vk; list.Add(i); }
        for (int j = vks.Length - 1; j >= 0; j--) { var i = new INPUT(); i.type = 1; i.ki.vk = vks[j]; i.ki.flags = 2; list.Add(i); }
        return SendInput((uint)list.Count, list.ToArray(), Marshal.SizeOf(typeof(INPUT)));
    }
    public static ushort[] Ramp() {
        IntPtr dc = GetDC(IntPtr.Zero); ushort[] r = new ushort[768];
        bool ok = GetDeviceGammaRamp(dc, r); ReleaseDC(IntPtr.Zero, dc);
        return ok ? r : null;
    }
}
'@
function BlockOn([string]$name) { (((& $k gui-state) | ConvertFrom-Json).blocks | Where-Object { $_.name -eq $name }).active }
function BlockLocked([string]$name) { $b = ((& $k gui-state) | ConvertFrom-Json).blocks | Where-Object { $_.name -eq $name }; [bool]$b.char_locked -or @($b.lock_order).Count -gt 0 }
# 'kc9k unlock' with no console: the service asks this session's agent
# for the challenge window (R-LOCK-5); returns the process and its output file.
function UnlockNoConsole([string]$name) {
    $id = Get-Random
    $outFile = Join-Path $env:RUNNER_TEMP "unlock-$id.txt"
    $empty = Join-Path $env:RUNNER_TEMP "empty-$id.txt"; Set-Content $empty ''
    $p = Start-Process $k "unlock $name" -WindowStyle Hidden -RedirectStandardInput $empty -RedirectStandardOutput $outFile -RedirectStandardError "$outFile.err" -PassThru
    return @($p, $outFile)
}
# The open challenge window: the agent's main window with its title.
# (PowerShell hands $null to a C# string as "", so FindWindow by class or
# title alone never matched.)
$challengeTitle = 'Killer Chicken 9000: unlock challenge'
function ChallengeHwnd {
    $p = Get-Process kc9k -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowTitle -eq $challengeTitle } | Select-Object -First 1
    if ($p) { return $p.MainWindowHandle } else { return [IntPtr]::Zero }
}
function ChallengeWindow {
    Until 30 { (ChallengeHwnd) -ne [IntPtr]::Zero } | Out-Null
    $h = ChallengeHwnd
    if ($h -eq [IntPtr]::Zero) {
        Get-Process kc9k -IncludeUserName -ErrorAction SilentlyContinue | ForEach-Object { Write-Host "  kc9k pid $($_.Id) session $($_.SessionId) user $($_.UserName) window $($_.MainWindowHandle) '$($_.MainWindowTitle)'" }
    }
    return $h
}
function UnlockSaid($u) {
    if (-not $u[0].HasExited) { Write-Host '  (unlock still running: stopped)'; Stop-Process -Id $u[0].Id -Force -ErrorAction SilentlyContinue }
    $text = (Get-Content $u[1] -Raw -ErrorAction SilentlyContinue) + (Get-Content "$($u[1]).err" -Raw -ErrorAction SilentlyContinue)
    Write-Host "  unlock said: $text"
    return $text
}

& $k block create ci-lock | Out-Host
& $k block add ci-lock example.org | Out-Host
& $k start ci-lock --lock-text kc --yes | Out-Host
Check 'the text-locked block is locked' { BlockLocked 'ci-lock' }
# A key injected with SendInput (no keyboard behind it) is refused.
$u = UnlockNoConsole 'ci-lock'
$hwnd = ChallengeWindow
Check 'the challenge window opens through the service and agent (no console)' { $hwnd -ne [IntPtr]::Zero }
if ($hwnd -ne [IntPtr]::Zero) { [Kc9kDesk]::Keys(0x4B) | Out-Null } # K
$u[0].WaitForExit(30000) | Out-Null
$said = UnlockSaid $u
# A killed unlock leaves no window behind (the service closes it).
$u = UnlockNoConsole 'ci-lock'
$hwnd = ChallengeWindow
Stop-Process -Id $u[0].Id -Force -ErrorAction SilentlyContinue
Check 'the challenge window closes when the unlock asking for it is ended' { $hwnd -ne [IntPtr]::Zero -and (Until 20 { (ChallengeHwnd) -eq [IntPtr]::Zero }) }
Check "the challenge window refuses an injected key (Raw Input, no device)" { $said -match "wasn't typed on the keyboard" -and (BlockLocked 'ci-lock') }
# Typed keys (window messages, as a keyboard's reach it) unlock it.
$u = UnlockNoConsole 'ci-lock'
$hwnd = ChallengeWindow
foreach ($c in @(0x6B, 0x63, 0x0D)) { [Kc9kDesk]::PostMessage($hwnd, 0x0102, [IntPtr]$c, [IntPtr]::Zero) | Out-Null; Start-Sleep -Milliseconds 150 }
$u[0].WaitForExit(30000) | Out-Null
$said = UnlockSaid $u
& $k stop ci-lock | Out-Host
$stopped = $LASTEXITCODE -eq 0
Check 'the challenge typed in the window unlocks the block (it can be switched off)' { $said -match 'unlock authorized' -and $stopped }

# A hotkey pressed in the session runs its action through the service.
& $k block create ci-hotkey | Out-Host
& $k block add ci-hotkey example.org | Out-Host
& $k hotkey add ctrl+alt+f9 ci-hotkey | Out-Host
Start-Sleep -Seconds 8 # the agent registers it (hotkeys.json)
[Kc9kDesk]::Keys(0x11, 0x12, 0x78) | Out-Null # Ctrl+Alt+F9
Check 'a hotkey pressed in the session starts its block' { Until 20 { BlockOn 'ci-hotkey' } }
& $k stop ci-hotkey | Out-Host
& $k hotkey remove 1 | Out-Host

# W6: gamma ramp and wallpaper in the session.
$plain = [Kc9kDesk]::Ramp()
if ($null -eq $plain) {
    Write-Host '::warning::no gamma ramp on this runner: gamma left to the VM'
} else {
    & $k circadian lane kelvin on --points '00:00=6500' | Out-Host
    & $k circadian on | Out-Host
    $preview = (& $k circadian preview kelvin 2500 2>&1 | Out-String)
    Write-Host $preview
    if ($preview -match "driver refused the colour ramp") {
        Write-Host "::warning::this runner's display driver refuses gamma ramps: gamma left to the VM"
    } else {
        Check 'a warm preview reaches the screen (blue below red at the top)' { Until 20 { $r = [Kc9kDesk]::Ramp(); $r -and $r[767] -lt $r[255] - 4000 } }
    }
    & $k circadian preview kelvin --end | Out-Host
    & $k circadian off | Out-Host
    Check 'circadian rhythm off gives the plain ramp back' { Until 20 { $r = [Kc9kDesk]::Ramp(); $r -and [math]::Abs($r[767] - $r[255]) -lt 600 } }
}
$wallBefore = (Get-ItemProperty 'HKCU:\Control Panel\Desktop').WallPaper
$pic = Get-ChildItem "$env:SystemRoot\Web\Wallpaper" -Recurse -Include *.jpg, *.png -ErrorAction SilentlyContinue | Select-Object -First 1
if ($pic) {
    $mine = Join-Path $env:RUNNER_TEMP ('kc9k-wall' + $pic.Extension)
    Copy-Item $pic.FullName $mine
    & $k wallpaper default custom $mine | Out-Host
    Check 'a wallpaper of your own reaches your desktop (copied to Program Files\KC9K)' { Until 30 { (Get-ItemProperty 'HKCU:\Control Panel\Desktop').WallPaper -like "*KC9K*" } }
    & $k wallpaper default latest | Out-Host
    if ($wallBefore) { Set-ItemProperty 'HKCU:\Control Panel\Desktop' WallPaper $wallBefore }
} else { Write-Host '::warning::no picture on this runner to set as wallpaper' }

# W18: one-click browser install, as SYSTEM through the service (winget).
& $k browsers install librewolf | Out-Host
Check 'LibreWolf installed by winget as SYSTEM through the service' { Test-Path "$env:ProgramFiles\LibreWolf\librewolf.exe" }
Check "'kc9k browsers' lists LibreWolf as installed" { (& $k browsers | Out-String) -match 'librewolf\s+installed' }

& $k uninstall | Out-Host
Check 'uninstall' { $LASTEXITCODE -eq 0 }
Until 60 { -not (Get-Service kc9k -ErrorAction SilentlyContinue) } | Out-Null
if ($failures -gt 0) { Write-Host "$failures check(s) failed"; exit 1 }
Write-Host 'all checks passed'
