# Checks of KC9K on Windows that need a desktop, a person, or a reboot
# (ROADMAP W1–W6: what windows-latest can't show). Run it in a Windows VM
# with a snapshot, signed in as an administrator, from an elevated
# PowerShell; it locks blocks for minutes and changes boot settings.
# Never on a computer you use, never on the owner's laptop.
#   pwsh ci/windows_vm_test.ps1 path\to\kc9k.exe
# The automatic checks that need only a signed-in desktop also run in CI
# (ci/windows_desktop_test.ps1, W27: challenge window, hotkey, gamma,
# wallpaper, winget as SYSTEM; ci/windows_service_test.ps1: WSL under
# strict mode); this script keeps them for a VM run and adds what needs a
# real browser, a real network, Hyper-V/Sandbox, a person or a reboot.
# Each automatic check prints PASS or FAIL; steps marked ASK wait for you
# to look and answer y or n. The end prints the failures.
param([Parameter(Mandatory = $true)][string]$Exe, [switch]$Reboots)
$ErrorActionPreference = 'Continue'
$failures = @()
function Check([string]$name, [scriptblock]$test) {
    $ok = $false
    try { $ok = & $test } catch { Write-Host "  $_" }
    if ($ok) { Write-Host "PASS: $name" } else { Write-Host "FAIL: $name"; $script:failures += $name }
}
function Ask([string]$question) {
    $a = Read-Host "ASK: $question (y/n)"
    if ($a -ne 'y') { Write-Host "FAIL: $question"; $script:failures += $question } else { Write-Host "PASS: $question" }
}

& $Exe install
$k = Join-Path $env:ProgramFiles 'KC9K\kc9k.exe'
Start-Sleep -Seconds 8
Check 'a session agent runs for you' { Get-Process kc9k -IncludeUserName -ErrorAction SilentlyContinue | Where-Object { $_.UserName -like "*$env:USERNAME" } }

# W2: web layers.
& $k block create vm-sites | Out-Host
& $k block add vm-sites example.com | Out-Host
& $k start vm-sites | Out-Host
Start-Sleep -Seconds 8
Check 'hosts sends example.com nowhere' { (Resolve-DnsName example.com -ErrorAction SilentlyContinue | Select-Object -First 1).IPAddress -in @('0.0.0.0', '::') }
Check 'the firewall refuses example.com by address' { -not (Test-NetConnection 93.184.215.14 -Port 443 -InformationLevel Quiet -WarningAction SilentlyContinue) }
Check "Edge's policy holds the blocklist" { (Get-Item 'HKLM:\SOFTWARE\Policies\Microsoft\Edge\URLBlocklist').Property.Count -gt 0 }
Start-Process msedge 'https://example.com'
Ask 'Edge shows example.com as blocked by your organisation'
if (Test-Path (Join-Path $env:ProgramFiles 'LibreWolf\librewolf.exe')) {
    Start-Process (Join-Path $env:ProgramFiles 'LibreWolf\librewolf.exe') 'about:addons'
    Ask 'LibreWolf shows the Killer Chicken extension, installed by policy, that you cannot remove'
}
# W2, W11: the extension guard closes a browser started for remote
# control while a website block is on, and with "block unsupported
# browsers" any browser KC9K can't give the extension yet (Chrome or Edge
# before the store listing, Firefox before Mozilla's signature: 2.12).
$edge = Start-Process msedge '--remote-debugging-port=9222 about:blank' -PassThru
Start-Sleep -Seconds 15
Check 'Edge started for remote control is closed at once (W11)' { -not (Get-Process msedge -ErrorAction SilentlyContinue | Where-Object { $_.Id -eq $edge.Id }) }
$ff = Join-Path $env:ProgramFiles 'Mozilla Firefox\firefox.exe'
if (-not (Test-Path $ff)) { & $k browsers install firefox | Out-Host }
if (Test-Path $ff) {
    Start-Process $ff 'https://example.com'
    Ask 'Firefox shows example.com as blocked (its policies.json and the registry policies)'
    & $k tamper unsupported-browsers on | Out-Host
    Start-Sleep -Seconds 20
    $waiting = ((& $k gui-state) | ConvertFrom-Json).browsers | Where-Object { $_.id -eq 'firefox' }
    if ($waiting.waiting) {
        Check 'Firefox (waiting for its extension) is closed with block unsupported browsers on' { -not (Get-Process firefox -ErrorAction SilentlyContinue) }
    } else {
        Start-Process $ff 'about:addons'
        Ask 'Firefox shows the Killer Chicken extension, installed by policy, that you cannot remove'
    }
    & $k tamper unsupported-browsers off | Out-Host
}
$store = Get-AppxPackage -AllUsers Mozilla.Firefox -ErrorAction SilentlyContinue
if ($store) {
    Start-Process "shell:AppsFolder\$($store.PackageFamilyName)!App" 'https://example.com'
    Ask "the Microsoft Store's Firefox shows example.com as blocked (registry policies, W26)"
} else { Write-Host "SKIP: the Microsoft Store's Firefox isn't installed (install it from the Store for W26)" }
& $k stop vm-sites | Out-Host

