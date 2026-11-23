# Install on Windows (the MSI)

- Covers: `windows/installer/` (`Package.wxs`, `Kc9k.Installer.wixproj`, `Actions/` custom actions, `License.rtf`, `test-msi.ps1`, `MsiUi/` (FlaUI console tool), `sign.ps1`), the `installer` job of `.github/workflows/windows-gui.yml`, `.github/workflows/windows-release.yml`, `cli/version.go`
- SPEC: §4.11, R-LOCK-9, §4.10 (block the installer), §6 2026-11-21 (signing)
- Connects to: [windows](windows.md) (Core: `kc9k install`/`uninstall`, the service), [install](install.md) (Linux's guards), [gui-windows](gui-windows.md)

## Code

- WiX 6, one x64 per-machine `KillerChicken9000.msi`, `WixUI_Minimal`.
  Windows-only build: `dotnet build windows/installer -p:GuiDir=…
  -p:Kc9kExe=… -p:ActionsDll=… -p:Version=x.y.z`. UpgradeCode
  `fa22d6e5-b444-4953-a861-4aabffdf469d` fixed; new ProductCode per build.
- MSI owns `Program Files\KC9K\gui\` (with `setup\kc9k.exe`), Start menu
  entry; `kc9k install` the rest ([windows](windows.md)).
- Custom actions (`Actions/CustomActions.cs`, DTF, .NET Fx 4.7.2,
  SYSTEM): `RunKc9k` runs `"[optional|]path|arguments"`, stdin closed (a
  question = refusal); non-zero exit shows kc9k's words and rolls back.
  `DetectSameOrNewer` sets `KC9K_GUARD` for same/newer installed version
  or repair.
- Sequence: `Kc9kGuard` (after `InstallInitialize`, if `KC9K_GUARD`):
  installed `kc9k.exe prepare-install` (refused while locked with block
  the installer on). `Kc9kInstall` (after
  `InstallFiles`): `gui\setup\kc9k.exe install`. `Kc9kUninstall` (after
  `RemoveFiles`, real removal only): `kc9k.exe uninstall`, refused while
  anything is locked. `MajorUpgrade` allows same/older (guard decides),
  removes the old after `afterInstallExecute`, so its uninstall never runs.
- Removal keeps `ProgramData\kc9k`, `%APPDATA%\kc9k\gui.json` and the
  Startup shortcut. With a settings password, removal from Apps & features
  is refused (MSI can't type it): use `kc9k uninstall` as administrator.
- Version: `kc9k version` → `kc9k VERSION (os/arch)` (`-ldflags -X
  …/internal/cli.Version=1.2.3`, else "dev"; no access password). GUI:
  `-p:Version`, `-p:InformationalVersion`.
- Signing (owner's certificate): `sign.ps1` (signtool, PFX from secrets
  `SIGNING_PFX_BASE64`/`SIGNING_PFX_PASSWORD`) signs both exes and the MSI
  when set. Azure Artifact Signing: Microsoft's trusted-signing action.
- Release (`windows-release.yml`): tag `v1.2.3` builds, signs, test-installs,
  publishes MSI + `.sha256`
  (ProductVersion: numbers before `-`/`+`). `workflow_dispatch`: no publish.
- CI (`installer` job, `test-msi.ps1 -A -B -Older -Newer`; builds 0.1.0
  ×2, 0.0.9, 0.2.0): full-UI install/removal via `MsiUi/` (presses
  Install/Yes/Next/Finish/OK, never Close); removal refused while locked;
  with block the installer on and a block locked: reinstall, older, repair
  refused; newer installs, block stays locked, one Apps & features entry;
  removal refused with kc9k's words. Signing checked with a throwaway
  self-signed certificate.
