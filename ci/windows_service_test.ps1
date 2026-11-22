# Installs Killer Chicken 9000 on a disposable Windows machine (GitHub's
# windows-latest runner, or a VM snapshot) and checks the service, the
# named pipe and uninstall (ROADMAP W1). Never run it on a computer you
# use: it installs a service and locks a block.
#   pwsh ci/windows_service_test.ps1 path\to\kc9k.exe
param([Parameter(Mandatory = $true)][string]$Exe)
$ErrorActionPreference = 'Continue'
$failures = 0
function Check([string]$name, [scriptblock]$test) {
    Write-Host "[$(Get-Date -Format HH:mm:ss)] $name"
    $ok = $false
    try { $ok = & $test } catch { Write-Host "  $_" }
    if ($ok) { Write-Host "PASS: $name" } else { Write-Host "::error::FAIL: $name"; $script:failures++ }
}

# K runs kc9k with a time limit, input and output redirected (not a
# console), prints what it printed, and sets $LASTEXITCODE.
function K {
    Write-Host ">> kc9k $($args -join ' ')"
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $script:k
    $psi.Arguments = ($args | ForEach-Object { if ("$_" -match '[\s"]') { '"' + ("$_" -replace '"', '\"') + '"' } else { "$_" } }) -join ' '
    $psi.UseShellExecute = $false
    $psi.RedirectStandardInput = $true
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $p = [System.Diagnostics.Process]::Start($psi)
    $p.StandardInput.Close()
    $out = $p.StandardOutput.ReadToEndAsync()
    $err = $p.StandardError.ReadToEndAsync()
    if (-not $p.WaitForExit(60000)) {
        Write-Host "::error::TIMEOUT: kc9k $($args -join ' ')"
        $p.Kill()
        $global:LASTEXITCODE = 124
    } else {
        $global:LASTEXITCODE = $p.ExitCode
    }
    $out.Result
    if ($err.Result) { Write-Host $err.Result }
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
$out = K block create ci-block 2>&1 | Out-String
Check 'a forwarded command runs in the service' { $LASTEXITCODE -eq 0 -and $out -match 'ci-block' }
K block info no-such-block 2>$null | Out-Null
Check 'a failing command exits non-zero' { $LASTEXITCODE -ne 0 }
$state = K gui-state | Out-String
Check 'gui-state is JSON' { $null -ne ($state | ConvertFrom-Json) }
K block add ci-block example.com | Out-Null
$list = K block info ci-block | Out-String
Check 'changes persist between commands' { $list -match 'example.com' }
$imported = Join-Path $env:TEMP 'kc9k-import.txt'
"example.org`nexample.net" | Set-Content -Path $imported
K block import-list ci-block blocked $imported 2>&1 | Out-Host
$list = K block info ci-block | Out-String
Check 'a file the caller names is read' { $list -match 'example.net' }
# The service could read the database; the caller can't, so neither may
# a command run for the caller (readAsCaller impersonates the caller).
K block import-list ci-block blocked (Join-Path $data 'kc9k.db') 2>&1 | Out-Host
Check 'a file only SYSTEM may read is not read for the caller' { $LASTEXITCODE -ne 0 }

# The data folder is SYSTEM's alone.
$listed = $true
try { Get-ChildItem $data -ErrorAction Stop | Out-Null } catch { $listed = $false }
Check 'an administrator can not list the data folder' { -not $listed }
$read = $true
try { Get-Content (Join-Path $data 'kc9k.db') -TotalCount 1 -ErrorAction Stop | Out-Null } catch { $read = $false }
Check 'an administrator can not read the database directly' { -not $read }

# Commands that never go through the pipe.
K service 2>$null
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
Start-Sleep -Seconds 8
schtasks.exe /Query /TN '\KC9K\Watchdog' 2>$null | Out-Null
Check 'the watchdog task deleted by an administrator is back within a cycle' { $LASTEXITCODE -eq 0 }
Check 'the service runs in Safe Mode too' { Test-Path 'HKLM:\SYSTEM\CurrentControlSet\Control\SafeBoot\Minimal\kc9k' }

# Strict mode while locked: deny entries for administrators. It closes
# programs run from people's own folders: the runner's are exempted (and
# listed, for the log).
$own = Get-Process | Where-Object { $_.Path -like "$env:SystemDrive\Users\*" -and $_.Path -notlike '*\AppData\*' -or $_.Path -like '*\AppData\Local\Temp\*' }
$own | ForEach-Object { Write-Host "  own folder: $($_.Name) $($_.Path)" }
$own | ForEach-Object { Split-Path $_.Path } | Sort-Object -Unique | ForEach-Object { K strict exempt add $_ | Out-Host }
K strict on | Out-Host
K start ci-block --for 3m --yes | Out-Host
Start-Sleep -Seconds 12
$hosts = Join-Path $env:SystemRoot 'System32\drivers\etc\hosts'
$wrote = $true
try { Add-Content -Path $hosts -Value '# kc9k test' -ErrorAction Stop } catch { $wrote = $false }
Check 'strict mode: an administrator can not write the hosts file' { -not $wrote }
Check 'strict mode: the service still writes the hosts file (the block is in it)' { (Get-Content $hosts -Raw) -match 'example\.com' }
reg.exe add 'HKLM\SYSTEM\CurrentControlSet\Services\kc9k' /v KC9KTest /d x /f 2>$null | Out-Null
Check "strict mode: an administrator can not change the service's registry key" { $LASTEXITCODE -ne 0 }

# Strict mode: Windows' ways to SYSTEM, privileges, accounts (W20-W22).
sc.exe create kc9kci binPath= "$env:SystemRoot\System32\cmd.exe /c exit" | Out-Host
$created = $LASTEXITCODE -eq 0
if ($created) { sc.exe delete kc9kci | Out-Null }
Check 'strict mode: an administrator can not create a service' { -not $created }
sc.exe config Spooler start= demand | Out-Host
Check "strict mode: an administrator can not reconfigure another service" { $LASTEXITCODE -ne 0 }
reg.exe add 'HKLM\SYSTEM\CurrentControlSet\Services\Spooler' /v KC9KTest /d x /f 2>$null | Out-Null
Check "strict mode: an administrator can not write the Services key tree" { $LASTEXITCODE -ne 0 }
Check 'strict mode: Windows Installer is off' { (Get-ItemProperty 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\Installer' -ErrorAction SilentlyContinue).DisableMSI -eq 2 }
$priv = whoami.exe /priv | Out-String
Write-Host $priv
Check "strict mode: people's programs lose the backup, restore, debug and take-ownership privileges" { $priv -notmatch 'SeBackupPrivilege|SeRestorePrivilege|SeDebugPrivilege|SeTakeOwnershipPrivilege' }
New-Item -ItemType Directory -Force 'C:\kc9kci' | Out-Null
Copy-Item "$env:SystemRoot\System32\PING.EXE" 'C:\kc9kci\VirtualBoxVM.exe'
$vm = Start-Process 'C:\kc9kci\VirtualBoxVM.exe' -ArgumentList '-n', '120', '127.0.0.1' -WindowStyle Hidden -PassThru
net.exe user kc9kci 'Kc9k-ci-1!x' /add | Out-Host
schtasks.exe /Create /TN kc9kci /TR "$env:SystemRoot\System32\cmd.exe /c exit" /SC ONCE /ST 23:59 /RU SYSTEM /F 2>&1 | Out-Host
$taskRefused = $LASTEXITCODE -ne 0
try { Set-WmiInstance -Namespace root/subscription -Class CommandLineEventConsumer -Arguments @{ Name = 'kc9kci'; CommandLineTemplate = 'cmd /c exit' } -ErrorAction Stop | Out-Null; $wmiRefused = $false } catch { Write-Host "  $_"; $wmiRefused = $true }
Start-Sleep -Seconds 40
Check 'strict mode: a virtual machine program is closed' { $vm.HasExited }
Get-ChildItem 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\Safer\CodeIdentifiers\0\Paths', 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\Safer\CodeIdentifiers\262144\Paths' -ErrorAction SilentlyContinue |
    ForEach-Object { Write-Host "  SRP $(Split-Path (Split-Path $_.PSParentPath) -Leaf): $((Get-ItemProperty $_.PSPath).ItemData)" }
New-Item -ItemType Directory -Force "$env:USERPROFILE\Downloads" | Out-Null
Copy-Item "$env:SystemRoot\System32\PING.EXE" "$env:USERPROFILE\Downloads\kc9kping.exe" -Force
$started = $true
try { Start-Process "$env:USERPROFILE\Downloads\kc9kping.exe" -ArgumentList '-n', '1', '127.0.0.1' -WindowStyle Hidden -Wait -ErrorAction Stop } catch { Write-Host "  $_"; $started = $false }
Get-ItemProperty 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\Safer\CodeIdentifiers' -ErrorAction SilentlyContinue | Out-Host
$fresh = & pwsh -NoProfile -Command "try { Start-Process '$env:USERPROFILE\Downloads\kc9kping.exe' -ArgumentList '-n','1','127.0.0.1' -WindowStyle Hidden -Wait -ErrorAction Stop; 'started' } catch { 'refused: ' + `$_ }"
Write-Host "  from a new process: $fresh"
cmd.exe /c "`"$env:USERPROFILE\Downloads\kc9kping.exe`" -n 1 127.0.0.1" | Out-Host
Write-Host "  from cmd: exit $LASTEXITCODE"
Write-Host "  from this (older) shell: started $started"
Check "strict mode: a program in your Downloads folder doesn't start from a new process or cmd (Software Restriction Policies)" { $fresh -like 'refused*' -and $LASTEXITCODE -ne 0 }
Check "strict mode: Docker's engine is stopped (where installed)" { -not (Get-Service docker -ErrorAction SilentlyContinue) -or (Get-Service docker).Status -ne 'Running' }
Check 'strict mode: an account made meanwhile is turned off' { -not (Get-LocalUser kc9kci -ErrorAction SilentlyContinue).Enabled }
Check 'strict mode: a task as SYSTEM is refused, or deleted within a cycle' { $taskRefused -or -not (schtasks.exe /Query /TN kc9kci 2>$null) }
Check 'strict mode: a WMI event consumer is refused, or deleted within a cycle' { $wmiRefused -or -not (Get-WmiObject -Namespace root/subscription -Class CommandLineEventConsumer -Filter "Name='kc9kci'") }

# Tamper protection "block time and date changes" during a lock (W23).
K tamper clock on | Out-Host
Start-Sleep -Seconds 8
$tz = (Get-TimeZone).Id
$other = if ($tz -eq 'UTC') { 'Pacific Standard Time' } else { 'UTC' }
Set-TimeZone -Id $other
Start-Sleep -Seconds 10
Check 'tamper clock: a changed time zone is put back during a lock' { (Get-TimeZone).Id -eq $tz }
$before = Get-Date
Set-Date -Adjust (New-TimeSpan -Hours 2) | Out-Null
Start-Sleep -Seconds 10
Check 'tamper clock: a clock moved ahead is put back during a lock' { [math]::Abs(((Get-Date) - $before).TotalMinutes) -lt 10 }

# Uninstall is refused while a block is locked.
K uninstall 2>&1 | Out-Host
Check 'uninstall is refused while locked' { $LASTEXITCODE -ne 0 -and (Get-Service kc9k -ErrorAction SilentlyContinue) }
Start-Sleep -Seconds 105
K stop ci-block | Out-Host
$wrote = $false
for ($i = 0; $i -lt 30 -and -not $wrote; $i++) {
    Start-Sleep -Seconds 2
    try { Add-Content -Path $hosts -Value '# kc9k test' -ErrorAction Stop; $wrote = $true } catch { }
}
Check 'after the lock the hosts file is writable again' { $wrote }
sc.exe create kc9kci binPath= "$env:SystemRoot\System32\cmd.exe /c exit" | Out-Null
Check 'after the lock an administrator can create a service again' { $LASTEXITCODE -eq 0 }
sc.exe delete kc9kci | Out-Null
$reconfigured = $false
for ($i = 0; $i -lt 75 -and -not $reconfigured; $i++) { sc.exe config Spooler start= auto | Out-Null; $reconfigured = $LASTEXITCODE -eq 0; if (-not $reconfigured) { Start-Sleep -Seconds 2 } }
Check 'after the lock an administrator can reconfigure services again (put back beside the cycle)' { $reconfigured }
Check 'after the lock Windows Installer is on again' { (Get-ItemProperty 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\Installer' -ErrorAction SilentlyContinue).DisableMSI -ne 2 }
Check "after the lock KC9K's Software Restriction Policies rules are gone" { -not (Get-ChildItem 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\Safer\CodeIdentifiers\0\Paths' -ErrorAction SilentlyContinue | Where-Object { $_.PSChildName -like '{4B43394B-*' }) }
Check 'after the lock the account made meanwhile is on again' { (Get-LocalUser kc9kci -ErrorAction SilentlyContinue).Enabled }
net.exe user kc9kci /delete | Out-Null
schtasks.exe /Delete /TN kc9kci /F 2>$null | Out-Null
K strict off | Out-Host
K tamper clock off | Out-Host
$out = K uninstall 2>&1 | Out-String
Write-Host $out
Check 'uninstall succeeds once nothing is locked' { $LASTEXITCODE -eq 0 -and $out -match 'program is removed' }
for ($i = 0; $i -lt 30 -and (Get-Service kc9k -ErrorAction SilentlyContinue); $i++) { Start-Sleep -Seconds 1 }
Check 'the service is gone' { -not (Get-Service kc9k -ErrorAction SilentlyContinue) }
for ($i = 0; $i -lt 60 -and (Test-Path $k); $i++) { Start-Sleep -Seconds 1 }
if (Test-Path $k) { Get-ChildItem -Recurse (Split-Path $k) | Select-Object FullName | Out-Host }
Check 'the program is gone' { -not (Test-Path $k) }
Check 'the data stays' { Test-Path $data }

if (Test-Path (Join-Path $data 'kc9k.log') -ErrorAction SilentlyContinue) { Write-Host '--- service log (as SYSTEM: not readable here)' }
if ($failures -gt 0) { Write-Host "$failures check(s) failed"; exit 1 }
Write-Host 'all checks passed'
