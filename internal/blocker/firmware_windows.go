package blocker

// firmware_windows.go — the firmware lock's Windows way to the firmware
// (W19): PowerShell scripts over the vendors' firmware WMI (Dell
// root\dcim\sysman, Lenovo root\wmi, HP root\HP\InstrumentedBIOS). What
// they answer is read by firmware_wmi.go. Passwords go over standard
// input, never on a command line.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func init() { platformFirmware = wmiFirmware{} }

type wmiFirmware struct{}

// firmwareReadScript prints the vendor, whether an admin (setup) password
// is set, and every enumeration setting with its possible values.
const firmwareReadScript = `$ErrorActionPreference = 'SilentlyContinue'
$out = @{ vendor = ''; password = $false; settings = @() }
$d = Get-CimInstance -Namespace root/dcim/sysman/biosattributes -ClassName EnumerationAttribute
if ($d) {
  $out.vendor = 'dell'
  $out.settings = @($d | ForEach-Object { @{ name = $_.AttributeName; current = [string]$_.CurrentValue; possible = @($_.PossibleValue) } })
  $p = Get-CimInstance -Namespace root/dcim/sysman/wmisecurity -ClassName PasswordObject | Where-Object { $_.NameId -eq 'Admin' }
  $out.password = [bool]($p.IsPasswordSet -eq 1)
} else {
  $l = Get-WmiObject -Namespace root/wmi -Class Lenovo_BiosSetting
  if ($l) {
    $out.vendor = 'lenovo'
    $sel = Get-WmiObject -Namespace root/wmi -Class Lenovo_GetBiosSelections
    $out.settings = @($l | Where-Object { $_.CurrentSetting } | ForEach-Object {
      $pair = (($_.CurrentSetting -split ';')[0]) -split ',', 2
      $possible = @()
      if ($sel) { $r = $sel.GetBiosSelections($pair[0]); if ($r.Selections) { $possible = @($r.Selections -split ',') } }
      @{ name = $pair[0]; current = [string]$pair[1]; possible = $possible } })
    $pw = Get-WmiObject -Namespace root/wmi -Class Lenovo_BiosPasswordSettings
    $out.password = [bool]($pw.PasswordState -band 2)
  } else {
    $h = Get-CimInstance -Namespace root/HP/InstrumentedBIOS -ClassName HP_BIOSEnumeration
    if ($h) {
      $out.vendor = 'hp'
      $out.settings = @($h | ForEach-Object { @{ name = $_.Name; current = [string]$_.CurrentValue; possible = @($_.PossibleValues) } })
      $pw = Get-CimInstance -Namespace root/HP/InstrumentedBIOS -ClassName HP_BIOSPassword | Where-Object { $_.Name -eq 'Setup Password' }
      $out.password = [bool]($pw.IsSet -eq 1)
    }
  }
}
$out | ConvertTo-Json -Depth 4 -Compress`

