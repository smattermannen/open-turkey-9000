# Installs Killer Chicken 9000 on a disposable Windows machine (GitHub's
# windows-latest runner, or a VM snapshot) and checks the service, the
# named pipe and uninstall (ROADMAP W1). Never run it on a computer you
# use: it installs a service and locks a block.
#   pwsh ci/windows_service_test.ps1 path\to\kc9k.exe
param([Parameter(Mandatory = $true)][string]$Exe)
$ErrorActionPreference = 'Continue'
$failures = 0
function Check([string]$name, [scriptblock]$test) {
    $ok = $false
    try { $ok = & $test } catch { Write-Host "  $_" }
    if ($ok) { Write-Host "PASS: $name" } else { Write-Host "::error::FAIL: $name"; $script:failures++ }
}

& $Exe install
Check 'install succeeds' { $LASTEXITCODE -eq 0 }
if ($failures -gt 0) { exit 1 }
$k = Join-Path $env:ProgramFiles 'KC9K\kc9k.exe'
$data = Join-Path $env:ProgramData 'kc9k'
Check 'kc9k.exe is in Program Files' { Test-Path $k }
Check 'the service runs' { (Get-Service kc9k).Status -eq 'Running' }
Check 'the service starts automatically' { (Get-Service kc9k).StartType -eq 'Automatic' }
$sc = sc.exe qfailure kc9k | Out-String
Check 'the service restarts on failure' { $sc -match 'RESTART' }
Check 'Program Files\KC9K is on the system PATH' {
    [Environment]::GetEnvironmentVariable('Path', 'Machine') -split ';' -contains (Join-Path $env:ProgramFiles 'KC9K') }

# Forwarded commands: output, exit codes, input.
Start-Sleep -Seconds 3
$out = & $k block create ci-block 2>&1 | Out-String
Check 'a forwarded command runs in the service' { $LASTEXITCODE -eq 0 -and $out -match 'ci-block' }
& $k block info no-such-block 2>$null | Out-Null
Check 'a failing command exits non-zero' { $LASTEXITCODE -ne 0 }
$state = & $k gui-state | Out-String
Check 'gui-state is JSON' { $null -ne ($state | ConvertFrom-Json) }
& $k block add ci-block example.com | Out-Null
$list = & $k block info ci-block | Out-String
Check 'changes persist between commands' { $list -match 'example.com' }
$imported = Join-Path $env:TEMP 'kc9k-import.txt'
"example.org`nexample.net" | Set-Content -Path $imported
& $k block import-list ci-block blocked $imported 2>&1 | Out-Host
$list = & $k block info ci-block | Out-String
Check 'a file the caller names is read' { $list -match 'example.net' }
# The service could read the database; the caller can't, so neither may
# a command run for the caller (readAsCaller impersonates the caller).
& $k block import-list ci-block blocked (Join-Path $data 'kc9k.db') 2>&1 | Out-Host
Check 'a file only SYSTEM may read is not read for the caller' { $LASTEXITCODE -ne 0 }

# The data folder is SYSTEM's alone.
$listed = $true
try { Get-ChildItem $data -ErrorAction Stop | Out-Null } catch { $listed = $false }
Check 'an administrator can not list the data folder' { -not $listed }
$read = $true
try { Get-Content (Join-Path $data 'kc9k.db') -TotalCount 1 -ErrorAction Stop | Out-Null } catch { $read = $false }
Check 'an administrator can not read the database directly' { -not $read }

# Commands that never go through the pipe.
& $k service 2>$null
Check "'kc9k service' outside Windows' service manager is refused" { $LASTEXITCODE -ne 0 }

# Tamper protection against an administrator (W5): the first cycle guards
# the service.
Start-Sleep -Seconds 8
sc.exe stop kc9k | Out-Null
Check 'an administrator can not stop the service' { $LASTEXITCODE -ne 0 -and (Get-Service kc9k).Status -eq 'Running' }
sc.exe config kc9k start= disabled | Out-Null
Check 'an administrator can not reconfigure the service' { $LASTEXITCODE -ne 0 -and (Get-Service kc9k).StartType -eq 'Automatic' }
sc.exe delete kc9k | Out-Null
Check 'an administrator can not delete the service' { $LASTEXITCODE -ne 0 }
schtasks.exe /Query /TN '\KC9K\Watchdog' | Out-Null
Check 'the watchdog task exists' { $LASTEXITCODE -eq 0 }
schtasks.exe /Delete /TN '\KC9K\Watchdog' /F 2>$null | Out-Null
Check 'an administrator can not delete the watchdog task' { $LASTEXITCODE -ne 0 }
Check 'the service runs in Safe Mode too' { Test-Path 'HKLM:\SYSTEM\CurrentControlSet\Control\SafeBoot\Minimal\kc9k' }

# Strict mode while locked: deny entries for administrators.
& $k strict on | Out-Host
& $k start ci-block --for 1m --yes | Out-Host
Start-Sleep -Seconds 8
$hosts = Join-Path $env:SystemRoot 'System32\drivers\etc\hosts'
$wrote = $true
try { Add-Content -Path $hosts -Value '# kc9k test' -ErrorAction Stop } catch { $wrote = $false }
Check 'strict mode: an administrator can not write the hosts file' { -not $wrote }
reg.exe add 'HKLM\SYSTEM\CurrentControlSet\Services\kc9k' /v KC9KTest /d x /f 2>$null | Out-Null
Check "strict mode: an administrator can not change the service's registry key" { $LASTEXITCODE -ne 0 }

# Uninstall is refused while a block is locked.
& $k uninstall 2>&1 | Out-Host
Check 'uninstall is refused while locked' { $LASTEXITCODE -ne 0 -and (Get-Service kc9k -ErrorAction SilentlyContinue) }
Start-Sleep -Seconds 65
& $k stop ci-block | Out-Host
Start-Sleep -Seconds 8
$wrote = $true
try { Add-Content -Path $hosts -Value '# kc9k test' -ErrorAction Stop } catch { $wrote = $false }
Check 'after the lock the hosts file is writable again' { $wrote }
& $k strict off | Out-Host
$out = & $k uninstall 2>&1 | Out-String
Write-Host $out
Check 'uninstall succeeds once nothing is locked' { $LASTEXITCODE -eq 0 -and $out -match 'program is removed' }
for ($i = 0; $i -lt 30 -and (Get-Service kc9k -ErrorAction SilentlyContinue); $i++) { Start-Sleep -Seconds 1 }
Check 'the service is gone' { -not (Get-Service kc9k -ErrorAction SilentlyContinue) }
for ($i = 0; $i -lt 15 -and (Test-Path $k); $i++) { Start-Sleep -Seconds 1 }
Check 'the program is gone' { -not (Test-Path $k) }
Check 'the data stays' { Test-Path $data }

if (Test-Path (Join-Path $data 'kc9k.log')) { Write-Host '--- service log (as SYSTEM: not readable here)' }
if ($failures -gt 0) { Write-Host "$failures check(s) failed"; exit 1 }
Write-Host 'all checks passed'
