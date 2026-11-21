# Install on Windows (the MSI)

- Covers: `windows/installer/` (`Package.wxs`, `Kc9k.Installer.wixproj`, `Actions/` custom actions, `License.rtf`, `test-msi.ps1`, `sign.ps1`), the `installer` job of `.github/workflows/windows-gui.yml`
- SPEC: §4.11, R-LOCK-9, §4.10 (block the installer), §6 2026-11-21 (signing)
- Connects to: [windows](windows.md) (Core: `kc9k install`/`uninstall`, the service), [install](install.md) (Linux's guards), [gui-windows](gui-windows.md)

## Code

- WiX 6 (`WixToolset.Sdk/6.0.2`), one x64 per-machine MSI,
  `KillerChicken9000.msi`, UI `WixUI_Minimal` (the licence, then install).
  Built on Windows only (WiX writes the MSI through Windows' msi.dll): `dotnet build windows/installer -p:GuiDir=<published GUI>
  -p:Kc9kExe=<kc9k.exe> -p:ActionsDll=<…CA.dll> -p:Version=<x.y.z>`.
  UpgradeCode `fa22d6e5-b444-4953-a861-4aabffdf469d` never changes;
  every build is a new ProductCode, so a reinstall is a major upgrade.
- Layout: the MSI owns `Program Files\KC9K\gui\` (the published GUI,
  harvested with `<Files Include>`), a copy of `kc9k.exe` in
  `gui\setup\`, and the Start menu entry. `Program Files\KC9K\kc9k.exe`,
  the service, PATH and the folder's SYSTEM ownership are `kc9k install`'s
  (Core, notes/windows.md); the GUI finds that `kc9k.exe` one folder up.
- Custom actions (`Actions/CustomActions.cs`, DTF, .NET Framework 4.7.2,
  run as SYSTEM): `RunKc9k` runs kc9k with `CustomActionData`
  "`[optional|]path|arguments`", standard input closed (a question is a
  refusal), output logged; on a non-zero exit it shows "Killer Chicken
  9000: <kc9k's words>" (an error dialog with UI, the log when quiet) and
  fails, so the MSI rolls back. `DetectSameOrNewer` sets `KC9K_GUARD` when
  an installed version of this UpgradeCode is the same or newer, or for a
  repair (`REINSTALL`).
- Sequence: `Kc9kGuard` (after `InstallInitialize`, with `KC9K_GUARD`):
  the installed `kc9k.exe prepare-install`, refused while locked with
  "block the installer" on (a newer version is never asked);
  `Kc9kInstall` (after `InstallFiles`): `gui\setup\kc9k.exe install`
  (upgrades in place); `Kc9kUninstall` (after `RemoveFiles`, on removal
  but not an upgrade's removal of the old version): `kc9k.exe uninstall`,
  refused while anything is locked (the GUI's files come back by
  rollback). `MajorUpgrade` allows the same and older versions (the guard
  decides) and removes the old version after the new one is in
  (`afterInstallExecute`), so its uninstall never runs.
- Data stays on removal (`ProgramData\kc9k`), as `kc9k uninstall` leaves it.
- Signing (owner's certificate): `sign.ps1` signs with signtool and a
  PFX from `SIGNING_PFX_BASE64`/`SIGNING_PFX_PASSWORD` (secrets); the
  workflow signs `kc9k.exe`, `kc9k-gui.exe` and the MSI only when they
  are set. For Azure Artifact Signing, replace the two steps with
  Microsoft's trusted-signing action (endpoint, account, profile).

## Built behaviour

- A settings password set: `kc9k uninstall` asks for it, and the MSI has
  no way to type it, so removal from Apps & features is refused with the
  CLI's words; remove with `kc9k uninstall` in an administrator's console.
- The GUI's own preferences (`%APPDATA%\kc9k\gui.json`) and the Startup
  shortcut it makes stay after removal, as per-user files.