// firmwareWriteScript reads a wmiRequest from standard input, writes each
// change, then the password when asked; it stops at the first refusal and
// prints what refused.
const firmwareWriteScript = `$ErrorActionPreference = 'Stop'
$req = [Console]::In.ReadToEnd() | ConvertFrom-Json
$pw = [string]$req.password
function Fail($what) { [Console]::Error.WriteLine($what); exit 1 }
switch ($req.vendor) {
  'dell' {
    $iface = Get-WmiObject -Namespace root/dcim/sysman/biosattributes -Class BIOSAttributeInterface
    $bytes = [Text.Encoding]::ASCII.GetBytes($pw)
    $type = if ($pw) { 1 } else { 0 }
    foreach ($c in $req.changes) {
      $r = $iface.SetAttribute([uint32]$type, [uint32]$bytes.Length, $bytes, $c.name, $c.value)
      if ($r.Status -ne 0) { Fail "setting $($c.name): status $($r.Status)" }
    }
    if ($req.set_password) {
      $sec = Get-WmiObject -Namespace root/dcim/sysman/wmisecurity -Class SecurityInterface
      $r = $sec.SetNewPassword([uint32]$type, [uint32]$bytes.Length, $bytes, 'Admin', $pw, [string]$req.new_password)
      if ($r.Status -ne 0) { Fail "password: status $($r.Status)" }
    }
  }
  'lenovo' {
    $auth = if ($pw) { ",$pw,ascii,us" } else { '' }
    $set = Get-WmiObject -Namespace root/wmi -Class Lenovo_SetBiosSetting
    foreach ($c in $req.changes) {
      $r = $set.SetBiosSetting("$($c.name),$($c.value)$auth")
      if ($r.return -ne 'Success') { Fail "setting $($c.name): $($r.return)" }
    }
    if ($req.changes.Count -gt 0) {
      $save = Get-WmiObject -Namespace root/wmi -Class Lenovo_SaveBiosSettings
      $r = $save.SaveBiosSettings($(if ($pw) { "$pw,ascii,us" } else { '' }))
      if ($r.return -ne 'Success') { Fail "saving: $($r.return)" }
    }
    if ($req.set_password) {
      $p = Get-WmiObject -Namespace root/wmi -Class Lenovo_SetBiosPassword
      $r = $p.SetBiosPassword("pap,$pw,$([string]$req.new_password),ascii,us")
      if ($r.return -ne 'Success') { Fail "password: $($r.return)" }
    }
  }
  'hp' {
    $iface = Get-WmiObject -Namespace root/HP/InstrumentedBIOS -Class HP_BIOSSettingInterface
    $auth = if ($pw) { "<utf-16/>$pw" } else { '' }
    foreach ($c in $req.changes) {
      $r = $iface.SetBIOSSetting($c.name, $c.value, $auth)
      if ($r.Return -ne 0) { Fail "setting $($c.name): return $($r.Return)" }
    }
    if ($req.set_password) {
      $new = if ($req.new_password) { "<utf-16/>$($req.new_password)" } else { '<utf-16/>' }
      $r = $iface.SetBIOSSetting('Setup Password', $new, $auth)
      if ($r.Return -ne 0) { Fail "password: return $($r.Return)" }
    }
  }
  default { Fail "no firmware interface" }
}`

// firmwareScript runs a script with stdin; a variable for tests.
var firmwareScript = func(script string, stdin []byte) ([]byte, error) {
	ps := filepath.Join(systemRoot(), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, ps, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Stdin = bytes.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (wmiFirmware) read() (FirmwareStatus, string, error) {
	out, err := firmwareScript(firmwareReadScript, nil)
	if err != nil {
		return FirmwareStatus{}, "", fmt.Errorf("reading the firmware's settings: %w", err)
	}
	return wmiStatus(bytes.TrimSpace(out))
}

func (f wmiFirmware) status() (FirmwareStatus, error) {
	s, _, err := f.read()
	return s, err
}

func (f wmiFirmware) write(req wmiRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if _, err := firmwareScript(firmwareWriteScript, data); err != nil {
		return fmt.Errorf("the firmware refused: %w", err)
	}
	return nil
}

func (f wmiFirmware) apply(password string, s FirmwareStatus) (map[string]string, error) {
	_, vendor, err := f.read()
	if err != nil {
		return nil, err
	}
	if err := checkWMIPassword(vendor, password); err != nil {
		return nil, err
	}
	req, before := lockRequest(vendor, password, s)
	if err := f.write(req); err != nil {
		return nil, err
	}
	return before, nil
}

func (f wmiFirmware) remove(password string, before map[string]string) error {
	s, vendor, err := f.read()
	if err != nil {
		return err
	}
	if !s.Available() {
		return fmt.Errorf("this computer's firmware offers no settings Windows can change")
	}
	return f.write(unlockRequest(vendor, password, before))
}
