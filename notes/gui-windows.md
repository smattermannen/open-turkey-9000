# GUI on Windows

- Covers: `windows/gui/` (`Kc9k.Core/`, `Kc9k.Gui/`, `FakeKc9k/`, `Kc9k.Tests/`, `Kc9k.UiTests/`, `fixtures/`, `check.sh`), `windows/Directory.Build.props`, `.github/workflows/windows-gui.yml`
- SPEC: §4.1–4.13, R-GUI-1–3, §6 2026-11-21 (Windows)
- Connects to: [gui](gui.md) (the GTK GUI, the feature reference), [testing](testing.md), [install-windows](install-windows.md)

## Code

- **Shape**: C#, WinUI 3 (App SDK 2.6, .NET 10, unpackaged,
  self-contained, x64), no XAML markup (pages built in C#, `Ui.cs`), so
  `windows/gui/check.sh` builds Core tests and the GUI on Linux
  (`EnableWindowsTargeting`); packaging needs Windows. `App` implements
  `IXamlMetadataProvider` (`XamlControlsXamlMetaDataProvider`);
  `EnableMsixTooling` puts theme resources in the PRI. One GUI per user
  (`AppInstance.FindOrRegisterForKey`; `KC9K_GUI_MULTI=1` for tests).
- **Contract** (`Core/Cli.cs`): `ProcessCli` runs `kc9k.exe` (`KC9K_CLI`,
  beside the GUI, one folder up, `PATH`) without a console, so a text
  lock's challenge opens its own window (R-LOCK-5). `CliSession` passes
  passwords on stdin as GTK does (`--access-lines N`, `--settings-line`,
  then the command's secret; block passwords via `gui-state --reveal`).
  `Controller` (`Shell/`) = GTK `Window`: commands in the background with
  a progress bar (R-GUI-3); state re-read after each command, every 5 s
  (stats 30 s); refusals in a dialog in the CLI's words; "needs the
  settings/access password" asks and reruns; `access_unused` passwords
  dropped; idle minutes forget access passwords.
- **Drafts** (§4.3, `Core/Drafts.cs`, GTK's `DRAFT_EDITS`/`DRAFT_READS`):
  an edit to the opened block goes to `NAME (draft)`, the first making it.
- **State** (`Core/State.cs`): typed `gui-state`/`stats --json`
  (unknown fields ignored; `TimeWindow` = window type). GTK words and
  typed values ported one to one (`Words.cs`, `Fields.cs`, `Entries.cs`,
  `Phrases.cs`), tested against the fixtures.
- **Design** (`Theme.cs`, R-GUI-3): styles as a XAML string loaded at
  run time, so `{ThemeResource}` follows light/dark/high contrast live
  (code-drawn colours: `Theme.Brush(key, element)`); Mica, or the
  darkness slider's solid background in dark. Every control has an
  accessible name and `AutomationId`; charts are plain elements.
  ContentDialogs show one at a time, so a dialog asking a second question
  hides, asks, and reopens with what was typed. `ConfirmLock` = R-LOCK-12.
- **Pages** (`Pages/`, one per GTK sidebar entry, `Tables.Pages`; Settings
  last; covered while its access area is closed, §3.11): `OverviewPage`
  (no "Get started" card, owner), `BlocksPage` (drag a block card onto a
  mode card, or context menu / Shift+F10) with `BlockView` (tab buttons
  Websites, Apps, Breaks & allowance, Bypass prevention; Start, then Add
  locks…), `AutomationView`, `ModeView` (monk mode, Frozen Chicken via
  `Device.cs`), `SchedulesPlaces.cs`, `HotkeysPage` (worded win+; CLI
  also takes super+), `PresetsPage`, `StatsPage` (own `stats --json` read
  with frame and filters; JSON export, GTK has none), `CircadianPage`,
  `SettingsPage` (Windows wording; `Startup`: Open at sign-in shortcut).
- **Controls** (`Controls/`): `EditableList` (§4.3 keys: Enter adds,
  Delete/Backspace removes, Ctrl+A, Ctrl+F, Enter/double-click edits;
  height per list kind in gui.json), `ListCards.cs`, `Locks.cs`
  (`LockPicker`, `TimerLockField`), `LockDialogs.ChooseOrder` (R-LOCK-6),
  `NepsisDialog` (`--step-json`), `AppPicker` (Start menu shortcuts and
  running programs; adds the program file's name), `WeekCalendar`
  (Canvas; keyboard way: the window list in `AutomationEditor`),
  `ClipEditor` (keyboard way: Points…), `Charts.cs`.
- **Shell**: `Files.cs`: file dialogs, list import (`list read` routes
  phrases to a phrase set)/export; `ImportProject` (Settings › Project):
  `project import --dry-run` shown, `ConfirmLock`, then `project import
  FILE [--yes]` (with locks or custom actions); `KC9K_GUI_IMPORT_FILE`
  for tests. `Tray.cs` (`Shell_NotifyIcon`, comctl32 subclass); menu
  `Core/TrayMenu.cs`: only strengthening actions (start a block/mode, monk
  mode, Frozen Chicken, new block), Open, Quit (blocks stay on).
- **Preferences** (`Prefs.cs`): `%APPDATA%\kc9k\gui.json`
  (`KC9K_GUI_PREFS`): theme, darkness, clock, weekday, tray, list heights,
  window geometry (only on a screen still there), open on. `Options` must
  be set before `Current` reads the file (static field order).

## Tests

- `FakeKc9k` (built as `kc9k.exe`) replays `fixtures/gui-state.json` and
  `stats.json` from `KC9K_FAKE_DIR`, answers `responses.json` (by leading
  arguments), logs each command and its stdin to `commands.log`. Fixtures
  from the real Linux CLI: `fixtures/make-fixtures.sh path/to/kc9k`, as a
  normal user.
- `Kc9k.UiTests` (FlaUI, UIA3) run the published GUI against the fake
  (`KC9K_GUI_EXE`, `KC9K_FAKE_EXE`), every page in light and dark,
  pressing via UI Automation (`GuiApp.Invoke`, retries a stale element 3×)
  and checking commands run; screenshots to `KC9K_SCREENSHOTS`; gui.log
  printed after. `GuiApp.EditState`/`Respond` change fixture state and
  answers.
- `ParityTests.cs` (W32): counterparts of the `ci/gui_*` GTK tests,
  including `gui_click_everything` (every button once, questions
  cancelled, never "Something went wrong"). `WindowsTests.cs`: W30–W43.
  `ProjectImportTests.cs`. `AccessibilityTests.cs` (W33): every control
  named and focusable; screenshots in high contrast and 200 % text (the
  person's settings restored exactly). `GeometryTests.cs` (W53/W54):
  widths 400–1920 DIP × text 100/150/200 % on every page and block tab:
  nothing outside the client area, no horizontal scroll, no ☰, title at
  one place, Blocks switches in one right column. `StabilityTests.cs`
  (W52). Tests are per-monitor DPI aware (`DpiAware.cs`): physical pixels,
  as gui.json.

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- Errors (W52, `App.cs`): all unhandled exceptions go to gui.log
  (`UnobservedTaskException` kept alive); a fatal one writes
  `gui-crash.txt`, shown once at the next start ("closed unexpectedly")
  and deleted. `Controller.ShowError` drops duplicate dialogs. Only the
  page shown is updated. Test hook `KC9K_GUI_TEST_THROW` =
  `window`/`task`/`crash` (XAML `DispatcherTimer`: an unheld
  `DispatcherQueueTimer` is collected).
- Layout (W54, as GTK): pages fill the width beside the sidebar (margins
  32 DIP, 16 below 600). `Ui.Side`/`Ui.SettingRow` put a row's controls
  in the right column, moved under the text when text would get < 200 DIP.
  Blocks row: name, pill, Continuous/Automated, breaks left; switch over
  lock link right (`RightColumn`). Titles, button rows, filters are
  `WrapPanel`s, never horizontal `StackPanel`s. Opened block's head ≤ half
  the height; tabs are wrapping toggle buttons (`SelectorBar` left tabs
  unrealized). `cardCache` keeps unchanged cards, so a state read never
  replaces a card under a click.
- Sidebar (W53, `MainWindow.PinSidebar`): pinned at 220 DIP, no ☰; below
  640 DIP (220 + `NarrowestPage` 420) icons only (`LeftCompact`, titles as
  tooltips), never over the page.
- Supported browsers (W30): `found` but not `installed` = unsupported, no
  Install button; Install shows `browsers install`'s words as "Getting
  NAME", not a refusal (`Controller.Run(refusalTitle:)`).
- Overview (W31, §4.1): each block on says its break and bypass prevention
  ladder position (`nepsis_level`, `nepsis_count`); Device line the global
  ladder's. GTK shows neither.
- Shortcuts (W34, `MainWindow.Shortcuts`, listed in Settings › Keyboard):
  Ctrl+1…9 pages, Ctrl+N New block…, F5 re-read, Alt+Left / Back key /
  mouse back from a block, mode or automation to the cards.
- Device protection (W39, `Words.ProtectionLevels`): principle 3's levels
  on Windows, which is on, firmware lock state (`firmware-lock --json`).
- Jump list (W36, `Shell/JumpList.cs`, `Core/StartArgs.cs`):
  AppUserModelID `KillerChicken9000.Gui` (also on the MSI's shortcut);
  tasks start `kc9k-gui.exe --page KEY` or `--do new-block|monk|freeze`,
  handed to a running window. Arguments only open pages and GUI dialogs,
  never a kc9k command.
- A refused switch flips back at once; a locked block's switch is greyed.
- Guided setup (Core W15): unknown step IDs as plain steps;
  `other-admins`, `builtin-admin` get Do it… (`ms-settings:otherusers`);
  `safe-mode` is information.
