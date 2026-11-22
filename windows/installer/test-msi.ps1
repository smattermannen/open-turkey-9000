# W7 check on windows-latest (a disposable administrator's machine):
#   test-msi.ps1 A.msi B.msi   (B: the same version, built again)
# 1. A installs: the service runs, kc9k.exe and the GUI are in place, the
#    Start menu entry exists, kc9k answers through the service.
# 2. A is removed: the service and program go.
# 3. A again, a block locked: removal is refused (kc9k's words in the log)
#    and everything stays.
# 4. "Block the installer" on while locked: B (a reinstall) is refused.
# W41, with -Older (0.0.9) and -Newer (0.2.0, its kc9k.exe says so):
# 1 and 2 run through the MSI's full UI (msi-ui.ps1 answers the dialogs
# as a person would); then, still locked with block the installer on:
# 5. the older version is refused; 6. a repair is refused; 7. the newer
# version installs, and the block stays locked; 8. removal through the
# full UI is refused with kc9k's words in a dialog.
param([string]$A, [string]$B, [string]$Older, [string]$Newer)
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

function MsiUi($arguments, $name) {
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot "msi-ui.ps1") -Arguments $arguments -Log "$logs\$name.log" -Transcript "$logs\$name-dialogs.txt" | Out-Host
    return $LASTEXITCODE
}
function Locked($name) {
    $s = (& $kc9k gui-state 2>&1 | Out-String)
    try { $b = ($s.Substring($s.IndexOf('{')) | ConvertFrom-Json).blocks | Where-Object name -eq $name } catch { return $false }
    return ($b -and $b.active -and $b.lock_seconds -gt 0)
}

# 1. Install, through the full UI
$code = if ($Older) { MsiUi "/i `"$A`"" "1-install-ui" } else { Msi "/i" $A "$logs\1-install.log" }
Check ($code -eq 0) "install exits 0 (got $code)"
Check (Test-Path $kc9k) "kc9k.exe in Program Files\KC9K"
Check (Test-Path $gui) "the GUI in Program Files\KC9K\gui"
$service = Get-Service kc9k -ErrorAction SilentlyContinue
Check ($service -and $service.Status -eq "Running") "the service runs"
Check (Test-Path (Join-Path $env:ProgramData "Microsoft\Windows\Start Menu\Programs\Killer Chicken 9000.lnk")) "the Start menu entry"
$state = & $kc9k gui-state 2>&1 | Out-String
Check ($state -match '"blocks"') "kc9k gui-state answers through the service"

# 2. Remove, through the full UI (as Apps & features does)
$code = if ($Older) { MsiUi "/x `"$A`"" "2-remove-ui" } else { Msi "/x" $A "$logs\2-remove.log" }
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

if ($Older -and $Newer) {
    # 5. An older version while locked
    $code = Msi "/i" $Older "$logs\6-older-locked.log"
    Check ($code -ne 0) "an older version while locked is refused (got $code)"
    $log = Get-Content "$logs\6-older-locked.log" -Raw -Encoding Unicode
    Check ($log -match "block the installer") "the older version's refusal names block the installer"
    Check ((& $kc9k version 2>&1 | Out-String) -notmatch "0\.0\.9") "the installed kc9k.exe is not the older one"

    # 6. A repair while locked
    $code = Msi "/fa" $A "$logs\7-repair-locked.log"
    Check ($code -ne 0) "a repair while locked is refused (got $code)"
    Check ((Get-Service kc9k -ErrorAction SilentlyContinue).Status -eq "Running") "the service still runs after the refused repair"

    # 7. A newer version over the locked install
    $code = Msi "/i" $Newer "$logs\8-newer-locked.log"
    Check ($code -eq 0) "a newer version installs over a locked install (got $code)"
    Start-Sleep -Seconds 20
    Check ((Get-Service kc9k -ErrorAction SilentlyContinue).Status -eq "Running") "the service runs after the upgrade"
    $v = & $kc9k version 2>&1 | Out-String
    Check ($v -match "0\.2\.0") "kc9k version says 0.2.0 (got $($v.Trim()))"
    Check (Locked "CI lock") "the block is still on and locked after the upgrade"
    Check (Test-Path $gui) "the GUI is there after the upgrade"
    $products = Get-CimInstance Win32_Product -Filter "Name='Killer Chicken 9000'" -ErrorAction SilentlyContinue
    Check (@($products).Count -eq 1) "one Killer Chicken 9000 in Apps & features (got $(@($products).Count))"

    # 8. Removal through the full UI while locked
    $code = MsiUi "/x `"$Newer`"" "9-remove-ui-locked"
    Check ($code -ne 0) "removal through the full UI while locked is refused (got $code)"
    $dialogs = Get-Content "$logs\9-remove-ui-locked-dialogs.txt" -Raw -ErrorAction SilentlyContinue
    Check ($dialogs -match "Killer Chicken 9000: ") "a dialog shows kc9k's own words"
    Check ((Get-Service kc9k -ErrorAction SilentlyContinue).Status -eq "Running") "the service still runs"
    Check (Locked "CI lock") "the block is still locked"
}

if ($failures -gt 0) { Write-Host "$failures check(s) failed"; exit 1 }
Write-Host "all MSI checks passed"
