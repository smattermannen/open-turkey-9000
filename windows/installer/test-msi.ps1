# W7 check on windows-latest (a disposable administrator's machine):
#   test-msi.ps1 A.msi B.msi   (B: the same version, built again)
# 1. A installs: the service runs, kc9k.exe and the GUI are in place, the
#    Start menu entry exists, kc9k answers through the service.
# 2. A is removed: the service and program go.
# 3. A again, a block locked: removal is refused (kc9k's words in the log)
#    and everything stays.
# 4. "Block the installer" on while locked: B (a reinstall) is refused.
param([string]$A, [string]$B)
$ErrorActionPreference = "Stop"
$failures = 0
function Check($ok, $what) {
    if ($ok) { Write-Host "ok   $what" } else { Write-Host "FAIL $what"; $script:failures++ }
}
function Msi($verb, $file, $log) {
    $p = Start-Process msiexec.exe -ArgumentList "$verb `"$file`" /qn /l*v `"$log`"" -Wait -PassThru
    return $p.ExitCode
}
$kc9k = Join-Path $env:ProgramFiles "KC9K\kc9k.exe"
$gui = Join-Path $env:ProgramFiles "KC9K\gui\kc9k-gui.exe"
$logs = Join-Path $env:RUNNER_TEMP "msi-logs"
New-Item -ItemType Directory -Force $logs | Out-Null

# 1. Install
$code = Msi "/i" $A "$logs\1-install.log"
Check ($code -eq 0) "install exits 0 (got $code)"
Check (Test-Path $kc9k) "kc9k.exe in Program Files\KC9K"
Check (Test-Path $gui) "the GUI in Program Files\KC9K\gui"
$service = Get-Service kc9k -ErrorAction SilentlyContinue
Check ($service -and $service.Status -eq "Running") "the service runs"
Check (Test-Path (Join-Path $env:ProgramData "Microsoft\Windows\Start Menu\Programs\Killer Chicken 9000.lnk")) "the Start menu entry"
$state = & $kc9k gui-state 2>&1 | Out-String
Check ($state -match '"blocks"') "kc9k gui-state answers through the service"

# 2. Remove
$code = Msi "/x" $A "$logs\2-remove.log"
Check ($code -eq 0) "removal exits 0 (got $code)"
Start-Sleep -Seconds 15
Check (-not (Get-Service kc9k -ErrorAction SilentlyContinue)) "the service is gone"
Check (-not (Test-Path $kc9k)) "kc9k.exe is gone"
Check (-not (Test-Path $gui)) "the GUI is gone"

# 3. Removal refused while locked
$code = Msi "/i" $A "$logs\3-install.log"
Check ($code -eq 0) "install again exits 0 (got $code)"
& $kc9k block create "CI lock" | Out-Host
& $kc9k block add "CI lock" example.com | Out-Host
& $kc9k start "CI lock" --for 30m --yes | Out-Host
$code = Msi "/x" $A "$logs\4-remove-locked.log"
Check ($code -ne 0) "removal while locked is refused (got $code)"
$log = Get-Content "$logs\4-remove-locked.log" -Raw -Encoding Unicode
Check ($log -match "Killer Chicken 9000: ") "kc9k's own words are in the log"
Check ((Get-Service kc9k -ErrorAction SilentlyContinue).Status -eq "Running") "the service still runs"
Check (Test-Path $gui) "the GUI is still there"

# 4. Reinstall refused with "block the installer" on while locked
& $kc9k tamper installer on | Out-Host
$code = Msi "/i" $B "$logs\5-reinstall-locked.log"
Check ($code -ne 0) "reinstalling the same version while locked is refused (got $code)"
$log = Get-Content "$logs\5-reinstall-locked.log" -Raw -Encoding Unicode
Check ($log -match "block the installer") "the refusal names block the installer"
Check ((Get-Service kc9k -ErrorAction SilentlyContinue).Status -eq "Running") "the service still runs"

if ($failures -gt 0) { Write-Host "$failures check(s) failed"; exit 1 }
Write-Host "all MSI checks passed"
