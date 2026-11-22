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
- **Pages** (`Pages/`): the GTK GUI's sidebar (`Tables.Pages`), Settings
  at the bottom as Windows apps have it; each page under a cover while its
  access area is closed (§3.11). `OverviewPage` (§4.1); `BlocksPage`
  (§4.2: mode and block cards, drag a block card onto a mode card, or its
  context menu, Shift+F10), with three views: `BlockView` (§4.3: the head
  stays put, tab buttons (toggle buttons that wrap) Websites, Apps, Breaks & allowance, Bypass
  prevention scroll; Start applies at once, once on it is Add locks…;
  Unlock…, Extend timer…, ⋯ rename, duplicate, block preset, users,
  delete), `AutomationView` (§4.5: `AutomationEditor` kind block, schedule
  lock, autostarts, automation presets), `ModeView` (modes, monk mode,
  and Frozen Chicken through `Device`: freeze now, scheduled freezes with
  locks, allowed apps); `SchedulesPage`, `PlacesPage` (`SchedulesPlaces.cs`),
  `HotkeysPage` (§4.13), `PresetsPage` (§4.4, recording), `StatsPage`
  (§4.9, its own `stats --json` read with the page's frame and filters,
  JSON export), `CircadianPage` (§4.10, `circadian show --json`),
  `SettingsPage` (§4.10 in SPEC's order, worded for Windows; `Startup`
  makes the Startup-folder shortcut for Open at sign-in).
- **Controls** (`Controls/`): `EditableList` (§4.3 keys: Enter adds,
  Delete/Backspace removes and the selection moves on, Ctrl+A, Ctrl+F,
  double-click or Enter edits; hidden entries behind a cover; a grip makes
  it taller, kept per kind of list in gui.json), `ListCard` and
  `PhraseSetCard` with `PhraseEditor` (`ListCards.cs`), `LockPicker` and
  `TimerLockField` (`Locks.cs`, on `Core.LockSpec`), `LockDialogs.ChooseOrder`
  (R-LOCK-6), `NepsisDialog` (the bypass prevention ladder as
  `--step-json`), `TriggerEditor` and `AppPicker` (Start menu shortcuts
  resolved through WScript.Shell, and running programs: an app is added by
  its program file's name), `CustomEditor`, `WeekCalendar` (drawn on a
  Canvas; its windows are also a list in `AutomationEditor`, the keyboard's
  way), `AutomationEditor` with `PlaceRules`, `ClipEditor` (Points… is the
  keyboard's way), `WallpaperEditor` (timeline plus rows), charts
  (`TopChart`, `DailyChart` in `StatsPage.cs`: elements, not pixels).
  `Shell/Files.cs`: Windows' file dialogs, list imports (`list read`
  routes phrases to a phrase set) and exports; `ImportProject` (W55,
  Settings → Import…, *(By claude)*): `project import --dry-run` shown,
  `ConfirmLock` for its locks, then `project import FILE [--yes]` (`--yes`
  when it has locks or custom actions); `KC9K_GUI_IMPORT_FILE` (tests)
  gives the file instead of the dialog. `Shell/Tray.cs`: the
  notification-area icon through `Shell_NotifyIcon` on the window's own
  handle (a comctl32 subclass gets its messages); Quit leaves blocks on.
  Its menu is `Core/TrayMenu.cs` (W35): what is on, Start a block ▸,
  Start a mode ▸, Monk mode…, Frozen Chicken…, New block…, Open, Quit;
  only actions that strengthen, each bringing the window forward first.
