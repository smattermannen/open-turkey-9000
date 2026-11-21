# Signs files with the owner's code-signing certificate (SPEC §6 2026-11-21
# later; W7). The certificate comes from the environment, never the repo:
#   SIGNING_PFX_BASE64    the .pfx, base64
#   SIGNING_PFX_PASSWORD  its password
# Azure Artifact Signing instead: replace this step with Microsoft's
# trusted-signing action (notes/install-windows.md).
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Files)
$ErrorActionPreference = "Stop"
$pfx = Join-Path $env:RUNNER_TEMP "kc9k-signing.pfx"
[IO.File]::WriteAllBytes($pfx, [Convert]::FromBase64String($env:SIGNING_PFX_BASE64))
try {
    $signtool = Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\bin" -Recurse -Filter signtool.exe |
        Where-Object FullName -match "x64" | Sort-Object FullName -Descending | Select-Object -First 1
    foreach ($f in $Files) {
        & $signtool.FullName sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 /f $pfx /p $env:SIGNING_PFX_PASSWORD /d "Killer Chicken 9000" $f
        if ($LASTEXITCODE -ne 0) { throw "signing $f failed" }
    }
} finally { Remove-Item $pfx -Force }