# W2: monk mode cuts the whole computer's network for its time.
& $k monk --for 3m --yes | Out-Host
Start-Sleep -Seconds 20
Check "monk mode: nothing reaches the internet" { -not (Test-NetConnection 1.1.1.1 -Port 443 -InformationLevel Quiet -WarningAction SilentlyContinue) }
Write-Host 'waiting for monk mode to end (3 minutes)'
Start-Sleep -Seconds 190
Check 'monk mode over: the network is back' { Test-NetConnection 1.1.1.1 -Port 443 -InformationLevel Quiet -WarningAction SilentlyContinue }

# W3: apps, titles, notifications, the challenge window.
& $k block create vm-apps | Out-Host
& $k block add-app vm-apps notepad | Out-Host
& $k start vm-apps | Out-Host
Start-Process notepad
Start-Sleep -Seconds 12
Check 'a blocked app is closed' { -not (Get-Process notepad -ErrorAction SilentlyContinue) }
& $k stop vm-apps | Out-Host
& $k block create vm-titles | Out-Host
& $k block add-title vm-titles 'KC9K title test' | Out-Host
& $k start vm-titles | Out-Host
Start-Process cmd '/k title KC9K title test'
Start-Sleep -Seconds 15
Check 'a window with a blocked title is closed' { -not (Get-Process cmd -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowTitle -like '*KC9K title test*' }) }
& $k stop vm-titles | Out-Host
Ask 'a notification from Killer Chicken 9000 appeared while those were closed'
& $k block create vm-lock | Out-Host
& $k start vm-lock --lock --lock-chars 20 --yes | Out-Host
Write-Host 'Now type the challenge in this console (raw input: no echo of a paste); then try pasting.'
& $k unlock vm-lock
Ask 'the challenge showed in this console, typing worked, and a paste was refused'
& $k block create vm-lock2 | Out-Host
& $k start vm-lock2 --lock --lock-chars 20 --yes | Out-Host
Write-Host 'Now the same with no console: the challenge window must open.'
Start-Process -Wait -WindowStyle Hidden $k 'unlock vm-lock2' -RedirectStandardInput NUL
Ask 'a challenge window opened; typed keys counted; Ctrl+V and the on-screen keyboard were refused'
& $k freeze --for 2m --mode lock --yes | Out-Host
Ask 'the screen locked, and unlocking it locked it again at once until the 2 minutes ended'

