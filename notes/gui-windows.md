# GUI on Windows

- Covers: `windows/gui/` (`Kc9k.Core/`, `Kc9k.Gui/`, `FakeKc9k/`, `Kc9k.Tests/`, `Kc9k.UiTests/`, `fixtures/`, `check.sh`), `windows/Directory.Build.props`, `.github/workflows/windows-gui.yml`
- SPEC: §4.1–4.13, R-GUI-1–3, §6 2026-11-21 (Windows)
- Connects to: [gui](gui.md) (the GTK GUI, the feature reference), [testing](testing.md), [install-windows](install-windows.md)

## Code

- **Shape**: C# on WinUI 3 (Windows App SDK 2.6, .NET 10, unpackaged,
  self-contained, x64), written without XAML markup: every page is built
  in C# (`Ui.cs` builders), so the same sources compile on Linux
  (`windows/gui/check.sh`: Core tests plus a compile of the GUI against
  WinUI with `EnableWindowsTargeting`); packaging (PRI, manifest) needs
  Windows and runs in CI. `App` implements `IXamlMetadataProvider` through
  WinUI's `XamlControlsXamlMetaDataProvider` (no XAML compiler to generate
  it) and needs `EnableMsixTooling` so the PRI holds WinUI's theme
  resources. `Program.Main` replaces the generated one; one GUI per user
  (`AppInstance.FindOrRegisterForKey`), a second start brings the first
  forward (`KC9K_GUI_MULTI=1` allows several, for tests).
- **The contract** (`Kc9k.Core/Cli.cs`): `ProcessCli` runs `kc9k.exe`
  (found by `KC9K_CLI`, next to the GUI, one folder up, then `PATH`) with
  no console window, so a text lock's challenge opens in the CLI's own
  window (R-LOCK-5). `CliSession` adds the GUI's passwords as the GTK GUI
  does: access passwords first (`--access-lines N`), then the settings
  password (`--settings-line`), then the command's own secret, all on
  standard input; block passwords go with `gui-state --reveal`.
  `Controller` (`Shell/`) is the GTK `Window`'s run/refresh: commands in
  the background with a thin progress bar (R-GUI-3: the window never
  freezes), the state read again after each command and every 5 s (stats
  every 30 s), a read that predates the last change read again; a refusal
  shown in a dialog in the CLI's words; "needs the settings/access
  password" asks for it and runs the command again; a password that opens
  nothing any more (`access_unused`) is dropped; idle minutes forget the
  access passwords.
- **Drafts** (§4.3, `Kc9k.Core/Drafts.cs`): the GTK GUI's `DRAFT_EDITS`
  and `DRAFT_READS` as `Drafts.EditPosition`/`ReadPosition`; a command
  that changes what the opened block's tabs show is moved to `NAME
  (draft)`, the first one making the draft (`block draft`, with the
  block password when it has one).
- **State** (`Kc9k.Core/State.cs`): typed models of `gui-state` and
  `stats --json`, read with snake_case names; a field the CLI adds is
  ignored until a property names it. `Core` window type is `TimeWindow`
  (WinUI has a `Window`). Words and typed values are ported one to one
  from the GTK GUI (`Words.cs`, `Fields.cs`: `DurationRule`,
  `NumberRule`, `LockSpec` = `LockPicker.chosen()`, `waiting_words`,
  `relock_lines`; `Entries.cs`: `split_entries`; `Phrases.cs`), with
  tests against the fixtures.
- **Design layer** (`Theme.cs`, R-GUI-3): cards, pills and text styles as
  a XAML string read at run time, so `{ThemeResource}` follows light,
  dark and high contrast live; Mica behind the window, a solid background
  from the darkness slider in dark (§4.10). Every control has an
  accessible name and the tests' `AutomationId`s; charts are built from
  ordinary elements (Narrator reads each row). Dialogs (`Ui/Dialogs.cs`)
  are ContentDialogs, one at a time; `ConfirmLock` is R-LOCK-12's one
  confirmation before `--yes`.
- **Pages**: the GTK GUI's sidebar (`Tables.Pages`), Settings at the
  bottom as Windows apps have it; each page under a cover while its access
  area is closed (§3.11). Built: Overview (§4.1). Not yet:
  `Pages/NotYet.cs` says so on the page.
- **Preferences** (`Prefs.cs`): `%APPDATA%\kc9k\gui.json` (`KC9K_GUI_PREFS`
  for tests): theme, darkness, clock, first weekday, tray, list heights,
  window size. Static field order matters: `Options` is set before
  `Current` reads the file.

## Tests

- `FakeKc9k` (built as `kc9k.exe`) replays `fixtures/gui-state.json` and
  `stats.json` from `KC9K_FAKE_DIR`, answers what `responses.json` lists
  (by leading arguments) and records each command with its standard input
  in `commands.log`. The fixtures come from the real Linux CLI:
  `fixtures/make-fixtures.sh path/to/kc9k` as a normal user (a test DB
  filled through kc9k commands; statistics written into the test DB).
- `Kc9k.UiTests` (FlaUI, UIA3) start the published GUI against the fake
  (`KC9K_GUI_EXE`, `KC9K_FAKE_EXE`), click through every page in light and
  dark, press buttons through UI Automation (`GuiApp.Invoke`, no pointer)
  and check the commands run; screenshots go to `KC9K_SCREENSHOTS` and are
  uploaded by the workflow (`windows-gui-screenshots`). The GUI log
  (`%LOCALAPPDATA%\kc9k\gui.log`) is printed after the tests.

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- Hotkeys are worded with win+ (the CLI also takes super+).