- **Dialogs over dialogs**: WinUI shows one ContentDialog at a time, so a
  dialog that asks a second question (Add action… in a hotkey, Add… in a
  clip's points) hides itself, asks, and opens again with what was typed.
- **Preferences** (`Prefs.cs`): `%APPDATA%\kc9k\gui.json` (`KC9K_GUI_PREFS`
  for tests): theme, darkness, clock, first weekday, tray, list heights,
  window size, place and maximized (restored only on a screen still there,
  else centred on the primary one), open on (Overview or the last page,
  W37). Static field order matters: `Options` is set before
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
  `GuiApp.EditState`/`Respond` change a test's fixture state and canned
  answers; `ready:` names the element that says the first state is shown,
  `arguments:` and `multi: false` start it as a jump list entry would.
- `ParityTests.cs` (W32): counterparts of `ci/gui_cards`, `gui_extend`,
  `gui_duration`, `gui_lists`, `gui_presets`, `gui_modes`,
  `gui_freeze_lock`, `gui_automation`, `gui_click_everything` (every
  button on every sidebar page once,
  each question cancelled; the window never ends or shows "Something went
  wrong"). `WindowsTests.cs`: W30–W43. `ProjectImportTests.cs` (W55,
  *(By claude)*): Import… with the dry run, its lock confirmed or declined. `AccessibilityTests.cs` (W33):
  every button, box, switch, list item and menu item on every page and
  block tab has a name and takes keyboard focus; screenshots in high
  contrast (`SPI_SETHIGHCONTRAST`, turned off after) and at 200 % text
  (`HKCU\Software\Microsoft\Accessibility\TextScaleFactor`, put back).
- `GeometryTests.cs` (W53/W54): windows of 400, 512, 800, 1024, 1366 and
  1920 DIP (resized with `SetWindowPos`; a width the screen can't hold is
  tested at the widest it got) × text at 100, 150 and 200 %, on every
  sidebar page and block tab: every element shown lies inside the client
  area left to right, no horizontal scroll bar, no ☰, the page title right
  of the sidebar and at one place on every page; on Blocks each row's
  switch and locks end in one right column. `StabilityTests.cs`: W52.
  The test process is per-monitor DPI aware (`DpiAware.cs`), so
  UI Automation, screenshots and moves are in physical pixels as gui.json
  is. High contrast reads the person's flags and scheme first and puts
  them back exactly; the name check skips a scroll bar's own parts.
  `GuiApp.Invoke(id)` finds and presses again (up to 3 times) when the
  element went stale while its page was being shown. The geometry test
  scrolls each block row into view before measuring it and presses a
  card once, after the page is idle (300 ms without layout changes).
  *(By claude)*

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- Errors and crashes (W52, `App.cs`): WinUI's `UnhandledException`,
  `TaskScheduler.UnobservedTaskException` (kept alive) and
  `AppDomain.UnhandledException` all go to gui.log; a terminating one
  writes `gui-crash.txt` beside gui.json, and the next start shows "closed
  unexpectedly at …, details in gui.log" once and removes it.
  `Controller.ShowError` drops a dialog whose title and text are already
  shown or waiting. `MainWindow.Apply` and the statistics update only the
  page shown; `Show` updates a page when it is shown. Test-only hook:
  `KC9K_GUI_TEST_THROW` = `window`/`task`/`crash` (`TestHook`, on a XAML
  `DispatcherTimer`: a `DispatcherQueueTimer` nobody holds is collected
  and never fires; `StabilityTests.cs`). *(By claude)*
- Page layout (W54, as the GTK builders): a page fills the width beside
  the sidebar (`Ui.Page`, `BlockView`: no maximum width; margins 32 DIP,
  16 below 600). `SidePanel` (`Ui.Side`, `Ui.SettingRow`) puts a row's
  switch, Lock… or buttons in the right column, at the card's right edge,
  and moves them under the text, still at the right, when the text would
  get less than 200 DIP. A block on the Blocks page is a row: name and
  pill, Continuous/Automated and its breaks on the left; on/off with its
  switch above its lock link on the right (`RightColumn`). Titles, button
  rows and filters are `WrapPanel`s, never horizontal `StackPanel`s that
  run past the right edge. An opened block's head (back to the Start/Stop
  row) takes at most half the height and scrolls by itself beyond that;
  its tabs sit under it, always shown, as toggle buttons that wrap onto
  more lines when narrow (a `SelectorBar` scrolled sideways and left
  tabs unrealized). The crash note waits for the window's content to
  load (`MainWindow.WhenShown`). The Blocks page keeps each card while
  what it shows is the same (`cardCache`): a state read never replaces a
  card under a click. *(By claude)* The Overview has no "Get started" card (owner
  2026-11-22; W38's first view is gone). Settings › Project's Import
  project… calls `Files.ImportProject`. *(By claude)*
- Sidebar (W53, `MainWindow.PinSidebar`): pinned open at 220 DIP like
  the GTK sidebar, no ☰; the page fills the rest. Below 640 DIP of window
  (220 + `NarrowestPage` 420) it shows its icons alone (`LeftCompact`, its
  titles as tooltips), still pinned: it never opens over or beside the
  page. *(By claude)*
- Hotkeys are worded with win+ (the CLI also takes super+).
- Colours drawn in code (calendar, clips, charts) are looked up for the
  theme the window shows (`Theme.Brush(key, element)`); the app starts in
  the chosen theme, and pills use styles, so a theme change applies live.
- Statistics can be saved as JSON (§4.9 JSON export) with the page's time
  frame and filters; the GTK GUI has no such button.
- Supported browsers (W30): a browser `gui-state` names as `found` but not
  `installed` (Chrome and Edge on Windows until the extension's store
  listing exists) is said to count as an unsupported browser, with no
  Install button; Install shows `browsers install`'s words (on Windows,
  where to get the browser) under "Getting NAME", not as a refusal
  (`Controller.Run(refusalTitle:)`).
- Overview (W31, §4.1): each block on now also says its break ("on a
  break for …", "on a Pomodoro break") and where its bypass prevention
  ladder stands ("step 2 of 3: 1 of 2 in 30 min", from `nepsis_level`
  and `nepsis_count`); the Device line has the global ladder's. The GTK
  GUI shows neither.
- Shortcuts (W34, `MainWindow.Shortcuts`, listed under Settings ›
  Keyboard): Ctrl+1…9 the sidebar's pages, Ctrl+N New block…, F5 reads
  the state and statistics again, Alt+Left, the Back key and the mouse's
  back button go from a block, mode or automation to the cards.
- Device protection (W39, `Words.ProtectionLevels`): Settings names the
  three levels of principle 3 as they hold on Windows, which is on now,
  and the firmware lock's state (`firmware-lock --json`, read quietly when
  the page is first shown and after its dialog).
- Jump list and start arguments (W36, `Shell/JumpList.cs`,
  `Core/StartArgs.cs`): the process's AppUserModelID is
  `KillerChicken9000.Gui` (also on the MSI's Start menu shortcut); the
  jump list's tasks (Overview, Blocks, New block…, Monk mode…, Frozen
  Chicken…, Statistics, Settings) start `kc9k-gui.exe --page KEY` or
  `--do new-block|monk|freeze`; a second start hands them to the running
  window. Arguments only open pages and the GUI's own dialogs, never a
  kc9k command.
- A card's switch whose command is refused is put back at once (the
  refusal leaves the state as it was, so the cards are drawn again;
  found by W32's `gui_cards` counterpart). A locked block's switch is
  greyed out.
- Guided setup on Windows (Core W15): unknown step IDs show as plain
  steps; `other-admins` and `builtin-admin` get Do it…, which opens
  Windows' Settings › Accounts › Other users (`ms-settings:otherusers`);
  `safe-mode` is information.

