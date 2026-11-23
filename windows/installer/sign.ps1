# Signs files with the owner's code signing (SPEC §6 2026-11-21 later; W7).
# Never from the repo; one of two ways, from the environment:
#   a certificate:  SIGNING_PFX_BASE64 (the .pfx, base64), SIGNING_PFX_PASSWORD
#   Azure Artifact Signing (Trusted Signing): AZURE_SIGNING_ENDPOINT (such as
#     https://weu.codesigning.azure.net), AZURE_SIGNING_ACCOUNT,
#     AZURE_SIGNING_PROFILE, and the app registration that may sign:
#     AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET (read by the
#     signing client); signtool with Microsoft's Trusted Signing dlib.
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Files)
$ErrorActionPreference = "Stop"
$signtool = Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\bin" -Recurse -Filter signtool.exe |
    Where-Object FullName -match "x64" | Sort-Object FullName -Descending | Select-Object -First 1
if ($env:AZURE_SIGNING_ACCOUNT) {
    # Microsoft.Trusted.Signing.Client from nuget.org: its dlib lets signtool sign through the account.
    $dir = Join-Path $env:RUNNER_TEMP "trusted-signing"
    New-Item -ItemType Directory -Force $dir | Out-Null
    $id = "microsoft.trusted.signing.client"
    $version = (Invoke-RestMethod "https://api.nuget.org/v3-flatcontainer/$id/index.json").versions | Where-Object { $_ -notmatch "-" } | Select-Object -Last 1
    Invoke-WebRequest "https://api.nuget.org/v3-flatcontainer/$id/$version/$id.$version.nupkg" -OutFile "$dir\client.zip"
    Expand-Archive "$dir\client.zip" "$dir\client" -Force
    $dlib = Get-ChildItem "$dir\client" -Recurse -Filter Azure.CodeSigning.Dlib.dll | Where-Object FullName -match "x64" | Select-Object -First 1
    if (-not $dlib) { throw "Azure.CodeSigning.Dlib.dll not in $id $version" }
    $metadata = Join-Path $dir "metadata.json"
    @{ Endpoint = $env:AZURE_SIGNING_ENDPOINT; CodeSigningAccountName = $env:AZURE_SIGNING_ACCOUNT; CertificateProfileName = $env:AZURE_SIGNING_PROFILE } |
        ConvertTo-Json | Set-Content -Encoding utf8 $metadata
    foreach ($f in $Files) {
        & $signtool.FullName sign /v /fd SHA256 /tr http://timestamp.acs.microsoft.com /td SHA256 /dlib $dlib.FullName /dmdf $metadata /d "Killer Chicken 9000" $f
        if ($LASTEXITCODE -ne 0) { throw "signing $f failed" }
    }
    exit 0
}
$pfx = Join-Path $env:RUNNER_TEMP "kc9k-signing.pfx"
[IO.File]::WriteAllBytes($pfx, [Convert]::FromBase64String($env:SIGNING_PFX_BASE64))
try {
    foreach ($f in $Files) {
        & $signtool.FullName sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 /f $pfx /p $env:SIGNING_PFX_PASSWORD /d "Killer Chicken 9000" $f
        if ($LASTEXITCODE -ne 0) { throw "signing $f failed" }
    }
} finally { Remove-Item $pfx -Force }
