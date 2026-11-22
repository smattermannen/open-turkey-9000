# Install on Windows (the MSI)

- Covers: `windows/installer/` (`Package.wxs`, `Kc9k.Installer.wixproj`, `Actions/` custom actions, `License.rtf`, `test-msi.ps1`, `MsiUi/` (FlaUI console tool), `sign.ps1`), the `installer` job of `.github/workflows/windows-gui.yml`, `.github/workflows/windows-release.yml`, `cli/version.go`
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

- Versions (W43): `kc9k version` prints `kc9k VERSION (os/arch)`; the
  version is set at build (`-ldflags "-X
  github.com/smattermannen/kc9k/internal/cli.Version=1.2.3"`), "dev"
  otherwise; no access password asked. The GUI's version is the
  assembly's (`-p:Version`, `-p:InformationalVersion`).
- Release (W40, `windows-release.yml`): a tag `v1.2.3` builds kc9k.exe
  with that version, the GUI and the MSI (ProductVersion: the numbers
  before any `-` or `+`), signs when the certificate's secrets are set,
  installs the MSI and checks `kc9k version`, removes it, then publishes a
  GitHub release with the MSI and `.sha256`. By hand
  (`workflow_dispatch`, a version) it builds and checks without
  publishing.
- CI checks (W41, `test-msi.ps1 -A -B -Older -Newer`; the `installer` job
  builds 0.1.0 twice, 0.0.9 and 0.2.0, each with its own kc9k.exe):
  install and removal through the full UI (`MsiUi/`, a FlaUI console tool:
  it ticks the licence box and presses Install,
  Yes, Next, Finish, OK or Close, and keeps each dialog's text), removal
  refused while locked, then with block the installer on and a block
  locked: a reinstall, an older version and a repair refused, a newer
  version installed with the block still locked and one entry in Apps &
  features, removal through the full UI refused with kc9k's words in a
  dialog.
- Signing proven (W42): the `installer` job makes a throwaway
  self-signed certificate, signs copies of kc9k.exe, kc9k-gui.exe and the
  MSI with `sign.ps1`, and checks each carries its signature.

## Built behaviour

- A settings password set: `kc9k uninstall` asks for it, and the MSI has
  no way to type it, so removal from Apps & features is refused with the
  CLI's words; remove with `kc9k uninstall` in an administrator's console.
- The GUI's own preferences (`%APPDATA%\kc9k\gui.json`) and the Startup
  shortcut it makes stay after removal, as per-user files.
