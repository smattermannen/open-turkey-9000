# W41: runs msiexec with its full UI and answers its dialogs through UI
# Automation, as a person clicking would: ticks the licence box, presses
# Install, Yes, Next, Finish, OK or Close (never Cancel, No or Back), and
# writes every dialog's text to the transcript. Windows PowerShell 5.1
# (UIAutomationClient ships with .NET Framework).
#   powershell -File msi-ui.ps1 -Arguments '/x "A.msi"' -Log x.log -Transcript t.txt
param([string]$Arguments, [string]$Log, [string]$Transcript, [int]$Seconds = 300)
$ErrorActionPreference = "Stop"
Add-Type -AssemblyName UIAutomationClient, UIAutomationTypes
$A = [System.Windows.Automation.AutomationElement]
$C = [System.Windows.Automation.Condition]
$TS = [System.Windows.Automation.TreeScope]
$p = Start-Process msiexec.exe -ArgumentList "$Arguments /l*v `"$Log`"" -PassThru
$seen = New-Object System.Collections.Generic.List[string]
$press = @("Install", "Yes", "Next", "Finish", "OK", "Close")
$deadline = (Get-Date).AddSeconds($Seconds)
while (-not $p.HasExited -and (Get-Date) -lt $deadline) {
    Start-Sleep -Milliseconds 700
    $pids = @($p.Id) + @(Get-CimInstance Win32_Process -Filter "Name='msiexec.exe'" | Select-Object -ExpandProperty ProcessId)
    $windows = $A::RootElement.FindAll($TS::Children, $C::TrueCondition) |
        Where-Object { $pids -contains $_.Current.ProcessId }
    foreach ($w in $windows) {
        try {
            $all = $w.FindAll($TS::Descendants, $C::TrueCondition)
            $texts = ($all | Where-Object { $_.Current.ControlType -eq [System.Windows.Automation.ControlType]::Text } |
                ForEach-Object { $_.Current.Name } | Where-Object { $_ }) -join " | "
            $line = "[$($w.Current.Name)] $texts"
            if (-not $seen.Contains($line)) { $seen.Add($line); Write-Host "dialog: $line" }
            foreach ($box in ($all | Where-Object { $_.Current.ControlType -eq [System.Windows.Automation.ControlType]::CheckBox })) {
                if ($box.Current.Name -match "accept") {
                    $t = $box.GetCurrentPattern([System.Windows.Automation.TogglePattern]::Pattern)
                    if ($t.Current.ToggleState -ne "On") { $t.Toggle() }
                }
            }
            $buttons = $all | Where-Object { $_.Current.ControlType -eq [System.Windows.Automation.ControlType]::Button -and $_.Current.IsEnabled }
            foreach ($name in $press) {
                $b = $buttons | Where-Object { ($_.Current.Name -replace '&', '') -eq $name } | Select-Object -First 1
                if ($b) {
                    Write-Host "press: $name"
                    $b.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
                    break
                }
            }
        } catch { } # a dialog closing while read
    }
}
if (-not $p.HasExited) { Write-Host "msiexec still running after $Seconds s"; $p.Kill(); $code = 9999 } else { $code = $p.ExitCode }
$seen | Set-Content -Path $Transcript -Encoding UTF8
Write-Host "msiexec exit $code"
exit $code