# W5: an administrator against KC9K.
sc.exe stop kc9k | Out-Null
Check 'an administrator can not stop the service' { (Get-Service kc9k).Status -eq 'Running' }
Stop-Process -Name kc9k -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 15
Check 'the service and the agent are back after being killed' { (Get-Service kc9k).Status -eq 'Running' -and (Get-Process kc9k -IncludeUserName | Where-Object { $_.UserName -like "*$env:USERNAME" }) }
& $k strict on | Out-Host
& $k start vm-sites --for 5m --yes | Out-Host
Start-Sleep -Seconds 8
takeown /f "$env:SystemRoot\System32\drivers\etc\hosts" | Out-Null
icacls "$env:SystemRoot\System32\drivers\etc\hosts" /reset | Out-Null
Start-Sleep -Seconds 8
$w = $true; try { Add-Content "$env:SystemRoot\System32\drivers\etc\hosts" '# vm test' -ErrorAction Stop } catch { $w = $false }
Check "strict mode put the hosts file's protection back after takeown and icacls /reset" { -not $w }
Start-Process notepad -ArgumentList '' ; Copy-Item "$env:SystemRoot\System32\notepad.exe" "$env:USERPROFILE\Downloads\portable.exe"
Start-Process "$env:USERPROFILE\Downloads\portable.exe"; Start-Sleep -Seconds 10
Check 'strict mode closes programs run from your own folders' { -not (Get-Process portable -ErrorAction SilentlyContinue) }
# W24: Windows Sandbox and a Hyper-V VM, where the VM has them (nested
# virtualisation on).
if (Get-Command WindowsSandbox.exe -ErrorAction SilentlyContinue) {
    Start-Process WindowsSandbox.exe; Start-Sleep -Seconds 20
    Check 'strict mode closes Windows Sandbox' { -not (Get-Process WindowsSandbox*, WindowsSandboxClient -ErrorAction SilentlyContinue) }
} else { Write-Host 'SKIP: Windows Sandbox not enabled (Enable-WindowsOptionalFeature -Online -FeatureName Containers-DisposableClientVM)' }
if (Get-Command Get-VM -ErrorAction SilentlyContinue) {
    $vm = Get-VM | Select-Object -First 1
    if ($vm) {
        Start-VM $vm -ErrorAction SilentlyContinue; Start-Sleep -Seconds 20
        Check 'strict mode stops a Hyper-V VM (its worker ended, vmms stopped)' { (Get-Service vmms).Status -ne 'Running' -or (Get-VM $vm.Name).State -ne 'Running' }
    } else { Write-Host 'SKIP: no Hyper-V VM made (New-VM -Name kc9kvm -MemoryStartupBytes 512MB)' }
} else { Write-Host 'SKIP: Hyper-V not enabled' }
if (wsl.exe -l -q 2>$null) {
    $w = Start-Process wsl.exe '-e sleep 300' -PassThru; Start-Sleep -Seconds 15
    Check 'strict mode closes a WSL 2 distribution' { $w.HasExited }
}
& $k status | Out-Host
Ask "'kc9k status' or the GUI counts the takeown as a bypass (tamper event)"

if ($Reboots) {
    Write-Host 'Reboot checks: take a snapshot first. Then run, one at a time:'
    Write-Host '  1. bcdedit /set {current} safeboot minimal; reboot: in Safe Mode, the block still holds (kc9k status, the hosts file), a freeze and strict mode do not (W13), and kc9k recover removes a settings password; bcdedit /deletevalue {current} safeboot; reboot.'
    Write-Host '  1b. With factory reset protection acting (a lock, no no-escape): F8 while booting shows Advanced Boot Options with Safe Mode (W13).'
    Write-Host '  2. kc9k no-escape on --yes; reboot holding Shift+Restart: the recovery environment must not open.'
    Write-Host '  3. Settings > System > Recovery > Reset this PC during a lock with factory reset protection on: it must fail.'
}

# W6: circadian rhythm.
& $k circadian preview colour 3000 2>&1 | Out-Host
Ask 'the screen turned warm for a few seconds, then back'
& $k circadian dark --help | Out-Null
Ask "after 'kc9k circadian dark' to dark (see its help), apps and Windows turned dark"

Write-Host ''
if ($failures.Count -gt 0) { Write-Host "$($failures.Count) failed:"; $failures | ForEach-Object { Write-Host "  $_" }; exit 1 }
Write-Host 'all checks passed'
