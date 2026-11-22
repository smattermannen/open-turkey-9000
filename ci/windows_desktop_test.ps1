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

& $k uninstall | Out-Host
Check 'uninstall' { $LASTEXITCODE -eq 0 }
Until 60 { -not (Get-Service kc9k -ErrorAction SilentlyContinue) } | Out-Null
if ($failures -gt 0) { Write-Host "$failures check(s) failed"; exit 1 }
Write-Host 'all checks passed'
